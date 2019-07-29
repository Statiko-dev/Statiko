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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/2016-10-01/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/auth"
	"github.com/Azure/go-autorest/autorest/azure"
	"golang.org/x/crypto/pkcs12"

	"smplatform/appconfig"
)

// Certificate can extract certificates (e.g. TLS certificates) stored in Azure Key Vault
type Certificate struct {
	Ctx       context.Context
	Client    keyvault.BaseClient
	VaultName string

	logger        *log.Logger
	authenticated bool
	vaultBaseURL  string
}

// Init the object
func (akv *Certificate) Init() error {
	akv.logger = log.New(os.Stdout, "azure-key-vault: ", log.Ldate|log.Ltime|log.LUTC)

	return nil
}

// GetKeyVaultClient initializes and authenticates the client to interact with Azure Key Vault
func (akv *Certificate) GetKeyVaultClient() error {
	// Create a new client
	akv.Client = keyvault.New()

	// If we have the auth data in the appconfig, expose them as env vars so the Azure SDK picks them up
	tenantID := appconfig.Config.GetString("azureKeyVault.servicePrincipal.tenantId")
	if len(tenantID) > 0 {
		os.Setenv("AZURE_TENANT_ID", tenantID)
	}
	clientID := appconfig.Config.GetString("azureKeyVault.servicePrincipal.clientId")
	if len(clientID) > 0 {
		os.Setenv("AZURE_CLIENT_ID", clientID)
	}
	clientSecret := appconfig.Config.GetString("azureKeyVault.servicePrincipal.clientSecret")
	if len(clientSecret) > 0 {
		os.Setenv("AZURE_CLIENT_SECRET", clientSecret)
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return err
	}
	akv.Client.Authorizer = authorizer
	akv.authenticated = true

	// Base URL for the vault
	akv.vaultBaseURL = fmt.Sprintf("https://%s.%s", akv.VaultName, azure.PublicCloud.KeyVaultDNSSuffix)

	return nil
}

func (akv *Certificate) getCertificateLastVersion(certificateName string) (string, error) {
	// List certificate versions
	list, err := akv.Client.GetCertificateVersionsComplete(akv.Ctx, akv.vaultBaseURL, certificateName, nil)
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
		if *item.Attributes.Enabled {
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

func (akv *Certificate) requestCertificatePFX(certificateName string, certificateVersion string) (interface{}, *x509.Certificate, error) {
	// The full certificate, including the key, is stored as a secret in Azure Key Vault, encoded as PFX
	pfx, err := akv.Client.GetSecret(akv.Ctx, akv.vaultBaseURL, certificateName, certificateVersion)
	if err != nil {
		return nil, nil, err
	}

	// Response is a Base64-Encoded PFX, with no passphrase
	pfxBytes, err := base64.StdEncoding.DecodeString(*pfx.Value)
	if err != nil {
		return nil, nil, err
	}
	return pkcs12.Decode(pfxBytes, "")
}

// GetCertificate returns the certificate and key from Azure Key Vault, encoded as PEM
func (akv *Certificate) GetCertificate(certificateName string, certificateVersion string) (string, []byte, []byte, error) {
	// Error if there's no authenticated client yet
	if !akv.authenticated {
		return certificateVersion, nil, nil, errors.New("Need to invoke GetKeyVaultClient() first")
	}

	// If we don't have a version specified, request the last one
	if len(certificateVersion) == 0 {
		akv.logger.Printf("Getting last version for %s\n", certificateName)
		var err error
		certificateVersion, err = akv.getCertificateLastVersion(certificateName)
		if err != nil {
			return certificateVersion, nil, nil, err
		}
		if certificateVersion == "" {
			return certificateVersion, nil, nil, errors.New("Certificate not found")
		}
	}

	// Request the certificate and key
	akv.logger.Printf("Getting PFX for %s, version %s\n", certificateName, certificateVersion)
	pfxKey, pfxCert, err := akv.requestCertificatePFX(certificateName, certificateVersion)
	if err != nil {
		return certificateVersion, nil, nil, err
	}

	// Marshal the x509 key
	keyX509, err := x509.MarshalPKCS8PrivateKey(pfxKey)
	if err != nil {
		return certificateVersion, nil, nil, err
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

	return certificateVersion, certPEM.Bytes(), keyPEM.Bytes(), nil
}
