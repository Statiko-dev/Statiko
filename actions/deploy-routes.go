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
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"

	"smplatform/lib"
	"smplatform/models"
)

type deployRequest struct {
	AppName    string `json:"app"`
	AppVersion string `json:"version"`
}

// Updates the deployment status in the database
func (rts *Routes) updateDeployment(deployment *models.Deployment, deploymentError error) {
	status := models.DeploymentStatusDeployed

	// If there's an error, log it too
	if deploymentError != nil {
		status = models.DeploymentStatusFailed
		deployment.Error = nulls.NewString(deploymentError.Error())
		rts.log.Println("[updateDeployment] deployment failed with error", deploymentError)
	}

	// Update the database
	deployment.Status = status
	err := models.DB.Save(deployment)
	if err != nil {
		rts.log.Println("[updateDeployment] database update failed with error", err)
	}

	// Reset status cache
	rts.statusCache = nil
}

// Deploys an app asynchronously
func (rts *Routes) startAsyncDeployment(deployment *models.Deployment) {
	// Stage the app to the /approot/apps folder
	rts.log.Println("[startAsyncDeployment] staging app")
	if err := rts.appManager.StageApp(deployment.AppName, deployment.AppVersion); err != nil {
		rts.updateDeployment(deployment, err)
		return
	}

	// Create the symlink to enable the app
	rts.log.Println("[startAsyncDeployment] activating app")
	if err := rts.appManager.ActivateApp(deployment.AppName+"-"+deployment.AppVersion, deployment.SiteID.String()); err != nil {
		rts.updateDeployment(deployment, err)
		return
	}

	// Update the deployment status
	rts.log.Println("[startAsyncDeployment] updating status in database")
	rts.updateDeployment(deployment, nil)

	// No need to reload the server, as the configuration didn't change (and the symlink was updated atomically)
}

// DeployHandler is the handler for POST /site/{site}/deploy, which deploys a new app
func (rts *Routes) DeployHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	// Get data from the form body
	request := &deployRequest{}
	if err := c.Bind(request); err != nil {
		return c.Error(400, errors.New("Invalid request body"))
	}

	// Validate request
	if len(request.AppName) < 1 {
		return c.Error(400, errors.New("Missing parameter 'appName'"))
	}
	if len(request.AppVersion) < 1 {
		return c.Error(400, errors.New("Missing parameter 'appVersion'"))
	}

	// Get the site id
	site := c.Param("site")
	if len(site) < 1 {
		return c.Error(400, errors.New("Invalid parameter 'site'"))
	}

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

	// Get UUID
	siteUUID, err := uuid.FromString(site)
	if err != nil {
		return c.Error(400, errors.New("Invalid parameter 'site'"))
	}

	// Check if there's already a deployment running for the same site, or if the same app has already been deployed
	count, err := tx.Where("site_id = ? AND (status = ? OR (app_name = ? AND app_version = ? AND status = ?))", site, models.DeploymentStatusRunning, request.AppName, request.AppVersion, models.DeploymentStatusDeployed).Count(models.Deployment{})
	if err != nil {
		return err
	}

	if count > 0 {
		return c.Error(409, errors.New("Cannot deploy this app because there's already a deployment running for the target site, or the same app has already been deployed on the target site"))
	}

	// Create a new deployment
	deployment := &models.Deployment{
		SiteID:     siteUUID,
		AppName:    request.AppName,
		AppVersion: request.AppVersion,
		Status:     models.DeploymentStatusRunning,
	}
	if err := tx.Create(deployment); err != nil {
		return err
	}

	// Run the deployment in background
	go rts.startAsyncDeployment(deployment)

	// Re-map values to the desired JSON format
	deployment.RemapJSON()

	return c.Render(200, r.JSON(deployment))
}
