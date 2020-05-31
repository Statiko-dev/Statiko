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
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"testing"
)

var (
	dir      string
	obj      Fs
	metadata map[string]string
)

const testFileName = "../.e2e-test/fixtures/simone-mascellari-h1SSRcFuHMk-unsplash.jpg"
const testFileDigest = "cae18d3ba38520dbd850f24b19739651a57e2a8bda4199f039b870173463c420"

// TestMain initializes all tests for this package
func TestMain(m *testing.M) {
	// Temp dir
	var err error
	dir, err = ioutil.TempDir("", "statikotest")
	if err != nil {
		log.Fatal(err)
	}

	// Metadata
	metadata = make(map[string]string)
	metadata["foo"] = "bar"
	metadata["hello"] = "world"

	// Run tests
	rc := m.Run()

	// Cleanup
	err = os.RemoveAll(dir)
	if err != nil {
		log.Fatal(err)
	}

	// Exit
	os.Exit(rc)
}

func openTestFile() io.ReadCloser {
	// Stream to test file
	in, err := os.Open(testFileName)
	if err != nil {
		log.Fatal(err)
	}
	return in
}

func calculateDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func sharedSetTest(t *testing.T, obj Fs) func() {
	return func() {
		t.Run("empty name", func(t *testing.T) {
			in := openTestFile()
			defer in.Close()
			err := obj.Set("", in, nil)
			if err != ErrNameEmptyInvalid {
				t.Error("Expected ErrNameEmptyInvalid, got", err)
			}
		})
		t.Run("normal", func(t *testing.T) {
			in := openTestFile()
			defer in.Close()
			err := obj.Set("testphoto.jpg", in, nil)
			if err != nil {
				t.Error("Got error", err)
			}
		})
		t.Run("file exists", func(t *testing.T) {
			in := openTestFile()
			defer in.Close()
			err := obj.Set("testphoto.jpg", in, nil)
			if err != ErrExist {
				t.Error("Expected ErrExist, got", err)
			}
		})
		t.Run("with metadata", func(t *testing.T) {
			in := openTestFile()
			defer in.Close()
			err := obj.Set("testphoto2.jpg", in, metadata)
			if err != nil {
				t.Error("Got error", err)
			}
		})
	}
}

func sharedGetTest(t *testing.T, obj Fs) func() {
	return func() {
		t.Run("empty name", func(t *testing.T) {
			_, _, _, err := obj.Get("")
			if err != ErrNameEmptyInvalid {
				t.Error("Expected ErrNameEmptyInvalid, got", err)
			}
		})
		t.Run("not existing", func(t *testing.T) {
			found, _, mData, err := obj.Get("notexist")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if mData != nil && len(mData) != 0 {
				t.Fatal("Expected metadata to be empty")
			}
			if found {
				t.Fatal("Expected found to be false")
			}
		})
		t.Run("normal", func(t *testing.T) {
			found, data, mData, err := obj.Get("testphoto.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if !found {
				t.Fatal("Expected found to be true")
			}
			if mData != nil && len(mData) != 0 {
				t.Fatal("Expected metadata to be empty")
			}
			read, err := ioutil.ReadAll(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(read) < 1 {
				t.Fatal("No data returned by the function")
			}
			if calculateDigest(read) != testFileDigest {
				t.Fatal("Downloaded file's digest doesn't match")
			}
		})
		t.Run("with metadata", func(t *testing.T) {
			found, data, mData, err := obj.Get("testphoto2.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if !found {
				t.Fatal("Expected found to be true")
			}
			if mData == nil || len(mData) == 0 {
				t.Fatal("Expected metadata not to be empty")
			}
			if !reflect.DeepEqual(mData, metadata) {
				t.Fatal("Metadata does not match")
			}
			read, err := ioutil.ReadAll(data)
			if err != nil {
				t.Fatal(err)
			}
			if len(read) < 1 {
				t.Fatal("No data returned by the function")
			}
			if calculateDigest(read) != testFileDigest {
				t.Fatal("Downloaded file's digest doesn't match")
			}
		})
	}
}

func sharedSetMetadataTest(t *testing.T, obj Fs) func() {
	return func() {
		setMetadata := map[string]string{
			"foo": "bar",
		}
		t.Run("empty name", func(t *testing.T) {
			err := obj.SetMetadata("", setMetadata)
			if err != ErrNameEmptyInvalid {
				t.Error("Expected ErrNameEmptyInvalid, got", err)
			}
		})
		t.Run("not existing", func(t *testing.T) {
			err := obj.SetMetadata("notexist", setMetadata)
			if err != ErrNotExist {
				t.Fatal("Expected ErrNotExist, got", err)
			}
		})
		t.Run("add metadata", func(t *testing.T) {
			err := obj.SetMetadata("testphoto.jpg", setMetadata)
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
		})
		t.Run("check metadata added", func(t *testing.T) {
			found, _, mData, err := obj.Get("testphoto.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if !found {
				t.Fatal("Expected found to be true")
			}
			if mData == nil || len(mData) == 0 {
				t.Fatal("Expected metadata not to be empty")
			}
			if !reflect.DeepEqual(mData, setMetadata) {
				t.Fatal("Metadata does not match")
			}
		})
		setMetadata["hello"] = "world"
		t.Run("update metadata", func(t *testing.T) {
			err := obj.SetMetadata("testphoto.jpg", setMetadata)
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
		})
		t.Run("check metadata added", func(t *testing.T) {
			found, _, mData, err := obj.Get("testphoto.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if !found {
				t.Fatal("Expected found to be true")
			}
			if mData == nil || len(mData) == 0 {
				t.Fatal("Expected metadata not to be empty")
			}
			if !reflect.DeepEqual(mData, setMetadata) {
				t.Fatal("Metadata does not match")
			}
		})
		t.Run("remove metadata", func(t *testing.T) {
			err := obj.SetMetadata("testphoto.jpg", nil)
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
		})
		t.Run("check metadata removed", func(t *testing.T) {
			found, _, mData, err := obj.Get("testphoto.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
			if !found {
				t.Fatal("Expected found to be true")
			}
			if mData != nil && len(mData) != 0 {
				t.Fatal("Expected metadata to be empty")
			}
		})
	}
}

func sharedDeleteTest(t *testing.T, obj Fs) func() {
	return func() {
		t.Run("empty name", func(t *testing.T) {
			err := obj.Delete("")
			if err != ErrNameEmptyInvalid {
				t.Error("Expected ErrNameEmptyInvalid, got", err)
			}
		})
		t.Run("not existing", func(t *testing.T) {
			err := obj.Delete("notexist")
			if err != ErrNotExist {
				t.Fatal("Expected ErrNotExist, got", err)
			}
		})
		t.Run("normal", func(t *testing.T) {
			err := obj.Delete("testphoto.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
		})
		t.Run("with metadata", func(t *testing.T) {
			err := obj.Delete("testphoto2.jpg")
			if err != nil {
				t.Fatal("Expected err to be nil, got", err)
			}
		})
	}
}
