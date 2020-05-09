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

package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/sync"
)

// SyncHandler is the handler for POST /sync, which forces a new sync
func SyncHandler(c *gin.Context) {
	// Queue a new run
	sync.QueueRun()

	// Return
	c.Status(http.StatusNoContent)
}
