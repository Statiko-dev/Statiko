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

package actions

import (
	"database/sql"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop"
	"github.com/pkg/errors"

	"smplatform/lib"
	"smplatform/models"
)

// CreateSiteHandler is the handler for POST /site, which creates a new site
// @Summary Creates a new site
// @Description Creates a new site in the local web server and configures it with the default app
// @Accept json
// @Produce json
// @Param domain body string true "Domain name" minimum(1)
// @Param tlsCertificate body string true "TLS Certificate name in the Key Vault" minimum(1)
// @Failure 500
// @Router /site [post]
func (rts *Routes) CreateSiteHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	// Get data from the form body
	site := &models.Site{}
	if err := c.Bind(site); err != nil {
		return c.Error(400, errors.New("Invalid request body"))
	}

	if len(site.Domain) < 1 {
		return c.Error(400, errors.New("You must specify the 'domain' key"))
	}
	if site.Domain == "_default" {
		return c.Error(400, errors.New("Cannot use '_default' as domain name"))
	}
	if len(site.TLSCertificate) < 1 {
		return c.Error(400, errors.New("You must specify the 'tlsCertificate' key"))
	}

	// Check if site exists already
	domains := make([]string, len(site.Aliases)+1)
	copy(domains, site.Aliases)
	domains[len(site.Aliases)] = site.Domain
	query := tx.Where("domain in (?)", domains)
	count, err := query.Count(&models.Domain{})
	if err != nil {
		return err
	}

	if count > 0 {
		return c.Error(409, errors.New("Domain or alias already exists"))
	}

	// Save the website
	// We're in a transaction, so it something fails it will be deleted
	if err := tx.Eager().Create(site); err != nil {
		return err
	}

	// Create the site's configuration and folders
	siteID := site.ID.String()
	if err := rts.ngConfig.ConfigureSite(site); err != nil {
		return err
	}
	if err := rts.appManager.CreateFolders(siteID); err != nil {
		// Rollback the previous step (ignoring errors)
		rts.ngConfig.RemoveSite(siteID)

		return err
	}

	// Get the TLS certificate
	if err := rts.appManager.GetTLSCertificate(siteID, site.TLSCertificate); err != nil {
		// If this failed, delete the Nginx's configuration for the site as that won't be rolled back automatically
		// Ignore errors in these steps
		rts.ngConfig.RemoveSite(siteID)
		rts.appManager.RemoveFolders(siteID)

		return err
	}

	// Reload the Nginx configuration
	if err := rts.ngConfig.RestartServer(); err != nil {
		// Likewise, rollback the changes on the filesystem
		rts.ngConfig.RemoveSite(siteID)
		rts.appManager.RemoveFolders(siteID)

		return err
	}

	// Reset status cache
	rts.statusCache = nil

	site.RemapJSON()
	return c.Render(200, r.JSON(site))
}

// ListSiteHandler is the handler for GET /site, which lists all sites
func (rts *Routes) ListSiteHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	// Get records from the database
	sites := []models.Site{}
	if err := tx.Eager().All(&sites); err != nil {
		return err
	}

	// Re-map to the original JSON structure
	for i := 0; i < len(sites); i++ {
		sites[i].RemapJSON()
	}

	return c.Render(200, r.JSON(sites))
}

// ShowSiteHandler is the handler for GET /site/{site}, which shows a site
// The `site` parameter can be a site id (GUID) or a domain
func (rts *Routes) ShowSiteHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	if site := c.Param("site"); len(site) > 0 {
		// If site is a domain name, we need to load the site ID first
		if !lib.IsValidUUID(site) {
			domain := &models.Domain{}
			err := tx.Where("domain = ?", site).First(domain)
			if err != nil {
				if errors.Cause(err) == sql.ErrNoRows {
					return c.Error(404, errors.New("Domain name not found"))
				}

				return err
			}

			site = domain.SiteID.String()
		}

		// Load the record and perform the joins
		result := &models.Site{}
		err := tx.Eager().Find(result, site)
		if err != nil {
			if errors.Cause(err) == sql.ErrNoRows {
				return c.Error(404, errors.New("Site id not found"))
			}

			return err
		}

		// Re-map to the original JSON structure
		result.RemapJSON()
		return c.Render(200, r.JSON(result))
	}

	return c.Error(400, errors.New("Invalid parameter 'site'"))
}

// DeleteSiteHandler is the handler for DELETE /site/{site}, which deletes a site
// The `site` parameter can be a site id (GUID) or a domain
func (rts *Routes) DeleteSiteHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	if site := c.Param("site"); len(site) > 0 {
		// If site is a domain name, we need to load the site ID first
		if !lib.IsValidUUID(site) {
			domain := &models.Domain{}
			err := tx.Where("domain = ?", site).First(domain)
			if err != nil {
				if errors.Cause(err) == sql.ErrNoRows {
					return c.Error(404, errors.New("Domain name not found"))
				}

				return err
			}

			// We can allow the deletion only using the primary domain
			// This is to avoid situations where users are just trying to delete an alias
			if !domain.IsDefault {
				return c.Error(400, errors.New("Cannot remove a site using an alias"))
			}

			site = domain.SiteID.String()
		}

		// Remove the nginx configuration, then reload the server's config
		if err := rts.ngConfig.RemoveSite(site); err != nil {
			return err
		}
		if err := rts.ngConfig.RestartServer(); err != nil {
			return err
		}

		// Delete data from the file system
		if err := rts.appManager.RemoveFolders(site); err != nil {
			return err
		}

		// Remove records from the database
		var query *pop.Query
		var count int
		var err error

		// First, delete the domains
		query = tx.RawQuery("DELETE FROM domains WHERE site_id = ?", site)
		count, err = query.ExecWithCount()
		if err != nil {
			return err
		}
		if count < 1 {
			return c.Error(404, errors.New("Site id not found"))
		}

		// Delete all deployments (if any)
		query = tx.RawQuery("DELETE FROM deployments WHERE site_id = ?", site)
		_, err = query.ExecWithCount()
		if err != nil {
			return err
		}

		// Delete the record
		// Note that we're still running within a database transaction
		query = tx.RawQuery("DELETE FROM sites WHERE id = ?", site)
		count, err = query.ExecWithCount()
		if err != nil {
			return err
		}
		if count < 1 {
			return c.Error(404, errors.New("Site id not found"))
		}

		// Reset status cache
		rts.statusCache = nil

		return c.Render(204, nil)
	}

	return c.Error(400, errors.New("Invalid parameter 'site'"))
}
