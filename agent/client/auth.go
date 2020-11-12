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

package client

import (
	"context"
	"errors"
	"time"

	"github.com/Azure/go-autorest/autorest/adal"
	"github.com/spf13/viper"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

const (
	rpcAuthPSK = iota
	rpcAuthAzureAD
	rpcAuthAuth0
)

const TokenRefreshTimeout = 10 * time.Second

// rpcAuth is the object implementing credentials.PerRPCCredentials that provides the auth info
type rpcAuth struct {
	typ     int
	psk     string
	azureAd *adal.ServicePrincipalToken
}

// Init the object setting the right kind of auth provider
func (a *rpcAuth) Init() (err error) {
	// TODO: Server needs to tell the client supported auth methods

	// Check if we have a PSK
	psk := viper.GetString("controller.auth.psk")
	if psk != "" {
		a.typ = rpcAuthPSK
		a.psk = psk
		return nil
	}

	// Check if we can authenticate via Azure
	tenantId := viper.GetString("controller.auth.azure.tenantId")
	clientId := viper.GetString("controller.auth.azure.clientId")
	clientSecret := viper.GetString("controller.auth.azure.clientSecret")
	if tenantId != "" && clientId != "" && clientSecret != "" {
		a.typ = rpcAuthAzureAD
		sp := &pb.ClusterOptions_AzureServicePrincipal{
			TenantId:     tenantId,
			ClientId:     clientId,
			ClientSecret: clientSecret,
		}
		a.azureAd, err = utils.GetAzureServicePrincipalToken("azure", sp)
		return err
	}

	return errors.New("no auth info found in the configuration")
}

// GetRequestMetadata returns the metadata containing the authorization key
func (a *rpcAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	auth := ""

	// Context with timeout for refreshing tokens
	ctxTimeout, cancel := context.WithTimeout(ctx, TokenRefreshTimeout)
	defer cancel()

	switch a.typ {

	// PSK
	case rpcAuthPSK:
		auth = a.psk

	// Azure AD
	case rpcAuthAzureAD:
		// Ensure the token is fresh - this is a blocking call
		err := a.azureAd.EnsureFreshWithContext(ctxTimeout)
		if err != nil {
			return nil, err
		}
		auth = a.azureAd.OAuthToken()
	}

	return map[string]string{
		"authorization": "Bearer " + auth,
	}, nil
}

// RequireTransportSecurity returns true because this kind of auth requires TLS
func (a *rpcAuth) RequireTransportSecurity() bool {
	return true
}
