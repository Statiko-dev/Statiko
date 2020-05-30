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
	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/utils"
)

// AzureStorage stores files on Azure Blob Storage
type AzureStorage struct {
	storageAccountName string
	storageContainer   string
	storagePipeline    pipeline.Pipeline
	storageURL         string
}

func (f *AzureStorage) Init() error {
	// Get the storage account name and key
	f.storageAccountName = appconfig.Config.GetString("repo.azure.account")
	f.storageContainer = appconfig.Config.GetString("repo.azure.container")
	if f.storageAccountName == "" || f.storageContainer == "" {
		return errors.New("configuration options repo.azure.account and repo.azure.container must be set")
	}

	// Storage endpoint
	f.storageURL = fmt.Sprintf("https://%s.blob.core.windows.net/%s", f.storageAccountName, f.storageContainer)

	// Authenticate with Azure Storage using an access key
	key := appconfig.Config.GetString("repo.azure.accessKey")
	var err error
	var credential azblob.Credential
	if key != "" {
		// Try to authenticate using a shared key
		credential, err = azblob.NewSharedKeyCredential(f.storageAccountName, key)
	} else {
		// Try to authenticate using a Service Principal
		credential, err = utils.GetAzureStorageCredentials()
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

func (f *AzureStorage) Set(name string, in io.Reader, metadata map[string]string) (err error) {
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
	_, err = azblob.UploadStreamToBlockBlob(context.Background(), in, blockBlobURL, azblob.UploadStreamToBlockBlobOptions{
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
			return fmt.Errorf("Azure Storage error failed while uploading the file: %s", stgErr.Response().Status)
		}
	}

	// Set metadata, if any
	if metadata != nil && len(metadata) > 0 {
		_, err = blockBlobURL.SetMetadata(context.Background(), metadata, azblob.BlobAccessConditions{})
		if err != nil {
			// Delete the file
			_ = f.Delete(name)
			return fmt.Errorf("error while setting metadata in Azure Storage: %v", err)
		}
	}

	return nil
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
