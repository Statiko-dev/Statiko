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

package utils

import (
	"strings"
)

// HeaderIsAllowed returns true if a header is allowed as response header
func HeaderIsAllowed(name string) bool {
	// Lowercase the name
	name = strings.ToLower(name)

	// Allow all X-* headers, and a list of safe headers
	// Cache-related headers and redirect headers are allowed even though they can also be set with a separate option
	if name[0:2] == "x-" ||
		name == "expires" ||
		name == "cache-control" ||
		name == "content-disposition" ||
		name == "content-encoding" ||
		name == "content-language" ||
		name == "content-md5" ||
		name == "content-security-policy" ||
		name == "content-type" ||
		name == "last-modified" ||
		name == "link" ||
		name == "location" ||
		name == "p3p" ||
		name == "pragma" ||
		name == "refresh" ||
		name == "set-cookie" ||
		name == "vary" ||
		name == "warning" {
		return true
	}
	return false
}
