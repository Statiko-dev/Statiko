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
	"fmt"
	"runtime"
)

// These variables will be set at build time
var (
	BuildID    string
	CommitHash string
	BuildTime  string
	ENV        string
)

// VersionString returns the app's version formatted as string
func VersionString() string {
	if BuildID == "" {
		return "canary " + runtime.Version()
	}
	return fmt.Sprintf("%s (%s; %s) %s", BuildID, CommitHash, BuildTime, runtime.Version())
}
