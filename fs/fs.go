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
	"strings"
)

// Singleton
var Instance Fs

// Get returns a store for the given connection string
func Get(connection string) (store Fs, err error) {
	store = nil

	// Get the name of the store
	pos := strings.Index(connection, ":")
	if pos < 1 {
		err = fmt.Errorf("invalid connection string")
		return
	}

	switch connection[0:pos] {
	case "file", "local":
		store = &Local{}
		err = store.Init(connection)
	case "azure", "azureblob":
		store = &AzureStorage{}
		err = store.Init(connection)
	case "s3", "minio":
		store = &S3{}
		err = store.Init(connection)
	default:
		err = fmt.Errorf("invalid connection string")
	}

	return
}

// Fs is the interface for the filesystem
type Fs interface {
	// Init the object, by passing a connection string
	Init(connection string) error

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
