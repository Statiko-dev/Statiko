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

	"github.com/ItalyPaleAle/statiko/appmanager"
	"github.com/ItalyPaleAle/statiko/notifications"
	"github.com/ItalyPaleAle/statiko/state"
	"github.com/ItalyPaleAle/statiko/webserver"
)

// Semaphore that allows only one operation at time
var semaphore = make(chan int, 1)

// Last time the sync was started
var lastSync *time.Time

// Last sync error
var syncError error

// QueueRun is a thread-safe version of Run that ensures that only one sync can happen at a time
func QueueRun() {
	semaphore <- 1
	syncError = nil
	go func() {
		syncError = runner()
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
	<-semaphore
	if syncError != nil {
		sendErrorNotification("Unrecoverable error running state synchronization: " + syncError.Error())
	}
	return syncError
}

// IsRunning returns true if the sync is running in background
func IsRunning() bool {
	return len(semaphore) == 1
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

	// Acquire a sync lock to ensure that only one node in the cluster can be running a sync at the same time
	// This is to solve various issues, including nodes each generating self-signed TLS certs, and to ensure that not all nodes are requesting the same resources from upstream
	// This does limit the scalability of the platform as it essentially makes every node doing sync sequentially, so we might revisit this choice in the future
	lock, err := state.Instance.AcquireSyncLock()
	if err != nil {
		logger.Println("Error while acquiring sync lock", err)
		return err
	}

	// Set the time
	now := time.Now()
	lastSync = &now

	// Boolean flag for the need to restart the webserver
	restartRequired := false

	// Get the list of sites
	sites := state.Instance.GetSites()

	// First, sync apps
	res, err := appmanager.Instance.SyncState(sites)
	if err != nil {
		logger.Println("Unrecoverable error while syncing apps:", err)

		// Release the sync lock
		syncErr := state.Instance.ReleaseSyncLock(lock)
		if syncErr != nil {
			logger.Println("Error while releasing sync lock", syncErr)
		}

		return err
	}
	restartRequired = restartRequired || res

	// Second, sync the web server configuration
	res, err = webserver.Instance.SyncConfiguration(sites)
	if err != nil {
		logger.Println("Error while syncing Nginx configuration:", err)

		// Release the sync lock
		syncErr := state.Instance.ReleaseSyncLock(lock)
		if syncErr != nil {
			logger.Println("Error while releasing sync lock", syncErr)
		}

		return err
	}
	restartRequired = restartRequired || res

	// Release the sync lock
	err = state.Instance.ReleaseSyncLock(lock)
	if err != nil {
		logger.Println("Error while releasing sync lock", err)
		return err
	}

	// Check if any site has an error
	for _, s := range sites {
		if s.Error != nil {
			sendErrorNotification("Site " + s.Domain + " has an error: " + s.Error.Error())
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

	logger.Println("Sync completed")

	return nil
}

// Send a notification to admins if there's an error
func sendErrorNotification(message string) {
	// Launch asynchronously and do not wait for completion
	go notifications.SendNotification(message)
}
