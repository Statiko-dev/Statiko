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

package httpserver

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ACMEChallengeHandler is the handler for GET /.well-known/acme-challenge/:token, which is used by the ACME challenge
func (s *HTTPServer) ACMEChallengeHandler(c *gin.Context) {
	// Host header
	// Check X-Forwarded-Host first, then Host
	host := ""
	if h := c.GetHeader("X-Forwarded-Host"); h != "" {
		host = h
	} else if h := c.GetHeader("Host"); h != "" {
		host = h
	} else {
		c.AbortWithError(http.StatusBadRequest, errors.New("could not find Host (or X-Forwarded-Host) header"))
		return
	}

	// Get token
	token := c.Param("token")
	if len(token) < 1 {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Get the response
	res, err := s.RPC.GetACMEChallengeResponse(token, host)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Respond
	c.Data(http.StatusOK, "text/plain", []byte(res.Response))
}
