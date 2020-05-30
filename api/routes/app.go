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

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/fs"
	"github.com/statiko-dev/statiko/utils"
)

// AppUploadHandler is the handler for POST /app, which is used to upload new app bundles
// The request body must be a multipart/form-data with a "file" field containing the bundle and an optional "signature" one
func AppUploadHandler(c *gin.Context) {
	// Get the file from the body
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Get and sanitize the app's name
	name := utils.SanitizeAppName(file.Filename)
	if name == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Filename for the file is empty or invalid",
		})
		return
	}

	// Get the stream to the file
	in, err := file.Open()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer in.Close()

	// Check if we have a signature to store together with the file
	signature := c.PostForm("signature")
	var metadata map[string]string
	if signature != "" {
		if len(signature) > 1024 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for signature cannot be longer than 1024 characters",
			})
			return
		}
		metadata = map[string]string{
			"signature": signature,
		}
	}

	// Store the file
	err = fs.Instance.Set(name, in, metadata)
	if err != nil {
		if err == fs.ErrExist {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "File already exists",
			})
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Response
	c.AbortWithStatus(http.StatusOK)
}

type appUpdateRequest struct {
	Signature string `json:"signature" form:"signature"`
}

// AppUpdateHandler is the handler for POST /app/:name, which updates the signature of a file
// The request may contain a "signature" field
func AppUpdateHandler(c *gin.Context) {
	// Get the app to update
	name := c.Param("name")
	name = utils.SanitizeAppName(name)
	if len(name) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'name' (app name)",
		})
		return
	}

	// Get data from the form body
	data := &appUpdateRequest{}
	if err := c.Bind(data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Field could be empty if we're trying to remove a signature
	var metadata map[string]string
	if data.Signature != "" {
		if len(data.Signature) > 1024 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for signature cannot be longer than 1024 characters",
			})
			return
		}
		metadata = map[string]string{
			"signature": data.Signature,
		}
	}

	// Update the metadata
	err := fs.Instance.SetMetadata(name, metadata)
	if err != nil {
		if err == fs.ErrNotExist {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "File does not exist",
			})
			return
		}
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Response
	c.AbortWithStatus(http.StatusOK)
}
