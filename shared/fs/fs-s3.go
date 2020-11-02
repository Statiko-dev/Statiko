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

	minio "github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// S3 stores files on a S3-compatible service
type S3 struct {
	client     *minio.Core
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
	// Core is a lower-level API, which is easier for us when requesting data
	minioOpts := &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKeyId, opts.SecretAccessKey, ""),
		Secure: tls,
	}
	var err error
	f.client, err = minio.NewCore(endpoint, minioOpts)
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
	obj, stat, _, err := f.client.GetObject(context.Background(), f.bucketName, name, minio.GetObjectOptions{})
	if err != nil {
		if obj != nil {
			obj.Close()
		}
		// Check if it's a minio error and it's a not found one
		e, ok := err.(minio.ErrorResponse)
		if ok && e.Code == "NoSuchKey" {
			err = nil
			found = false
		}
		return
	}

	// Check if the file exists but it's empty
	if stat.Size == 0 {
		obj.Close()
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
	// Request the list of files
	resp := f.client.Client.ListObjects(ctx, f.bucketName, minio.ListObjectsOptions{})

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
	_, err = f.client.StatObject(ctx, f.bucketName, name, minio.StatObjectOptions{})
	if err == nil {
		return ErrExist
	} else if minio.ToErrorResponse(err).Code != "NoSuchKey" {
		return err
	}

	// Upload the file
	if metadata != nil && len(metadata) == 0 {
		metadata = nil
	}
	_, err = f.client.Client.PutObject(ctx, f.bucketName, name, in, -1, minio.PutObjectOptions{
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
	stat, err := f.client.StatObject(context.Background(), f.bucketName, name, minio.StatObjectOptions{})
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

	// Create a copy of the object to the same location to add metadata
	src := minio.CopySrcOptions{
		Bucket: f.bucketName,
		Object: name,
	}
	dst := minio.CopyDestOptions{
		Bucket:          f.bucketName,
		Object:          name,
		ReplaceMetadata: true,
	}

	// Set metadata
	// If left unset, metadata is removed
	if len(metadata) > 0 {
		dst.UserMetadata = metadata
	}

	_, err := f.client.Client.CopyObject(context.Background(), dst, src)
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
	_, err = f.client.StatObject(context.Background(), f.bucketName, name, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			err = ErrNotExist
		}
		return
	}

	return f.client.RemoveObject(context.Background(), f.bucketName, name, minio.RemoveObjectOptions{})
}
