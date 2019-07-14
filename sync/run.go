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
	_ "smplatform/db"
	"smplatform/state"
	webserver "smplatform/webserver2"
)

// Run ensures the system is in the correct state
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
