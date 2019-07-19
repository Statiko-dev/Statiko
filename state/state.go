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
	// Read the state from disk
	var err error
	m.state, err = readState()
	if err != nil {
		return err
	}

	return nil
}

// DumpState exports the entire state
func (m *Manager) DumpState() (*NodeState, error) {
	// Deep clone the state
	obj := &NodeState{}
	copier.Copy(obj, m.state)

	// Remove all errors
	for i := range obj.Sites {
		if obj.Sites[i].Error != nil {
			obj.Sites[i].Error = nil
		}
		if obj.Sites[i].ErrorStr != nil {
			obj.Sites[i].ErrorStr = nil
		}
	}

	return obj, nil
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

	// Write the file to disk
	obj, err := m.DumpState()
	if err != nil {
		return err
	}
	if err := writeState(obj); err != nil {
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

	// Write the file to disk
	obj, err := m.DumpState()
	if err != nil {
		return err
	}
	if err := writeState(obj); err != nil {
		return err
	}

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

	// Write the file to disk
	obj, err := m.DumpState()
	if err != nil {
		return err
	}
	if err := writeState(obj); err != nil {
		return err
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

	// Write the file to disk
	obj, err := m.DumpState()
	if err != nil {
		return err
	}
	if err := writeState(obj); err != nil {
		return err
	}

	return nil
}
