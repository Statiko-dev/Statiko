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
	"errors"
	"io"
	"strings"

	"github.com/minio/minio-go"
	"github.com/statiko-dev/statiko/appconfig"
)

// S3 stores files on a S3-compatible service
type S3 struct {
	client     *minio.Client
	bucketName string
}

func (f *S3) Init() error {
	// Get the access key
	accessKeyId := appconfig.Config.GetString("repo.s3.accessKeyId")
	secretAccessKey := appconfig.Config.GetString("repo.s3.secretAccessKey")
	if accessKeyId == "" || secretAccessKey == "" {
		return errors.New("repo.s3.accessKeyId and repo.s3.secretAccessKey must be set")
	}

	// Bucket name
	f.bucketName = appconfig.Config.GetString("repo.s3.bucket")
	if f.bucketName == "" {
		return errors.New("repo.s3.bucket must be set")
	}

	// Endpoint; defaults value is "s3.amazonaws.com"
	endpoint := appconfig.Config.GetString("repo.s3.endpoint")
	if endpoint == "" {
		return errors.New("repo.s3.endpoint must be set")
	}

	// Enable TLS
	tls := !appconfig.Config.GetBool("repo.s3.noTLS")

	// Initialize minio client object for connecting to S3
	var err error
	f.client, err = minio.New(endpoint, accessKeyId, secretAccessKey, tls)
	if err != nil {
		return err
	}

	return nil
}

func (f *S3) Get(name string) (found bool, data io.ReadCloser, metadata map[string]string, err error) {
	if name == "" {
		err = ErrNameEmptyInvalid
		return
	}

	// Request the file from S3
	obj, err := f.client.GetObject(f.bucketName, name, minio.GetObjectOptions{})
	if err != nil {
		obj.Close()
		obj = nil
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			found = false
			err = nil
		}
		return
	}

	// Check if the file exists but it's empty
	stat, err := obj.Stat()
	if err != nil || stat.Size == 0 {
		obj.Close()
		obj = nil
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			err = nil
		}
		found = false
		return
	}

	found = true

	// Get metadata
	if stat.Metadata != nil && len(stat.Metadata) > 0 {
		metadata = make(map[string]string)
		for key, val := range stat.Metadata {
			if val != nil && len(val) == 1 {
				if strings.HasPrefix(key, "X-Amz-Meta") {
					metadata[strings.ToLower(key[11:])] = val[0]
				}
			}
		}
	}

	// Set the response stream
	data = obj

	return
}

func (f *S3) Set(name string, in io.Reader, metadata map[string]string) (err error) {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	// Check if the target exists already
	// Expect this to return an error that says NoSuchKey
	_, err = f.client.StatObject(f.bucketName, name, minio.StatObjectOptions{})
	if err == nil {
		return ErrExist
	} else if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		return err
	}

	// Upload the file
	if metadata != nil && len(metadata) == 0 {
		metadata = nil
	}
	_, err = f.client.PutObject(f.bucketName, name, in, -1, minio.PutObjectOptions{
		UserMetadata: metadata,
	})
	if err != nil {
		return err
	}

	return nil
}

func (f *S3) SetMetadata(name string, metadata map[string]string) error {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	if metadata == nil || len(metadata) == 0 {
		metadata = map[string]string{
			// Fix an issue with Minio not being able to delete metadata
			"": "",
		}
	}

	// Create a copy of the object to the same location to add metadata
	src := minio.NewSourceInfo(f.bucketName, name, nil)
	dst, err := minio.NewDestinationInfo(f.bucketName, name, nil, metadata)
	if err != nil {
		return err
	}
	err = f.client.CopyObject(dst, src)
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return ErrNotExist
		}
		return err
	}

	return nil
}

func (f *S3) Delete(name string) (err error) {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	// Check if the object exists
	_, err = f.client.StatObject(f.bucketName, name, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			err = ErrNotExist
		}
		return
	}

	return f.client.RemoveObject(f.bucketName, name)
}
