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

// GetStateHandler is the handler for GET /state, which dumps the state
func (s *APIServer) GetStateHandler(c *gin.Context) {
	obj, err := s.State.DumpState()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, obj)
}

// PutStateHandler is the handler for PUT /state (and POST /state), which replaces the state with the input
func (s *APIServer) PutStateHandler(c *gin.Context) {
	// Get updated state from the body
	st := &pb.State{}
	if err := c.BindJSON(st); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Replace the state
	if err := s.State.ReplaceState(st); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Status(http.StatusNoContent)
}
