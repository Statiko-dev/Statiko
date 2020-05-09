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
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/statiko-dev/statiko/api"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/webserver"
	"github.com/statiko-dev/statiko/worker"
)

func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Init notifications client
	if err := notifications.InitNotifications(); err != nil {
		panic(err)
	}

	// Start all background workers
	worker.StartWorker()

	// Sync the state
	// Do this in a synchronous way to ensure the node starts up properly
	if err := sync.Run(); err != nil {
		panic(err)
	}

	// Ensure Nginx is running
	if err := webserver.Instance.EnsureServerRunning(); err != nil {
		panic(err)
	}

	// Handle SIGUSR1 signals
	handleResyncSignal()

	// Start the API server
	api.Server.Start()
}

// Listens for SIGUSR1 signals and triggers a new sync
func handleResyncSignal() {
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGUSR1)
	go func() {
		for {
			<-sigc
			log.Println("Received SIGUSR1, trigger a re-sync")

			// Restart the API server
			api.Server.Restart()

			// Force a sync
			go sync.QueueRun()
		}
	}()
}
