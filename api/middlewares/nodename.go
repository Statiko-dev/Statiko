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

package middlewares

import (
	"github.com/statiko-dev/statiko/appconfig"

	"github.com/gin-gonic/gin"
)

// NodeName middleware that adds the "X-STK-Node" header containing the node name
func NodeName() gin.HandlerFunc {
	return func(c *gin.Context) {
		hostname := appconfig.Config.GetString("nodeName")
		if len(hostname) == 0 {
			return
		}

		c.Header("X-STK-Node", hostname)
	}
}
