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

package state

import (
	"time"

	"github.com/statiko-dev/statiko/utils"
)

const (
	TLSCertificateImported    = "imported"
	TLSCertificateSelfSigned  = "selfsigned"
	TLSCertificateLetsEncrypt = "letsencrypt"
)

// NodeState represents the global state of the node
type NodeState struct {
	Sites    []SiteState       `json:"sites"`
	Secrets  map[string][]byte `json:"secrets,omitempty"`
	DHParams *NodeDHParams     `json:"dhparams,omitempty"`
}

// SiteState represents the state of a single site
type SiteState struct {
	// Domains: primary and aliases
	Domain  string   `json:"domain" binding:"required,ne=_default"`
	Aliases []string `json:"aliases" binding:"dive,ne=_default"`

	// TLS configuration
	TLS *SiteTLS `json:"tls"`

	// App
	App *SiteApp `json:"app"`
}

// SiteTLS represents the TLS configuration for the site
type SiteTLS struct {
	Type        string  `json:"type"`
	Certificate *string `json:"cert,omitempty"`
	Version     *string `json:"ver,omitempty"`
}

// SiteApp represents the state of an app deployed or being deployed
type SiteApp struct {
	// App details
	Name    string `json:"name" binding:"required"`
	Version string `json:"version" binding:"required"`

	// App manifest (for internal use)
	Manifest *utils.AppManifest `json:"-"`
}

// NodeDHParams represents the DH Parameters file (PEM-encoded) and their age
type NodeDHParams struct {
	Date *time.Time `json:"time"`
	PEM  string     `json:"pem"`
}

// SiteHealth represents the health of each site in the node
type SiteHealth map[string]error

// NodeHealth contains information and health for a node in the cluster
type NodeHealth struct {
	NodeName string            `json:"node"`
	Sites    map[string]string `json:"sites"`
}

// WorkerController is the interface for the controller
type WorkerController interface {
	Init(store StateStore)
	IsLeader() bool
	AddJob(job utils.JobData) (string, error)
	CompleteJob(jobID string) error
	WaitForJob(jobID string, ch chan error)
}

// StateStore is the interface for the state stores
type StateStore interface {
	Init() error
	AcquireLock(name string, timeout bool) (interface{}, error)
	ReleaseLock(leaseID interface{}) error
	GetState() *NodeState
	SetState(*NodeState) error
	WriteState() error
	ReadState() error
	Healthy() (bool, error)
	OnStateUpdate(func())
	ClusterHealth() (map[string]NodeHealth, error)
	StoreNodeHealth(health SiteHealth) error
}
