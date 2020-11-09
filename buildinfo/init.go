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

package buildinfo

import (
	"bytes"
	"io"
	"os"
)

// Destination for logs
var LogDestination io.ReadWriter = os.Stdout

func init() {
	// If there's an ENV passed via environmental variables, override the value hardcoded
	e := os.Getenv("GO_ENV")
	if e != "" {
		ENV = e
	}

	// If there's no ENV, fallback to development
	if ENV == "" {
		ENV = "development"
	}

	// In test environment, we're putting logs in a separate stream
	if ENV == "test" {
		LogDestination = &bytes.Buffer{}
	}
}
