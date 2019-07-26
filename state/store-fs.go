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
	"encoding/json"
	"io/ioutil"

	"github.com/google/renameio"

	"smplatform/appconfig"
	"smplatform/utils"
)

type stateStoreFS struct {
	state *NodeState
}

// Init initializes the object
func (s *stateStoreFS) Init() (err error) {
	// Read the state from disk
	err = s.ReadState()
	return
}

// GetState returns the full state
func (s *stateStoreFS) GetState() *NodeState {
	return s.state
}

// StoreState replaces the current state
func (s *stateStoreFS) SetState(state *NodeState) (err error) {
	s.state = state
	return
}

// WriteState stores the state on disk
func (s *stateStoreFS) WriteState() (err error) {
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
func (s *stateStoreFS) ReadState() (err error) {
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

		// Parse JSON
		s.state = &NodeState{}
		err = json.Unmarshal(data, s.state)
	} else {
		logger.Println("Will create new state file", path)

		// File doesn't exist, so load an empty state
		sites := make([]SiteState, 0)
		s.state = &NodeState{
			Sites: sites,
		}

		// Write the empty state to disk
		err = s.WriteState()
	}

	return
}

// Healthy returns always true
func (s *stateStoreFS) Healthy() (bool, error) {
	return true, nil
}
