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
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ItalyPaleAle/statiko/appconfig"
	"github.com/ItalyPaleAle/statiko/azurekeyvault"
)

// KeyVaultInfoHandler is the handler for GET /keyvaultinfo, which returns the name of the Azure Key Vault instance used
func KeyVaultInfoHandler(c *gin.Context) {
	// Ensure we have all the data
	codesignKeyName := appconfig.Config.GetString("azure.keyVault.codesignKey.name")
	codesignKeyVersion := appconfig.Config.GetString("azure.keyVault.codesignKey.version")
	if codesignKeyName == "" || codesignKeyVersion == "" || codesignKeyVersion == "latest" {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error": "Empty codesign key name or version, or latest version hasn't been resolved yet",
		})
		return
	}

	// Response
	response := struct {
		Name               string `json:"name"`
		URL                string `json:"url"`
		CodesignKeyName    string `json:"codesignKeyName"`
		CodesignKeyVersion string `json:"codesignKeyVersion"`
	}{
		Name:               azurekeyvault.GetInstance().VaultName,
		URL:                azurekeyvault.GetInstance().BaseURL(),
		CodesignKeyName:    codesignKeyName,
		CodesignKeyVersion: codesignKeyVersion,
	}

	c.JSON(http.StatusOK, response)
}
