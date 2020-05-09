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

package statuscheck

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
)

// Semaphore that allows only one operation at time
var semaphore = make(chan int, 1)

// ResetHealthCache resets the cached status
// Note that this function might block the current goroutine if there's another operation running, so plan accordingly
func ResetHealthCache() {
	semaphore <- 1
	healthCache = nil
	<-semaphore
}

// GetHealthCache returns the health of sites
// Will return a cached response if available
// Note that this function might block the current goroutine if there's another operation running, so plan accordingly
func GetHealthCache() (result []utils.SiteHealth) {
	semaphore <- 1

	// If the state has changed, we need to invalidate the healthCache
	u := state.Instance.LastUpdated()
	if u != stateUpdatedTime {
		healthCache = nil
		stateUpdatedTime = u
	}

	// If the health cache is null or outdated, re-generate it
	diff := time.Since(appTestedTime).Seconds()
	if healthCache == nil || diff > float64(StatusCheckInterval-1) {
		// If there's a deployment, reset this, as apps aren't tested
		if sync.IsRunning() {
			appTestedTime = time.Time{}
		} else {
			// Otherwise, mark as tested
			appTestedTime = time.Now()
		}

		hasError := updateHealthCache()
		result = healthCache

		if hasError {
			// If there's an error, make the test happen again right away on the next call
			healthCache = nil
		}
	} else {
		result = healthCache
	}

	<-semaphore

	return
}

// RequestHealth makes a request to the app and checks its health (requests status code 2xx)
func RequestHealth(domain string, app string, ch chan<- utils.SiteHealth) {
	var statusCode int
	var responseSize int

	// Build the request object
	reqURL, _ := url.Parse("https://localhost")
	req := http.Request{
		Method: "GET",
		// URL is always localhost as we're connecting to the nginx server
		// The domain is specified in the Host header
		URL:  reqURL,
		Host: domain,
	}

	// Make the request
	resp, err := httpClient.Do(&req)
	now := time.Now()
	if err != nil {
		ch <- utils.SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        err.Error(),
		}
		return
	}

	// Check if status code is 2xx
	statusCode = resp.StatusCode
	if statusCode < 200 || statusCode > 299 {
		ch <- utils.SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        fmt.Errorf("Invalid status code: %d", resp.StatusCode).Error(),
		}
		return
	}

	// Read the response body to calculate the size
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- utils.SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        err.Error(),
		}
		return
	}
	responseSize = len(bodyBytes)
	if responseSize < 1 {
		ch <- utils.SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        fmt.Errorf("Invalid response size: %d", responseSize).Error(),
		}
		return
	}

	// Success!
	ch <- utils.SiteHealth{
		Domain:       domain,
		App:          &app,
		StatusCode:   &statusCode,
		ResponseSize: &responseSize,
		Time:         &now,
		Error:        "",
	}
}

// Fetch new data for the health cache
func updateHealthCache() (hasError bool) {
	hasError = false

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
		if siteErr := state.Instance.GetSiteHealth(s.Domain); siteErr != nil {
			var appStr *string
			if s.App != nil {
				str := s.App.Name + "-" + s.App.Version
				appStr = &str
			}
			healthCache = append(healthCache, utils.SiteHealth{
				Domain: s.Domain,
				App:    appStr,
				Error:  siteErr.Error(),
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
		if health.Error != "" {
			hasError = true
			errStr := fmt.Sprintf("Status check failed for domain %v: %v", health.Domain, health.Error)
			logger.Println(errStr)

			// If we are here, the app did not have an error before (sites with deployment errors were not in the list of sites whose health we check)
			// So, let's notify the admin
			if notificationsSent[health.Domain] != errStr {
				notificationsSent[health.Domain] = errStr
				go notifications.SendNotification(errStr)
			}
		} else {
			notificationsSent[health.Domain] = ""
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
		RequestHealth(j.domain, j.bundle, res)
		//logger.Println("Worker", id, "finished requesting health for", j.domain)
	}
}

// Result for health check jobs
type healthcheckJob struct {
	domain string
	bundle string
}
