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
)

// Singleton
var Instance Fs

// Get returns a store for the given type
func Get(typ string) (store Fs, err error) {
	store = nil

	switch typ {
	case "file", "local":
		store = &Local{}
		err = store.Init()
	case "azure", "azureblob":
		store = &AzureStorage{}
		err = store.Init()
	case "s3", "minio":
		store = &S3{}
		err = store.Init()
	default:
		err = fmt.Errorf("invalid store type")
	}

	return
}

// Fs is the interface for the filesystem
type Fs interface {
	// Init the object
	Init() error

	// Get returns a stream to a file in the filesystem
	Get(name string, out io.Writer) (found bool, metadata map[string]string, err error)

	// Set writes a stream to the file in the filesystem
	Set(name string, in io.Reader, metadata map[string]string) (err error)

	// Delete a file from the filesystem
	Delete(name string) (err error)
}

// Errors
var (
	ErrNameEmptyInvalid  = errors.New("name is empty or invalid")
	ErrConnStringInvalid = errors.New("invalid connection string")
	ErrNotExist          = errors.New("file already exists")
)
