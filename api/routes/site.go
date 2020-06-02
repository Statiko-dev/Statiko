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

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/certificates/azurekeyvault"
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

	// If we're creating a temporary site, generate a domain name
	if site.Temporary {
		// Ensure a domain is set
		tempDomain := appconfig.Config.GetString("temporarySites.domain")
		if tempDomain == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Configuration option `temporarySites.domain` must be set before creating a temporary site",
			})
			return
		}
		if tempDomain[0] != '.' {
			// Ensure there's a dot at the beginning
			tempDomain = "." + tempDomain
		}

		// Temporary sites cannot have domain names or aliases
		if site.Domain != "" || len(site.Aliases) > 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Temporary sites cannot have a defined domain name or alias",
			})
			return
		}
		// Temporary domains cannot use TLS certificates from ACME, to avoid rate limiting
		if site.TLS != nil && site.TLS.Type == state.TLSCertificateACME {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Temporary sites cannot request TLS certificates from ACME",
			})
			return
		}
		site.Domain = petname.Generate(3, "-") + tempDomain
	} else {
		// Ensure that the domain name is set
		if site.Domain == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Field 'domain' is required",
			})
			return
		}
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

	// Self-signed TLS certificates are default when no value is specified
	// If the value is "acme", request a certificate from ACME
	if site.TLS == nil || site.TLS.Type == "" || site.TLS.Type == state.TLSCertificateSelfSigned {
		// Self-signed
		site.TLS = &state.SiteTLS{
			Type:        state.TLSCertificateSelfSigned,
			Certificate: nil,
			Version:     nil,
		}
	} else if site.TLS.Type == state.TLSCertificateACME {
		// ACME
		site.TLS = &state.SiteTLS{
			Type:        state.TLSCertificateACME,
			Certificate: nil,
			Version:     nil,
		}
	} else if site.TLS.Type == state.TLSCertificateImported && site.TLS.Certificate != nil && *site.TLS.Certificate != "" {
		// Imported
		// Check if the certificate exists
		key, cert, err := state.Instance.GetCertificate(state.TLSCertificateImported, []string{*site.TLS.Certificate})
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
	} else if site.TLS.Type == state.TLSCertificateAzureKeyVault && site.TLS.Certificate != nil && *site.TLS.Certificate != "" {
		// Azure Key Vault
		// Check if the certificate exists
		exists, err := azurekeyvault.GetInstance().CertificateExists(*site.TLS.Certificate)
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
	} else {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "Invalid TLS configuration",
		})
		return
	}

	// Ensure an empty version is stored as nil
	if site.TLS != nil && site.TLS.Version != nil && *site.TLS.Version == "" {
		site.TLS.Version = nil
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

// ShowSiteHandler is the handler for GET /site/:domain, which shows a site
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

// DeleteSiteHandler is the handler for DELETE /site/:domain, which deletes a site
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

// PatchSiteHandler is the handler for PATCH /site/:domain, which replaces a site
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
	for k, v := range update {
		t := reflect.TypeOf(v)

		switch strings.ToLower(k) {
		case "tls":
			if t != nil && t.Kind() == reflect.Map {
				vMap := v.(map[string]interface{})
				certType, ok := vMap["type"].(string)
				if !ok {
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{
						"error": "Missing key tls.type",
					})
					return
				}
				switch certType {
				case state.TLSCertificateSelfSigned, state.TLSCertificateACME:
					// Self-signed certificate and ACME
					site.TLS = &state.SiteTLS{
						Type: certType,
					}
				case state.TLSCertificateImported:
					// Imported certificate
					// Get the certificate name
					name, ok := vMap["cert"].(string)
					if !ok || name == "" {
						c.AbortWithStatusJSON(http.StatusConflict, gin.H{
							"error": "Missing or invalid key tls.cert for imported certificate",
						})
						return
					}

					// Check if the certificate exists
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
					site.TLS = &state.SiteTLS{
						Type:        state.TLSCertificateImported,
						Certificate: &name,
					}
				case state.TLSCertificateAzureKeyVault:
					// Certificate stored in Azure Key Vault
					// Get the certificate name
					name, ok := vMap["cert"].(string)
					if !ok || name == "" {
						c.AbortWithStatusJSON(http.StatusConflict, gin.H{
							"error": "Missing or invalid key tls.cert for Azure Key Vault certificate",
						})
						return
					}

					// Get the certificate version, if any
					version, ok := vMap["ver"].(string)
					if !ok {
						version = ""
					}

					// Check if the certificate exists
					exists, err := azurekeyvault.GetInstance().CertificateExists(name)
					if err != nil {
						c.AbortWithError(http.StatusInternalServerError, err)
						return
					}
					if !exists {
						c.AbortWithStatusJSON(http.StatusConflict, gin.H{
							"error": "Certificate does not exist in Azure Key Vault",
						})
						return
					}
					site.TLS = &state.SiteTLS{
						Type:        state.TLSCertificateAzureKeyVault,
						Certificate: &name,
					}
					if version != "" {
						site.TLS.Version = &version
					}
				default:
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{
						"error": "Invalid TLS certificate type",
					})
					return
				}
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
