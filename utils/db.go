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

package utils

import (
	"github.com/jinzhu/gorm"
)

// TruncateTable deletes all records from a table, within a transaction
func TruncateTable(db *gorm.DB, table string) error {
	db.Exec("DELETE FROM " + table)
	if db.Error != nil {
		return db.Error
	}
	return nil
}
