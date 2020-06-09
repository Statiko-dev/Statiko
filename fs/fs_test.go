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
	"time"
)

var (
	dir      string
	obj      Fs
	metadata map[string]string
)

const testFileName = "../.e2e-test/fixtures/simone-mascellari-h1SSRcFuHMk-unsplash.jpg"
const testFileDigest = "29118369f295f81324bbff85e370f8da6c33ea27498733a293d5ce5b361b7ca0"
const testFileSize = 275857

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

func sharedListTest(t *testing.T, obj Fs) func() {
	return func() {
		list, err := obj.List()
		if err != nil {
			t.Fatal(err)
		}
		time5MinsAgo := time.Now().Add(-5 * time.Minute)
		// List can have more elements, we'll just check for the ones we created
		if len(list) < 2 {
			t.Fatalf("List needs to have at least 2 elements, got %d", len(list))
		}
		found := 0
		for _, el := range list {
			// Look for these 2 files
			if el.Name == "testphoto.jpg" || el.Name == "testphoto2.jpg" {
				// Ensure the file was created recently and size is the expected one
				if el.Size != testFileSize {
					t.Errorf("Size for file %s does not match the required one: %d", el.Name, el.Size)
				}
				if !el.LastModified.After(time5MinsAgo) {
					t.Errorf("LastModified for file %s is not within the last 5 minutes: %v", el.Name, el.LastModified)
				}
				found++
				if found == 2 {
					break
				}
			}
		}
		if found != 2 {
			t.Fatal("List does not contain both testphoto.jpg and testphoto2.jpg")
		}
	}
}

func sharedGetMetadataTest(t *testing.T, obj Fs) func() {
	return func() {
		t.Run("empty name", func(t *testing.T) {
			_, err := obj.GetMetadata("")
			if err != ErrNameEmptyInvalid {
				t.Error("Expected ErrNameEmptyInvalid, got", err)
			}
		})
		t.Run("not existing", func(t *testing.T) {
			_, err := obj.GetMetadata("notexist")
			if err != ErrNotExist {
				t.Fatal("Expected ErrNotExist, got", err)
			}
		})
		t.Run("with metadata", func(t *testing.T) {
			res, err := obj.GetMetadata("testphoto2.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res, metadata) {
				t.Fatal("Metadata does not match")
			}
		})
		t.Run("without metadata", func(t *testing.T) {
			res, err := obj.GetMetadata("testphoto.jpg")
			if err != nil {
				t.Fatal(err)
			}
			if res != nil && len(res) != 0 {
				t.Fatalf("Metadata object not empty as expected: %v", res)
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
