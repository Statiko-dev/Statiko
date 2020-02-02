/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/go-autorest/autorest/azure"
	"golang.org/x/crypto/pkcs12"

	"smplatform/appconfig"
	"smplatform/utils"
)

// Singleton for Client
var instance *Client

// GetInstance returns the (initialized) singleton
func GetInstance() *Client {
	if instance == nil {
		// Initialize the singleton
		instance = &Client{
			VaultName: appconfig.Config.GetString("azure.keyVault.name"),
		}
		if err := instance.Init(); err != nil {
			panic(err)
		}
	}
	return instance
}

// Client can extract public keys and certificates (e.g. TLS certificates) stored in Azure Key Vault
type Client struct {
	KeyVault  keyvault.BaseClient
	VaultName string

	ctx           context.Context
	logger        *log.Logger
	authenticated bool
}

// Init the object
func (akv *Client) Init() error {
	akv.logger = log.New(os.Stdout, "azure-key-vault: ", log.Ldate|log.Ltime|log.LUTC)

	// Context
	akv.ctx = context.Background()

	// Init the Key Vault client
	if err := akv.initKeyVaultClient(); err != nil {
		return err
	}

	return nil
}

// BaseURL returns the base URL for all operations with the Key Vault
func (akv *Client) BaseURL() string {
	return fmt.Sprintf("https://%s.%s", akv.VaultName, azure.PublicCloud.KeyVaultDNSSuffix)
}

// Initializes and authenticates the client to interact with Azure Key Vault
func (akv *Client) initKeyVaultClient() error {
	// Create a new client
	akv.KeyVault = keyvault.New()

	// If we have the auth data in the appconfig, expose them as env vars so the Azure SDK picks them up
	tenantID := appconfig.Config.GetString("azure.app.tenantId")
	if len(tenantID) > 0 {
		os.Setenv("AZURE_TENANT_ID", tenantID)
	}
	clientID := appconfig.Config.GetString("azure.app.clientId")
	if len(clientID) > 0 {
		os.Setenv("AZURE_CLIENT_ID", clientID)
	}
	clientSecret := appconfig.Config.GetString("azure.app.clientSecret")
	if len(clientSecret) > 0 {
		os.Setenv("AZURE_CLIENT_SECRET", clientSecret)
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return err
	}
	akv.KeyVault.Authorizer = authorizer
	akv.authenticated = true

	return nil
}

// GetPublicKey returns the public portion of a key stored inside Azure Key Vault
func (akv *Client) GetPublicKey(keyName string, keyVersion string) (string, *rsa.PublicKey, error) {
	// If we don't have a version, get the latest
	var err error
	if keyVersion == "" || keyVersion == "latest" {
		keyVersion, err = akv.getKeyLastVersion(keyName)
		if err != nil {
			return keyVersion, nil, err
		}
	}

	// Get the public key
	res, err := akv.KeyVault.GetKey(akv.ctx, akv.BaseURL(), keyName, keyVersion)
	if err != nil {
		return keyVersion, nil, err
	}

	// Check if the key is there
	if res.Key == nil {
		return keyVersion, nil, errors.New("Empty key")
	}
	key := res.Key
	if key.Kty != keyvault.RSA && key.Kty != keyvault.RSAHSM {
		return keyVersion, nil, errors.New("Returned key is not a RSA key")
	}

	// Check attributes
	now := time.Now()
	if res.Attributes == nil {
		return keyVersion, nil, errors.New("Invalid key attributes")
	}
	if res.Attributes.Enabled == nil || !*res.Attributes.Enabled {
		return keyVersion, nil, errors.New("Key is not enabled")
	}
	if res.Attributes.Expires != nil {
		expires := time.Time(*res.Attributes.Expires)
		if expires.Before(now) {
			return keyVersion, nil, errors.New("Key has expired")
		}
	}
	if res.Attributes.NotBefore != nil {
		nbf := time.Time(*res.Attributes.NotBefore)
		if now.Before(nbf) {
			return keyVersion, nil, errors.New("Key's not-before date is in the future")
		}
	}

	// Construct the RSA key from the JSONWebKey object
	if key.N == nil || *key.N == "" || key.E == nil || *key.E == "" {
		return keyVersion, nil, errors.New("Invalid key: missing N or E parameters")
	}

	pubKey, err := utils.ParseRSAPublicKey(*key.N, *key.E)
	if err != nil {
		return keyVersion, nil, err
	}

	return keyVersion, pubKey, nil
}

// Returns the last version of a key
func (akv *Client) getKeyLastVersion(keyName string) (string, error) {
	// List key versions
	list, err := akv.KeyVault.GetKeyVersionsComplete(akv.ctx, akv.BaseURL(), keyName, nil)
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

// Returns the last version of a certificate
func (akv *Client) getCertificateLastVersion(certificateName string) (string, error) {
	// List certificate versions
	list, err := akv.KeyVault.GetCertificateVersionsComplete(akv.ctx, akv.BaseURL(), certificateName, nil)
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
	pfx, err := akv.KeyVault.GetSecret(akv.ctx, akv.BaseURL(), certificateName, certificateVersion)
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
		certificateVersion, err = akv.getCertificateLastVersion(certificateName)
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
