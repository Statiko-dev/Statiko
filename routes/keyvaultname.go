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

	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
)

// keyVaultNameResponse is the response from the POST /uploadauth route
type keyVaultNameResponse struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// KeyVaultNameHandler is the handler for GET /keyvaultname, which returns the name of the Azure Key Vault instance used
func KeyVaultNameHandler(c *gin.Context) {
	// Vault name and URL
	vaultName := appconfig.Config.GetString("azureKeyVault.name")
	response := keyVaultNameResponse{
		Name: vaultName,
		URL:  fmt.Sprintf("https://%s.%s", vaultName, azure.PublicCloud.KeyVaultDNSSuffix),
	}

	c.JSON(http.StatusOK, response)
}
