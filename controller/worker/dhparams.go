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
)

// Init the DH parameters worker
func (w *Worker) initDHParamsWorker() {
	w.dhparamsLogger = log.New(os.Stdout, "worker/dhparams: ", log.Ldate|log.Ltime|log.LUTC)

	// Ensure the number of bits is 1024, 2048 or 4096
	bits := appconfig.Config.GetInt("tls.dhparams.bits")
	if bits != 1024 && bits != 2048 && bits != 4096 {
		panic(errors.New("tls.dhparams.bits must be 1024, 2048 or 4096"))
	}

	// Get the max age for dhparams
	maxAgeDays := appconfig.Config.GetInt("tls.dhparams.maxAge")

	// If < 0, never regenerate dhparams (besides the first time)
	if maxAgeDays < 0 {
		w.dhparamsRegeneration = false
	} else {
		// Must be at least one week and no more than 2 years
		if maxAgeDays < 7 || maxAgeDays > 720 {
			panic(errors.New("tls.dhparams.maxAge must be between 7 and 720 days; to disable automatic regeneration, set a negative value"))
		}
		w.dhparamsRegeneration = true
		w.dhparamsMaxAge = time.Duration((-1) * time.Duration(maxAgeDays) * 24 * time.Hour)
	}
}

// In background, periodically re-generate DH parameters
func (w *Worker) startDHParamsWorker(ctx context.Context) {
	// Run every 2 days, but re-generate the DH params every N days (default 120)
	dhparamsInterval := time.Duration(48 * time.Hour)

	go func() {
		// Run right away
		err := w.dhparamsWorker(ctx)
		if err != nil {
			w.dhparamsLogger.Println("Worker error:", err)
		}

		if w.dhparamsRegeneration {
			// Run on ticker
			ticker := time.NewTicker(dhparamsInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					err := w.dhparamsWorker(ctx)
					if err != nil {
						w.dhparamsLogger.Println("Worker error:", err)
					}
				case <-ctx.Done():
					w.dhparamsLogger.Println("Worker's context canceled")
					return
				}
			}
		} else {
			w.dhparamsLogger.Println("DH params regeneration disabled - exiting worker")
		}
	}()
}

// Generate a new set of DH parameters if needed
func (w *Worker) dhparamsWorker(parentCtx context.Context) error {
	w.dhparamsLogger.Println("Starting dhparams worker")

	beforeTime := time.Now().Add(w.dhparamsMaxAge)

	// Get a sub-context
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Get the current DH parameters
	// We'll regenerate them only if dhparamsRegeneration is true, or if we're using the default ones
	_, date := w.State.GetDHParams()
	if date == nil || (w.dhparamsRegeneration && date.Before(beforeTime)) {
		// In background, periodically check if we have new DH parameters
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_, newDate := w.State.GetDHParams()
					if newDate != nil && (date == nil || !newDate.Equal(*date)) {
						w.dhparamsLogger.Println("DH parameters have been updated externally")
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
		w.dhparamsLogger.Printf("DH parameters expired; starting generation with %d bits\n", bits)
		result, err := dhparam.GenerateWithContext(ctx, bits, dhparam.GeneratorTwo, nil)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				w.dhparamsLogger.Println("DH parameters generation aborted")
				return nil
			} else {
				return err
			}
		}
		pem, err := result.ToPEM()
		if err != nil {
			return err
		}

		// Store the updated params
		// This also broadcasts them to all agents
		err = w.State.SetDHParams(string(pem))
		if err != nil {
			return err
		}

		w.dhparamsLogger.Println("Done: DH parameters generated")
	} else {
		w.dhparamsLogger.Println("DH parameters still valid")
	}

	return nil
}
