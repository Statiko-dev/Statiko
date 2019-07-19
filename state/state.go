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

import (
	"errors"
	"time"

	"github.com/jinzhu/copier"

	"smplatform/utils"
)

// Manager is the state manager class
type Manager struct {
	state   *NodeState
	updated *time.Time
}

// Init loads the state from the store
func (m *Manager) Init() error {
	// Debug: use a pre-defined state
	sites := make([]SiteState, 4)
	tls1 := "site1"
	tls1Version := "72cd150c5f394bd190749cdb22d0f731"
	sites[0] = SiteState{
		ClientCaching:         true,
		TLSCertificate:        &tls1,
		TLSCertificateVersion: &tls1Version,
		Domain:                "site1.local",
		Aliases:               []string{"site1-alias.local", "mysite.local"},
		App: &SiteApp{
			Name:    "app1",
			Version: "1",
		},
	}
	tls2 := "site2"
	tls2Version := "5b66a6296e894c2fa6a28af196d10bf6"
	sites[1] = SiteState{
		ClientCaching:         false,
		TLSCertificate:        &tls2,
		TLSCertificateVersion: &tls2Version,
		Domain:                "site2.local",
		Aliases:               []string{"site2-alias.local"},
		App: &SiteApp{
			Name:    "app2",
			Version: "1.0.1",
		},
	}
	tls3 := "site3"
	tls3Version := "8b164f4577244c4aa8eb54a31b45c70c"
	sites[2] = SiteState{
		ClientCaching:         false,
		TLSCertificate:        &tls3,
		TLSCertificateVersion: &tls3Version,
		Domain:                "site3.local",
		Aliases:               []string{"site3-alias.local"},
		App: &SiteApp{
			Name:    "app3",
			Version: "200",
		},
	}
	sites[3] = SiteState{
		ClientCaching:  false,
		TLSCertificate: &tls3,
		Domain:         "site4.local",
		Aliases:        nil,
		App: &SiteApp{
			Name:    "app2",
			Version: "1.2.0",
		},
	}

	m.ReplaceState(&NodeState{
		Sites: sites,
	})

	return nil
}

// DumpState exports the entire state
func (m *Manager) DumpState() (*NodeState, error) {
	// Deep clone the state
	var obj NodeState
	copier.Copy(&obj, m.state)

	// Remove all errors
	for _, s := range obj.Sites {
		if s.Error != nil {
			s.Error = nil
		}
		if s.ErrorStr != nil {
			s.ErrorStr = nil
		}
	}

	return &obj, nil
}

// ReplaceState replaces the full state for the node with the provided one
func (m *Manager) ReplaceState(state *NodeState) error {
	// Ensure that errors aren't included
	for _, s := range state.Sites {
		if s.Error != nil {
			s.Error = nil
		}
		if s.ErrorStr != nil {
			s.ErrorStr = nil
		}
	}

	// Replace the state
	m.state = state
	m.setUpdated()

	return nil
}

// setUpdated sets the updated time in the object
func (m *Manager) setUpdated() {
	now := time.Now()
	m.updated = &now
}

// LastUpdated returns the time the state was updated last
func (m *Manager) LastUpdated() *time.Time {
	return m.updated
}

// GetSites returns the list of all sites
func (m *Manager) GetSites() []SiteState {
	return m.state.Sites
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *SiteState {
	for _, s := range m.state.Sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			return &s
		}
	}

	return nil
}

// AddSite adds a site to the store
func (m *Manager) AddSite(site *SiteState) error {
	// Reset the error
	site.Error = nil
	site.ErrorStr = nil

	// Add the site
	m.state.Sites = append(m.state.Sites, *site)
	m.setUpdated()

	return nil
}

// UpdateSite updates a site with the same Domain
func (m *Manager) UpdateSite(site *SiteState, setUpdated bool) error {
	// Sync ErrorStr with Error
	if site.Error != nil {
		errorStr := site.Error.Error()
		site.ErrorStr = &errorStr
	} else {
		site.ErrorStr = nil
	}

	// Replace in the memory state
	found := false
	for i, s := range m.state.Sites {
		if s.Domain == site.Domain {
			// Replace the element
			m.state.Sites[i] = *site

			found = true
			break
		}
	}

	if !found {
		return errors.New("Site not found")
	}

	// Check if we need to set the object as updated
	if setUpdated {
		m.setUpdated()
	}

	return nil
}

// DeleteSite remvoes a site from the store
func (m *Manager) DeleteSite(domain string) error {
	found := false
	for i, s := range m.state.Sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			// Remove the element
			m.state.Sites[i] = m.state.Sites[len(m.state.Sites)-1]
			m.state.Sites = m.state.Sites[:len(m.state.Sites)-1]

			found = true
			break
		}
	}

	if !found {
		return errors.New("Site not found")
	}

	m.setUpdated()

	return nil
}
