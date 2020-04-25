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

package sync

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/statiko-dev/statiko/state"
)

// Package-wide properties
var (
	logger *log.Logger
)

// Init method for the package
func init() {
	// Initialize the logger
	logger = log.New(os.Stdout, "sync: ", log.Ldate|log.Ltime|log.LUTC)

	// Set callback so if the state is updated because of external events, a sync is triggered
	state.Instance.OnStateUpdate(func() {
		go QueueRun()
	})

	// Force a sync every time we receive a SIGHUP signal
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGUSR1)
	go func() {
		for {
			s := <-handleSIGUSR1()
			if s == syscall.SIGUSR1 {
				logger.Println("Received SIGUSR1, trigger a sync")
				go Run()
			}
		}
	}()
}

func handleSIGUSR1() chan os.Signal {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGUSR1)
	return sig
}
