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
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/statiko-dev/statiko/agent/managerclient"
	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/shared/fs"
)

// Agent is the class that manages the agent app
type Agent struct {
	store     fs.Fs
	notifier  *notifications.Notifications
	logger    *log.Logger
	rpcClient *managerclient.RPCClient
}

// Run the agent app
func (a *Agent) Run() (err error) {
	// Init the store
	fsType := appconfig.Config.GetString("repo.type")
	a.store, err = fs.Get(fsType)
	if err != nil {
		return err
	}

	// Init and start the gRPC client
	a.rpcClient = &managerclient.RPCClient{}
	a.rpcClient.Init()
	err = a.rpcClient.Connect()
	if err != nil {
		return err
	}

	time.Sleep(2 * time.Hour)

	/*
		// Sync the state
		// Do this in a synchronous way to ensure the node starts up properly
		if err := sync.Run(); err != nil {
			panic(err)
		}

		// Ensure Nginx is running
		if err := webserver.Instance.EnsureServerRunning(); err != nil {
			panic(err)
		}
	*/

	// Handle SIGUSR1 signals
	a.handleResyncSignal()

	return nil
}

// Listens for SIGUSR1 signals and triggers a new sync
func (a *Agent) handleResyncSignal() {
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGUSR1)
	go func() {
		for range sigc {
			log.Println("Received SIGUSR1, trigger a re-sync")

			// Force a sync
			//go sync.QueueRun()
		}
	}()
}
