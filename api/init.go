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

package api

import (
	"log"
	"net/http"
	"os"
	"time"
)

// Package-wide properties
var (
	logger     *log.Logger
	httpClient *http.Client
	Server     *APIServer
)

// Init method for the package
func init() {
	// Initialize the logger
	logger = log.New(os.Stdout, "routes: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the HTTP Client
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	// Initialize the API server
	Server = &APIServer{}
	Server.Init()
}
