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
	"sort"
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

	// Sync status
	res.Sync = utils.NodeSync{
		Running:  sync.IsRunning(),
		LastSync: sync.LastSync(),
	}

	// Check if we need to force a refresh
	forceQs := c.Query("force")
	if forceQs == "1" || forceQs == "true" || forceQs == "t" || forceQs == "y" || forceQs == "yes" {
		healthCache = nil
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
		// If there's a deployment, reset this, as apps aren't tested
		if sync.IsRunning() {
			appTestedTime = time.Time{}
		} else {
			// Otherwise, mark as tested
			appTestedTime = time.Now()
		}

		hasError, hasAppError := updateHealthCache()
		res.Health = healthCache

		if hasError {
			// If there's an error, make the test happen again right away
			healthCache = nil
		}
		if hasError || hasAppError {
			statusCode = http.StatusServiceUnavailable
		}
	} else {
		res.Health = healthCache
	}

	c.JSON(statusCode, res)
}

type healthcheckJob struct {
	domain string
	bundle string
}

func updateHealthCache() (hasError bool, hasAppError bool) {
	hasError = false
	hasAppError = false

	// Get list of sites
	sites := state.Instance.GetSites()

	// Use a worker pool to limit concurrency to 3
	jobs := make(chan healthcheckJob, 4)
	res := make(chan utils.SiteHealth, len(sites))

	// Spin up 3 backround workers
	for w := 1; w <= 3; w++ {
		go updateHealthCacheWorker(w, jobs, res)
	}

	// Update the cached data
	requested := 0
	healthCache = make([]utils.SiteHealth, 0)
	for _, s := range sites {
		// Skip sites that have deployment errors
		if s.Error != nil {
			hasAppError = true
			healthCache = append(healthCache, utils.SiteHealth{
				Domain:   s.Domain,
				App:      nil,
				Error:    s.Error,
				ErrorStr: s.ErrorStr,
			})
			continue
		}

		// Request health only if there's an app deployed
		// Also, skip this if there's a deployment running
		if s.App != nil && !sync.IsRunning() {
			// Check if the jobs channel is full
			for len(jobs) == cap(jobs) {
				// Pause this until the channel is not at capacity anymore
				time.Sleep(time.Millisecond * 5)
			}

			// Start the request in parallel
			jobs <- healthcheckJob{
				domain: s.Domain,
				bundle: s.App.Name + "-" + s.App.Version,
			}
			requested++
		} else {
			// No app deployed, so show the site only
			healthCache = append(healthCache, utils.SiteHealth{
				Domain: s.Domain,
				App:    nil,
			})
		}
	}
	close(jobs)

	// Read responses
	for i := 0; i < requested; i++ {
		health := <-res
		if health.Error != nil {
			hasError = true
			health.ErrorStr = new(string)
			*health.ErrorStr = health.Error.Error()
			logger.Printf("Error in domain %v: %v\n", health.Domain, health.Error)
		}
		healthCache = append(healthCache, health)
	}
	close(res)

	// Sort the result
	sort.Slice(healthCache, func(i, j int) bool {
		return healthCache[i].Domain < healthCache[j].Domain
	})

	return
}

// Background worker for the updateHealthCache function
func updateHealthCacheWorker(id int, jobs <-chan healthcheckJob, res chan<- utils.SiteHealth) {
	for j := range jobs {
		//logger.Println("Worker", id, "started requesting health for", j.domain)
		utils.RequestHealth(j.domain, j.bundle, res)
		//logger.Println("Worker", id, "finished requesting health for", j.domain)
	}
}
