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
	"net/http"

	"github.com/gin-gonic/gin"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// AgentOptionsSetHandler is the handler for POST /agentoptions/:name, which sets options for agents
func (s *APIServer) AgentOptionsSetHandler(c *gin.Context) {
	// Get the agent name
	name := c.Query("name")
	if name == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Empty agent name",
		})
		return
	}

	// Get data from the form body
	opts := &pb.AgentOptions{}
	if err := c.BindJSON(opts); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	//

	c.Status(http.StatusOK)
}
