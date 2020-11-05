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
	"time"

	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/state"
	"github.com/statiko-dev/statiko/shared/notifications"
)

// Worker manages background workers
type Worker struct {
	State        *state.Manager
	Certificates *certificates.Certificates
	Notifier     *notifications.Notifications

	ctx    context.Context
	cancel context.CancelFunc

	// For the DHParams worker
	dhparamsLogger       *log.Logger
	dhparamsMaxAge       time.Duration
	dhparamsRegeneration bool

	// For the cert monitor worker
	certMonitorLogger        *log.Logger
	certMonitorNotifications map[string]int
	certMonitorChecks        []int
	certMonitorRefreshCh     chan int
}

// Start all the background workers
func (w *Worker) Start() {
	w.ctx, w.cancel = context.WithCancel(context.Background())

	w.initDHParamsWorker()
	w.initCertMonitorWorker()

	w.startDHParamsWorker(w.ctx)
	w.startCertMonitorWorker(w.ctx)
}

// Stop the background worker
func (w *Worker) Stop() {
	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
}
