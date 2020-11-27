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
	"github.com/google/uuid"

	"github.com/statiko-dev/statiko/controller/certificates"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

type certAddRequest struct {
	Certificate string `json:"cert" form:"cert"`
	Key         string `json:"key" form:"key"`
	// Force accepting certificates that have expired too
	Force bool `json:"force" form:"force"`
}

type certListResponseItem struct {
	ID        string     `json:"id"`
	Name      string     `json:"name,omitempty"`
	Domains   []string   `json:"domains,omitempty"`
	NotBefore *time.Time `json:"nbf,omitempty"`
	Expiry    *time.Time `json:"exp,omitempty"`
}

type certListResponse []certListResponseItem

// ImportCertificateHandler is the handler for POST /certificate, which imports a new certificate
// Request must contain an object with a key and a certificate, both PEM-encoded
func (s *APIServer) ImportCertificateHandler(c *gin.Context) {
	// Get data from the form body
	data := &certAddRequest{}
	if err := c.Bind(data); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

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
	certObj := &pb.TLSCertificate{
		Type: pb.TLSCertificate_IMPORTED,
	}
	certObj.SetCertificateProperties(certX509)
	key := []byte(data.Key)
	cert := []byte(data.Certificate)

	// Generate a certificate ID
	u, err := uuid.NewRandom()
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	certId := u.String()

	// Store the certificate
	err = s.State.SetCertificate(certObj, certId, key, cert)
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
		if cert.Type == pb.TLSCertificate_IMPORTED {
			continue
		}
		var nbf, exp time.Time
		if cert.XNbf > 0 {
			nbf = time.Unix(cert.XNbf, 0)
		}
		if cert.XExp > 0 {
			exp = time.Unix(cert.XExp, 0)
		}
		result = append(result, certListResponseItem{
			ID:        id,
			Domains:   cert.XDomains,
			NotBefore: &nbf,
			Expiry:    &exp,
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
		if cert == nil || cert.Type != pb.TLSCertificate_IMPORTED {
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

// RefreshCertificateHandler is the handler for POST /certificate/refresh, which triggers a refresh of certificates
// This has no response. It's a POST request because it does trigger actions
func (s *APIServer) RefreshCertificateHandler(c *gin.Context) {
	// Trigger a refresh
	s.State.TriggerCertRefresh()
	c.Status(http.StatusNoContent)
}
