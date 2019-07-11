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

package utils

import (
	"archive/tar"
	"compress/bzip2"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gofrs/uuid"
)

// RequestJSON fetches a JSON document from the web
func RequestJSON(client *http.Client, url string, target interface{}) error {
	var err error

	// Request the file
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 399 {
		b, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	defer resp.Body.Close()

	// Decode the JSON into the target
	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		return err
	}
	return nil
}

/*// KeysOfMap returns the list of keys of a map
func KeysOfMap(m map[string]interface{}) (keys []string) {
	keys = make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	return
}*/

// RemoveContents remove all contents within a directory
// Source: https://stackoverflow.com/a/33451503/192024
func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

// CopyFile copies the src file to dst. Any existing file will be overwritten and will not copy file attributes.
// Source: https://stackoverflow.com/a/21061062/192024
func CopyFile(src string, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// PathExists returns true if the path exists on disk
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// EnsureFolder creates a folder if it doesn't exist already
func EnsureFolder(path string) error {
	exists, err := PathExists(path)
	if err != nil {
		return err
	} else if !exists {
		if err := os.Mkdir(path, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

// SymlinkAtomic creates a symbolic link atomically, replacing existing ones if any
func SymlinkAtomic(src string, dest string) error {
	// First, create a temporary symlink, then rename it, so the entire operation is atomic
	tmpLink := dest + "_" + RandString(6)
	if err := os.Symlink(src, tmpLink); err != nil {
		return err
	}
	if err := os.Rename(tmpLink, dest); err != nil {
		return err
	}
	return nil
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandString returns a random string of n bytes
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

// UntarBZ2 takes a destination path and a reader; a tar reader loops over the tarfile
// creating the file structure at 'dst' along the way, and writing any files
// Source: https://medium.com/@skdomino/taring-untaring-files-in-go-6b07cf56bc07
func UntarBZ2(dst string, r io.Reader) error {
	bzr := bzip2.NewReader(r)

	tr := tar.NewReader(bzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dst, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

// IsValidUUID returns true if the string is a valid UUID (any version)
func IsValidUUID(s string) bool {
	_, err := uuid.FromString(s)
	return err == nil
}
