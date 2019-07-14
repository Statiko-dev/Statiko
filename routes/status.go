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
	"smplatform/utils"
)

// Last time the health checks were run
var appTestedTime = time.Time{}

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

	// Get list of apps
	sites := state.Instance.GetSites()
	res.Apps = make([]utils.NodeApps, len(sites))
	for i, s := range sites {
		el := utils.NodeApps{
			Domain: s.Domain,
		}
		if s.App != nil {
			el.AppName = &s.App.Name
			el.AppVersion = &s.App.Version
			el.Deployed = s.App.Time
		}

		res.Apps[i] = el
	}

	// Response status code
	statusCode := http.StatusOK

	// Test if the actual apps are responding (just to be sure), but only every 5 minutes
	diff := time.Since(appTestedTime).Seconds()
	if diff > 299 {
		appTestedTime = time.Now()

		// Update the cached data
		ch := make(chan utils.SiteHealth)
		requested := 0
		for _, app := range res.Apps {
			// Ignore sites that have no apps deployed
			if app.AppName == nil || app.AppVersion == nil {
				continue
			}

			// Start the request in parallel
			go utils.RequestHealth(app.Domain, ch)
			requested++
		}

		// Read responses
		res.Health = make([]utils.SiteHealth, requested)
		hasError := false
		for i := 0; i < requested; i++ {
			health := <-ch
			if health.Error != nil {
				hasError = true
				health.ErrorStr = new(string)
				*health.ErrorStr = health.Error.Error()
				logger.Printf("Error in domain %v: %v\n", health.Domain, health.Error)
			}
			res.Health[i] = health
		}

		if hasError {
			// If there's an error, make the test happen again right away
			appTestedTime = time.Time{}
			statusCode = http.StatusServiceUnavailable
		}
	}

	c.JSON(statusCode, res)
}
