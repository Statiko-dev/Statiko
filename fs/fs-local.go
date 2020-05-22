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
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"

	"github.com/statiko-dev/statiko/utils"
)

// Local is the local file system
type Local struct {
	basePath string
}

func (f *Local) Init(connection string) error {
	// Ensure that connection starts with "local:" or "file:"
	if !strings.HasPrefix(connection, "local:") && !strings.HasPrefix(connection, "file:") {
		return fmt.Errorf("invalid scheme")
	}

	// Get the path
	path := connection[strings.Index(connection, ":")+1:]

	// Expand the tilde if needed
	path, err := homedir.Expand(path)
	if err != nil {
		return err
	}

	// Get the absolute path
	path, err = filepath.Abs(path)
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

func (f *Local) Get(name string, out io.Writer) (found bool, err error) {
	if name == "" {
		err = errors.New("name is empty")
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
	defer file.Close()

	// Check if the file has any content
	stat, err := file.Stat()
	if err != nil {
		return
	}
	if stat.Size() == 0 {
		found = false
		return
	}

	// Copy the file to the out stream
	_, err = io.Copy(out, file)
	if err != nil {
		return
	}

	return
}

func (f *Local) Set(name string, in io.Reader) (err error) {
	if name == "" {
		return errors.New("name is empty")
	}

	// Create intermediate folders if needed
	dir := path.Dir(name)
	if dir != "" {
		err = os.MkdirAll(f.basePath+dir, os.ModePerm)
		if err != nil {
			return
		}
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

	return nil
}

func (f *Local) Delete(name string) (err error) {
	if name == "" {
		return errors.New("name is empty")
	}

	// Delete the file
	return os.Remove(f.basePath + name)
}
