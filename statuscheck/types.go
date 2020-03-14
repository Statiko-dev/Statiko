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

type healthcheckJob struct {
	domain string
	bundle string
}
