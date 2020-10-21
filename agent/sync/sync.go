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
	"log"
	"os"
	"time"

	"github.com/statiko-dev/statiko/agent/appmanager"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/agent/webserver"
	//"github.com/statiko-dev/statiko/notifications"
)

// Sync is the main controller for synchronizing the system's state with the desired state
type Sync struct {
	// AgentState object
	AgentState *state.AgentState
	// AppManager object
	AppManager *appmanager.Manager

	// Semaphore that allows only one operation at time
	semaphore chan int
	// Semaphore that indicates if there's already one sync waiting
	isWaiting chan int
	// Boolean notifying if the first sync has completed
	startupComplete bool
	// Last time the sync was started
	lastSync *time.Time
	// Last sync error
	syncError error
	// Logger
	logger *log.Logger
}

// Init the object
func (s *Sync) Init() {
	// Init properties
	s.semaphore = make(chan int, 1)
	s.isWaiting = make(chan int, 1)
	s.startupComplete = false

	// Init logger
	s.logger = log.New(os.Stdout, "sync: ", log.Ldate|log.Ltime|log.LUTC)
}

// StartupComplete returns true if the first sync has completed
func (s *Sync) StartupComplete() bool {
	return s.startupComplete
}

// QueueRun is a thread-safe version of Run that ensures that only one sync can happen at a time
func (s *Sync) QueueRun() {
	// No need to trigger multiple sync in a row: if there's already one waiting, then don't queue a second one, since they would pick the same state
	select {
	case s.isWaiting <- 1:
		break
	default:
		return
	}
	s.semaphore <- 1
	<-s.isWaiting
	s.syncError = nil
	go func() {
		s.syncError = s.runner()
		s.startupComplete = true
		if s.syncError != nil {
			s.logger.Println("Error returned by async run", s.syncError)
			s.sendErrorNotification("Unrecoverable error running state synchronization: " + s.syncError.Error())
		}
		<-s.semaphore
	}()
}

// Run ensures the system is in the correct state
// You should use QueueRun in most cases
func (s *Sync) Run() error {
	s.semaphore <- 1
	s.syncError = s.runner()
	s.startupComplete = true
	<-s.semaphore
	if s.syncError != nil {
		s.sendErrorNotification("Unrecoverable error running state synchronization: " + s.syncError.Error())
	}
	return s.syncError
}

// IsRunning returns true if the sync is running in background
func (s *Sync) IsRunning() bool {
	return len(s.semaphore) > 0
}

// LastSync returns the time when the last sync started
func (s *Sync) LastSync() *time.Time {
	return s.lastSync
}

// SyncError returns the error (if any) during the last sync
func (s *Sync) SyncError() error {
	return s.syncError
}

// Function actually executing the sync
func (s *Sync) runner() error {
	s.logger.Println("Starting sync")

	// Set the time
	now := time.Now()
	s.lastSync = &now

	// Boolean flag for the need to restart the webserver
	restartRequired := false

	// Get the list of sites
	sites := s.AgentState.GetSites()

	// First, sync apps
	res, restartServer, err := s.AppManager.SyncState(sites)
	if err != nil {
		s.logger.Println("Unrecoverable error while syncing apps:", err)

		return err
	}
	restartRequired = restartRequired || res

	// Second, sync the web server configuration
	res, err = webserver.Instance.SyncConfiguration(sites)
	if err != nil {
		s.logger.Println("Error while syncing Nginx configuration:", err)

		return err
	}
	restartRequired = restartRequired || res

	// Check if any site has an error
	for _, el := range sites {
		if siteErr := s.AgentState.GetSiteHealth(el.Domain); siteErr != nil {
			s.sendErrorNotification("Site " + el.Domain + " has an error: " + siteErr.Error())
		}
	}

	// If we've updated anything that requires restarting nginx, do it
	if restartRequired {
		if err := webserver.Instance.RestartServer(); err != nil {
			s.logger.Println("Error while restarting Nginx:", err)
			return err
		}
	}

	// Restarting the API server if needed
	if restartServer {
		//ServerRestartFunc()
	}

	s.logger.Println("Sync completed")

	return nil
}

// Send a notification to admins if there's an error
func (s *Sync) sendErrorNotification(message string) {
	// Launch asynchronously and do not wait for completion
	//go notifications.SendNotification(message)
}
