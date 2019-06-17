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

package actions

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"time"

	"smplatform/lib"
	"smplatform/models"
	"smplatform/webserver"
)

// Message is a struct for a response message
type Message struct {
	Message string `json:"message"`
}

// Routes is the main struct containing all route handlers
type Routes struct {
	appManager  *lib.Manager
	ngConfig    *webserver.NginxConfig
	log         *log.Logger
	statusCache *NodeStatus
	httpClient  *http.Client
}

// Init itializes the object
func (rts *Routes) Init() error {
	// Initialize the logger
	rts.log = log.New(os.Stdout, "routes: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the HTTP client that will be used for monitoring
	// Very short TTL as requests are made to the same server
	// Additionally, disables validation of TLS certificates
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	rts.httpClient = &http.Client{
		Transport: tr,
		Timeout:   1 * time.Second,
	}

	// Initialize the ngConfig parameter
	rts.ngConfig = &webserver.NginxConfig{}
	if err := rts.ngConfig.Init(); err != nil {
		return err
	}

	// Initialize the webapp manager parameter
	rts.appManager = &lib.Manager{}
	if err := rts.appManager.Init(); err != nil {
		return err
	}

	// At startup, we need to cancel all pending deployments in the database (because the app that was processing them is dead!)
	rts.removePendingDeployments()

	// Also at startup, sync the system's state
	rts.syncState()

	return nil
}

// Removes all pending deployments from the database, marking them as failed
func (rts *Routes) removePendingDeployments() {
	count, err := models.DB.RawQuery("UPDATE deployments SET status = ? WHERE status = ?", models.DeploymentStatusFailed, models.DeploymentStatusRunning).ExecWithCount()
	if err != nil {
		// On the very first launch, the table might not exist
		if err.Error() == "no such table: deployments" {
			rts.log.Println("[removePendingDeployments] Datatabase has not been initialized yet")
		} else {
			// Otherwise, return
			rts.log.Fatalln("[removePendingDeployments] Error while updating pending deployments in database", err)
			return
		}
	}
	if count > 0 {
		rts.log.Printf("[removePendingDeployments] Canceled %d pending deployments\n", count)
	}
}

// Ensures the system is in the correct state
// For now, this is limited to ensuring the configuration for Nginx is correct
func (rts *Routes) syncState() {
	// Get records from the database
	sites := []models.Site{}
	if err := models.DB.Eager().All(&sites); err != nil {
		rts.log.Fatalln("[syncState] Error while querying database", err)
		return
	}

	// Re-map to the original JSON structure
	for i := 0; i < len(sites); i++ {
		sites[i].RemapJSON()
	}

	// Sync all sites
	if err := rts.ngConfig.SyncConfiguration(sites); err != nil {
		rts.log.Fatalln("[syncState] Error while syncing Nginx configuration", err)
		return
	}

	// Restart Nginx server
	if err := rts.ngConfig.RestartServer(); err != nil {
		rts.log.Fatalln("[syncState] Error while restarting Nginx", err)
		return
	}
}
