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

package startup

import (
	"smplatform/appmanager"
	"smplatform/db"
	"smplatform/webserver"
)

// These functions are used during app's startup to sync the state

// RemovePendingDeployments deletes all pending deployments from the database, marking them as failed
func RemovePendingDeployments() error {
	err := db.Connection.Exec("UPDATE deployments SET status = ? WHERE status = ?", db.DeploymentStatusFailed, db.DeploymentStatusRunning).Error
	if err != nil {
		logger.Fatalln("[removePendingDeployments] Error while updating pending deployments in database:", err)
		return err
	}
	count := db.Connection.RowsAffected
	if count > 0 {
		logger.Printf("[removePendingDeployments] Canceled %d pending deployments\n", count)
	}

	return nil
}

// SyncState ensures the system is in the correct state
// For now, this is limited to ensuring the configuration for Nginx is correct
func SyncState() error {
	// Get records from the database
	sites := []db.Site{}
	if err := db.Connection.Preload("Domains").Find(&sites).Error; err != nil {
		logger.Fatalln("[syncState] Error while querying database:", err)
		return err
	}

	// Re-map to the original JSON structure
	for i := 0; i < len(sites); i++ {
		sites[i].RemapJSON()
	}

	// Sync all sites
	if err := webserver.Instance.SyncConfiguration(sites); err != nil {
		logger.Fatalln("[syncState] Error while syncing Nginx configuration:", err)
		return err
	}

	// Ensure that the appRoot is created (but do not reset it)
	if err := appmanager.Instance.InitAppRoot(false); err != nil {
		return err
	}

	// Ensure the folder for the default site exists
	if err := appmanager.Instance.CreateFolders("_default"); err != nil {
		return err
	}

	// Restart Nginx server
	if err := webserver.Instance.RestartServer(); err != nil {
		logger.Fatalln("[syncState] Error while restarting Nginx:", err)
		return err
	}

	return nil
}
