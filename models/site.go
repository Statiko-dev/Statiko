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
	"time"

	"github.com/gobuffalo/pop"
	"github.com/gobuffalo/uuid"
)

// Site is the model for a site definition
type Site struct {
	// Built-in and required
	ID        uuid.UUID `db:"id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// Fields
	ClientCaching  bool   `json:"clientCaching" db:"client_caching"`
	TLSCertificate string `json:"tlsCertificate" db:"tls_certificate"`

	// These are input fields (from the JSON) but not stored in the database as is
	Domain  string   `json:"domain" db:"-"`
	Aliases []string `json:"aliases" db:"-"` // Not stored in the DB

	// Domains
	Domains Domains `json:"-" has_many:"domains" fk_id:"site_id"`
}

// BeforeSave is executed before the object is saved in the database
func (s *Site) BeforeSave(tx *pop.Connection) error {
	// Populate the Domain list
	s.Domains = make(Domains, 1+len(s.Aliases))
	for i := -1; i < len(s.Aliases); i++ {
		if i == -1 {
			// Default domain
			domain := Domain{
				Domain:    s.Domain,
				IsDefault: true,
			}
			s.Domains[0] = domain
		} else {
			// Aliases
			domain := Domain{
				Domain:    s.Aliases[i],
				IsDefault: false,
			}
			s.Domains[i+1] = domain
		}
	}

	return nil
}

// RemapJSON brings the output back to the structure passed as input
// We cannot use AfterFind because that's invoked before the joins are executed
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
