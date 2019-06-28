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

	"smplatform/db"
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
	// Check if we have the status cached
	if statusCache == nil {
		// Reset the health check time too
		appTestedTime = time.Time{}
		// Create the cache
		statusCache = &utils.NodeStatus{}

		// Load the status from the database
		sql := `
		SELECT
			sites.site_id AS site_id,
			domains.domain AS domain,
			deployments.app_name AS app_name,
			deployments.app_version AS app_version,
			deployments.time AS time
		FROM sites
		LEFT JOIN domains
			ON domains.site_id = sites.site_id AND domains.is_default = 1
		LEFT JOIN deployments
			ON deployments.deployment_id = (
				SELECT deployment_id
				FROM deployments
				WHERE deployments.site_id = sites.site_id AND deployments.status = 1
				ORDER BY deployments.time DESC
				LIMIT 1
			)
		`
		if err := db.Connection.Raw(sql).Scan(&statusCache.Apps).Error; err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
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
		for _, app := range statusCache.Apps {
			// Ignore sites that have no apps deployed
			if app.AppName == nil || app.AppVersion == nil {
				continue
			}

			// Start the request in parallel
			go utils.RequestHealth(app.Domain, ch)
			requested++
		}

		// Read responses
		statusCache.Health = make([]utils.SiteHealth, requested)
		hasError := false
		for i := 0; i < requested; i++ {
			health := <-ch
			if health.Error != nil {
				hasError = true
				health.ErrorStr = new(string)
				*health.ErrorStr = health.Error.Error()
				logger.Printf("Error in domain %v: %v\n", health.Domain, health.Error)
			}
			statusCache.Health[i] = health
		}

		if hasError {
			// If there's an error, make the test happen again right away
			appTestedTime = time.Time{}
			statusCode = http.StatusServiceUnavailable
		}
	}

	c.JSON(statusCode, statusCache)
}
