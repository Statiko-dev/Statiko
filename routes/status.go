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

	"github.com/ItalyPaleAle/statiko/state"
	"github.com/ItalyPaleAle/statiko/statuscheck"
	"github.com/ItalyPaleAle/statiko/sync"
	"github.com/ItalyPaleAle/statiko/webserver"
)

// StatusHandler is the handler for GET /status (with an optional domain as in /status/:domain), which returns the status and health of the node
func StatusHandler(c *gin.Context) {
	// Check if the state is healthy
	healthy, err := state.Instance.StoreHealth()
	if !healthy {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":   "State is not healthy",
			"details": err.Error(),
		})
		return
	}

	// Response object
	res := &statuscheck.NodeStatus{}

	// Nginx server status
	// Ignore errors in this command
	nginxStatus, _ := webserver.Instance.Status()
	res.Nginx = statuscheck.NginxStatus{
		Running: nginxStatus,
	}

	// Sync status
	syncError := sync.SyncError()
	syncErrorStr := ""
	if syncError != nil {
		syncErrorStr = syncError.Error()
	}
	res.Sync = statuscheck.NodeSync{
		Running:   sync.IsRunning(),
		LastSync:  sync.LastSync(),
		SyncError: syncErrorStr,
	}

	// Store status
	storeHealth, _ := state.Instance.StoreHealth()
	res.Store = statuscheck.NodeStore{
		Healthy: storeHealth,
	}

	// Check if we need to force a refresh
	forceQs := c.Query("force")
	if forceQs == "1" || forceQs == "true" || forceQs == "t" || forceQs == "y" || forceQs == "yes" {
		statuscheck.ResetHealthCache()
	}

	// Response status code
	statusCode := http.StatusOK

	// Test if the actual apps are responding (just to be sure), but only every 5 minutes
	res.Health = statuscheck.GetHealthCache()

	// If we're requesting a domain only, filter the results
	if domain := c.Param("domain"); len(domain) > 0 {
		// Get the main domain for the site, if we're being passed an alias
		siteObj := state.Instance.GetSite(domain)
		if siteObj != nil && siteObj.Domain != "" {
			// Main domain for the site
			domain = siteObj.Domain

			// Check if we have the health object for this site, and if it has any deployment error
			var domainHealth *statuscheck.SiteHealth
			appError := false
			for _, el := range res.Health {
				if el.Domain == domain {
					domainHealth = &el
					if el.Error != nil {
						appError = true
					}
					break
				}
			}

			if domainHealth != nil {
				res.Health = make([]statuscheck.SiteHealth, 1)
				res.Health[0] = *domainHealth
			} else {
				res.Health = nil
			}

			// If there's a deployment error for the app, and we're requesting a domain only, return a 503 response
			if appError {
				statusCode = http.StatusServiceUnavailable
			}
		} else {
			// Site not found, so return a 404
			statusCode = http.StatusNotFound
			res.Health = nil
		}
	} else {
		// We've requested all sites; return an error status code if they're all failing
		errorCount := 0
		total := len(res.Health)
		for _, el := range res.Health {
			if el.Error != nil {
				errorCount++
			} else if el.App == nil {
				// Ignore sites that have no apps and no errors in the counts
				total--
			}
		}
		if total > 0 && errorCount == total {
			// All are failing, return a 503 status
			statusCode = http.StatusServiceUnavailable
		}
	}

	// If Nginx isn't working, status code is always 503
	if !nginxStatus {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, res)
}
