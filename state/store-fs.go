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

// Store the state on disk
func writeState(state *NodeState) error {
	path := appconfig.Config.GetString("store")
	logger.Println("Writing state to disk", path)

	// Convert to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write to disk
	if err := renameio.WriteFile(path, data, 0644); err != nil {
		return err
	}

	return nil
}

// Read the state from disk
func readState() (state *NodeState, err error) {
	path := appconfig.Config.GetString("store")
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
		state = &NodeState{}
		err = json.Unmarshal(data, state)
	} else {
		logger.Println("Will create new state file", path)

		// File doesn't exist, so load an empty state
		sites := make([]SiteState, 0)
		state = &NodeState{
			Sites: sites,
		}

		// Write the empty state to disk
		err = writeState(state)
	}

	return
}
