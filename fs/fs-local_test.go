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
	"io/ioutil"
	"reflect"
	"testing"

	"github.com/statiko-dev/statiko/appconfig"
)

func TestLocalInit(t *testing.T) {
	t.Run("empty path", func(t *testing.T) {
		o := &Local{}
		appconfig.Config.Set("repo.local.path", "")
		err := o.Init()
		if err == nil {
			t.Fatal("Expected error, but got none")
		}
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &Local{}
		appconfig.Config.Set("repo.local.path", dir)
		err := obj.Init()
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestLocalSet(t *testing.T) {
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

func TestLocalGet(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		_, _, _, err := obj.Get("")
		if err != ErrNameEmptyInvalid {
			t.Error("Expected ErrNameEmptyInvalid, got", err)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		_, _, _, err := obj.Get(".metadata.file")
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

func TestLocalDelete(t *testing.T) {
	t.Run("empty name", func(t *testing.T) {
		err := obj.Delete("")
		if err != ErrNameEmptyInvalid {
			t.Error("Expected ErrNameEmptyInvalid, got", err)
		}
	})
	t.Run("invalid name", func(t *testing.T) {
		err := obj.Delete(".metadata.file")
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
