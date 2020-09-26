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

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
)

// DeploySiteHandler is the handler for POST/PUT /site/{domain}/app, which deploys an app
func (s *APIServer) DeploySiteHandler(c *gin.Context) {
	// Get the site to update (domain name)
	domain := c.Param("domain")
	if len(domain) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
		return
	}

	// Get the site from the state object
	site := state.Instance.GetSite(domain)
	if site == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Domain name not found",
		})
		return
	}

	// Get the app to deploy from the body
	var app state.SiteApp
	if err := c.Bind(&app); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}
	if !app.Validate() {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid app name",
		})
		return
	}

	site.App = &app

	// Update the app
	if err := state.Instance.UpdateSite(site, true); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Queue a sync
	sync.QueueRun()

	// Respond with "No content"
	c.Status(http.StatusNoContent)
}
