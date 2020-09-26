/*
Copyright © 2020 Alessandro Segala (@ItalyPaleAle)

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

package proto

import "github.com/statiko-dev/statiko/utils"

// This file contains additional methods added to the protobuf object

// GetSite searches the list of sites to return the one matching the requested domain (including aliases)
func (x *State) GetSite(domain string) *State_Site {
	sites := x.GetSites()
	if sites == nil {
		return nil
	}

	for _, s := range sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			return s
		}
	}

	return nil
}
