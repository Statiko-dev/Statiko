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
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/statuscheck"
	"github.com/statiko-dev/statiko/utils"
)

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

	// Check if we need to force a refresh
	forceQs := c.Query("force")
	if forceQs == "1" || forceQs == "true" || forceQs == "t" || forceQs == "y" || forceQs == "yes" {
		statuscheck.ResetHealthCache()
		statuscheck.UpdateStoredNodeHealth()
	}

	// Get node health
	health := state.Instance.GetNodeHealth()

	// Response object
	// Copy properties to avoid modifying the object in the state
	res := &utils.NodeStatus{
		NodeName: health.NodeName,
		Nginx:    health.Nginx,
		Sync:     health.Sync,
		Store:    health.Store,
	}

	// Response status code
	statusCode := http.StatusOK

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
			for _, el := range health.Health {
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
				if !isAuthenticated {
					if domainHealth.App != nil {
						app := "<hidden>"
						domainHealth.App = &app
					}
					if domainHealth.Error != "" {
						domainHealth.Error = "<hidden error>"
					}
				}

				res.Health = []utils.SiteHealth{domainHealth}
			} else {
				res.Health = []utils.SiteHealth{}
			}

			// If there's a deployment error for the app, and we're requesting a domain only, return a 503 response
			if appError {
				statusCode = http.StatusServiceUnavailable
			}
		} else {
			// Site not found, so return a 404
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	} else {
		// We've requested all sites; return an error status code if they're all failing
		errorCount := 0
		var total int = 0
		if health.Health != nil {
			total = len(health.Health)
		}
		if total > 0 {
			obj := make([]utils.SiteHealth, total)
			for i, el := range health.Health {
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
					el.Domain = censorDomainName(el.Domain)
					if el.Error != "" {
						el.Error = "<hidden error>"
					}
				}
				obj[i] = el
			}
			res.Health = obj
		}
		if total > 0 && errorCount == total {
			// All are failing, return a 503 status
			statusCode = http.StatusServiceUnavailable
		}
	}

	// If Nginx isn't working, status code is always 503
	if !res.Nginx.Running {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, res)
}

// Censors most digits in the domain name. Maintains the first 2 digits of each sub-domain, plus the full TLD
func censorDomainName(domain string) string {
	// Get the various parts of the domain name
	parts := strings.Split(domain, ".")

	// Do the censorship
	result := ""
	l := len(parts)
	for i := 0; i < l; i++ {
		if len(result) > 0 {
			result += "."
		}
		// If there's more than one part, then don't censor the TLD
		if l > 1 && i == l-1 {
			result += parts[i]
		} else {
			result += parts[i][0:2] + "**"
		}
	}

	return result
}
