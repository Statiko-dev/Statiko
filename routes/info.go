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
	"os"

	"github.com/gin-gonic/gin"

	"smplatform/buildinfo"
)

// InfoResponse is the response for the /info request
type InfoResponse struct {
	AuthMethod string `json:"authMethod"`
	Version    string `json:"version"`
	Hostname   string `json:"hostname"`
}

// InfoHandler is the handler for GET /info, which returns information about the agent running
func InfoHandler(c *gin.Context) {
	hostname, err := os.Hostname()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	info := InfoResponse{
		AuthMethod: "sharedkey",
		Version:    buildinfo.BuildID + " (" + buildinfo.CommitHash + "; " + buildinfo.BuildTime + ")",
		Hostname:   hostname,
	}

	c.JSON(http.StatusOK, info)
}
