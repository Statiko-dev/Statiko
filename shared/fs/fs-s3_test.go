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
	"testing"

	minio "github.com/minio/minio-go/v7"
	"github.com/spf13/viper"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

func TestS3Init(t *testing.T) {
	opts := &pb.ClusterOptions_StoreS3{
		Endpoint: viper.GetString("repo.s3.endpoint"),
		NoTls:    viper.GetBool("repo.s3.noTLS"),
	}

	// Generate a bucket name and get the region
	// NOTE: This key from viper isn't part of the config file
	bucket := "statikotest" + RandString(6)
	region := viper.GetString("s3-region")

	t.Run("empty credentials", func(t *testing.T) {
		o := &S3{}
		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing accessKeyId, but got none")
		}
		opts.AccessKeyId = viper.GetString("repo.s3.accessKeyId")

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing secretAccessKey, but got none")
		}
		opts.SecretAccessKey = viper.GetString("repo.s3.secretAccessKey")

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing bucket, but got none")
		}
		opts.Bucket = bucket
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &S3{}
		if err := obj.Init(opts); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("create test bucket", func(t *testing.T) {
		// Create the bucket
		objS3 := obj.(*S3)
		err := objS3.client.MakeBucket(context.Background(), bucket, minio.MakeBucketOptions{
			Region: region,
		})
		if err != nil {
			t.Fatal("Unexpected error", err)
		}
		t.Log("Created bucket", bucket)
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

func TestS3Cleanup(t *testing.T) {
	objS3 := obj.(*S3)

	// Delete all files first
	objectsCh := objS3.client.Client.ListObjects(context.Background(), objS3.bucketName, minio.ListObjectsOptions{
		Recursive: true,
	})
	deleteCh := objS3.client.Client.RemoveObjects(context.Background(), objS3.bucketName, objectsCh, minio.RemoveObjectsOptions{})
	for e := range deleteCh {
		t.Log("Deleted object", e.ObjectName)
	}

	// Delete the bucket
	err := objS3.client.RemoveBucket(context.Background(), objS3.bucketName)
	if err != nil {
		t.Errorf("error while removing the bucket %s: %s", objS3.bucketName, err)
		return
	}

	t.Log("Deleted bucket", objS3.bucketName)
}
