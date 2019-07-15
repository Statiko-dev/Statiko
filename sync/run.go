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

package sync

import (
	appmanager "smplatform/appmanager2"
	"smplatform/state"
	webserver "smplatform/webserver2"
)

// Semaphore that allows only one operation at time
var semaphore = make(chan int, 1)

// QueueRun is a thread-safe version of Run that ensures that only one sync can happen at a time
func QueueRun() {
	semaphore <- 1
	go func() {
		err := Run()
		if err != nil {
			// TODO: DO SOMETHING
			logger.Println(err)
		}
		<-semaphore
	}()
}

// Run ensures the system is in the correct state
// You should not use this function directly; use QueueRun instead
func Run() error {
	// Boolean flag for the need to restart the webserver
	restartRequired := false

	// Get the list of sites
	sites := state.Instance.GetSites()

	// First, sync the web server configuration
	res, err := webserver.Instance.SyncConfiguration(sites)
	if err != nil {
		logger.Fatalln("[syncState] Error while syncing Nginx configuration:", err)
		return err
	}
	restartRequired = restartRequired || res

	// Second, sync apps
	res, err = appmanager.Instance.SyncState(sites)
	if err != nil {
		logger.Fatalln("[syncState] Error while syncing apps:", err)
		return err
	}
	restartRequired = restartRequired || res

	// If we've updated anything that requires restarting nginx, do it
	if restartRequired {
		if err := webserver.Instance.RestartServer(); err != nil {
			logger.Fatalln("[syncState] Error while restarting Nginx:", err)
			return err
		}
	}

	return nil
}
