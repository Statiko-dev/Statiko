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
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/shared/utils"
)

// ACMEChallengeHandler is the handler for GET /.well-known/acme-challenge/:token, which is used by the ACME challenge
// This is the ACME challenge for the controller's certificate only, and it's not used by challenges for the various sites' certificates
func (s *APIServer) ACMEChallengeHandler(c *gin.Context) {
	// Host header
	host := utils.GetRequestHost(c)
	if host == "" {
		c.AbortWithError(http.StatusBadRequest, errors.New("could not find Host (or X-Forwarded-Host) header"))
		return
	}

	// Ensure that the host is identical to the nodeName
	nodeName := viper.GetString("nodeName")
	if host != nodeName {
		c.AbortWithError(http.StatusBadRequest, errors.New("request is for a different host"))
		return
	}

	// Get token
	token := c.Param("token")
	if len(token) < 1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Get the response from the secret store
	message, err := s.State.GetSecret("acme/challenges/" + token)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	parts := strings.SplitN(string(message), "|", 2)

	// Check the host
	if parts[0] != nodeName {
		c.AbortWithError(http.StatusBadRequest, errors.New("request is for a different host"))
		return
	}

	// Respond
	c.Data(http.StatusOK, "text/plain", []byte(parts[1]))
}
