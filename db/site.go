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

// Site is the model for a site definition
type Site struct {
	// ID
	SiteID uuid.UUID `json:"id" gorm:"type:uuid;primary_key;"`

	// Fields
	ClientCaching  bool   `json:"clientCaching" gorm:"client_caching"`
	TLSCertificate string `json:"tlsCertificate" gorm:"tls_certificate"`

	// These are input fields (from the JSON) but not stored in the database as is
	Domain  string   `json:"domain" gorm:"-"`  // Not stored in the DB
	Aliases []string `json:"aliases" gorm:"-"` // Not stored in the DB

	// Domains
	Domains []Domain `json:"-" gorm:"foreignkey:SiteID;association_foreignkey:SiteID"`
}

// BeforeCreate is executed before the object is created
func (s *Site) BeforeCreate(scope *gorm.Scope) error {
	// Generate the UUID
	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}
	if err := scope.SetColumn("SiteID", uuid); err != nil {
		return err
	}

	return nil
}

// BeforeSave is executed before the object is saved (created or updated) in the database
func (s *Site) BeforeSave(scope *gorm.Scope) error {
	// Populate the Domain list
	domains := make([]Domain, 1+len(s.Aliases))
	for i := -1; i < len(s.Aliases); i++ {
		if i == -1 {
			// Default domain
			domain := Domain{
				Domain:    s.Domain,
				IsDefault: true,
			}
			domains[0] = domain
		} else {
			// Aliases
			domain := Domain{
				Domain:    s.Aliases[i],
				IsDefault: false,
			}
			domains[i+1] = domain
		}
	}
	if err := scope.SetColumn("Domains", domains); err != nil {
		return err
	}

	return nil
}

// RemapJSON brings the output back to the structure passed as input
func (s *Site) RemapJSON() error {
	// Re-map to "domain" (default) and "aliases"
	size := len(s.Domains) - 1
	if size < 0 {
		size = 0
	}
	s.Aliases = make([]string, size)
	append := 0
	for i := 0; i < len(s.Domains); i++ {
		if s.Domains[i].IsDefault {
			s.Domain = s.Domains[i].Domain
		} else {
			s.Aliases[append] = s.Domains[i].Domain
			append++
		}
	}

	return nil
}
