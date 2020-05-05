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
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/azurekeyvault"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
)

// CreateSiteHandler is the handler for POST /site, which creates a new site
func CreateSiteHandler(c *gin.Context) {
	// Get data from the form body
	site := &state.SiteState{}
	if err := c.Bind(site); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Check if site exists already
	domains := make([]string, len(site.Aliases)+1)
	copy(domains, site.Aliases)
	domains[len(site.Aliases)] = site.Domain
	for _, el := range domains {
		if state.Instance.GetSite(el) != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "Domain or alias already exists",
			})
			return
		}
	}

	// Self-signed TLS certificates are default when no value is specified, or when value is "selfsigned"
	// If the value is "letsencrypt", request a certificate from Let's Encrypt (not yet implemented)
	if site.TLSCertificate == nil || *site.TLSCertificate == "" || *site.TLSCertificate == "selfsigned" {
		site.TLSCertificateType = state.TLSCertificateSelfSigned
		site.TLSCertificate = nil
		site.TLSCertificateVersion = nil
	} else if *site.TLSCertificate == "letsencrypt" {
		/*site.TLSCertificateType = state.TLSCertificateLetsEncrypt
		site.TLSCertificate = nil
		site.TLSCertificateVersion = nil*/
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "Support for Let's Encrypt not yet implemented",
		})
		return
	} else {
		site.TLSCertificateType = state.TLSCertificateImported

		// Check if the certificate exists
		exists, err := azurekeyvault.GetInstance().CertificateExists(*site.TLSCertificate)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if !exists {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "TLS certificate does not exist in the key vault",
			})
			return
		}
	}

	// Add the website to the store
	if err := state.Instance.AddSite(site); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Queue a sync
	sync.QueueRun()

	// Respond with "No Content"
	c.Status(http.StatusNoContent)
}

// ListSiteHandler is the handler for GET /site, which lists all sites
func ListSiteHandler(c *gin.Context) {
	// Get records from the state object
	sites := state.Instance.GetSites()

	c.JSON(http.StatusOK, sites)
}

// ShowSiteHandler is the handler for GET /site/{domain}, which shows a site
func ShowSiteHandler(c *gin.Context) {
	if domain := c.Param("domain"); len(domain) > 0 {
		// Get the site from the state object
		site := state.Instance.GetSite(domain)
		if site == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Domain name not found",
			})
			return
		}

		c.JSON(http.StatusOK, site)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
	}
}

// DeleteSiteHandler is the handler for DELETE /site/{domain}, which deletes a site
func DeleteSiteHandler(c *gin.Context) {
	if domain := c.Param("domain"); len(domain) > 0 {
		// Get the site from the state object to check if it exists
		if site := state.Instance.GetSite(domain); site == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Domain name not found",
			})
			return
		}

		// Delete the record
		if err := state.Instance.DeleteSite(domain); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Queue a sync
		sync.QueueRun()

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
	}
}

// PatchSiteHandler is the handler for PATCH /site/{domain}, which replaces a site
func PatchSiteHandler(c *gin.Context) {
	// Get the site to update (domain name)
	domain := c.Param("domain")
	if len(domain) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
		return
	}

	// Get the site from the state object
	site := state.Instance.GetSite(domain)
	if site == nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
			"error": "Domain name not found",
		})
		return
	}

	// Get data to update from the body
	var update map[string]interface{}
	if err := c.Bind(&update); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body: " + err.Error(),
		})
		return
	}

	// Iterate through the fields in the input and update site
	updated := false
	updatedTLS := false
	updatedTLSVersion := false
	for k, v := range update {
		t := reflect.TypeOf(v)

		switch strings.ToLower(k) {
		case "tlscertificate":
			if t != nil && t.Kind() == reflect.String {
				str := v.(string)
				if str == "letsencrypt" {
					site.TLSCertificate = nil
					site.TLSCertificateType = state.TLSCertificateLetsEncrypt
				} else if str == "selfsigned" || str == "" {
					site.TLSCertificate = nil
					site.TLSCertificateType = state.TLSCertificateSelfSigned
				} else {
					site.TLSCertificate = &str
					site.TLSCertificateType = state.TLSCertificateImported

					// Check if the certificate exists
					exists, err := azurekeyvault.GetInstance().CertificateExists(*site.TLSCertificate)
					if err != nil {
						c.AbortWithError(http.StatusInternalServerError, err)
						return
					}
					if !exists {
						c.AbortWithStatusJSON(http.StatusConflict, gin.H{
							"error": "TLS certificate does not exist in the key vault",
						})
						return
					}
				}
				updatedTLS = true
				updated = true
			} else if t == nil {
				site.TLSCertificate = nil
				site.TLSCertificateType = state.TLSCertificateSelfSigned
				updatedTLS = true
				updated = true
			}
		case "tlscertificateversion":
			if t.Kind() == reflect.String {
				str := v.(string)
				site.TLSCertificateVersion = &str
				updatedTLSVersion = true
				updated = true
			}
		case "aliases":
			if t == nil {
				// Reset the aliases slice
				site.Aliases = make([]string, 0)
			} else if t.Kind() == reflect.Slice {
				updated = true

				// Reset the aliases slice
				site.Aliases = make([]string, 0)

				// Check if the aliases exist already
				for _, a := range v.([]interface{}) {
					// Element must be a string
					if reflect.TypeOf(a).Kind() != reflect.String {
						c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
							"error": "Invalid type for element in the aliases list",
						})
						return
					}

					str := a.(string)

					// Aliases can't be defined somewhere else (but can be defined in this same site!)
					ok := state.Instance.GetSite(str)
					if ok != nil && ok.Domain != site.Domain {
						c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
							"error": "Alias " + str + " already exists",
						})
						return
					}

					// Add the alias
					site.Aliases = append(site.Aliases, str)
				}
			}
		}
	}

	// If we have updated the TLS certificate, but not the version, reset the version
	if updatedTLS && (!updatedTLSVersion || site.TLSCertificate == nil || site.TLSCertificateType != state.TLSCertificateImported) {
		site.TLSCertificateVersion = nil
	}

	// Update the site object if something has changed
	if updated {
		if err := state.Instance.UpdateSite(site, true); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Queue a sync
		sync.QueueRun()
	}

	// Respond with "No content"
	c.Status(http.StatusNoContent)
}
