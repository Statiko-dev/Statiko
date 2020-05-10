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

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/state"
)

// ACMEChallengeHandler is the handler for GET /.well-known/acme-challenge/:token, which is used by the ACME challenge
func ACMEChallengeHandler(c *gin.Context) {
	token := c.Param("token")
	if len(token) < 1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Get the response from the secret store
	keyAuth, err := state.Instance.GetSecret("acme/challenges/" + token)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	// Respond
	c.Data(http.StatusOK, "text/plain", keyAuth)
}
