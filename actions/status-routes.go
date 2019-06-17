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

package actions

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/uuid"

	"smplatform/models"
)

// NodeApps contains the current list of apps defined
type NodeApps struct {
	SiteID     uuid.UUID    `json:"id" db:"site_id"`
	Domain     string       `json:"domain" db:"domain"`
	AppName    nulls.String `json:"appName" db:"app_name"`
	AppVersion nulls.String `json:"appVersion" db:"app_version"`
	Updated    nulls.Time   `json:"updated" db:"updated"`
}

// SiteHealth contains the results of the health checks for each individual app
type SiteHealth struct {
	Domain       string    `json:"domain"`
	StatusCode   int       `json:"status"`
	ResponseSize int       `json:"size"`
	Error        error     `json:"-"`
	ErrorStr     string    `json:"error"`
	Time         time.Time `json:"time"`
}

// NodeStatus contains the current status of the node
type NodeStatus struct {
	Apps   []NodeApps   `json:"apps"`
	Health []SiteHealth `json:"health"`
}

// Last time the health checks were run
var appTestedTime = time.Time{}

// RequestHealth makes a request to the app and checks its health (status code 2xx)
func RequestHealth(domain string, httpClient *http.Client, ch chan<- SiteHealth) {
	var statusCode int
	var responseSize int

	// Build the request object
	reqURL, _ := url.Parse("https://127.0.0.1")
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
		ch <- SiteHealth{
			Domain:       domain,
			StatusCode:   statusCode,
			ResponseSize: responseSize,
			Time:         now,
			Error:        err,
		}
		return
	}

	// Check if status code is 2xx
	statusCode = resp.StatusCode
	if statusCode < 200 || statusCode > 299 {
		ch <- SiteHealth{
			Domain:       domain,
			StatusCode:   statusCode,
			ResponseSize: responseSize,
			Time:         now,
			Error:        fmt.Errorf("Invalid status code: %d", resp.StatusCode),
		}
		return
	}

	// Read the response body to calculate the size
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- SiteHealth{
			Domain:       domain,
			StatusCode:   statusCode,
			ResponseSize: responseSize,
			Time:         now,
			Error:        err,
		}
		return
	}
	responseSize = len(bodyBytes)
	if responseSize < 1 {
		ch <- SiteHealth{
			Domain:       domain,
			StatusCode:   statusCode,
			ResponseSize: responseSize,
			Time:         now,
			Error:        fmt.Errorf("Invalid response size: %d", responseSize),
		}
		return
	}

	// Success!
	ch <- SiteHealth{
		Domain:       domain,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		Time:         now,
		Error:        nil,
	}
}

// StatusHandler is the handler for GET /status, which returns the status and health of the node
// @Summary Returns the status and health of the node
// @Description Returns an object listing the sites currently running and the apps deployed.
// @Description This route is designed and optimized for periodic health-checks too.
// @Descriptions In fact, responses are cached and served from memory for fast responses, unless any configuration has changed.
// @Produce json
// @Success 200 {array} actions.NodeStatus
// @Router /status [get]
func (rts *Routes) StatusHandler(c buffalo.Context) error {
	// Check if we have the status cached
	if rts.statusCache == nil {
		// Reset the health check time too
		appTestedTime = time.Time{}
		// Create the cache
		rts.statusCache = &NodeStatus{}

		// Load the status from the database
		err := models.DB.RawQuery(`
		SELECT
			sites.id AS site_id,
			domains.domain AS domain,
			deployments.app_name AS app_name,
			deployments.app_version AS app_version,
			deployments.updated_at AS updated
		FROM sites
		LEFT JOIN domains
			ON domains.site_id = sites.id AND domains.is_default = 1
		LEFT JOIN deployments
			ON deployments.id = (
				SELECT id
				FROM deployments
				WHERE deployments.site_id = sites.id AND deployments.status = 1
				ORDER BY deployments.updated_at DESC
				LIMIT 1
			)
		`).All(&rts.statusCache.Apps)
		if err != nil {
			return err
		}
	}

	// Response status code
	statusCode := 200

	// Test if the actual apps are responding (just to be sure), but only every 5 minutes
	diff := time.Since(appTestedTime).Seconds()
	if diff > 299 {
		appTestedTime = time.Now()

		// Update the cached data
		ch := make(chan SiteHealth)
		requested := 0
		for _, app := range rts.statusCache.Apps {
			// Ignore sites that have no apps deployed
			if !app.AppName.Valid || !app.AppVersion.Valid {
				continue
			}

			// Start the request in parallel
			go RequestHealth(app.Domain, rts.httpClient, ch)
			requested++
		}

		// Read responses
		rts.statusCache.Health = make([]SiteHealth, requested)
		hasError := false
		for i := 0; i < requested; i++ {
			health := <-ch
			if health.Error != nil {
				hasError = true
				health.ErrorStr = health.Error.Error()
				rts.log.Printf("Error in domain %v: %v\n", health.Domain, health.Error)
			}
			rts.statusCache.Health[i] = health
		}

		if hasError {
			// If there's an error, make the test happen again right away
			appTestedTime = time.Time{}
			statusCode = 503
		}
	}

	return c.Render(statusCode, r.JSON(rts.statusCache))
}
