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
	"encoding/json"
	"io/ioutil"

	"github.com/google/renameio"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/utils"
)

type StateStoreFile struct {
	state *NodeState
}

// Init initializes the object
func (s *StateStoreFile) Init() (err error) {
	// Read the state from disk
	err = s.ReadState()
	return
}

// AcquireLock acquires a lock, with an optional timeout
func (s *StateStoreFile) AcquireLock(name string, timeout bool) (interface{}, error) {
	// When storing state in a file, we're operating in single-node mode
	return nil, nil
}

// ReleaseLock releases a lock
func (s *StateStoreFile) ReleaseLock(leaseID interface{}) error {
	// When storing state in a file, we're operating in single-node mode
	return nil
}

// GetState returns the full state
func (s *StateStoreFile) GetState() *NodeState {
	return s.state
}

// StoreState replaces the current state
func (s *StateStoreFile) SetState(state *NodeState) (err error) {
	s.state = state
	return
}

// WriteState stores the state on disk
func (s *StateStoreFile) WriteState() (err error) {
	path := appconfig.Config.GetString("state.file.path")
	logger.Println("Writing state to disk", path)

	// Convert to JSON
	var data []byte
	data, err = json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return
	}

	// Write to disk
	err = renameio.WriteFile(path, data, 0644)
	return
}

// ReadState reads the state from disk
func (s *StateStoreFile) ReadState() (err error) {
	path := appconfig.Config.GetString("state.file.path")
	logger.Println("Reading state from disk", path)

	// Check if the file exists
	var exists bool
	exists, err = utils.PathExists(path)
	if err != nil {
		return
	}

	if exists {
		// Read from disk
		var data []byte
		data, err = ioutil.ReadFile(path)
		if err != nil {
			return
		}

		// File exists, but it's empty
		if len(data) == 0 {
			s.createStateFile(path)
		} else {
			// Parse JSON
			s.state = &NodeState{}
			err = json.Unmarshal(data, s.state)
		}
	} else {
		s.createStateFile(path)
	}

	return
}

// Healthy returns always true
func (s *StateStoreFile) Healthy() (bool, error) {
	return true, nil
}

// OnStateUpdate isn't used with this store
func (s *StateStoreFile) OnStateUpdate(callback func()) {
	// NOOP
}

// ClusterHealth returns the health of all members in the cluster
func (s *StateStoreFile) ClusterHealth() (map[string]NodeHealth, error) {
	// Sites: convert errors to strings
	health := Instance.GetAllSiteHealth()
	sites := make(map[string]string, len(health))
	for k, v := range health {
		if v != nil {
			sites[k] = v.Error()
		}
	}

	// Response
	res := make(map[string]NodeHealth, 1)
	res["0"] = NodeHealth{
		NodeName: appconfig.Config.GetString("nodeName"),
		Sites:    sites,
	}

	return res, nil
}

// StoreNodeHealth isn't used with this store
func (s *StateStoreFile) StoreNodeHealth(health *utils.NodeStatus) error {
	return nil
}

// Create a new state file
func (s *StateStoreFile) createStateFile(path string) (err error) {
	logger.Println("Will create new state file", path)

	// File doesn't exist, so load an empty state
	sites := make([]SiteState, 0)
	s.state = &NodeState{
		Sites: sites,
	}

	// Write the empty state to disk
	err = s.WriteState()

	return
}
