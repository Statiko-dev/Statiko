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
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/certificates/azurekeyvault"
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
		if site.EnableAcme {
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
	domains := append([]string{site.Domain}, site.Aliases...)
	for _, el := range domains {
		if s.State.GetSite(el) != nil {
			c.AbortWithStatusJSON(http.StatusConflict, gin.H{
				"error": "Domain or alias already exists",
			})
			return
		}
	}

	// Generate a TLS certificate
	certId, err := s.genTls(site)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	// When ACME is enabled, start the background process that requests a cert from ACME
	if site.EnableAcme {
		// TODO: THIS
		//defer state.Instance.TriggerRefreshCerts()
	}
	site.GeneratedTlsId = certId

	// Check if we have an imported certificate
	if site.ImportedTlsId != "" {
		// Check if the cert is from Azure Key Vault
		if strings.HasPrefix(strings.ToLower(site.ImportedTlsId), "akv:") {
			// Ensure we have a name and optionally a version
			pos := strings.Index(site.ImportedTlsId, "/")
			var name, version string
			if pos == -1 {
				name = site.ImportedTlsId[4:]
			} else {
				name = site.ImportedTlsId[4:pos]
				version = site.ImportedTlsId[(pos + 1):]
			}

			// If the version is empty (or "latest"), then get the latest version
			if version == "" || strings.ToLower(version) == "latest" {
				var err error
				version, err = azurekeyvault.GetInstance().GetCertificateLastVersion(name)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
					return
				}
				if version == "" {
					c.AbortWithStatusJSON(http.StatusConflict, gin.H{
						"error": "TLS certificate does not exist in Azure Key Vault",
					})
					return
				}
			} else {
				// Ensure that the certificate exists
				exists, err := azurekeyvault.GetInstance().CertificateExists(name)
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
			}

			// Retrieve the certificate so we can inspect it
			_, _, _, certX509, err := azurekeyvault.GetInstance().GetCertificate(name, version)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			if certX509 == nil {
				c.AbortWithStatusJSON(http.StatusConflict, gin.H{
					"error": "empty certificate properties returned by Azure Key Vault",
				})
				return
			}

			// Set the updated value
			site.ImportedTlsId = "akv:" + name + "/" + version
		} else {
			// This is a certificate in the state store
			// Ensure it exists and it's not self-signed or from ACME
			obj, err := s.State.GetCertificateInfo(site.ImportedTlsId)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Error with imported certificate: " + err.Error(),
				})
				return
			}
			if obj == nil || obj.Type == pb.TLSCertificate_SELF_SIGNED || obj.Type == pb.TLSCertificate_ACME {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Imported certificate not found or certificate is not an imported one",
				})
				return
			}
		}
	}

	// Ensure there's no App passed
	site.App = nil

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
	site := proto.Clone(s.State.GetSite(domain)).(*pb.Site)
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
	regenerateTls := false
	for k, v := range update {
		t := reflect.TypeOf(v)

		switch strings.ToLower(k) {
		// Enable/disable ACME
		case "enableacme", "enable_acme":
			enabled, ok := v.(bool)
			// Ignore invalid
			if !ok {
				break
			}
			// Temporary domains cannot use TLS certificates from ACME, to avoid rate limiting
			if site.Temporary && enabled {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Temporary sites cannot request TLS certificates from ACME",
				})
				return
			}
			site.EnableAcme = enabled
			updated = true
			regenerateTls = true
		// Imported TLS certificate ID
		case "importedtlsid", "imported_tls_id":
			certId, ok := v.(string)
			// Ignore invalid
			if !ok {
				break
			}

			// If we're setting a certificate, ensure it exists and it's not self-signed or from ACME
			if certId != "" {
				obj, err := s.State.GetCertificateInfo(certId)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
						"error": "Error with imported certificate: " + err.Error(),
					})
					return
				}
				if obj == nil || obj.Type == pb.TLSCertificate_SELF_SIGNED || obj.Type == pb.TLSCertificate_ACME {
					c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
						"error": "Imported certificate not found or certificate is not an imported one",
					})
					return
				}
			}
			site.ImportedTlsId = certId
			updated = true

			// If we're unsetting the certificate ID, then we must re-generate the self-signed cert
			if certId == "" {
				regenerateTls = true
			}

		// Aliases
		case "aliases":
			// Aliases can't be updated for temporary sites
			if site.Temporary {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Cannot set aliases for a temporary site",
				})
				return
			}
			// Reset the aliases slice
			site.Aliases = make([]string, 0)

			if t != nil && t.Kind() == reflect.Slice {
				updated = true
				regenerateTls = true

				// Check if the aliases exist already
				for _, a := range v.([]interface{}) {
					// Element must be a string
					str, ok := a.(string)
					if !ok {
						c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
							"error": "Invalid type for element in the aliases list",
						})
						return
					}

					// Aliases can't be defined somewhere else (but can be defined in this same site!)
					found := s.State.GetSite(str)
					if found != nil && found.Domain != site.Domain {
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

	// Regenerate the TLS certificate if needed
	if regenerateTls {
		certId, err := s.genTls(site)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		// When ACME is enabled, start the background process that requests a cert from ACME
		if site.EnableAcme {
			// TODO: THIS
			//defer state.Instance.TriggerRefreshCerts()
		}
		site.GeneratedTlsId = certId
	}

	// Update the site object if something has changed
	if updated {
		if err := s.State.UpdateSite(site); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
	}

	// Respond with the site
	c.JSON(http.StatusOK, site)
}

// Internal function that generates a new self-signed TLS certificate (also for ACME)
func (s *APIServer) genTls(site *pb.Site) (certId string, err error) {
	// Generate a TLS certificate, either self-signed or from ACME
	generatedTls := &pb.TLSCertificate{
		Type: pb.TLSCertificate_SELF_SIGNED,
	}
	if site.EnableAcme {
		generatedTls.Type = pb.TLSCertificate_ACME
	}

	// Generate a certificate ID
	u, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	certId = u.String()

	// Generate the new TLS certificate
	// Even for sites that use ACME, we still need a temporary self-signed certificate
	domains := append([]string{site.Domain}, site.Aliases...)
	key, cert, err := certificates.GenerateTLSCert(domains...)
	if err != nil {
		return "", err
	}

	// Get the x509 object to set the other properties
	certX509, err := certificates.GetX509(cert)
	if err != nil {
		return "", err
	}
	generatedTls.SetCertificateProperties(certX509)

	// Save the certificate
	err = s.State.SetCertificate(generatedTls, certId, key, cert)
	if err != nil {
		return "", err
	}

	return certId, nil
}
