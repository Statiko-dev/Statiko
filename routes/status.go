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
	"time"

	"github.com/gin-gonic/gin"

	"smplatform/state"
	"smplatform/sync"
	"smplatform/utils"
)

// Last time the health checks were run
var appTestedTime = time.Time{}

// Last time the state was updated
var stateUpdatedTime *time.Time

// Cached health data
var healthCache []utils.SiteHealth

// StatusHandler is the handler for GET /status, which returns the status and health of the node
// @Summary Returns the status and health of the node
// @Description Returns an object listing the sites currently running and the apps deployed.
// @Description This route is designed and optimized for periodic health-checks too.
// @Descriptions In fact, responses are cached and served from memory for fast responses, unless any configuration has changed.
// @Produce json
// @Success 200 {array} actions.NodeStatus
// @Router /status [get]
func StatusHandler(c *gin.Context) {
	// Response object
	res := &utils.NodeStatus{}

	// Sync status
	res.Sync = utils.NodeSync{
		Running:  sync.IsRunning(),
		LastSync: sync.LastSync(),
	}

	// Response status code
	statusCode := http.StatusOK

	// If the state has changed, we need to invalidate the healthCache
	u := state.Instance.LastUpdated()
	if u != stateUpdatedTime {
		healthCache = nil
		stateUpdatedTime = u
	}

	// Test if the actual apps are responding (just to be sure), but only every 5 minutes
	diff := time.Since(appTestedTime).Seconds()
	if healthCache == nil || diff > 299 {
		appTestedTime = time.Now()

		// Update the cached data
		requested := 0
		sites := state.Instance.GetSites()
		ch := make(chan utils.SiteHealth, len(sites))
		healthCache = make([]utils.SiteHealth, 0)
		for _, s := range sites {
			// Request health only if there's an app being deployed
			if s.App != nil {
				// Start the request in parallel
				go utils.RequestHealth(s.Domain, s.App.Name+"-"+s.App.Version, ch)
				requested++
			} else {
				// No app deployed, so show the site only
				healthCache = append(healthCache, utils.SiteHealth{
					Domain: s.Domain,
					App:    nil,
				})
			}
		}

		// Read responses
		hasError := false
		for i := 0; i < requested; i++ {
			health := <-ch
			if health.Error != nil {
				hasError = true
				health.ErrorStr = new(string)
				*health.ErrorStr = health.Error.Error()
				logger.Printf("Error in domain %v: %v\n", health.Domain, health.Error)
			}
			healthCache = append(healthCache, health)
		}

		res.Health = healthCache

		if hasError {
			// If there's an error, make the test happen again right away
			healthCache = nil
			statusCode = http.StatusServiceUnavailable
		}
	} else {
		res.Health = healthCache
	}

	c.JSON(statusCode, res)
}
