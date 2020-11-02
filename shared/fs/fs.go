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
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// Get returns a store for the given type
func Get(typ string, opts interface{}) (store Fs, err error) {
	store = nil

	switch typ {
	case "file", "local":
		store = &Local{}
		err = store.Init(opts)
	case "azure", "azureblob":
		store = &AzureStorage{}
		err = store.Init(opts)
	case "s3", "minio":
		store = &S3{}
		err = store.Init(opts)
	default:
		err = fmt.Errorf("invalid repo type")
	}

	return
}

// Fs is the interface for the filesystem
type Fs interface {
	// Init the object
	Init(opts interface{}) error

	// Get returns a stream to a file in the filesystem
	Get(name string) (found bool, data io.ReadCloser, metadata map[string]string, err error)

	// TODO: GetWithContext

	// List returns the list of files in the filesystem
	List() ([]FileInfo, error)

	// ListWithContext is like List, but accepts a custom context object
	ListWithContext(ctx context.Context) ([]FileInfo, error)

	// Set writes a stream to the file in the filesystem
	Set(name string, in io.Reader, metadata map[string]string) (err error)

	// SetWithContext is like Set, but accepts a custom context object
	SetWithContext(ctx context.Context, name string, in io.Reader, metadata map[string]string) (err error)

	// GetMetadata returns the metadata for the file only
	GetMetadata(name string) (metadata map[string]string, err error)

	// TODO: GetMetadataWithContext

	// SetMetadata updates a file's metadata in the filesystem
	SetMetadata(name string, metadata map[string]string) error

	// TODO: SetMetadataWithContext

	// Delete a file from the filesystem
	Delete(name string) (err error)

	// TODO: DeleteWithContext
}

// FileInfo object returned by the List methods
type FileInfo struct {
	Name         string    `json:"name"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"lastModified"`
}

// Errors
var (
	ErrNameEmptyInvalid  = errors.New("name is empty or invalid")
	ErrConnStringInvalid = errors.New("invalid connection string")
	ErrExist             = errors.New("file already exists")
	ErrNotExist          = errors.New("file does not exist")
)
