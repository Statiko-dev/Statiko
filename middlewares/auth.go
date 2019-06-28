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

package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
)

// Auth middleware that checks the Authorization header in the request
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check the Authorization header, and stop invalid requests
		auth := c.GetHeader("Authorization")
		if len(auth) == 0 || auth != appconfig.Config.GetString("auth") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Authorization header",
			})
			return
		}
	}
}
