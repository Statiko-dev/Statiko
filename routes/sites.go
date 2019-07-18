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
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"

	"smplatform/state"
	"smplatform/sync"
)

// CreateSiteHandler is the handler for POST /site, which creates a new site
// @Summary Creates a new site
// @Description Creates a new site in the local web server
// @Accept json
// @Produce json
// @Param domain body string true "Domain name" minimum(1)
// @Param tlsCertificate body string true "TLS Certificate name in the Key Vault" minimum(1)
// @Failure 500
// @Router /site [post]
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

	// Ensure the TLS Certificate Version is empty
	site.TLSCertificateVersion = nil

	// Add the website to the store
	err := state.Instance.AddSite(site)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Queue a sync
	sync.QueueRun()

	// Response
	c.Status(http.StatusCreated)
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
		// Delete the record
		err := state.Instance.DeleteSite(domain)
		if err != nil {
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
		case "clientcaching":
			if t.Kind() == reflect.Bool {
				site.ClientCaching = v.(bool)
				updated = true
			}
		case "tlscertificate":
			if t.Kind() == reflect.String {
				str := v.(string)
				site.TLSCertificate = &str
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
	if updatedTLS && !updatedTLSVersion {
		site.TLSCertificateVersion = nil
	}

	// Update the site object if something has changed
	if updated {
		state.Instance.UpdateSite(site, true)

		// Queue a sync
		sync.QueueRun()
	}

	c.Status(http.StatusNoContent)
}
