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
	"bytes"
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

func TestInit(t *testing.T) {
	obj = &Local{}
	err := obj.Init("local:" + dir)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestSet(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		in := openTestFile()
		defer in.Close()
		err := obj.Set("", in, nil)
		if err != ErrNameEmptyInvalid {
			t.Error("Expected ErrNameEmptyInvalid, got", err)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		in := openTestFile()
		defer in.Close()
		err := obj.Set(".metadata.file", in, nil)
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
		if err != ErrNotExist {
			t.Error("Expected ErrNotExist, got", err)
		}
	})
	t.Run("metadata", func(t *testing.T) {
		in := openTestFile()
		defer in.Close()
		err := obj.Set("testphoto2.jpg", in, metadata)
		if err != nil {
			t.Error("Got error", err)
		}
	})
	t.Run("inspect folder", func(t *testing.T) {
		list, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 3 {
			t.Fatal("expected 3 files in temp folder, got", len(list))
		}
		files := make([]string, len(list))
		for i, e := range list {
			files[i] = e.Name()
		}
		if !reflect.DeepEqual(files, []string{".metadata.testphoto2.jpg", "testphoto.jpg", "testphoto2.jpg"}) {
			t.Error("list of files does not match:", files)
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		_, _, err := obj.Get("", nil)
		if err != ErrNameEmptyInvalid {
			t.Error("Expected ErrNameEmptyInvalid, got", err)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		_, _, err := obj.Get(".metadata.file", nil)
		if err != ErrNameEmptyInvalid {
			t.Error("Expected ErrNameEmptyInvalid, got", err)
		}
	})
	t.Run("not existing", func(t *testing.T) {
		found, metadata, err := obj.Get("notexist", nil)
		if err != nil {
			t.Fatal("Expected err to be nil, got", err)
		}
		if metadata != nil || len(metadata) != 0 {
			t.Fatal("Expected metadata to be empty")
		}
		if found {
			t.Fatal("Expected found to be false")
		}
	})
	t.Run("normal", func(t *testing.T) {
		buf := &bytes.Buffer{}
		found, metadata, err := obj.Get("testphoto.jpg", buf)
		if err != nil {
			t.Fatal("Expected err to be nil, got", err)
		}
		if !found {
			t.Fatal("Expected found to be true")
		}
		if metadata != nil || len(metadata) != 0 {
			t.Fatal("Expected metadata to be empty")
		}
		if buf.Len() < 1 {
			t.Fatal("No data returned by the function")
		}
		if calculateDigest(buf.Bytes()) != testFileDigest {
			t.Fatal("Downlaoded file's digest doesn't match")
		}
	})
}
