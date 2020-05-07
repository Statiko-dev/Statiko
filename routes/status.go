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
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/statuscheck"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
	"github.com/statiko-dev/statiko/webserver"
)

var hiddenError = errors.New("<hidden error>")

// StatusHandler is the handler for GET /status (with an optional domain as in /status/:domain), which returns the status and health of the node
func StatusHandler(c *gin.Context) {
	isAuthenticated := c.GetBool("authenticated")

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
	res := &utils.NodeStatus{}

	// Node name
	res.NodeName = appconfig.Config.GetString("nodeName")

	// Nginx server status
	// Ignore errors in this command
	nginxStatus, _ := webserver.Instance.Status()
	res.Nginx = utils.NginxStatus{
		Running: nginxStatus,
	}

	// Sync status
	syncError := sync.SyncError()
	syncErrorStr := ""
	if syncError != nil {
		syncErrorStr = syncError.Error()
	}
	res.Sync = utils.NodeSync{
		Running:   sync.IsRunning(),
		LastSync:  sync.LastSync(),
		SyncError: syncErrorStr,
	}

	// Store status
	storeHealth, _ := state.Instance.StoreHealth()
	res.Store = utils.NodeStore{
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
	healthCache := statuscheck.GetHealthCache()

	// If we're requesting a domain only, filter the results
	if domain := c.Param("domain"); len(domain) > 0 {
		// Get the main domain for the site, if we're being passed an alias
		siteObj := state.Instance.GetSite(domain)
		if siteObj != nil && siteObj.Domain != "" {
			// Main domain for the site
			domain = siteObj.Domain

			// Check if we have the health object for this site, and if it has any deployment error
			var domainHealth utils.SiteHealth
			found := false
			appError := false
			for _, el := range healthCache {
				if el.Domain == domain {
					domainHealth = el
					found = true
					if !el.IsHealthy() {
						appError = true
					}
					break
				}
			}

			if found {
				// If we're not authenticated, do not display the app name, nor the full error
				// In this case, the user requested a domain only, so they know the domain anyways
				if !isAuthenticated && domainHealth.App != nil {
					app := "<hidden>"
					domainHealth.App = &app
					if domainHealth.Error != nil {
						domainHealth.Error = hiddenError
					}
				}

				res.Health = []utils.SiteHealth{domainHealth}
			}

			// If there's a deployment error for the app, and we're requesting a domain only, return a 503 response
			if appError {
				statusCode = http.StatusServiceUnavailable
			}
		} else {
			// Site not found, so return a 404
			statusCode = http.StatusNotFound
		}
	} else {
		// We've requested all sites; return an error status code if they're all failing
		errorCount := 0
		total := len(healthCache)
		if total > 0 {
			res.Health = make([]utils.SiteHealth, total)
			for i, el := range healthCache {
				if !el.IsHealthy() {
					errorCount++
				} else if el.App == nil {
					// Ignore sites that have no apps and no errors in the counts
					total--
				}

				// If we're not authenticated, do not display the app and domain name
				if !isAuthenticated {
					if el.App != nil {
						app := "<hidden>"
						el.App = &app
					}
					el.Domain = "Domain " + strconv.Itoa(i+1)
					if el.Error != nil {
						el.Error = hiddenError
					}
				}
				res.Health[i] = el
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
