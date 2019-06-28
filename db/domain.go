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
	"github.com/gofrs/uuid"
	"github.com/jinzhu/gorm"
)

// Domain is the model for a domain name
// There's a 1:many relationship between sites and domains
type Domain struct {
	// ID
	DomainID uuid.UUID `gorm:"type:uuid;primary_key;"`

	// Links
	SiteID uuid.UUID `gorm:"type:uuid;index:site_id"`

	// Fields
	Domain    string `gorm:"unique_index:domain;not null"`
	IsDefault bool
}

// BeforeSave is executed before the object is saved in the database
func (d *Domain) BeforeSave(scope *gorm.Scope) error {
	// Generate the UUID
	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}
	scope.SetColumn("DomainID", uuid)

	return nil
}
