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
func (akv *Certificate) Init() (err error) {
	akv.logger = log.New(os.Stdout, "azure-key-vault: ", log.Ldate|log.Ltime|log.LUTC)

	return nil
}

// GetKeyVaultClient initializes and authenticates the client to interact with Azure Key Vault
func (akv *Certificate) GetKeyVaultClient() (err error) {
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

func (akv *Certificate) requestCertificateVersion(certificateName string) (version string, err error) {
	// List certificate versions
	list, err := akv.Client.GetCertificateVersionsComplete(akv.Ctx, akv.vaultBaseURL, certificateName, nil)
	if err != nil {
		return "", err
	}

	// Iterate through the list and get the last version
	var lastItemDate time.Time
	var lastItemVersion string
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

func (akv *Certificate) requestCertificatePFX(certificateName string, certificateVersion string) (key interface{}, cert *x509.Certificate, err error) {
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
func (akv *Certificate) GetCertificate(certificateName string) (certificate []byte, key []byte, err error) {
	// Error if there's no authenticated client yet
	if !akv.authenticated {
		return nil, nil, errors.New("Need to invoke GetKeyVaultClient() first")
	}

	// List certificate versions
	akv.logger.Printf("Getting certificate version for %s\n", certificateName)
	certificateVersion, err := akv.requestCertificateVersion(certificateName)
	if err != nil {
		return nil, nil, err
	}

	// Request the certificate and key
	akv.logger.Printf("Getting PFX for %s\n", certificateName)
	pfxKey, pfxCert, err := akv.requestCertificatePFX(certificateName, certificateVersion)
	keyX509, err := x509.MarshalPKCS8PrivateKey(pfxKey)
	if err != nil {
		return nil, nil, err
	}

	// Convert to PEM
	akv.logger.Printf("Converting to PEM for %s\n", certificateName)
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

	return certPEM.Bytes(), keyPEM.Bytes(), nil
}
