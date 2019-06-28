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

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"smplatform/appmanager"
	"smplatform/db"
	"smplatform/utils"
	"smplatform/webserver"
)

// AdoptHandler is the handler for POST /adopt, which tells the application to reset the state
// @Summary Tells the application to adopt the node, initializing it to a clean state
// @Description The Platform Controller node can invoke this method to "adopt" the node.
// @Description This action is destructive as will delete all existing sites and applications, and reset the node to a clean state.
// @Produce json
// @Success 200 {object} actions.Message
// @Failure 500
// @Router /adopt [post]
func AdoptHandler(c *gin.Context) {
	// DB transaction
	tx := db.Connection.Begin()
	defer func() {
		if tx != nil {
			// Rollback automatically in case of error
			tx.Rollback()
		}
	}()

	// Reset Nginx configuration
	if err := webserver.Instance.ResetConfiguration(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Initialize the app root
	if err := appmanager.Instance.InitAppRoot(true); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Create a folder for the default site
	if err := appmanager.Instance.CreateFolders("_default"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Delete all data from the database
	if err := utils.TruncateTable(tx, "sites"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err := utils.TruncateTable(tx, "domains"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	if err := utils.TruncateTable(tx, "deployments"); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Reload the Nginx configuration
	if err := webserver.Instance.RestartServer(); err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	// Reset status cache
	statusCache = nil

	// Commit
	if err := tx.Commit().Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	tx = nil

	// Respond
	c.JSON(http.StatusOK, gin.H{
		"message": "adopted",
	})
}
