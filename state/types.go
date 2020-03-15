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

import "github.com/ItalyPaleAle/statiko/utils"

// NodeState represents the global state of the node
type NodeState struct {
	Sites   []SiteState       `json:"sites"`
	Secrets map[string][]byte `json:"secrets"`
}

// SiteState represents the state of a single site
type SiteState struct {
	// Configuration
	TLSCertificateSelfSigned bool    `json:"tlsCertificateSelfSigned"`
	TLSCertificate           *string `json:"tlsCertificate"`
	TLSCertificateVersion    *string `json:"tlsCertificateVersion"`

	// Domains: primary and aliases
	Domain  string   `json:"domain" binding:"required,ne=_default"`
	Aliases []string `json:"aliases" binding:"dive,ne=_default"`

	// Deployment error
	Error    error   `json:"-"`
	ErrorStr *string `json:"error" binding:"-"` // Not allowed as input

	// App
	App *SiteApp `json:"app"`
}

// SiteApp represents the state of an app deployed or being deployed
type SiteApp struct {
	// App details
	Name    string `json:"name" binding:"required"`
	Version string `json:"version" binding:"required"`

	// App manifest (for internal use)
	Manifest *utils.AppManifest `json:"-"`
}

// Internal use

// Interface for the state stores
type stateStore interface {
	Init() error
	AcquireStateLock() (interface{}, error)
	ReleaseStateLock(interface{}) error
	AcquireSyncLock() (interface{}, error)
	ReleaseSyncLock(interface{}) error
	GetState() *NodeState
	SetState(*NodeState) error
	WriteState() error
	ReadState() error
	Healthy() (bool, error)
	OnStateUpdate(func())
}
