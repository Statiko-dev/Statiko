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
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop"
)

func truncateTable(tx *pop.Connection, table string) error {
	query := tx.RawQuery("DELETE FROM " + table)
	if err := query.Exec(); err != nil {
		return err
	}
	return nil
}

// AdoptHandler is the handler for POST /adopt, which tells the application to reset the state
// @Summary Tells the application to adopt the node, initializing it to a clean state
// @Description The Platform Controller node can invoke this method to "adopt" the node.
// @Description This action is destructive as will delete all existing sites and applications, and reset the node to a clean state.
// @Produce json
// @Success 200 {object} actions.Message
// @Failure 500
// @Router /adopt [post]
func (rts *Routes) AdoptHandler(c buffalo.Context) error {
	// DB transaction
	tx := c.Value("tx").(*pop.Connection)

	// Reset Nginx configuration
	if err := rts.ngConfig.ResetConfiguration(); err != nil {
		return err
	}

	// Initialize the app root
	if err := rts.appManager.InitAppRoot(); err != nil {
		return err
	}

	// Create a folder for the default site
	if err := rts.appManager.CreateFolders("_default"); err != nil {
		return err
	}

	// Delete all data from the database
	if err := truncateTable(tx, "sites"); err != nil {
		return err
	}
	if err := truncateTable(tx, "domains"); err != nil {
		return err
	}
	if err := truncateTable(tx, "deployments"); err != nil {
		return err
	}

	// Reload the Nginx configuration
	if err := rts.ngConfig.RestartServer(); err != nil {
		return err
	}

	// Reset status cache
	rts.statusCache = nil

	return c.Render(200, r.JSON(Message{"Adopted"}))
}
