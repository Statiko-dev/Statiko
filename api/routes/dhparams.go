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
	"time"

	dhparam "github.com/Luzifer/go-dhparam"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/state"
)

type dhParamsRequest struct {
	DHParams string `json:"dhparams" form:"dhparams"`
}

type dhParamsResponse struct {
	Type       string     `json:"type"`
	Date       *time.Time `json:"date,omitempty"`
	Generating bool       `json:"generating"`
}

// DHParamsSetHandler is the handler for POST /dhparams, which stores new DH parameters (PEM-encoded)
func DHParamsSetHandler(c *gin.Context) {
	// Get data from the form body
	data := &dhParamsRequest{}
	if err := c.Bind(data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}
	if data.DHParams == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "'dhparams' field must not be empty",
		})
		return
	}

	// Validate the DH parameters
	dh, err := dhparam.Decode([]byte(data.DHParams))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid DH parameters",
		})
		return
	}
	errs, ok := dh.Check()
	if !ok {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid DH parameters",
			"msg":   errs,
		})
		return
	}

	// Re-encode to PEM
	pem, err := dh.ToPEM()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Store the DH parameters
	err = state.Instance.SetDHParams(string(pem))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Return
	c.Status(http.StatusNoContent)
}

// DHParamsGetHandler is the handler for GET /dhparams, which returns wether DH parameters exist or are being re-generated
// The response type is "builtin" when the cluster is using the built-in DH parameters, and "cluster" when it's using DH parameters generated for the cluster
func DHParamsGetHandler(c *gin.Context) {
	// Get the current DH params's age
	_, date := state.Instance.GetDHParams()

	// Response
	response := &dhParamsResponse{}
	if date == nil {
		response.Type = "builtin"
	} else {
		response.Type = "cluster"
		response.Date = date
	}
	response.Generating = state.Instance.DHParamsGenerating

	c.JSON(http.StatusOK, response)
}
