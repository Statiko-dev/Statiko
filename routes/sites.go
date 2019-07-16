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

	"smplatform/state"
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
	// Get records from the state object
	sites := state.Instance.GetSites()

	c.JSON(http.StatusOK, sites)
}

// ShowSiteHandler is the handler for GET /site/{site}, which shows a site
// The `site` parameter can be a site id (GUID) or a domain
func ShowSiteHandler(c *gin.Context) {
	if site := c.Param("site"); len(site) > 0 {
		// Get the site from the state object
		site := state.Instance.GetSite(site)
		if site == nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{
				"error": "Domain name not found",
			})
			return
		}

		c.JSON(http.StatusOK, site)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
	}
}

// DeleteSiteHandler is the handler for DELETE /site/{site}, which deletes a site
// The `site` parameter can be a site id (GUID) or a domain
func DeleteSiteHandler(c *gin.Context) {
	if site := c.Param("site"); len(site) > 0 {
		// Delete the record
		err := state.Instance.DeleteSite(site)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}

		// Queue a sync
		sync.QueueRun()

		c.Status(http.StatusNoContent)
	} else {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
	}
}

// TODO: Route to update a site's configuration
