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
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/minio/minio-go"
)

// S3 stores files on a S3-compatible service
type S3 struct {
	client     *minio.Client
	bucketName string
}

func (f *S3) Init(connection string) error {
	// Ensure the connection string is valid and extract the parts
	// connection mus start with "s3:"
	// Then it must contain the bucket name
	if !strings.HasPrefix(connection, "s3:") || len(connection) < 4 {
		return ErrConnStringInvalid
	}
	f.bucketName = connection[3:]

	// Get the access key
	accessKeyId := os.Getenv("AWS_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKeyId == "" || secretAccessKey == "" {
		return errors.New("environmental variables AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are not defined")
	}

	// Endpoint
	// If not set, defaults to "s3.amazonaws.com"
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		endpoint = "s3.amazonaws.com"
	}

	// Enable TLS
	// If not set, defaults to true
	tls := true
	tlsStr := strings.ToLower(os.Getenv("S3_TLS"))
	if tlsStr == "0" || tlsStr == "n" || tlsStr == "no" || tlsStr == "false" {
		tls = false
	}

	// Initialize minio client object for connecting to S3
	var err error
	f.client, err = minio.New(endpoint, accessKeyId, secretAccessKey, tls)
	if err != nil {
		return err
	}

	return nil
}

func (f *S3) Get(name string, out io.Writer) (found bool, metadata map[string]string, err error) {
	if name == "" {
		err = ErrNameEmptyInvalid
		return
	}

	// Request the file from S3
	obj, err := f.client.GetObject(f.bucketName, name, minio.GetObjectOptions{})
	if err != nil {
		return
	}

	// Check if the file exists but it's empty
	stat, err := obj.Stat()
	if err != nil || stat.Size == 0 {
		found = false
		return
	}

	// Get metadata
	if stat.Metadata != nil && len(stat.Metadata) > 0 {
		metadata = make(map[string]string)
		for key, val := range stat.Metadata {
			if val != nil && len(val) == 1 {
				fmt.Println(key, val)
				metadata[key] = val[0]
			}
		}
	}

	// Copy the response body to the out stream
	_, err = io.Copy(out, obj)
	if err != nil {
		return
	}

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
		return ErrNotExist
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

func (f *S3) Delete(name string) (err error) {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	return f.client.RemoveObject(f.bucketName, name)
}
