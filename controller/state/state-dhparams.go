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
	"time"

	"github.com/statiko-dev/statiko/shared/defaults"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetDHParams returns the PEM-encoded DH parameters and their date
func (m *Manager) GetDHParams() (string, *time.Time) {
	// Check if we DH parameters; if not, return the default ones
	state := m.store.GetState()
	if state == nil || state.DhParams == nil || state.DhParams.Date == 0 || state.DhParams.Pem == "" {
		return defaults.DefaultDHParams, nil
	}

	// Return the saved DH parameters
	date := time.Unix(state.DhParams.Date, 0)
	return state.DhParams.Pem, &date
}

// SetDHParams stores new PEM-encoded DH parameters
func (m *Manager) SetDHParams(val string) error {
	if val == "" || val == defaults.DefaultDHParams {
		return errors.New("val is empty or invalid")
	}

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

	// Store the value and increase the version
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	now := time.Now()
	state.DhParams = &pb.DHParams{
		Pem:  val,
		Date: now.Unix(),
	}
	state.Version++

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}
