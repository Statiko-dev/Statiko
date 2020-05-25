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

package fs

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/utils"
)

// Local is the local file system
type Local struct {
	basePath string
}

func (f *Local) Init() error {
	// Get the path
	path := appconfig.Config.GetString("repo.local.path")
	if path == "" {
		return errors.New("repo.local.path must be set")
	}

	// Get the absolute path
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// Ensure the path ends with a /
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}

	// Lastly, ensure the path exists
	err = utils.EnsureFolder(path)
	if err != nil {
		return err
	}

	f.basePath = path

	return nil
}

func (f *Local) Get(name string) (found bool, data io.ReadCloser, metadata map[string]string, err error) {
	if name == "" || strings.HasPrefix(name, ".metadata.") {
		err = ErrNameEmptyInvalid
		return
	}

	found = true

	// Open the file
	file, err := os.Open(f.basePath + name)
	if err != nil {
		if os.IsNotExist(err) {
			found = false
			err = nil
		}
		return
	}

	// Check if the file has any content
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		file = nil
		return
	}
	if stat.Size() == 0 {
		file.Close()
		file = nil
		found = false
		return
	}

	// Get the metadata
	read, err := ioutil.ReadFile(f.basePath + ".metadata." + name)
	if err != nil {
		if os.IsNotExist(err) {
			read = nil
			err = nil
		} else {
			file.Close()
			file = nil
			return
		}
	}
	if read != nil && len(read) > 0 {
		metadata = make(map[string]string)
		err = json.Unmarshal(read, &metadata)
		if err != nil {
			file.Close()
			file = nil
			return
		}
	}

	// Set the response stream
	data = file

	return
}

func (f *Local) Set(name string, in io.Reader, metadata map[string]string) (err error) {
	if name == "" || strings.HasPrefix(name, ".metadata.") {
		return ErrNameEmptyInvalid
	}

	// Create intermediate folders if needed
	dir := path.Dir(name)
	if dir != "" {
		err = os.MkdirAll(f.basePath+dir, os.ModePerm)
		if err != nil {
			return
		}
	}

	// Check if the file already exists
	exists, err := utils.FileExists(f.basePath + name)
	if err != nil {
		return
	}
	if exists {
		return ErrExist
	}

	// Create the file
	file, err := os.Create(f.basePath + name)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the stream to file
	_, err = io.Copy(file, in)
	if err != nil {
		return err
	}

	// Store metadata
	if metadata != nil && len(metadata) > 0 {
		// Serialize metadata to JSON
		var enc []byte
		enc, err = json.Marshal(metadata)
		if err != nil {
			// Delete the file
			_ = f.Delete(name)
			return
		}

		err = ioutil.WriteFile(f.basePath+".metadata."+name, enc, 0644)
		if err != nil {
			// Delete the file
			_ = f.Delete(name)
			return
		}
	}

	return nil
}

func (f *Local) Delete(name string) error {
	if name == "" || strings.HasPrefix(name, ".metadata.") {
		return ErrNameEmptyInvalid
	}

	// Delete the file and the metadata
	err1 := os.Remove(f.basePath + name)
	err2 := os.Remove(f.basePath + ".metadata." + name)
	if err1 != nil {
		if os.IsNotExist(err1) {
			return ErrNotExist
		}
		return err1
	}
	if err2 != nil && !os.IsNotExist(err2) {
		// Ignore the error when the metadata file doesn't exist
		return err2
	}

	return nil
}
