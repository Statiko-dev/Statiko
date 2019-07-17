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

	// Iterate through the SiteState interface and see what fields can be updated
	t := reflect.TypeOf(state.SiteState{})

	// List of fields that we can update
	// Note that this doesn't include the Aliases field by design
	updateable := make(map[string]string)
	types := make(map[string]reflect.Type)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		patchTag, okP := field.Tag.Lookup("patch")
		jsonTag, okJ := field.Tag.Lookup("json")
		if okJ && okP && patchTag == "yes" {
			updateable[jsonTag] = field.Name
			types[jsonTag] = field.Type
		}
	}

	// Iterate through the fields in the input and update site
	siteValue := reflect.ValueOf(site).Elem()
	updated := false
	for k, v := range update {
		// Check if this field can be updated
		upd, ok := updateable[k]
		typ := types[k]
		if !ok {
			continue
		}

		// Update if it's the right type
		vType := reflect.TypeOf(v)
		vTypePtr := reflect.PtrTo(vType)
		if typ == vType {
			// Right type
			set := siteValue.FieldByName(upd)
			vVal := reflect.ValueOf(v)
			set.Set(vVal)
			updated = true
		} else if typ == vTypePtr {
			// Pointer to the type
			set := siteValue.FieldByName(upd)

			switch typ.String() {
			case "*string":
				str := v.(string)
				vVal := reflect.ValueOf(&str)
				set.Set(vVal)
				updated = true
			}
		} else {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
				"error": "Invalid type for: " + k,
			})
			return
		}
	}

	// TODO: check updated aliases

	if updated {
		state.Instance.UpdateSite(site, true)

		// Queue a sync
		sync.QueueRun()
	}

	c.JSON(http.StatusOK, site)
}
