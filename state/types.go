package state

import (
	"time"
)

// NodeState represents the global state of the node
type NodeState struct {
	Sites []SiteState
}

// SiteState represents the state of a single site
type SiteState struct {
	// Configuration
	ClientCaching  bool    `json:"clientCaching"`
	TLSCertificate *string `json:"tlsCertificate"`

	// Domains: primary and aliases
	Domain  string   `json:"domain"`
	Aliases []string `json:"aliases"`

	// App
	App *SiteApp `json:"app"`
}

// SiteApp represents the state of an app deployed or being deployed
type SiteApp struct {
	// App details
	Name    string     `json:"name"`
	Version string     `json:"version"`
	Time    *time.Time `json:"time"`
}

// Manager is the state manager class
type Manager struct {
	state *NodeState
}
