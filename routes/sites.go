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

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"

	"smplatform/appmanager"
	"smplatform/db"
	"smplatform/state"
	"smplatform/utils"
	"smplatform/webserver"
	"smplatform/sync"
)

// CreateSiteInput is the message clients send to create a site
type CreateSiteInput struct {
	Domain         string   `json:"domain"`
	Aliases        []string `json:"aliases"`
	ClientCaching  bool     `json:"clientCaching"`
	TLSCertificate *string  `json:"tlsCertificate"`
}

// CreateSiteHandler is the handler for POST /site, which creates a new site
// @Summary Creates a new site
// @Description Creates a new site in the local web server and configures it with the default app
// @Accept json
// @Produce json
// @Param domain body string true "Domain name" minimum(1)
// @Param tlsCertificate body string true "TLS Certificate name in the Key Vault" minimum(1)
// @Failure 500
// @Router /site [post]
func CreateSiteHandler(c *gin.Context) {
	// Get data from the form body
	site := &CreateSiteInput{}
	if err := c.Bind(site); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if len(site.Domain) < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "You must specify the 'domain' key",
		})
		return
	}
	if site.Domain == "_default" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Cannot use '_default' as domain name",
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

	// Add the website to the store
	err := state.Instance.AddSite(&state.SiteState{
		Domain:         site.Domain,
		Aliases:        site.Aliases,
		ClientCaching:  site.ClientCaching,
		TLSCertificate: site.TLSCertificate,
	})
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
	// Get records from the database
	sites := []db.Site{}
	if err := db.Connection.Preload("Domains").Find(&sites).Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Re-map to the original JSON structure
	for i := 0; i < len(sites); i++ {
		sites[i].RemapJSON()
	}

	c.JSON(http.StatusOK, sites)
}

// ShowSiteHandler is the handler for GET /site/{site}, which shows a site
// The `site` parameter can be a site id (GUID) or a domain
func ShowSiteHandler(c *gin.Context) {
	if site := c.Param("site"); len(site) > 0 {
		// If site is a domain name, we need to load the site ID first
		if !utils.IsValidUUID(site) {
			domain := &db.Domain{}
			err := db.Connection.Where("domain = ?", site).First(domain).Error
			if err != nil {
				// Check if the error is because of the record not found
				if gorm.IsRecordNotFoundError(err) {
					c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
						"error": "Domain name not found",
					})
					return
				}
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			site = domain.SiteID.String()
		}

		// Load the record and perform the joins
		result := &db.Site{}
		err := db.Connection.Preload("Domains").Where("site_id = ?", site).First(result).Error
		if err != nil {
			// Check if the error is because of the record not found
			if gorm.IsRecordNotFoundError(err) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
					"error": "Domain name not found",
				})
				return
			}
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Re-map to the original JSON structure
		result.RemapJSON()
		c.JSON(http.StatusOK, result)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
	}
}

// DeleteSiteHandler is the handler for DELETE /site/{site}, which deletes a site
// The `site` parameter can be a site id (GUID) or a domain
func DeleteSiteHandler(c *gin.Context) {
	// DB transaction
	tx := db.Connection.Begin()
	defer func() {
		if tx != nil {
			// Rollback automatically in case of error
			tx.Rollback()
		}
	}()

	if site := c.Param("site"); len(site) > 0 {
		// If site is a domain name, we need to load the site ID first
		if !utils.IsValidUUID(site) {
			domain := &db.Domain{}
			err := db.Connection.Where("domain = ?", site).First(domain).Error
			if err != nil {
				// Check if the error is because of the record not found
				if gorm.IsRecordNotFoundError(err) {
					c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
						"error": "Domain name not found",
					})
					return
				}
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}

			// We can allow the deletion only using the primary domain
			// This is to avoid situations where users are just trying to delete an alias
			if !domain.IsDefault {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "Cannot remove a site using an alias",
				})
				return
			}

			site = domain.SiteID.String()
		}

		// Remove the nginx configuration, then reload the server's config
		if err := webserver.Instance.RemoveSite(site); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		if err := webserver.Instance.RestartServer(); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Delete data from the file system
		if err := appmanager.Instance.RemoveFolders(site); err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Remove records from the database
		var query *gorm.DB

		// First, delete the domains
		query = tx.Raw("DELETE FROM domains WHERE site_id = ?", site)
		if query.Error != nil {
			c.AbortWithError(http.StatusInternalServerError, query.Error)
			return
		}
		if query.RowsAffected < 1 {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Site id not found",
			})
			return
		}

		// Delete all deployments (if any)
		query = tx.Raw("DELETE FROM deployments WHERE site_id = ?", site)
		if query.Error != nil {
			c.AbortWithError(http.StatusInternalServerError, query.Error)
			return
		}

		// Delete the record
		// Note that we're still running within a database transaction
		query = tx.Raw("DELETE FROM sites WHERE id = ?", site)
		if query.Error != nil {
			c.AbortWithError(http.StatusInternalServerError, query.Error)
			return
		}
		if query.RowsAffected < 1 {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Site id not found",
			})
			return
		}

		// Commit
		if err := tx.Commit().Error; err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		tx = nil

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
	}
}
