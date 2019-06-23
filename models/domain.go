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

	"github.com/gobuffalo/uuid"
)

// Domain is the model for a domain name
// There's a 1:many relationship between sites and domains
type Domain struct {
	// Built-in and required
	ID        uuid.UUID `json:"id" db:"id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`

	// Links
	Site   Site      `belongs_to:"site"`
	SiteID uuid.UUID `db:"site_id"`

	// Fields
	Domain    string `db:"domain"`
	IsDefault bool   `db:"is_default"`
}

// Domains is a list of domains
type Domains []Domain
