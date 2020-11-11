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
	"errors"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

// GetSites returns the list of all sites
func (m *Manager) GetSites() []*pb.Site {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	return state.Sites
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *pb.Site {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	return state.GetSite(domain)
}

// AddSite adds a site to the store
func (m *Manager) AddSite(site *pb.Site) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Get the current state
	state := m.store.GetState()

	// Ensure that the TLS certificates referenced exist
	if site.GeneratedTlsId != "" {
		if _, ok := state.Certificates[site.GeneratedTlsId]; !ok {
			return errors.New("the site references a generated TLS certificate that doesn't exist")
		}
	}
	if site.ImportedTlsId != "" {
		if _, ok := state.Certificates[site.ImportedTlsId]; !ok {
			return errors.New("the site references an imported TLS certificate that doesn't exist")
		}
	}

	// Add the site and increase the version
	state.Sites = append(state.Sites, site)
	state.Version++
	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// UpdateSite updates a site with the same Domain
func (m *Manager) UpdateSite(site *pb.Site) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Get the current state
	state := m.store.GetState()

	// Ensure that the TLS certificates referenced exist
	if site.GeneratedTlsId != "" {
		if _, ok := state.Certificates[site.GeneratedTlsId]; !ok {
			return errors.New("the site references a generated TLS certificate that doesn't exist")
		}
	}
	if site.ImportedTlsId != "" {
		if _, ok := state.Certificates[site.ImportedTlsId]; !ok {
			return errors.New("the site references an imported TLS certificate that doesn't exist")
		}
	}

	// Replace in the memory state
	found := false
	for i, s := range state.Sites {
		if s.Domain == site.Domain {
			// If the generated TLS certificate has changed, remove the old one
			if s.GeneratedTlsId != site.GeneratedTlsId {
				cert := state.Certificates[s.GeneratedTlsId]
				if cert != nil && (cert.Type == pb.TLSCertificate_SELF_SIGNED || cert.Type == pb.TLSCertificate_ACME) {
					delete(state.Certificates, s.GeneratedTlsId)
				}
			}

			// Replace the element
			state.Sites[i] = site

			// Increase the version since we made a change
			state.Version++

			found = true
			break
		}
	}

	if !found {
		return errors.New("site not found")
	}

	m.setUpdated()

	// Commit the state to the store
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

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Update the state
	found := false
	state := m.store.GetState()
	for i, s := range state.Sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			// Remove all generated certificates too
			if s.GeneratedTlsId != "" {
				cert := state.Certificates[s.GeneratedTlsId]
				if cert != nil && (cert.Type == pb.TLSCertificate_SELF_SIGNED || cert.Type == pb.TLSCertificate_ACME) {
					delete(state.Certificates, s.GeneratedTlsId)
				}
			}

			// Remove the element
			state.Sites[i] = state.Sites[len(state.Sites)-1]
			state.Sites = state.Sites[:len(state.Sites)-1]

			found = true
			break
		}
	}

	// If we made a change, save the state
	if found {
		// Increase the version and mark as updated
		state.Version++
		m.setUpdated()

		// Commit the state to the store
		if err := m.store.WriteState(); err != nil {
			return err
		}
	}

	return nil
}
