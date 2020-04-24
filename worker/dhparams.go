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
	"crypto/rand"
	"encoding/binary"
	"log"
	"os"
	"time"

	dhparam "github.com/Luzifer/go-dhparam"

	"github.com/ItalyPaleAle/statiko/state"
	"github.com/ItalyPaleAle/statiko/sync"
)

// Logger for this file
var dhparamsLogger *log.Logger

// Regenerate DH params if they're older than 90 days
const dhparamsMaxAge = time.Duration((-1) * 90 * 24 * time.Hour)

// In background, periodically re-generate DH parameters
func startDHParamsWorker() {
	// How often to regenerate DH params: check every 3-6 days
	// Run every 3-6 days, but re-generate the DH params every 90 days
	var rnd uint64
	if err := binary.Read(rand.Reader, binary.BigEndian, &rnd); err != nil {
		panic(err)
	}
	hours := (rnd%25)*3 + 72

	// Set variables
	dhparamsInterval := time.Duration(time.Duration(hours) * time.Hour)
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

	beforeTime := time.Now().Add(dhparamsMaxAge)
	needsSync := false

	// Get the current DH parameters
	_, date := state.Instance.GetDHParams()
	if date == nil || date.Before(beforeTime) {
		// Need to regenerate the DH parameters
		dhparamsLogger.Println("DH parameters expired; starting generation")
		result, err := dhparam.Generate(4096, dhparam.GeneratorTwo, nil)
		if err != nil {
			return err
		}
		pem, err := result.ToPEM()
		if err != nil {
			return err
		}
		err = state.Instance.SetDHParams(string(pem))
		if err != nil {
			return err
		}
	} else {
		dhparamsLogger.Println("DH parameters still valid")
	}

	// If we need to queue a sync
	if needsSync {
		sync.QueueRun()
	}

	dhparamsLogger.Println("Done")

	return nil
}
