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
	_ "smplatform/appmanager"
	_ "smplatform/db"
	"smplatform/state"
	webserver "smplatform/webserver2"
)

// Run ensures the system is in the correct state
func Run() error {
	// Get the list of sites
	sites := state.Instance.GetSites()

	// First, sync the web server configuration
	restartRequired, err := webserver.Instance.SyncConfiguration(sites)
	if err != nil {
		logger.Fatalln("[syncState] Error while syncing Nginx configuration:", err)
		return err
	}

	/*// Ensure that the appRoot is created (but do not reset it)
	if err := appmanager.Instance.InitAppRoot(false); err != nil {
		return err
	}

	// Ensure the folder for the default site exists
	if err := appmanager.Instance.CreateFolders("_default"); err != nil {
		return err
	}*/

	// If we've updated anything that requires restarting nginx, do it
	if restartRequired {
		if err := webserver.Instance.RestartServer(); err != nil {
			logger.Fatalln("[syncState] Error while restarting Nginx:", err)
			return err
		}
	}

	return nil
}
