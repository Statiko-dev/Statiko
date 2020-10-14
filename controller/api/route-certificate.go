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
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/certificates/azurekeyvault"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

type certAddRequest struct {
	Type        string `json:"type" form:"type"`
	Name        string `json:"name" form:"name"`
	Certificate string `json:"cert" form:"cert"`
	Key         string `json:"key" form:"key"`
	Force       bool   `json:"force" form:"force"`
}

type certListResponseItem struct {
	Type    string   `json:"type"`
	ID      string   `json:"id"`
	Name    string   `json:"name,omitempty"`
	Domains []string `json:"domains,omitempty"`
}

type certListResponse []certListResponseItem

// ImportCertificateHandler is the handler for POST /certificate, which adds a new certificate
// For imported certificates, data must be an object with a key and a certificate, both PEM-encoded
// For certificates imported from Azure Key Vault, the data must be an object with the certificate name
func (s *APIServer) ImportCertificateHandler(c *gin.Context) {
	// Get data from the form body
	data := &certAddRequest{}
	if err := c.Bind(data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	var (
		certObj   *pb.TLSCertificate
		key, cert []byte
	)

	// Switch depending on the certificate type
	switch strings.ToUpper(data.Type) {

	// Imported certificates
	case pb.TLSCertificate_IMPORTED.String():
		// Ensure we have the certificate and key
		if data.Certificate == "" || data.Key == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Fields 'cert' and 'key' must not be empty",
			})
			return
		}

		// Validate the certificate
		certX509, err := certificates.GetX509([]byte(data.Certificate))
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
			if certX509.NotAfter.Before(now.Add(12 * time.Hour)) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("certificate has expired or has less than 12 hours of validity: %v", certX509.NotAfter),
				})
				return
			}

			// Check "NotBefore"
			if !certX509.NotBefore.Before(now) {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("certificate's NotBefore is in the future: %v", certX509.NotBefore),
				})
				return
			}
		}

		// Check the key, just to see if it's PEM-encoded correctly
		block, _ := pem.Decode([]byte(data.Key))
		if block == nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Invalid key PEM block",
			})
			return
		}

		// Data to save
		certObj = &pb.TLSCertificate{
			Type: pb.TLSCertificate_IMPORTED,
		}
		key = []byte(data.Key)
		cert = []byte(data.Certificate)

	// Certs imported from Azure Key Vault
	case pb.TLSCertificate_AZURE_KEY_VAULT.String():
		// Ensure we have the name
		if data.Name == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Field 'name'",
			})
			return
		}

		// Check if the certificate exists
		exists, err := azurekeyvault.GetInstance().CertificateExists(data.Name)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if !exists {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "TLS certificate does not exist in Azure Key Vault",
			})
			return
		}

		// Data to save
		certObj = &pb.TLSCertificate{
			Type: pb.TLSCertificate_AZURE_KEY_VAULT,
			Name: data.Name,
		}
		key = nil
		cert = nil

	// Other types of certificates cannot be imported
	default:
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid TLS certificate type",
		})
	}

	// Store the certificate
	err := s.State.SetCertificate(certObj, "", key, cert)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Respond with "No Content"
	c.Status(http.StatusNoContent)
}

// ListCertificateHandler is the handler for GET /certificate, which lists all certificates currently stored
func (s *APIServer) ListCertificateHandler(c *gin.Context) {
	result := certListResponse{}

	// Get the list of certificates from the state object
	for id, cert := range s.State.ListCertificates() {
		// Only list certificates that are imported
		if cert.Type == pb.TLSCertificate_SELF_SIGNED || cert.Type == pb.TLSCertificate_ACME {
			continue
		}
		result = append(result, certListResponseItem{
			Type:    strings.ToLower(cert.Type.String()),
			ID:      id,
			Name:    cert.Name,
			Domains: cert.Domains,
		})
	}

	// Respond
	c.JSON(http.StatusOK, result)
}

// DeleteCertificateHandler is the handler for DELETE /certificate/{id}, which removes a certificate from the store
// Only imported certificates not used by any site can be deleted
func (s *APIServer) DeleteCertificateHandler(c *gin.Context) {
	if id := c.Param("id"); len(id) > 0 {
		id = strings.ToLower(id)

		// Check if the certificate exists in the store and if it's an imported one
		cert, err := s.State.GetCertificateInfo(id)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if cert == nil || cert.Type != pb.TLSCertificate_IMPORTED || cert.Type != pb.TLSCertificate_AZURE_KEY_VAULT {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "TLS certificate does not exist in store or it's not an imported certificate",
			})
			return
		}

		// Check if any site is using the certificate
		sites := s.State.GetSites()
		for _, s := range sites {
			if s.ImportedTlsId == id || s.GeneratedTlsId == id {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error": "TLS certificate is in use and can't be removed",
				})
				return
			}
		}

		// Delete the certificate
		if err := s.State.DeleteCertificate(id); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'id'",
		})
	}
}
