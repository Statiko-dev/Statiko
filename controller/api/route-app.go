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

package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/shared/fs"
	"github.com/statiko-dev/statiko/utils"
)

// AppUploadHandler is the handler for POST /app, which is used to upload new app bundles
// The request body must be a multipart/form-data with a "file" field containing the bundle, a "name" field containing the name, and a "type" one containing the type (file extension)
// Optionally, pass a "signature" and/or "hash" fielld
func (s *APIServer) AppUploadHandler(c *gin.Context) {
	// Get the file from the body
	file, err := c.FormFile("file")
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Get and sanitize the app's name
	name := utils.SanitizeAppName(c.PostForm("name"))
	if name == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Filename for the file is empty or invalid",
		})
		return
	}

	// Get and validate the app's type
	typ := c.PostForm("type")
	if typ == "" || !utils.StringInSlice(utils.ArchiveExtensions, "."+typ) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "File extension is empty or invalid",
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

	// Store file type
	metadata := make(map[string]string)
	metadata["type"] = typ

	// Check if we have a signature to store together with the file
	signature := c.PostForm("signature")
	if signature != "" {
		if len(signature) > 1024 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for 'signature' cannot be longer than 1024 characters",
			})
			return
		}
		metadata["signature"] = signature
	}

	// Check if we have a hash
	hash := c.PostForm("hash")
	if hash != "" {
		if len(hash) > 64 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for 'hash' cannot be longer than 64 characters",
			})
			return
		}
		metadata["hash"] = hash
	}

	// Store the file
	err = s.Store.SetWithContext(c.Request.Context(), name, in, metadata)
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
	c.Status(http.StatusNoContent)
}

type appUpdateRequest struct {
	Signature string `json:"signature" form:"signature"`
	Hash      string `json:"hash" form:"hash"`
}

// AppUpdateHandler is the handler for POST /app/:name, which updates the signature of a file
// The request may contain a "signature" field or a "hash" onne
func (s *APIServer) AppUpdateHandler(c *gin.Context) {
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

	// Get the current metadata
	metadata, err := s.Store.GetMetadata(name)
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
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Reset the signature and hash
	metadata["signature"] = ""
	metadata["hash"] = ""

	// Fields could be empty if we're trying to remove a signature/hash
	if data.Signature != "" {
		if len(data.Signature) > 1024 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for 'signature' cannot be longer than 1024 characters",
			})
			return
		}
		metadata["signature"] = data.Signature
	}
	if data.Hash != "" {
		if len(data.Hash) > 64 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Value for 'hash' cannot be longer than 64 characters",
			})
			return
		}
		metadata["hash"] = data.Hash
	}

	// Update the metadata
	err = s.Store.SetMetadata(name, metadata)
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
	c.Status(http.StatusNoContent)
}

// AppListHandler is the handler for GET /app which returns the list of apps
func (s *APIServer) AppListHandler(c *gin.Context) {
	// Get the list
	list, err := s.Store.ListWithContext(c.Request.Context())
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Response
	c.JSON(http.StatusOK, list)
}

// AppDeleteHandler is the handler for DELETE /app/:name which removes an app from the storage
func (s *APIServer) AppDeleteHandler(c *gin.Context) {
	// Get the app to delete
	name := c.Param("name")
	name = utils.SanitizeAppName(name)
	if len(name) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'name' (app name)",
		})
		return
	}

	// Delete the app
	err := s.Store.Delete(name)
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
	c.Status(http.StatusNoContent)
}
