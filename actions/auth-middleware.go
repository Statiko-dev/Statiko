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
	"github.com/pkg/errors"

	"smplatform/appconfig"
)

// AuthMiddleware is a middleware that checks the presence and value of the Authorization header
func AuthMiddleware(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		// Get the value of the Authorization header
		r := c.Request()
		auth := r.Header.Get("Authorization")
		if len(auth) == 0 || auth != appconfig.Config.GetString("auth") {
			return c.Error(401, errors.New("Invalid authorization token"))
		}

		// Continue processing
		return next(c)
	}
}
