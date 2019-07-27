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

	"smplatform/appconfig"
	"smplatform/utils"
)

// Manager is the state manager class
type Manager struct {
	updated *time.Time
	store   stateStore
}

// Init loads the state from the store
func (m *Manager) Init() (err error) {
	// Get store type
	typ := appconfig.Config.GetString("state.store")
	switch typ {
	case "file":
		m.store = &stateStoreFS{}
	case "etcd":
		m.store = &stateStoreEtcd{}
	default:
		err = errors.New("Invalid value for configuration `state.store`; valid options are `file` or `etcd`")
		return
	}
	err = m.store.Init()
	return
}

// DumpState exports the entire state
func (m *Manager) DumpState() (*NodeState, error) {
	return m.store.GetState(), nil
}

// ReplaceState replaces the full state for the node with the provided one
func (m *Manager) ReplaceState(state *NodeState) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

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
	if err := m.store.SetState(state); err != nil {
		return err
	}
	m.setUpdated()

	// Write the file to disk
	if err := m.store.WriteState(); err != nil {
		return err
	}

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
	state := m.store.GetState()
	return state.Sites
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *SiteState {
	sites := m.GetSites()
	for _, s := range sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			return &s
		}
	}

	return nil
}

// AddSite adds a site to the store
func (m *Manager) AddSite(site *SiteState) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Reset the error in the site object
	site.Error = nil
	site.ErrorStr = nil

	// Add the site
	state := m.store.GetState()
	state.Sites = append(state.Sites, *site)
	m.setUpdated()

	// Write the file to disk
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// UpdateSite updates a site with the same Domain
func (m *Manager) UpdateSite(site *SiteState, setUpdated bool) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Sync ErrorStr with Error
	if site.Error != nil {
		errorStr := site.Error.Error()
		site.ErrorStr = &errorStr
	} else {
		site.ErrorStr = nil
	}

	// Replace in the memory state
	found := false
	state := m.store.GetState()
	for i, s := range state.Sites {
		if s.Domain == site.Domain {
			// Replace the element
			state.Sites[i] = *site

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

	// Write the file to disk
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// DeleteSite remvoes a site from the store
func (m *Manager) DeleteSite(domain string) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Update the state
	found := false
	state := m.store.GetState()
	for i, s := range state.Sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			// Remove the element
			state.Sites[i] = state.Sites[len(state.Sites)-1]
			state.Sites = state.Sites[:len(state.Sites)-1]

			found = true
			break
		}
	}

	if !found {
		return errors.New("Site not found")
	}

	m.setUpdated()

	// Write the file to disk
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// StoreHealth returns true if the store is healthy
func (m *Manager) StoreHealth() (healthy bool, err error) {
	return m.store.Healthy()
}

// OnStateUpdate stores the callback to invoke if the state is updated because of an external event
func (m *Manager) OnStateUpdate(callback func()) {
	m.store.OnStateUpdate(callback)
}
