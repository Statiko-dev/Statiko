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
	ClientCaching         bool    `json:"clientCaching"`
	TLSCertificate        *string `json:"tlsCertificate"`
	TLSCertificateVersion *string `json:"tlsCertificateVersion"`

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
	Name    string     `json:"name" binding:"required"`
	Version string     `json:"version" binding:"required"`
	Time    *time.Time `json:"time" binding:"-"` // Not allowed as input
}
