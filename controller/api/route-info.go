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

package api

import (
	"fmt"
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
)

// infoResponse is the response for the /info route
type infoResponse struct {
	AuthMethods []string            `json:"authMethods"`
	AzureAD     *openIDInfoResponse `json:"azureAD,omitempty"`
	Auth0       *openIDInfoResponse `json:"auth0,omitempty"`
	Version     string              `json:"version"`
	Hostname    string              `json:"hostname"`
}

// openIDInfoResponse is part of the infoResponse struct
type openIDInfoResponse struct {
	AuthorizeURL string `json:"authorizeUrl"`
	TokenURL     string `json:"tokenUrl"`
	ClientID     string `json:"clientId"`
}

// InfoHandler is the handler for GET /info, which returns information about the agent running
func (s *APIServer) InfoHandler(c *gin.Context) {
	// Check auth info
	authMethods := make([]string, 0)
	var azureADInfo, auth0Info *openIDInfoResponse
	if viper.GetBool("auth.psk.enabled") {
		authMethods = append(authMethods, "psk")
	}

	// Only one of Azure AD and Auth0 can be enabled at the same time
	azureADEnabled := viper.GetBool("auth.azureAD.enabled")
	auth0Enabled := viper.GetBool("auth.auth0.enabled")
	if azureADEnabled && !auth0Enabled {
		authMethods = append(authMethods, "azureAD")

		// Get the URL where users can authenticate
		tenantId := viper.GetString("auth.azureAD.tenantId")
		clientId := viper.GetString("auth.azureAD.clientId")
		if tenantId != "" && clientId != "" {
			azureADInfo = &openIDInfoResponse{
				AuthorizeURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantId),
				TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantId),
				ClientID:     clientId,
			}
		}
	}
	if auth0Enabled && !azureADEnabled {
		authMethods = append(authMethods, "auth0")

		// Get the URL where users can authenticate
		clientId := viper.GetString("auth.auth0.clientId")
		domain := viper.GetString("auth.auth0.domain")
		if clientId != "" && domain != "" {
			auth0Info = &openIDInfoResponse{
				AuthorizeURL: fmt.Sprintf("https://%s/authorize", domain),
				TokenURL:     fmt.Sprintf("https://%s/oauth/token", domain),
				ClientID:     clientId,
			}
		}
	}

	// Version string
	version := fmt.Sprintf("%s (%s; %s) %s", buildinfo.BuildID, buildinfo.CommitHash, buildinfo.BuildTime, runtime.Version())

	// Response
	info := infoResponse{
		AuthMethods: authMethods,
		AzureAD:     azureADInfo,
		Auth0:       auth0Info,
		Version:     version,
		Hostname:    viper.GetString("nodeName"),
	}

	c.JSON(http.StatusOK, info)
}
