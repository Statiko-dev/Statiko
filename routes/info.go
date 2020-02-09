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

package routes

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItalyPaleAle/statiko/appconfig"
	"github.com/ItalyPaleAle/statiko/buildinfo"
)

// infoResponse is the response for the /info route
type infoResponse struct {
	AuthMethods []string             `json:"authMethods"`
	AzureAD     *azureADInfoResponse `json:"azureAD,omitempty"`
	Version     string               `json:"version"`
	Hostname    string               `json:"hostname"`
}

// azureADInfoResponse is part of the infoResponse struct
type azureADInfoResponse struct {
	AuthorizeURL string `json:"authorizeUrl"`
	TokenURL     string `json:"tokenUrl"`
	ClientID     string `json:"clientId"`
}

// InfoHandler is the handler for GET /info, which returns information about the agent running
func InfoHandler(c *gin.Context) {
	// Check auth info
	authMethods := make([]string, 0)
	var azureADInfo *azureADInfoResponse
	if appconfig.Config.GetBool("auth.psk.enabled") {
		authMethods = append(authMethods, "psk")
	}
	if appconfig.Config.GetBool("auth.azureAD.enabled") {
		authMethods = append(authMethods, "azureAD")

		// Get the URL where users can authenticate
		tenantId := appconfig.Config.GetString("auth.azureAD.tenantId")
		clientId := appconfig.Config.GetString("auth.azureAD.clientId")

		azureADInfo = &azureADInfoResponse{
			AuthorizeURL: fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize", tenantId),
			TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", tenantId),
			ClientID:     clientId,
		}
	}

	// Response
	info := infoResponse{
		AuthMethods: authMethods,
		AzureAD:     azureADInfo,
		Version:     buildinfo.BuildID + " (" + buildinfo.CommitHash + "; " + buildinfo.BuildTime + ")",
		Hostname:    appconfig.Config.GetString("nodeName"),
	}

	c.JSON(http.StatusOK, info)
}
