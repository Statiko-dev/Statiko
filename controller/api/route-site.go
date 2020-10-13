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
	"fmt"
	"math/rand"
	"net/http"
	"reflect"
	"strings"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/certificates/azurekeyvault"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// CreateSiteHandler is the handler for POST /site, which creates a new site
func (s *APIServer) CreateSiteHandler(c *gin.Context) {
	// Get data from the form body
	site := &pb.Site{}
	if err := c.BindJSON(site); err != nil {
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
		if site.Tls != nil && site.Tls.Type == pb.State_Site_TLS_ACME {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Temporary sites cannot request TLS certificates from ACME",
			})
			return
		}
		site.Domain = fmt.Sprintf("%s-%d%s", petname.Generate(3, "-"), (rand.Intn(899) + 100), tempDomain)
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
		if s.State.GetSite(el) != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "Domain or alias already exists",
			})
			return
		}
	}

	tlsType := pb.State_Site_TLS_NULL
	if site.Tls != nil {
		tlsType = site.Tls.GetType()
	}
	switch tlsType {
	case pb.State_Site_TLS_ACME:
		// ACME
		site.Tls = &pb.State_Site_TLS{
			Type:        pb.State_Site_TLS_ACME,
			Certificate: "",
			Version:     "",
		}
	case pb.State_Site_TLS_IMPORTED:
		// Imported
		if site.Tls.Certificate == "" {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "Missing name for imported TLS certificate",
			})
			return
		}
		// Check if the certificate exists
		key, cert, err := s.State.GetCertificate(pb.State_Site_TLS_IMPORTED, []string{site.Tls.Certificate})
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
	case pb.State_Site_TLS_AZURE_KEY_VAULT:
		// Azure Key Vault
		if site.Tls.Certificate == "" {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "Missing name for TLS certificate from Azure Key Vault",
			})
			return
		}
		// Check if the certificate exists
		exists, err := azurekeyvault.GetInstance().CertificateExists(site.Tls.Certificate)
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
	case pb.State_Site_TLS_NULL, pb.State_Site_TLS_SELF_SIGNED:
		// Self-signed TLS certificates are default when no value is specified
		site.Tls = &pb.State_Site_TLS{
			Type:        pb.State_Site_TLS_SELF_SIGNED,
			Certificate: "",
			Version:     "",
		}
	default:
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "Invalid TLS configuration",
		})
		return
	}

	// Add the website to the store
	if err := s.State.AddSite(site); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Respond with the site
	c.JSON(http.StatusOK, site)
}

// ListSiteHandler is the handler for GET /site, which lists all sites
func (s *APIServer) ListSiteHandler(c *gin.Context) {
	// Get records from the state object
	sites := s.State.GetSites()

	c.JSON(http.StatusOK, sites)
}

// ShowSiteHandler is the handler for GET /site/:domain, which shows a site
func (s *APIServer) ShowSiteHandler(c *gin.Context) {
	if domain := c.Param("domain"); len(domain) > 0 {
		// If we're getting a temporary site, add the domain automatically
		if utils.IsTruthy(c.Query("temporary")) {
			// Ensure a domain is set
			tempDomain := appconfig.Config.GetString("temporarySites.domain")
			if tempDomain == "" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Configuration option `temporarySites.domain` must be set before requesting a temporary site",
				})
				return
			}
			if tempDomain[0] != '.' {
				// Ensure there's a dot at the beginning
				tempDomain = "." + tempDomain
			}
			// Append the temporary domain
			domain += tempDomain
		}

		// Get the site from the state object
		site := s.State.GetSite(domain)
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
func (s *APIServer) DeleteSiteHandler(c *gin.Context) {
	if domain := c.Param("domain"); len(domain) > 0 {
		// If we're getting a temporary site, add the domain automatically
		if utils.IsTruthy(c.Query("temporary")) {
			// Ensure a domain is set
			tempDomain := appconfig.Config.GetString("temporarySites.domain")
			if tempDomain == "" {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error": "Configuration option `temporarySites.domain` must be set before requesting a temporary site",
				})
				return
			}
			if tempDomain[0] != '.' {
				// Ensure there's a dot at the beginning
				tempDomain = "." + tempDomain
			}
			// Append the temporary domain
			domain += tempDomain
		}

		// Get the site from the state object to check if it exists
		if site := s.State.GetSite(domain); site == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Domain name not found",
			})
			return
		}

		// Delete the record
		if err := s.State.DeleteSite(domain); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
	}
}

// PatchSiteHandler is the handler for PATCH /site/:domain, which replaces a site
func (s *APIServer) PatchSiteHandler(c *gin.Context) {
	// Get the site to update (domain name)
	domain := c.Param("domain")
	if len(domain) == 0 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'domain'",
		})
		return
	}

	// If we're getting a temporary site, add the domain automatically
	if utils.IsTruthy(c.Query("temporary")) {
		// Ensure a domain is set
		tempDomain := appconfig.Config.GetString("temporarySites.domain")
		if tempDomain == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": "Configuration option `temporarySites.domain` must be set before requesting a temporary site",
			})
			return
		}
		if tempDomain[0] != '.' {
			// Ensure there's a dot at the beginning
			tempDomain = "." + tempDomain
		}
		// Append the temporary domain
		domain += tempDomain
	}

	// Get the site from the state object
	site := s.State.GetSite(domain)
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
				case pb.State_Site_TLS_ACME.String():
					if site.Temporary {
						c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
							"error": "Temporary sites cannot request TLS certificates from ACME",
						})
						return
					}
					site.Tls = &pb.State_Site_TLS{
						Type: pb.State_Site_TLS_ACME,
					}
				case pb.State_Site_TLS_SELF_SIGNED.String():
					site.Tls = &pb.State_Site_TLS{
						Type: pb.State_Site_TLS_SELF_SIGNED,
					}
				case pb.State_Site_TLS_IMPORTED.String():
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
					key, cert, err := s.State.GetCertificate(pb.State_Site_TLS_IMPORTED, []string{name})
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
					site.Tls = &pb.State_Site_TLS{
						Type:        pb.State_Site_TLS_IMPORTED,
						Certificate: name,
					}
				case pb.State_Site_TLS_AZURE_KEY_VAULT.String():
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
					site.Tls = &pb.State_Site_TLS{
						Type:        pb.State_Site_TLS_AZURE_KEY_VAULT,
						Certificate: name,
						Version:     version,
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
			// Aliases can't be updated for temporary sites
			if site.Temporary {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Cannot set aliases for a temporary site",
				})
				return
			}
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
					ok := s.State.GetSite(str)
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
		if err := s.State.UpdateSite(site, true); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	// Respond with the site
	c.JSON(http.StatusOK, site)
}
