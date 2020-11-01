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
	"errors"
	"io"
	"strings"

	"github.com/minio/minio-go"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// S3 stores files on a S3-compatible service
type S3 struct {
	client     *minio.Client
	bucketName string
}

func (f *S3) Init(optsI interface{}) error {
	// Cast opts to pb.ClusterOptions_StorageS3
	opts, ok := optsI.(*pb.ClusterOptions_StorageS3)
	if !ok || opts == nil {
		return errors.New("invalid options object")
	}

	// Get the access key
	if opts.AccessKeyId == "" || opts.SecretAccessKey == "" {
		return errors.New("options `accessKeyId` and `secretAccessKey` must be set")
	}

	// Bucket name
	f.bucketName = opts.GetBucket()
	if f.bucketName == "" {
		return errors.New("option `bucket` must be set")
	}

	// Endpoint; default value is "s3.amazonaws.com"
	endpoint := opts.GetEndpoint()
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	// Enable TLS
	tls := !opts.NoTls

	// Initialize minio client object for connecting to S3
	var err error
	f.client, err = minio.New(endpoint, opts.AccessKeyId, opts.SecretAccessKey, tls)
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

func (f *S3) List() ([]FileInfo, error) {
	return f.ListWithContext(context.Background())
}

func (f *S3) ListWithContext(ctx context.Context) ([]FileInfo, error) {
	// Channel to cancel the operation
	doneCh := make(chan struct{})
	doneSent := false
	defer func() {
		if !doneSent {
			doneCh <- struct{}{}
		}
		close(doneCh)
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				doneCh <- struct{}{}
				doneSent = true
				return
			case <-doneCh:
				return
			}
		}
	}()

	// Request the list of files
	resp := f.client.ListObjectsV2(f.bucketName, "", false, doneCh)

	// Iterate through the response
	list := make([]FileInfo, 0)
	for msg := range resp {
		if msg.Err != nil {
			return nil, msg.Err
		}
		list = append(list, FileInfo{
			Name:         msg.Key,
			Size:         msg.Size,
			LastModified: msg.LastModified,
		})
	}

	return list, nil
}

func (f *S3) Set(name string, in io.Reader, metadata map[string]string) (err error) {
	return f.SetWithContext(context.Background(), name, in, metadata)
}

func (f *S3) SetWithContext(ctx context.Context, name string, in io.Reader, metadata map[string]string) (err error) {
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
	_, err = f.client.PutObjectWithContext(ctx, f.bucketName, name, in, -1, minio.PutObjectOptions{
		UserMetadata: metadata,
	})
	if err != nil {
		return err
	}

	return nil
}

func (f *S3) GetMetadata(name string) (metadata map[string]string, err error) {
	if name == "" {
		return nil, ErrNameEmptyInvalid
	}

	// Request the metadata from S3
	stat, err := f.client.StatObject(f.bucketName, name, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			err = ErrNotExist
		}
		return nil, err
	}

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

	return metadata, nil
}

func (f *S3) SetMetadata(name string, metadata map[string]string) error {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	if metadata == nil || len(metadata) == 0 {
		metadata = map[string]string{
			// Fix an issue with Minio not being able to delete metadata
			// See: https://github.com/minio/minio-go/issues/1295
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
