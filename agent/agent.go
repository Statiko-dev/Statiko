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
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/statiko-dev/statiko/agent/appmanager"
	"github.com/statiko-dev/statiko/agent/certificates"
	"github.com/statiko-dev/statiko/agent/client"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/agent/sync"
	"github.com/statiko-dev/statiko/agent/webserver"
	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/shared/fs"
)

// Agent is the class that manages the agent app
type Agent struct {
	store      fs.Fs
	agentState *state.AgentState
	notifier   *notifications.Notifications
	logger     *log.Logger
	certs      *certificates.AgentCertificates
	rpcClient  *client.RPCClient
	syncClient *sync.Sync
	appManager *appmanager.Manager
	webserver  *webserver.NginxConfig
}

// Run the agent app
func (a *Agent) Run() (err error) {
	// Ensure that the node name is set
	if appconfig.Config.GetString("nodeName") == "" {
		return errors.New("nodeName must be set in the configuration")
	}

	// Init the store
	fsType := appconfig.Config.GetString("repo.type")
	a.store, err = fs.Get(fsType)
	if err != nil {
		return err
	}

	// Init the state object
	a.agentState = &state.AgentState{}
	a.agentState.Init()

	// Request the initial state
	state, err := a.rpcClient.GetState()
	if err != nil {
		return err
	}
	a.agentState.ReplaceState(state)

	// Init and start the gRPC client
	a.rpcClient = &client.RPCClient{
		AgentState: a.agentState,
	}
	a.rpcClient.Init()
	err = a.rpcClient.Connect()
	if err != nil {
		return err
	}

	// Init the certs object
	a.certs = &certificates.AgentCertificates{
		State: a.agentState,
		RPC:   a.rpcClient,
	}
	err = a.certs.Init()
	if err != nil {
		return err
	}

	// Init the app manager object
	a.appManager = &appmanager.Manager{
		State:        a.agentState,
		Certificates: a.certs,
		Fs:           a.store,
	}
	err = a.appManager.Init()
	if err != nil {
		return err
	}

	// Init the webserver object
	a.webserver = &webserver.NginxConfig{
		State:      a.agentState,
		AppManager: a.appManager,
	}
	err = a.webserver.Init()
	if err != nil {
		return err
	}

	// Init the sync client
	a.syncClient = &sync.Sync{
		State:     a.agentState,
		Webserver: a.webserver,
	}
	a.syncClient.Init()

	// Perform an initial state sync
	// Do this in a synchronous way to ensure the node starts up properly
	if err := a.syncClient.Run(); err != nil {
		panic(err)
	}

	time.Sleep(2 * time.Hour)

	/*
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

			// Force a sync, asynchronously
			go a.syncClient.QueueRun()
		}
	}()
}
