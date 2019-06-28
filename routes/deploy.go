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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"

	"smplatform/appmanager"
	"smplatform/db"
	"smplatform/utils"
)

type deployRequest struct {
	AppName    string `json:"app"`
	AppVersion string `json:"version"`
}

// Updates the deployment status in the database
func updateDeployment(deployment *db.Deployment, deploymentError error) {
	status := db.DeploymentStatusDeployed

	// If there's an error, log it too
	if deploymentError != nil {
		status = db.DeploymentStatusFailed
		deployment.Error = new(string)
		*deployment.Error = deploymentError.Error()
		logger.Println("[updateDeployment] deployment failed with error", deploymentError)
	}

	// Update the database
	now := time.Now()
	deployment.Time = &now
	deployment.Status = status
	err := db.Connection.Save(deployment).Error
	if err != nil {
		logger.Println("[updateDeployment] database update failed with error", err)
	}

	// Reset status cache
	statusCache = nil
}

// Deploys an app asynchronously
func startAsyncDeployment(deployment *db.Deployment) {
	// Stage the app to the /approot/apps folder
	logger.Println("[startAsyncDeployment] staging app")
	if err := appmanager.Instance.StageApp(deployment.AppName, deployment.AppVersion); err != nil {
		updateDeployment(deployment, err)
		return
	}

	// Create the symlink to enable the app
	logger.Println("[startAsyncDeployment] activating app")
	if err := appmanager.Instance.ActivateApp(deployment.AppName+"-"+deployment.AppVersion, deployment.SiteID.String()); err != nil {
		updateDeployment(deployment, err)
		return
	}

	// Update the deployment status
	logger.Println("[startAsyncDeployment] updating status in database")
	updateDeployment(deployment, nil)

	// No need to reload the server, as the configuration didn't change (and the symlink was updated atomically)
}

// DeployHandler is the handler for POST /site/{site}/deploy, which deploys a new app
func DeployHandler(c *gin.Context) {
	// DB transaction
	tx := db.Connection.Begin()
	defer func() {
		if tx != nil {
			// Rollback automatically in case of error
			tx.Rollback()
		}
	}()

	// Get data from the form body
	request := &deployRequest{}
	if err := c.Bind(request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	// Validate request
	if len(request.AppName) < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing parameter 'appName'",
		})
		return
	}
	if len(request.AppVersion) < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Missing parameter 'appVersion'",
		})
		return
	}

	// Get the site id
	site := c.Param("site")
	if len(site) < 1 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
		return
	}

	// If site is a domain name, we need to load the site ID first
	if !utils.IsValidUUID(site) {
		domain := &db.Domain{}
		err := tx.Where("domain = ?", site).First(domain).Error
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

	// Get UUID
	siteUUID, err := uuid.FromString(site)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"error": "Invalid parameter 'site'",
		})
		return
	}

	// Check if there's already a deployment running for the same site, or if the same app has already been deployed
	var count int
	err = tx.Model(&db.Deployment{}).Where("site_id = ? AND (status = ? OR (app_name = ? AND app_version = ? AND status = ?))", site, db.DeploymentStatusRunning, request.AppName, request.AppVersion, db.DeploymentStatusDeployed).Count(&count).Error
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	if count > 0 {
		c.AbortWithStatusJSON(http.StatusConflict, gin.H{
			"error": "Cannot deploy this app because there's already a deployment running for the target site, or the same app has already been deployed on the target site",
		})
		return
	}

	// Create a new deployment
	deployment := &db.Deployment{
		SiteID:     siteUUID,
		AppName:    request.AppName,
		AppVersion: request.AppVersion,
		Status:     db.DeploymentStatusRunning,
	}
	if err := tx.Create(deployment).Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Run the deployment in background
	go startAsyncDeployment(deployment)

	// Commit
	if err := tx.Commit().Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	tx = nil

	// Re-map values to the desired JSON format
	deployment.RemapJSON()

	c.JSON(http.StatusOK, deployment)
}
