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

package worker

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/statiko-dev/statiko/statuscheck"
)

// Logger for this file
var healthLogger *log.Logger

// In background, periodically check the status of the sites
func startHealthWorker(ctx context.Context) {
	// Set variables
	// This runs every minute, but the cache is refreshed only if it's older than N minutes (configured in the statuscheck module)
	// So, the cache might be older than N minutes, and it's fine
	healthInterval := time.Duration(statuscheck.StatusCheckInterval) * time.Second
	healthLogger = log.New(os.Stdout, "worker/health: ", log.Ldate|log.Ltime|log.LUTC)

	go func() {
		// Wait for startup
		waitForStartup()

		// Wait 15 seconds at node startup
		// No need to run right away, as the sync module will make this code run
		time.Sleep(15 * time.Second)

		// Run on ticker
		ticker := time.NewTicker(healthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := healthWorker()
				if err != nil {
					healthLogger.Println("Worker error:", err)
				}
			case <-ctx.Done():
				healthLogger.Println("Worker's context canceled")
				return
			}
		}
	}()
}

// Update the health cache
func healthWorker() error {
	healthLogger.Println("Refreshing health cache")
	return statuscheck.UpdateStoredNodeHealth()
}
