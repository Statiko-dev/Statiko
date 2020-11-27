/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, version 3 of the License.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package azurekeyvault

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.1/keyvault"
	"github.com/Azure/go-autorest/autorest/azure"
	"golang.org/x/crypto/pkcs12"

	"github.com/statiko-dev/statiko/buildinfo"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

// Timeout (in seconds) for operations
const requestTimeout = 15 * time.Second

// Client can extract public keys and certificates (e.g. TLS certificates) stored in Azure Key Vault
type Client struct {
	KeyVault  keyvault.BaseClient
	VaultName string

	logger        *log.Logger
	authenticated bool
}

// Init the object
func (akv *Client) Init(sp *pb.ClusterOptions_AzureServicePrincipal) error {
	akv.logger = log.New(buildinfo.LogDestination, "azure-key-vault: ", log.Ldate|log.Ltime|log.LUTC)

	// Create a new client
	akv.KeyVault = keyvault.New()

	// Authorize with Key Vault
	authorizer, err := utils.GetAzureAuthorizer("keyvault", sp)
	if err != nil {
		return err
	}
	akv.KeyVault.Authorizer = authorizer
	akv.authenticated = true

	return nil
}

// BaseURL returns the base URL for all operations with the Key Vault
func (akv *Client) BaseURL() string {
	return fmt.Sprintf("https://%s.%s", akv.VaultName, azure.PublicCloud.KeyVaultDNSSuffix)
}

// Returns the last version of a key
func (akv *Client) getKeyLastVersion(keyName string) (string, error) {
	// List key versions
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	list, err := akv.KeyVault.GetKeyVersionsComplete(ctx, akv.BaseURL(), keyName, nil)
	cancel()
	if err != nil {
		return "", err
	}

	// Iterate through the list and get the last version
	var lastItemDate time.Time
	lastItemVersion := ""
	for list.NotDone() {
		// Get element
		item := list.Value()
		// Filter only enabled items
		if item.Attributes.Enabled != nil && *item.Attributes.Enabled && item.Kid != nil && item.Attributes.Updated != nil {
			// Get the most recent element
			updatedTime := time.Time(*item.Attributes.Updated)
			if lastItemDate.IsZero() || updatedTime.After(lastItemDate) {
				lastItemDate = updatedTime

				// Get the ID
				parts := strings.Split(*item.Kid, "/")
				lastItemVersion = parts[len(parts)-1]
			}
		}
		// Iterate to next
		list.Next()
	}

	return lastItemVersion, nil
}

// GetCertificateLastVersion returns the last version of a certificate
func (akv *Client) GetCertificateLastVersion(certificateName string) (string, error) {
	// List certificate versions
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	list, err := akv.KeyVault.GetCertificateVersionsComplete(ctx, akv.BaseURL(), certificateName, nil)
	cancel()
	if err != nil {
		return "", err
	}

	// Iterate through the list and get the last version
	var lastItemDate time.Time
	lastItemVersion := ""
	for list.NotDone() {
		// Get element
		item := list.Value()
		// Filter only enabled items
		if item.Attributes.Enabled != nil && *item.Attributes.Enabled && item.ID != nil && item.Attributes.Updated != nil {
			// Get the most recent element
			updatedTime := time.Time(*item.Attributes.Updated)
			if lastItemDate.IsZero() || updatedTime.After(lastItemDate) {
				lastItemDate = updatedTime

				// Get the ID
				parts := strings.Split(*item.ID, "/")
				lastItemVersion = parts[len(parts)-1]
			}
		}
		// Iterate to next
		list.Next()
	}

	return lastItemVersion, nil
}

// Get the PFX of a certificate inside Azure Key Vault
func (akv *Client) requestCertificatePFX(certificateName string, certificateVersion string) (interface{}, *x509.Certificate, error) {
	// The full certificate, including the key, is stored as a secret in Azure Key Vault, encoded as PFX
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	pfx, err := akv.KeyVault.GetSecret(ctx, akv.BaseURL(), certificateName, certificateVersion)
	cancel()
	if err != nil {
		return nil, nil, err
	}

	// Check attributes
	now := time.Now()
	if pfx.Attributes == nil {
		return nil, nil, errors.New("Invalid certificate attributes")
	}
	if pfx.Attributes.Enabled == nil || !*pfx.Attributes.Enabled {
		return nil, nil, errors.New("Certificate is not enabled")
	}
	if pfx.Attributes.Expires != nil {
		expires := time.Time(*pfx.Attributes.Expires)
		if expires.Before(now) {
			return nil, nil, errors.New("Certificate has expired")
		}
	}
	if pfx.Attributes.NotBefore != nil {
		nbf := time.Time(*pfx.Attributes.NotBefore)
		if now.Before(nbf) {
			return nil, nil, errors.New("Certificate's not-before date is in the future")
		}
	}

	// Response is a Base64-Encoded PFX, with no passphrase
	pfxBytes, err := base64.StdEncoding.DecodeString(*pfx.Value)
	if err != nil {
		return nil, nil, err
	}
	return pkcs12.Decode(pfxBytes, "")
}

// GetCertificate returns the certificate and key from Azure Key Vault, encoded as PEM
func (akv *Client) GetCertificate(certificateName string, certificateVersion string) (string, []byte, []byte, *x509.Certificate, error) {
	// If we don't have a version specified, request the last one
	if len(certificateVersion) == 0 {
		akv.logger.Printf("Getting last version for %s\n", certificateName)
		var err error
		certificateVersion, err = akv.GetCertificateLastVersion(certificateName)
		if err != nil {
			return certificateVersion, nil, nil, nil, err
		}
		if certificateVersion == "" {
			return certificateVersion, nil, nil, nil, errors.New("Certificate not found")
		}
	}

	// Request the certificate and key
	akv.logger.Printf("Getting PFX for %s, version %s\n", certificateName, certificateVersion)
	pfxKey, pfxCert, err := akv.requestCertificatePFX(certificateName, certificateVersion)
	if err != nil {
		return certificateVersion, nil, nil, nil, err
	}

	// Marshal the x509 key
	keyX509, err := x509.MarshalPKCS8PrivateKey(pfxKey)
	if err != nil {
		return certificateVersion, nil, nil, nil, err
	}

	// Convert to PEM
	akv.logger.Printf("Converting to PEM for %s, version %s\n", certificateName, certificateVersion)
	keyBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyX509,
	}
	var keyPEM bytes.Buffer
	pem.Encode(&keyPEM, keyBlock)

	certBlock := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: pfxCert.Raw,
	}
	var certPEM bytes.Buffer
	pem.Encode(&certPEM, certBlock)

	return certificateVersion, certPEM.Bytes(), keyPEM.Bytes(), pfxCert, nil
}

// CertificateExists returns true whether if the certificate exists on the Key Vault
func (akv *Client) CertificateExists(certificateName string) (exists bool, err error) {
	// Get the last version of the certificate to test if it exists
	version, err := akv.GetCertificateLastVersion(certificateName)
	if version != "" {
		exists = true
	}
	return
}
