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
	"errors"
	"log"
	"os"
	"time"

	dhparam "github.com/Luzifer/go-dhparam"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
)

// Logger for this file
var dhparamsLogger *log.Logger

// Control how often to regenerate DH parameters
var dhparamsMaxAge time.Duration
var dhparamsRegeneration bool

// In background, periodically re-generate DH parameters
func startDHParamsWorker(ctx context.Context) {
	// Ensure the number of bits is 1024, 2048 or 4096
	bits := appconfig.Config.GetInt("tls.dhparams.bits")
	if bits != 1024 && bits != 2048 && bits != 4096 {
		panic(errors.New("tls.dhparams.bits must be 1024, 2048 or 4096"))
	}
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
	// Run every 2 days, but re-generate the DH params every N days (default 120)
	dhparamsInterval := time.Duration(48 * time.Hour)
	dhparamsLogger = log.New(os.Stdout, "worker/dhparams: ", log.Ldate|log.Ltime|log.LUTC)

	go func() {
		// Wait for startup
		waitForStartup()

		// Run right away
		err := dhparamsWorker(ctx)
		if err != nil {
			dhparamsLogger.Println("Worker error:", err)
		}

		if dhparamsRegeneration {
			// Run on ticker
			ticker := time.NewTicker(dhparamsInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := dhparamsWorker(ctx)
					if err != nil {
						dhparamsLogger.Println("Worker error:", err)
					}
				case <-ctx.Done():
					dhparamsLogger.Println("Worker's context canceled")
					return
				}
			}
		} else {
			dhparamsLogger.Println("DH params regeneration disabled - exiting worker")
		}
	}()
}

// Generate a new set of DH parameters if needed
func dhparamsWorker(parentCtx context.Context) error {
	dhparamsLogger.Println("Starting dhparams worker")

	beforeTime := time.Now().Add(dhparamsMaxAge)
	needsSync := false

	// Get a sub-context
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Get the current DH parameters
	// We'll regenerate them only if dhparamsRegeneration is true, or if we're using the default ones
	_, date := state.Instance.GetDHParams()
	if date == nil || (dhparamsRegeneration && date.Before(beforeTime)) {
		// In background, periodically check if we have new DH parameters
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_, newDate := state.Instance.GetDHParams()
					if newDate != nil && (date == nil || !newDate.Equal(*date)) {
						dhparamsLogger.Println("DH parameters have been updated externally")
						cancel()
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		// Need to regenerate the DH parameters
		bits := appconfig.Config.GetInt("tls.dhparams.bits")
		dhparamsLogger.Printf("DH parameters expired; starting generation with %d bits\n", bits)
		result, err := dhparam.GenerateWithContext(ctx, bits, dhparam.GeneratorTwo, nil)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				dhparamsLogger.Println("DH parameters generation aborted")
				return nil
			} else {
				return err
			}
		}
		pem, err := result.ToPEM()
		if err != nil {
			return err
		}
		err = state.Instance.SetDHParams(string(pem))
		if err != nil {
			return err
		}

		// Queue a new sync
		needsSync = true

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
