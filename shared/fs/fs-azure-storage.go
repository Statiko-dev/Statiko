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
	"fmt"
	"io"
	"net/url"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// AzureStorage stores files on Azure Blob Storage
type AzureStorage struct {
	storageAccountName string
	storageContainer   string
	storagePipeline    pipeline.Pipeline
	storageURL         string
}

func (f *AzureStorage) Init(optsI interface{}) error {
	// Cast opts to pb.ClusterOptions_StorageAzure
	opts, ok := optsI.(*pb.ClusterOptions_StorageAzure)
	if !ok || opts == nil {
		return errors.New("invalid options object")
	}

	// Get the storage account name and key
	f.storageAccountName = opts.GetAccount()
	f.storageContainer = opts.GetContainer()
	if f.storageAccountName == "" || f.storageContainer == "" {
		return errors.New("configuration options `account` and `container` must be set")
	}

	// Check if we're using TLS
	protocol := "https"
	if opts.NoTls {
		protocol = "http"
	}

	// Check if need to use a custom storage endpoint (e.g. for Azurite)
	if opts.CustomEndpoint != "" {
		f.storageURL = fmt.Sprintf("%s://%s/%s/%s", protocol, opts.CustomEndpoint, f.storageAccountName, f.storageContainer)
	} else {
		// Storage account endpoint suffix to support Azure China, Azure Germany, Azure Gov, or Azure Stack
		endpointSuffix := opts.EndpointSuffix
		if endpointSuffix == "" {
			endpointSuffix = "core.windows.net"
		}

		// Storage endpoint
		f.storageURL = fmt.Sprintf("%s://%s.blob.%s/%s", protocol, f.storageAccountName, endpointSuffix, f.storageContainer)
	}

	// Authenticate with Azure Storage using an access key
	key := opts.GetAccessKey()
	var err error
	var credential azblob.Credential
	if key != "" {
		// Try to authenticate using a shared key
		credential, err = azblob.NewSharedKeyCredential(f.storageAccountName, key)
	} else {
		// Try to authenticate using a Service Principal
		credential, err = utils.GetAzureStorageCredentials(opts.GetAuth())
	}
	if err != nil {
		return err
	}
	f.storagePipeline = azblob.NewPipeline(credential, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			MaxTries: 3,
		},
	})

	return nil
}

func (f *AzureStorage) Get(name string) (found bool, data io.ReadCloser, metadata map[string]string, err error) {
	if name == "" {
		err = ErrNameEmptyInvalid
		return
	}

	found = true

	// Create the blob URL
	u, err := url.Parse(f.storageURL + "/" + name)
	if err != nil {
		return
	}
	blockBlobURL := azblob.NewBlockBlobURL(*u, f.storagePipeline)

	// Download the file
	resp, err := blockBlobURL.Download(context.Background(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			err = fmt.Errorf("network error while downloading the file: %s", err.Error())
		} else {
			// Blob not found
			if stgErr.ServiceCode() == "BlobNotFound" {
				found = false
				err = nil
				return
			}
			err = fmt.Errorf("azure Storage error while downloading the file: %s", stgErr.Response().Status)
		}
		return
	}
	data = resp.Body(azblob.RetryReaderOptions{
		MaxRetryRequests: 3,
	})

	// Check if the file exists but it's empty
	if resp.ContentLength() == 0 {
		found = false
		data.Close()
		data = nil
		return
	}

	// Get the metadata
	metadata = resp.NewMetadata()

	return
}

func (f *AzureStorage) List() ([]FileInfo, error) {
	return f.ListWithContext(context.Background())
}

func (f *AzureStorage) ListWithContext(ctx context.Context) ([]FileInfo, error) {
	// Create the container URL
	u, err := url.Parse(f.storageURL)
	if err != nil {
		return nil, err
	}
	containerUrl := azblob.NewContainerURL(*u, f.storagePipeline)

	// Request the list
	list := make([]FileInfo, 0)
	marker := azblob.Marker{}
	more := true
	for more {
		resp, err := containerUrl.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{})
		if err != nil {
			if stgErr, ok := err.(azblob.StorageError); !ok {
				return nil, fmt.Errorf("network error while listing filles: %s", err.Error())
			} else {
				return nil, fmt.Errorf("Azure Storage error while listing filles: %s", stgErr.Response().Status)
			}
		}

		// Check if there's more
		marker = resp.NextMarker
		if !marker.NotDone() {
			fmt.Println("Done")
			more = false
		} else {
			fmt.Println("Not Done")
		}

		// Iterate through the response
		if resp == nil || resp.Segment.BlobItems == nil {
			return nil, errors.New("invalid response object")
		}
		for _, el := range resp.Segment.BlobItems {
			size := int64(0)
			if el.Properties.ContentLength != nil {
				size = *el.Properties.ContentLength
			}
			list = append(list, FileInfo{
				Name:         el.Name,
				Size:         size,
				LastModified: el.Properties.LastModified,
			})
		}
	}

	return list, nil
}

func (f *AzureStorage) Set(name string, in io.Reader, metadata map[string]string) (err error) {
	return f.SetWithContext(context.Background(), name, in, metadata)
}

func (f *AzureStorage) SetWithContext(ctx context.Context, name string, in io.Reader, metadata map[string]string) (err error) {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	// Create the blob URL
	u, err := url.Parse(f.storageURL + "/" + name)
	if err != nil {
		return err
	}
	blockBlobURL := azblob.NewBlockBlobURL(*u, f.storagePipeline)

	// Access conditions for blob uploads: disallow the operation if the blob already exists
	// See: https://docs.microsoft.com/en-us/rest/api/storageservices/specifying-conditional-headers-for-blob-service-operations#Subheading1
	accessConditions := azblob.BlobAccessConditions{
		ModifiedAccessConditions: azblob.ModifiedAccessConditions{
			IfNoneMatch: "*",
		},
	}

	// Upload the blob
	_, err = azblob.UploadStreamToBlockBlob(ctx, in, blockBlobURL, azblob.UploadStreamToBlockBlobOptions{
		BufferSize:       3 * 1024 * 1024,
		MaxBuffers:       2,
		AccessConditions: accessConditions,
	})
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			return fmt.Errorf("network error while uploading the file: %s", err.Error())
		} else {
			if stgErr.ServiceCode() == "BlobAlreadyExists" {
				return ErrExist
			}
			return fmt.Errorf("Azure Storage error while uploading the file: %s", stgErr.Response().Status)
		}
	}

	// Set metadata, if any
	if metadata != nil && len(metadata) > 0 {
		_, err = blockBlobURL.SetMetadata(ctx, metadata, azblob.BlobAccessConditions{})
		if err != nil {
			// Delete the file
			_ = f.Delete(name)
			return fmt.Errorf("error while setting metadata in Azure Storage: %v", err)
		}
	}

	return nil
}

func (f *AzureStorage) GetMetadata(name string) (metadata map[string]string, err error) {
	if name == "" {
		return nil, ErrNameEmptyInvalid
	}

	// Create the blob URL
	u, err := url.Parse(f.storageURL + "/" + name)
	if err != nil {
		return nil, err
	}
	blockBlobURL := azblob.NewBlockBlobURL(*u, f.storagePipeline)

	// Download the file
	resp, err := blockBlobURL.GetProperties(context.Background(), azblob.BlobAccessConditions{})
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			err = fmt.Errorf("network error while downloading the file: %s", err.Error())
		} else {
			// Blob not found
			if stgErr.ServiceCode() == "BlobNotFound" {
				return nil, ErrNotExist
			}
			err = fmt.Errorf("azure Storage error while downloading the file: %s", stgErr.Response().Status)
		}
		return nil, err
	}
	metadata = resp.NewMetadata()
	return metadata, nil
}

func (f *AzureStorage) SetMetadata(name string, metadata map[string]string) error {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	if metadata == nil || len(metadata) == 0 {
		metadata = nil
	}

	// Create the blob URL
	u, err := url.Parse(f.storageURL + "/" + name)
	if err != nil {
		return err
	}
	blockBlobURL := azblob.NewBlockBlobURL(*u, f.storagePipeline)

	// Set metadata
	_, err = blockBlobURL.SetMetadata(context.Background(), metadata, azblob.BlobAccessConditions{})
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			return fmt.Errorf("network error while setting metadata: %s", err.Error())
		} else {
			if stgErr.ServiceCode() == "BlobNotFound" {
				return ErrNotExist
			}
			return fmt.Errorf("Azure Storage error while setting metadata: %s", stgErr.Response().Status)
		}
	}

	return nil
}

func (f *AzureStorage) Delete(name string) (err error) {
	if name == "" {
		return ErrNameEmptyInvalid
	}

	// Create the blob URL
	u, err := url.Parse(f.storageURL + "/" + name)
	if err != nil {
		return
	}
	blockBlobURL := azblob.NewBlockBlobURL(*u, f.storagePipeline)

	// Delete the blob
	_, err = blockBlobURL.Delete(context.Background(), azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			return fmt.Errorf("network error while deleting the file: %s", err.Error())
		} else {
			if stgErr.ServiceCode() == "BlobNotFound" {
				return ErrNotExist
			}
			return fmt.Errorf("Azure Storage error while deleting the file: %s", stgErr.Response().Status)
		}
	}
	return
}
