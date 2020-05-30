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
	"os"
	"reflect"
	"testing"

	"github.com/statiko-dev/statiko/appconfig"
)

func TestAzureStorageInit(t *testing.T) {
	t.Run("empty credentials", func(t *testing.T) {
		o := &AzureStorage{}

		appconfig.Config.Set("repo.azure.account", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.azure.account, but got none")
		}
		appconfig.Config.Set("repo.azure.account", os.Getenv("REPO_AZURE_ACCOUNT"))

		appconfig.Config.Set("repo.azure.container", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.azure.container, but got none")
		}
		appconfig.Config.Set("repo.azure.container", "fs-test")

		// Uses the service principal for authenticating
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &AzureStorage{}
		if err := obj.Init(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAzureStorageSet(t *testing.T) {
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

func TestAzureStorageGet(t *testing.T) {
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

func TestAzureStorageSetMetadata(t *testing.T) {
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

func TestAzureStorageDelete(t *testing.T) {
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
