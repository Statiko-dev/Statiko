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

func TestS3Init(t *testing.T) {
	t.Run("empty credentials", func(t *testing.T) {
		o := &S3{}

		appconfig.Config.Set("repo.s3.accessKeyId", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.s3.accessKeyId, but got none")
		}
		appconfig.Config.Set("repo.s3.accessKeyId", os.Getenv("REPO_S3_ACCESS_KEY_ID"))

		appconfig.Config.Set("repo.s3.secretAccessKey", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.s3.secretAccessKey, but got none")
		}
		appconfig.Config.Set("repo.s3.secretAccessKey", os.Getenv("REPO_S3_SECRET_ACCESS_KEY"))

		appconfig.Config.Set("repo.s3.bucket", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.s3.bucket, but got none")
		}
		appconfig.Config.Set("repo.s3.bucket", os.Getenv("REPO_S3_BUCKET"))

		appconfig.Config.Set("repo.s3.endpoint", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.s3.endpoint, but got none")
		}
		appconfig.Config.Set("repo.s3.endpoint", os.Getenv("REPO_S3_ENDPOINT"))
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &S3{}
		if err := obj.Init(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestS3Set(t *testing.T) {
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

func TestS3Get(t *testing.T) {
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

func TestS3Delete(t *testing.T) {
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
