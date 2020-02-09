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
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gin-gonic/gin"

	"github.com/ItalyPaleAle/statiko/appconfig"
	"github.com/ItalyPaleAle/statiko/utils"
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

	// Azure Storage authorization
	credential, err := utils.GetAzureStorageCredentials()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Get Azure Storage configuration
	azureStorageAccount := appconfig.Config.GetString("azure.storage.account")
	azureStorageContainer := appconfig.Config.GetString("azure.storage.appsContainer")
	azureStorageSuffix, err := utils.GetAzureStorageEndpointSuffix()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Ensure that the blob doesn't exist already
	archiveName := app.Name + "-" + app.Version + ".tar.bz2"
	archiveURL := fmt.Sprintf("https://%s.blob.%s/%s/%s", azureStorageAccount, azureStorageSuffix, azureStorageContainer, archiveName)
	u, err := url.Parse(archiveURL)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	archiveBlobURL := azblob.NewBlobURL(*u, azblob.NewPipeline(credential, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{MaxTries: 3},
	}))
	properties, err := archiveBlobURL.GetProperties(context.TODO(), azblob.BlobAccessConditions{})
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

	// Request the user delegation key
	// Since the Azure Storage SDK for Go doesn't support this yet, we need to invoke the REST API directly
	// Start by getting the token
	var token string
	if m, ok := credential.(interface{ Token() string }); ok {
		token = m.Token()
	} else {
		c.AbortWithError(http.StatusInternalServerError, errors.New("credentials object does not contain the 'Token' method"))
		return
	}

	// Build the request
	// Reference: https://docs.microsoft.com/en-us/rest/api/storageservices/get-user-delegation-key
	now := time.Now()
	timeStart := now.UTC().Format("2006-01-02T15:04:05Z")
	timeEnd := now.Add(2 * time.Hour).UTC().Format("2006-01-02T15:04:05Z")
	azureStorageAPIVersion := "2018-11-09"
	userDelegationKeyUrl := fmt.Sprintf("https://%s.blob.%s/?restype=service&comp=userdelegationkey", azureStorageAccount, azureStorageSuffix)
	reqBody := bytes.NewBufferString(`<?xml version="1.0" encoding="utf-8"?>  
	<KeyInfo>  
		<Start>` + timeStart + `</Start>
		<Expiry>` + timeEnd + `</Expiry>
	</KeyInfo>`)
	req, err := http.NewRequest("POST", userDelegationKeyUrl, reqBody)
	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("x-ms-version", azureStorageAPIVersion)
	res, err := httpClient.Do(req)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode >= 399 {
		b, _ := ioutil.ReadAll(res.Body)
		c.AbortWithError(http.StatusInternalServerError, fmt.Errorf("response error: %s\n", string(b)))
		return
	}

	// Get the response body and parse the XML
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	var body struct {
		SignedOid     string `xml:"SignedOid"`
		SignedTid     string `xml:"SignedTid"`
		SignedStart   string `xml:"SignedStart"`
		SignedExpiry  string `xml:"SignedExpiry"`
		SignedService string `xml:"SignedService"`
		SignedVersion string `xml:"SignedVersion"`
		Value         string `xml:"Value"`
	}
	err = xml.Unmarshal(bodyBytes, &body)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Generate the SAS URL
	// Reference: https://docs.microsoft.com/en-us/rest/api/storageservices/create-user-delegation-sas#construct-a-user-delegation-sas
	// .NET Implementation: https://github.com/Azure/azure-sdk-for-net/blob/20985657349e7baab94fabba344120aa32483943/sdk/storage/Azure.Storage.Blobs/src/Sas/BlobSasBuilder.cs#L266
	canonicalizedResource := fmt.Sprintf("/blob/%s/%s/%s", azureStorageAccount, azureStorageContainer, archiveName)
	stringToSign := strings.Join([]string{
		"rw",                   // signedPermissions: Read and Write
		timeStart,              // signedStart
		timeEnd,                // signedEnd
		canonicalizedResource,  // canonicalizedTesource
		body.SignedOid,         // signedOid
		body.SignedTid,         // signedTid
		body.SignedStart,       // signedKeyStart
		body.SignedExpiry,      // signedKeyExpiry
		body.SignedService,     // signedKeyService
		body.SignedVersion,     // signedKeyVerion
		"",                     // signedIP
		"https",                // signedProtocol
		azureStorageAPIVersion, // signedVersion
		"b",                    // signedResource
		"",                     // signedSnapshotTime
		"",                     // rscc (Cache-Control header)
		"",                     // rscd (Cache-Disposition header)
		"",                     // rsce (Content-Encoding header)
		"",                     // rscl (Content-Language header)
		"",                     // rsct (Content-Type header)
	}, "\n")
	key, err := base64.StdEncoding.DecodeString(body.Value)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	h := hmac.New(sha256.New, key)
	h.Write([]byte(stringToSign))
	signature := h.Sum(nil)
	signatureB64 := base64.StdEncoding.EncodeToString(signature)

	// Signed URL with SAS
	qs := &url.Values{}
	qs.Set("sv", azureStorageAPIVersion) // signedVersion
	qs.Set("sr", "b")                    // signedResource
	qs.Set("st", timeStart)              // signedStart
	qs.Set("se", timeEnd)                // signedExpiry
	qs.Set("sp", "rw")                   // signedPermissions
	qs.Set("spr", "https")               // signedProtocol
	qs.Set("skoid", body.SignedOid)      // signedOid
	qs.Set("sktid", body.SignedTid)      // signedTid
	qs.Set("skt", body.SignedStart)      // signedKeyStart
	qs.Set("ske", body.SignedExpiry)     // signedKeyExpiry
	qs.Set("sks", body.SignedService)    // signedKeyService
	qs.Set("skv", body.SignedVersion)    // signedKeyVersion
	qs.Set("sig", signatureB64)          // signature
	// Missing signedIp (sip) as we don't use that
	signedArchiveURL := archiveURL + "?" + qs.Encode()

	// Response
	response := uploadAuthResponse{
		ArchiveURL: signedArchiveURL,
	}
	c.JSON(http.StatusOK, response)

	/*// Generate a SAS token for the app's bundle
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
	*/
}
