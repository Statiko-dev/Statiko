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

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

func TestS3Init(t *testing.T) {
	opts := &pb.ClusterOptions_StorageS3{
		Endpoint: os.Getenv("REPO_S3_ENDPOINT"),
		NoTls:    utils.IsTruthy(os.Getenv("REPO_S3_NO_TLS")),
	}
	t.Run("empty credentials", func(t *testing.T) {
		o := &S3{}
		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing accessKeyId, but got none")
		}
		opts.AccessKeyId = os.Getenv("REPO_S3_ACCESS_KEY_ID")

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing secretAccessKey, but got none")
		}
		opts.SecretAccessKey = os.Getenv("REPO_S3_SECRET_ACCESS_KEY")

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing bucket, but got none")
		}
		opts.Bucket = os.Getenv("REPO_S3_BUCKET")
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &S3{}
		if err := obj.Init(opts); err != nil {
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

func TestS3GetMetadata(t *testing.T) {
	sharedGetMetadataTest(t, obj)()
}

func TestS3SetMetadata(t *testing.T) {
	sharedSetMetadataTest(t, obj)()
}

func TestS3Delete(t *testing.T) {
	sharedDeleteTest(t, obj)()
}
