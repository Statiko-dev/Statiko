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
	time1 := time.Unix(1562821787, 0)
	sites[0] = SiteState{
		ClientCaching:  true,
		TLSCertificate: &tls1,
		Domain:         "site1.local",
		Aliases:        []string{"site1-alias.local", "mysite.local"},
		App: &SiteApp{
			Name:    "app1",
			Version: "1",
			Time:    &time1,
		},
	}
	tls2 := "site2"
	time2 := time.Unix(1562820787, 0)
	sites[1] = SiteState{
		ClientCaching:  false,
		TLSCertificate: &tls2,
		Domain:         "site2.local",
		Aliases:        []string{"site2-alias.local"},
		App: &SiteApp{
			Name:    "app2",
			Version: "1.0.1",
			Time:    &time2,
		},
	}
	tls3 := "site3"
	time3 := time.Unix(1562820887, 0)
	sites[2] = SiteState{
		ClientCaching:  false,
		TLSCertificate: &tls3,
		Domain:         "site3.local",
		Aliases:        []string{"site3-alias.local"},
		App: &SiteApp{
			Name:    "app3",
			Version: "200",
			Time:    &time3,
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
			Time:    &time3,
		},
	}

	m.state = &NodeState{
		Sites: sites,
	}
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
	m.state.Sites = append(m.state.Sites, *site)
	m.setUpdated()
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
	m.setUpdated()

	if !found {
		return errors.New("Site not found")
	}
	return nil
}
