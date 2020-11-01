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

package utils

import (
	"errors"
	"fmt"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/Azure/go-autorest/autorest/azure"
)

// Interface for an object that provides the methods for getting an Azure Service Principal
// This should be used with pb.ClusterOptions_AzureServicePrincipal
type azureSPProvider interface {
	GetTenantId() string
	GetClientId() string
	GetClientSecret() string
}

// Initializes the authentication objects for Azure
func initAzure(sp azureSPProvider) (*azure.Environment, *adal.OAuthConfig, error) {
	tid := sp.GetTenantId()
	if tid == "" {
		return nil, nil, errors.New("sp.TenantId must be defined")
	}

	// Get Azure environment properties
	env, err := azure.EnvironmentFromName("AZUREPUBLICCLOUD")
	azureEnv := &env
	if err != nil {
		return nil, nil, err
	}
	azureOAuthConfig, err := adal.NewOAuthConfig(azureEnv.ActiveDirectoryEndpoint, tid)
	if err != nil {
		return nil, nil, err
	}
	if azureOAuthConfig == nil {
		return nil, nil, fmt.Errorf("unable to configure authentication for Azure tenant %s", tid)
	}

	return azureEnv, azureOAuthConfig, nil
}

// GetAzureEndpoint returns the endpoint for the Azure service
// Service can be one of:
// - "azure" for Azure Resource Manager
// - "keyvault" for Azure Key Vault
// - "storage" for Azure Storage
func GetAzureEndpoint(service string, sp azureSPProvider) (endpoint string, err error) {
	azureEnv, _, err := initAzure(sp)
	if err != nil {
		return
	}

	switch service {
	case "azure":
		endpoint = azureEnv.ResourceManagerEndpoint
		break
	case "keyvault":
		endpoint = azureEnv.ResourceIdentifiers.KeyVault
		break
	case "storage":
		endpoint = azureEnv.ResourceIdentifiers.Storage
		break
	default:
		return "", errors.New("invalid service name: " + service)
	}

	return
}

// GetAzureStorageEndpointSuffix returns the endpoint suffix for Azure Storage in this environment
func GetAzureStorageEndpointSuffix(sp azureSPProvider) (string, error) {
	azureEnv, _, err := initAzure(sp)
	if err != nil {
		return "", err
	}

	return azureEnv.StorageEndpointSuffix, nil
}

// GetAzureOAuthConfig returns the adal.OAuthConfig object that can be used to authenticate against Azure resources
func GetAzureOAuthConfig(sp azureSPProvider) (*adal.OAuthConfig, error) {
	_, azureOAuthConfig, err := initAzure(sp)
	if err != nil {
		return nil, err
	}

	return azureOAuthConfig, nil
}

// GetAzureAuthorizer returns the autorest.Authorizer object for the Azure SDK, for a given service
// See GetAzureEndpoint for the list of services
func GetAzureAuthorizer(service string, sp azureSPProvider) (autorest.Authorizer, error) {
	// Get the Service Principal token
	spt, err := GetAzureServicePrincipalToken(service, sp)
	if err != nil {
		return nil, err
	}

	// Build the authorizer
	authorizer := autorest.NewBearerAuthorizer(spt)

	return authorizer, nil
}

// GetAzureServicePrincipalToken returns a Service Principal token inside an adal.ServicePrincipalToken object, for a given service
// Note that the returned token needs to be refreshed with the `Refresh()` method right away before it can be used
// See GetAzureEndpoint for the list of services
func GetAzureServicePrincipalToken(service string, sp azureSPProvider) (*adal.ServicePrincipalToken, error) {
	// Get the OAuth configuration
	oauthConfig, err := GetAzureOAuthConfig(sp)
	if err != nil {
		return nil, err
	}

	// Get the endpoint
	endpoint, err := GetAzureEndpoint(service, sp)
	if err != nil {
		return nil, err
	}

	// Service Principal-based authorization
	clientId := sp.GetClientId()
	clientSecret := sp.GetClientSecret()
	if clientId == "" || clientSecret == "" {
		return nil, errors.New("sp.ClientId and sp.ClientSecret must be defined")
	}
	spt, err := adal.NewServicePrincipalToken(*oauthConfig, clientId, clientSecret, endpoint)
	if err != nil {
		return nil, err
	}

	return spt, nil
}

// GetAzureStorageCredentials returns a azblob.Credential object that can be used to authenticate an Azure Blob Storage SDK pipeline
func GetAzureStorageCredentials(sp azureSPProvider) (azblob.Credential, error) {
	// Azure Storage authorization
	spt, err := GetAzureServicePrincipalToken("storage", sp)
	if err != nil {
		return nil, err
	}

	// Token refresher function
	var tokenRefresher azblob.TokenRefresher
	tokenRefresher = func(credential azblob.TokenCredential) time.Duration {
		logger.Println("Refreshing Azure Storage auth token")

		// Get a new token
		err := spt.Refresh()
		if err != nil {
			panic(err)
		}
		token := spt.Token()
		credential.SetToken(token.AccessToken)

		// Return the expiry time (2 minutes before the token expires)
		exp := token.Expires().Sub(time.Now().Add(2 * time.Minute))
		logger.Println("Received new token, valid for", exp)
		return exp
	}

	// Credential object
	credential := azblob.NewTokenCredential("", tokenRefresher)
	return credential, nil
}
