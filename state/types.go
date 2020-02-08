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

package state

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
	Manifest *AppManifest `json:"-"`
}

// AppOptions is used by the AppManifest struct to represent options for a specific location or file type
type AppOptions struct {
	ClientCaching string            `yaml:"clientCaching"`
	Headers       map[string]string `yaml:"headers"`
	CleanHeaders  map[string]string `yaml:"-"`
}

// AppManifest represents the manifest of an app
type AppManifest struct {
	Files     map[string]AppOptions `yaml:"files"`
	Locations map[string]AppOptions `yaml:"locations"`
	Rewrite   map[string]string     `yaml:"rewrite"`
	Page403   string                `yaml:"page403"`
	Page404   string                `yaml:"page404"`
}

// Internal use

// Interface for the state stores
type stateStore interface {
	Init() error
	AcquireLock() (interface{}, error)
	ReleaseLock(interface{}) error
	GetState() *NodeState
	SetState(*NodeState) error
	WriteState() error
	ReadState() error
	Healthy() (bool, error)
	OnStateUpdate(func())
}
