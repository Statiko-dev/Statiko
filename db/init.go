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

package db

import (
	"github.com/jinzhu/gorm"
	// Import the SQLite dialect
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"smplatform/appconfig"
)

// Connection holds the connection to the database
var Connection *gorm.DB

// Init the database connection
func Init() {
	var err error
	Connection, err = gorm.Open("sqlite3", appconfig.Config.GetString("db"))
	if err != nil {
		panic("Failed to connect database")
	}

	// Migrate the schema
	Connection.AutoMigrate(&Domain{})
	Connection.AutoMigrate(&Site{})
	Connection.AutoMigrate(&Deployment{})

	// In development mode, enable logging
	if appconfig.ENV == "development" {
		Connection.LogMode(true)
	}
}
