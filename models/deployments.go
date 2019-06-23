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

package models

import (
	"fmt"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/uuid"
)

// Deployment status enum
const (
	DeploymentStatusRunning  = -1
	DeploymentStatusFailed   = 0
	DeploymentStatusDeployed = 1
)

// Deployment is the model for a deployment of an application
type Deployment struct {
	// Built-in and required
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// Links
	Site   Site      `json:"-" belongs_to:"site"`
	SiteID uuid.UUID `json:"site" db:"site_id"`

	// Fields
	AppName    string       `json:"app" db:"app_name"`
	AppVersion string       `json:"version" db:"app_version"`
	Status     int          `json:"-" db:"status"`
	Error      nulls.String `json:"deploymentError" db:"error"`

	// Alias for representing the status as string
	StatusStr string `json:"status" db:"-"`
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
