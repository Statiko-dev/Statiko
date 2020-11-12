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
	"time"

	"github.com/spf13/viper"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

const (
	rpcAuthNone = iota
	rpcAuthPSK
	rpcAuthOAuth
)

const TokenRefreshTimeout = 10 * time.Second

// Contains details for authenticating via OAuth
// It's just like pb.ClusterOptions_Auth_OAuthInfo, but it also has a client secret value
type rpcOAuth struct {
	*pb.ClusterOptions_Auth_OAuthInfo
	ClientSecret string
}

// rpcAuth is the object implementing credentials.PerRPCCredentials that provides the auth info
type rpcAuth struct {
	typ        int
	psk        string
	oauth      *rpcOAuth
	oauthToken string
}

// Init the object setting the right kind of auth provider, by passing the authOpts object
func (a *rpcAuth) Init(authOpts *pb.ClusterOptions_Auth) {
	// Reset the object
	a.typ = rpcAuthNone
	a.psk = ""
	a.oauth = nil
	a.oauthToken = ""

	// If the object is empty, then just set authentication to nil
	if authOpts == nil {
		return
	}

	// Check if the server supports a PSK and we have it
	if authOpts.Psk {
		// We need a PSK configured
		psk := viper.GetString("controller.auth.psk")
		if psk != "" {
			a.typ = rpcAuthPSK
			a.psk = psk
			return
		}
	}

	// Check if we can authenticate using Azure AD
	if authOpts.AzureAd != nil && authOpts.AzureAd.ClientId != "" {
		// We need a client secret configured
		clientSecret := viper.GetString("controller.auth.azureClientSecret")
		if clientSecret != "" {
			a.typ = rpcAuthOAuth
			a.oauth = &rpcOAuth{
				ClusterOptions_Auth_OAuthInfo: authOpts.AzureAd,
				ClientSecret:                  clientSecret,
			}
			return
		}
	}

	// Check if we can authenticate using Auth0
	if authOpts.Auth0 != nil && authOpts.Auth0.ClientId != "" {
		// We need a client secret configured
		clientSecret := viper.GetString("controller.auth.auth0ClientSecret")
		if clientSecret != "" {
			a.typ = rpcAuthOAuth
			a.oauth = &rpcOAuth{
				ClusterOptions_Auth_OAuthInfo: authOpts.Auth0,
				ClientSecret:                  clientSecret,
			}
			return
		}
	}

	return
}

// GetRequestMetadata returns the metadata containing the authorization key
func (a *rpcAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	auth := ""

	// Context with timeout for refreshing tokens
	ctxTimeout, cancel := context.WithTimeout(ctx, TokenRefreshTimeout)
	defer cancel()

	// Type of auth
	switch a.typ {

	// PSK
	case rpcAuthPSK:
		auth = a.psk

	// OAuth
	case rpcAuthOAuth:
		// Ensure the token is fresh - this is a blocking call
		err := a.azureAd.EnsureFreshWithContext(ctxTimeout)
		if err != nil {
			return nil, err
		}
		auth = a.azureAd.OAuthToken()

	// No auth (includes case of rpcAuthNone)
	default:
		return nil, nil
	}

	return map[string]string{
		"authorization": "Bearer " + auth,
	}, nil
}

// RequireTransportSecurity returns true because this kind of auth requires TLS
func (a *rpcAuth) RequireTransportSecurity() bool {
	return true
}
