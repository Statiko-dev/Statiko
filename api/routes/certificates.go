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
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
)

type certAddRequest struct {
	Name        string `json:"name" form:"name"`
	Certificate string `json:"cert" form:"cert"`
	Key         string `json:"key" form:"key"`
	Force       bool   `json:"force" form:"force"`
}

var certNameRegEx *regexp.Regexp

// ImportCertificateHandler is the handler for POST /certificate, which stores a new certificate
// Certificate must be an object with a key and a certificate, both PEM-encoded
// Certificate name must be a lowercase string with letters, numbers, dashes and dots only, and must begin with a letter
func ImportCertificateHandler(c *gin.Context) {
	// Get data from the form body
	data := &certAddRequest{}
	if err := c.Bind(data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}
	if data.Name == "" || data.Certificate == "" || data.Key == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Fields 'name', 'cert' and 'key' must not be empty",
		})
		return
	}

	// Validate the name
	if certNameRegEx == nil {
		certNameRegEx = regexp.MustCompile("^([a-z][a-zA-Z0-9\\.\\-]*)$")
	}
	data.Name = strings.ToLower(data.Name)
	if !certNameRegEx.MatchString(data.Name) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Certificate name must contain letters, numbers, dots and dashes only, and it must begin with a letter",
		})
		return
	}

	// Validate the certificate
	block, _ := pem.Decode([]byte(data.Certificate))
	if block == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid certificate PEM block",
		})
		return
	}
	certObj, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid certificate: " + err.Error(),
		})
		return
	}

	// Check if the certificate is valid, unless the "force" option is true
	// We're not validating the certificate's chain
	if !data.Force {
		now := time.Now()

		// Check "NotAfter" (require at least 12 hours)
		if certObj.NotAfter.Before(now.Add(12 * time.Hour)) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("certificate has expired or has less than 12 hours of validity: %v", certObj.NotAfter),
			})
			return
		}

		// Check "NotBefore"
		if !certObj.NotBefore.Before(now) {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("certificate's NotBefore is in the future: %v", certObj.NotBefore),
			})
			return
		}
	}

	// Check the key, just if it's PEM-encoded correctly
	block, _ = pem.Decode([]byte(data.Key))
	if block == nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid key PEM block",
		})
		return
	}

	// Store the certificate
	err = state.Instance.SetCertificate(state.TLSCertificateImported, []string{data.Name}, []byte(data.Key), []byte(data.Certificate))
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// We'll trigger a sync in case an existing certificate was updated
	sync.QueueRun()

	// Respond with "No Content"
	c.Status(http.StatusNoContent)
}

// ListCertificateHandler is the handler for GET /certificate, which lists all certificates currently stored (names only)
func ListCertificateHandler(c *gin.Context) {
	// Get the list of certificates from the state object
	certs := state.Instance.ListImportedCertificates()
	c.JSON(http.StatusOK, certs)
}

// DeleteCertificateHandler is the handler for DELETE /certificate/{name}, which removes a certificate from the store
// Only certificates not used by any site can be deleted
func DeleteCertificateHandler(c *gin.Context) {
	if name := c.Param("name"); len(name) > 0 {
		name = strings.ToLower(name)

		// Check if the certificate exists in the store
		key, cert, err := state.Instance.GetCertificate(state.TLSCertificateImported, []string{name})
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if key == nil || len(key) == 0 || cert == nil || len(cert) == 0 {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "TLS certificate does not exist in store",
			})
			return
		}

		// Check if any site is using the certificate
		sites := state.Instance.GetSites()
		for _, s := range sites {
			if s.TLS != nil &&
				s.TLS.Type == state.TLSCertificateImported &&
				s.TLS.Certificate != nil &&
				*s.TLS.Certificate == name {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error": "TLS certificate is in use and can't be removed",
				})
				return
			}
		}

		// Delete the certificate
		if err := state.Instance.RemoveCertificate(state.TLSCertificateImported, []string{name}); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'name'",
		})
	}
}
