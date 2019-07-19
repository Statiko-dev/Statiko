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

package state

import (
	"log"
	"os"
)

// Instance is a singleton for Manager
var Instance *Manager

// Logger
var logger *log.Logger

// Init the singleton
func init() {
	// Initialize the logger
	logger = log.New(os.Stdout, "state: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the singleton
	Instance = &Manager{}
	if err := Instance.Init(); err != nil {
		panic(err)
	}
}
