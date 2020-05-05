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
	"encoding/json"
	"time"
)

// SiteHealth contains the results of the health checks for each individual app
type SiteHealth struct {
	Domain       string     `json:"domain"`
	App          *string    `json:"app"`
	StatusCode   *int       `json:"-"`
	ResponseSize *int       `json:"-"`
	Error        error      `json:"-"`
	Time         *time.Time `json:"time,omitempty"`
}

// IsHealthy returns true if the site is in a healthy state
func (h *SiteHealth) IsHealthy() bool {
	// If there's an error, it's unhealthy by default
	if h.Error != nil {
		return false
	}

	// If there's no app, it's healthy by default
	if h.App == nil {
		return true
	}

	// Check response status code and size
	if h.StatusCode != nil && *h.StatusCode >= 200 && *h.StatusCode < 300 &&
		h.ResponseSize != nil && *h.ResponseSize > 0 {
		return true
	}

	// Otherwise, false
	return false
}

// MarshalJSON implements a custom JSON serializer for the SiteHealth object
func (h *SiteHealth) MarshalJSON() ([]byte, error) {
	// Error string - if any
	errorStr := ""
	if h.Error != nil {
		errorStr = h.Error.Error()
	}

	// Marshal the JSON object
	type Alias SiteHealth
	return json.Marshal(&struct {
		*Alias
		Healthy  bool   `json:"healthy"`
		ErrorStr string `json:"error,omitempty"`
	}{
		Healthy:  h.IsHealthy(),
		ErrorStr: errorStr,
		Alias:    (*Alias)(h),
	})
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

type healthcheckJob struct {
	domain string
	bundle string
}
