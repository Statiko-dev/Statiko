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
	"errors"
	"log"
	"os"
	"time"

	dhparam "github.com/Luzifer/go-dhparam"

	"github.com/ItalyPaleAle/statiko/appconfig"
	"github.com/ItalyPaleAle/statiko/state"
	"github.com/ItalyPaleAle/statiko/sync"
)

// Logger for this file
var dhparamsLogger *log.Logger

// Control how often to regenerate DH parameters
var dhparamsMaxAge time.Duration
var dhparamsRegeneration bool

// In background, periodically re-generate DH parameters
func startDHParamsWorker() {
	// How often to regenerate DH params: check every 3-6 days
	// Run every 3-6 days, but re-generate the DH params every N days (default 120)
	var rnd uint64
	if err := binary.Read(rand.Reader, binary.BigEndian, &rnd); err != nil {
		panic(err)
	}
	hours := (rnd%25)*3 + 72

	// Get the max age for dhparams
	maxAgeDays := appconfig.Config.GetInt("tls.dhparams.maxAge")
	// If < 0, never regenerate dhparams (besides the first time)
	if maxAgeDays < 0 {
		dhparamsRegeneration = false
	} else {
		// Must be at least one week and no more than 2 years
		if maxAgeDays < 7 || maxAgeDays > 720 {
			panic(errors.New("tls.dhparams.maxAge must be between 7 and 720 days; to disable automatic regeneration, set a negative value"))
		}
		dhparamsRegeneration = true
		dhparamsMaxAge = time.Duration((-1) * time.Duration(maxAgeDays) * 24 * time.Hour)
	}

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

		if dhparamsRegeneration {
			// Run on ticker
			for range ticker.C {
				err := dhparamsWorker()
				if err != nil {
					dhparamsLogger.Println("Worker error:", err)
				}
			}
		} else {
			dhparamsLogger.Println("DH params regeneration disabled - exiting worker")
		}
	}()
}

// Generate a new set of DH parameters if needed
func dhparamsWorker() error {
	dhparamsLogger.Println("Starting dhparams worker")

	beforeTime := time.Now().Add(dhparamsMaxAge)
	needsSync := false

	// Get the current DH parameters
	// We'll regenerate them only if dhparamsRegeneration is true, or if we're using the default ones
	_, date := state.Instance.GetDHParams()
	if date == nil || (dhparamsRegeneration && date.Before(beforeTime)) {
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

		dhparamsLogger.Println("Done: DH parameters generated")
	} else {
		dhparamsLogger.Println("DH parameters still valid")
	}

	// If we need to queue a sync
	if needsSync {
		sync.QueueRun()
	}

	return nil
}
