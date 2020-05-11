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

package sync

import (
	"time"

	"github.com/statiko-dev/statiko/appmanager"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/webserver"
)

// Semaphore that allows only one operation at time
var semaphore = make(chan int, 1)

// Semaphore that indicates if there's already one sync waiting
var isWaiting = make(chan int, 1)

// Boolean notifying if the first sync has completed
var StartupComplete = false

// Function that restarts the API server
var ServerRestartFunc func()

// Last time the sync was started
var lastSync *time.Time

// Last sync error
var syncError error

// QueueRun is a thread-safe version of Run that ensures that only one sync can happen at a time
func QueueRun() {
	// No need to trigger multiple sync in a row: if there's already one waiting, then don't queue a second one, since they would pick the same state
	select {
	case isWaiting <- 1:
		break
	default:
		return
	}
	semaphore <- 1
	<-isWaiting
	syncError = nil
	go func() {
		syncError = runner()
		StartupComplete = true
		if syncError != nil {
			logger.Println("Error returned by async run", syncError)
			sendErrorNotification("Unrecoverable error running state synchronization: " + syncError.Error())
		}
		<-semaphore
	}()
}

// Run ensures the system is in the correct state
// You should use QueueRun in most cases
func Run() error {
	semaphore <- 1
	syncError = runner()
	StartupComplete = true
	<-semaphore
	if syncError != nil {
		sendErrorNotification("Unrecoverable error running state synchronization: " + syncError.Error())
	}
	return syncError
}

// IsRunning returns true if the sync is running in background
func IsRunning() bool {
	return len(semaphore) > 0
}

// LastSync returns the time when the last sync started
func LastSync() *time.Time {
	return lastSync
}

// SyncError returns the error (if any) during the last sync
func SyncError() error {
	return syncError
}

// Function actually executing the sync
func runner() error {
	logger.Println("Starting sync")

	// Set the time
	now := time.Now()
	lastSync = &now

	// Set the sync running flag in the node health
	health := state.Instance.GetNodeHealth()
	health.Sync.Running = true
	health.Sync.LastSync = &now
	err := state.Instance.SetNodeHealth(health)
	if err != nil {
		return err
	}

	// Boolean flag for the need to restart the webserver
	restartRequired := false

	// Get the list of sites
	sites := state.Instance.GetSites()

	// First, sync apps
	res, restartServer, err := appmanager.Instance.SyncState(sites)
	if err != nil {
		logger.Println("Unrecoverable error while syncing apps:", err)

		return err
	}
	restartRequired = restartRequired || res

	// Second, sync the web server configuration
	res, err = webserver.Instance.SyncConfiguration(sites)
	if err != nil {
		logger.Println("Error while syncing Nginx configuration:", err)

		return err
	}
	restartRequired = restartRequired || res

	// Check if any site has an error
	for _, s := range sites {
		if siteErr := state.Instance.GetSiteHealth(s.Domain); siteErr != nil {
			sendErrorNotification("Site " + s.Domain + " has an error: " + siteErr.Error())
		}
	}

	// If we've updated anything that requires restarting nginx, do it
	if restartRequired {
		if err := webserver.Instance.RestartServer(); err != nil {
			logger.Println("Error while restarting Nginx:", err)
			return err
		}

		// Sleep for 0.15 seconds waiting for the server to restart
		time.Sleep(150 * time.Millisecond)
	}

	// Restarting the API server if needed
	if restartServer {
		ServerRestartFunc()
	}

	logger.Println("Sync completed")

	// Trigger a refresh of the node health after 5 seconds
	go func() {
		time.Sleep(5 * time.Second)
		state.Instance.TriggerRefreshHealth()
	}()

	return nil
}

// Send a notification to admins if there's an error
func sendErrorNotification(message string) {
	// Launch asynchronously and do not wait for completion
	go notifications.SendNotification(message)
}
