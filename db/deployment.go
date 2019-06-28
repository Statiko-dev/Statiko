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

package db

import (
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"
)

// Deployment status enum
const (
	DeploymentStatusRunning  = -1
	DeploymentStatusFailed   = 0
	DeploymentStatusDeployed = 1
)

// Deployment is the model for a deployment of an application
type Deployment struct {
	// ID
	DeploymentID uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`

	// Links
	Site   Site      `json:"-" gorm:"foreignkey:SiteID;association_foreignkey:SiteID"`
	SiteID uuid.UUID `json:"siteId"`

	// Fields
	AppName    string    `json:"app"`
	AppVersion string    `json:"version"`
	Status     int       `json:"-"`
	Error      string    `json:"deploymentError"`
	Time       time.Time `json:"time"`

	// Alias for representing the status as string
	StatusStr string `json:"status" gorm:"-"`
}

// BeforeCreate is executed before the object is created
func (d *Deployment) BeforeCreate(scope *gorm.Scope) error {
	// Generate the UUID
	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}
	if err := scope.SetColumn("DeploymentID", uuid); err != nil {
		return err
	}

	return nil
}

// RemapJSON brings the output back to the structure passed as input
func (d *Deployment) RemapJSON() error {
	// Convert the status from code to string
	switch d.Status {
	case DeploymentStatusRunning:
		d.StatusStr = "running"
	case DeploymentStatusFailed:
		d.StatusStr = "failed"
	case DeploymentStatusDeployed:
		d.StatusStr = "done"
	default:
		return fmt.Errorf("Invalid deployment status code found: %d", d.Status)
	}

	return nil
}
