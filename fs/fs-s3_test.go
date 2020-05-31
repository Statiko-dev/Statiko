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
	"os"
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
	sharedSetTest(t, obj)()
}

func TestS3Get(t *testing.T) {
	sharedGetTest(t, obj)()
}

func TestS3List(t *testing.T) {
	sharedListTest(t, obj)()
}

func TestS3SetMetadata(t *testing.T) {
	sharedSetMetadataTest(t, obj)()
}

func TestS3Delete(t *testing.T) {
	sharedDeleteTest(t, obj)()
}
