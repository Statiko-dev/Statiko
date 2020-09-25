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

package main

import (
	"math/rand"
	"time"

	"github.com/statiko-dev/statiko/api"
	"github.com/statiko-dev/statiko/fs"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/worker"
)

func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Store
	if err := fs.Startup(); err != nil {
		panic(err)
	}

	// Init notifications client
	if err := notifications.InitNotifications(); err != nil {
		panic(err)
	}

	// Start all background workers
	worker.StartWorker()

	// Start the API server
	api.Server.Start()
}
