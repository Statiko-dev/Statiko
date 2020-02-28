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

package utils

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// SiteHealth contains the results of the health checks for each individual app
type SiteHealth struct {
	Domain       string     `json:"domain"`
	App          *string    `json:"app"`
	StatusCode   *int       `json:"status,omitempty"`
	ResponseSize *int       `json:"size,omitempty"`
	Error        error      `json:"-"`
	ErrorStr     *string    `json:"error,omitempty"`
	Time         *time.Time `json:"time,omitempty"`
}

// NodeSync contains information on the sync status
type NodeSync struct {
	Running   bool       `json:"running"`
	LastSync  *time.Time `json:"lastSync"`
	SyncError string     `json:"syncError,omitempty"`
}

// NginxStatus contains information on the status of the Nginx server
type NginxStatus struct {
	Running bool `json:"running"`
}

// NodeStore contains information on the status of the store
// Particularly useful if using etcd as store
type NodeStore struct {
	Healthy bool `json:"healthy"`
}

// NodeStatus contains the current status of the node
type NodeStatus struct {
	Nginx  NginxStatus  `json:"nginx"`
	Sync   NodeSync     `json:"sync"`
	Store  NodeStore    `json:"store"`
	Health []SiteHealth `json:"health"`
}

// RequestHealth makes a request to the app and checks its health (status code 2xx)
func RequestHealth(domain string, app string, ch chan<- SiteHealth) {
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
		ch <- SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        err,
		}
		return
	}

	// Check if status code is 2xx
	statusCode = resp.StatusCode
	if statusCode < 200 || statusCode > 299 {
		ch <- SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        fmt.Errorf("Invalid status code: %d", resp.StatusCode),
		}
		return
	}

	// Read the response body to calculate the size
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ch <- SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        err,
		}
		return
	}
	responseSize = len(bodyBytes)
	if responseSize < 1 {
		ch <- SiteHealth{
			Domain:       domain,
			App:          &app,
			StatusCode:   &statusCode,
			ResponseSize: &responseSize,
			Time:         &now,
			Error:        fmt.Errorf("Invalid response size: %d", responseSize),
		}
		return
	}

	// Success!
	ch <- SiteHealth{
		Domain:       domain,
		App:          &app,
		StatusCode:   &statusCode,
		ResponseSize: &responseSize,
		Time:         &now,
		Error:        nil,
	}
}
