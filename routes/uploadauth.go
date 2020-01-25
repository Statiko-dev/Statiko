/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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

package routes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
)

// uploadAuthRequest is the request body for the POST /uploadauth route
type uploadAuthRequest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// uploadAuthResponse is the response from the POST /uploadauth route
type uploadAuthResponse struct {
	ArchiveURL string `json:"archiveUrl"`
}

// UploadAuthHandler is the handler for POST /uploadauth, which returns the SAS token to authorize uploads to Azure Blob Storage
func UploadAuthHandler(c *gin.Context) {
	// Get data from the form body
	app := &uploadAuthRequest{}
	if err := c.Bind(app); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}
	if app.Name == "" || app.Version == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "'name' and 'version' fields must not be empty",
		})
		return
	}

	// Get Azure Storage configuration
	azureStorageAccount := appconfig.Config.GetString("azureStorage.account")
	azureStorageKey := appconfig.Config.GetString("azureStorage.key")
	azureStorageContainer := appconfig.Config.GetString("azureStorage.container")
	credential, err := azblob.NewSharedKeyCredential(azureStorageAccount, azureStorageKey)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Ensure that the blob doesn't exist already
	archiveName := app.Name + "-" + app.Version + ".tar.bz2"
	archiveURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", azureStorageAccount, azureStorageContainer, archiveName)
	u, err := url.Parse(archiveURL)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	archiveBlobURL := azblob.NewBlobURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{MaxTries: 3},
	}))
	ctx := context.Background()
	properties, err := archiveBlobURL.GetProperties(ctx, azblob.BlobAccessConditions{})
	if err != nil {
		// If the error is a Not Found (404), then we're good
		if stgErr, ok := err.(azblob.StorageError); !ok {
			// Not an Azure Blob Storage error (network error?)
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		} else if stgErr.Response().StatusCode != http.StatusNotFound {
			// An Azure Blob Storage error, but not a 404
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}
	if properties != nil && properties.StatusCode() == http.StatusOK {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "A bundle for the same app and version has already been uploaded",
		})
		return
	}

	// Generate a SAS token for the app's bundle
	blobSASSigValues := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(2 * time.Hour),
		ContainerName: azureStorageContainer,
		BlobName:      archiveName,

		// Get a blob-level SAS token
		Permissions: azblob.BlobSASPermissions{Read: true, Write: true}.String(),
	}
	archiveSasQueryParams, err := blobSASSigValues.NewSASQueryParameters(credential)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	archiveQp := archiveSasQueryParams.Encode()
	signedArchiveURL := archiveURL + "?" + archiveQp

	// Reponse
	response := uploadAuthResponse{
		ArchiveURL: signedArchiveURL,
	}

	c.JSON(http.StatusOK, response)
}
