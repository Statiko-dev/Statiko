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

package statuscheck

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"
)

// Package-wide properties
var (
	// Interval (in seconds) between tests
	StatusCheckInterval int = 300

	// Last time the state was updated
	stateUpdatedTime *time.Time

	// Cached health data
	healthCache []SiteHealth

	// Last time the health checks were run
	appTestedTime time.Time

	// List sites we sent a notification for, to avoid spamming admins
	notificationsSent map[string]string

	// Logger
	logger *log.Logger

	// Client for HTTP requests
	httpClient *http.Client
)

// Init method for the package
func init() {
	// Initialize the logger
	logger = log.New(os.Stdout, "statuscheck: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the HTTP client that will be used for monitoring
	// Very short TTL as requests are made to the same server
	// Additionally, disables validation of TLS certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient = &http.Client{
		Transport: tr,
		Timeout:   1500 * time.Millisecond,
	}

	// Initialize the map with notifications sent
	notificationsSent = make(map[string]string)
}
