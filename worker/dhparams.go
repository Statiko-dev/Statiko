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
	"log"
	"os"
	"time"

	"github.com/ItalyPaleAle/statiko/sync"
)

// Logger for this file
var dhparamsLogger *log.Logger

// Regenerate DH params if they're older than 30 days
const dhparamsMaxAge = time.Duration(30 * 24 * time.Hour)

// In background, periodically re-generate DH parameters
func startDHParamsWorker() {
	// Set variables
	dhparamsInterval := time.Duration(3 * 24 * time.Hour) // Run every 3 days, but re-generate the DH params every 30 days
	dhparamsLogger = log.New(os.Stdout, "worker/dhparams: ", log.Ldate|log.Ltime|log.LUTC)

	ticker := time.NewTicker(dhparamsInterval)
	go func() {
		// Run right away
		err := dhparamsWorker()
		if err != nil {
			dhparamsLogger.Println("Worker error:", err)
		}

		// Run on ticker
		for range ticker.C {
			err := dhparamsWorker()
			if err != nil {
				dhparamsLogger.Println("Worker error:", err)
			}
		}
	}()
}

// Generate a new set of DH parameters if needed
func dhparamsWorker() error {
	dhparamsLogger.Println("Starting dhparams worker")

	now := time.Now()
	needsSync := false

	_ = now

	// If we need to queue a sync
	if needsSync {
		sync.QueueRun()
	}

	dhparamsLogger.Println("Done")

	return nil
}
