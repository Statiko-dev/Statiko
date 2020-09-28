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

package appmanager

import (
	"bytes"
	"io/ioutil"
	"os"

	"github.com/google/renameio"

	"github.com/statiko-dev/statiko/utils"
)

// Creates a folder if it doesn't exist already
func ensureFolderWithUpdated(path string) (updated bool, err error) {
	updated = false
	exists := false
	exists, err = utils.FolderExists(path)
	if err != nil {
		return
	}
	if !exists {
		err = utils.EnsureFolder(path)
		if err != nil {
			return
		}
		updated = true
	}
	return
}

// Creates a symbolic link dst pointing to src, if it doesn't exist or if it's pointing to the wrong destination
func createLinkIfNeeded(src string, dst string) (updated bool, err error) {
	err = nil
	updated = false

	// First, check if dst exists
	var exists bool
	exists, err = utils.PathExists(dst)
	if err != nil {
		return
	}

	if exists {
		// Check if the link points to the right place
		var link string
		link, err = os.Readlink(dst)
		if err != nil {
			return
		}

		if link != src {
			updated = true
		} else {
			// Nothing to do
			return
		}
	} else {
		updated = true
	}

	// If we need to create a link
	if updated {
		err = renameio.Symlink(src, dst)
		// No need to return on error; that will happen right away anyways
	}

	return
}

// Writes a file on disk if its content differ from val
// Returns true if the file has been updated
func writeFileIfChanged(filename string, val []byte) (bool, error) {
	// Read the existing file
	read, err := ioutil.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if len(read) > 0 && bytes.Compare(read, val) == 0 {
		// Nothing to do here
		return false, nil
	}

	// Write the updated file
	err = ioutil.WriteFile(filename, val, 0644)
	if err != nil {
		return false, err
	}

	// File has been updated
	return true, nil
}
