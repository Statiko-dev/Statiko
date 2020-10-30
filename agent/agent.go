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
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/agent/appmanager"
	"github.com/statiko-dev/statiko/agent/certificates"
	"github.com/statiko-dev/statiko/agent/client"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/agent/sync"
	"github.com/statiko-dev/statiko/agent/webserver"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/shared/fs"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Agent is the class that manages the agent app
type Agent struct {
	logger      *log.Logger
	store       fs.Fs
	agentState  *state.AgentState
	notifier    *notifications.Notifications
	certs       *certificates.AgentCertificates
	rpcClient   *client.RPCClient
	syncClient  *sync.Sync
	appManager  *appmanager.Manager
	webserver   *webserver.NginxConfig
	clusterOpts *pb.ClusterOptions
}

// Run the agent app
func (a *Agent) Run() (err error) {
	// Init logger
	a.logger = log.New(os.Stdout, "agent: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the configuration
	err = a.LoadConfig()
	if err != nil {
		return err
	}

	// Init the store
	// TODO: GET THIS FROM CONTROLLER
	fsType := viper.GetString("repo.type")
	a.store, err = fs.Get(fsType)
	if err != nil {
		return err
	}

	// Init the state object
	a.agentState = &state.AgentState{}
	a.agentState.Init()

	// Context for the agent app
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Init and start the gRPC client
	a.rpcClient = &client.RPCClient{
		Ctx: ctx,
	}
	a.rpcClient.Init()
	err = a.rpcClient.Connect()
	if err != nil {
		return err
	}

	// Callback for receiving new states
	a.rpcClient.StateUpdate = func(state *pb.StateMessage) {
		a.agentState.ReplaceState(state)
	}

	// Request the options for the cluster
	a.clusterOpts, err = a.rpcClient.GetClusterOptions()
	if err != nil {
		return err
	}

	// Request the initial state
	state, err := a.rpcClient.GetState()
	if err != nil {
		return err
	}
	a.agentState.ReplaceState(state)

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
		ClusterOpts:  a.clusterOpts,
	}
	err = a.appManager.Init()
	if err != nil {
		return err
	}

	// Init the webserver object
	a.webserver = &webserver.NginxConfig{
		State:       a.agentState,
		AppManager:  a.appManager,
		ClusterOpts: a.clusterOpts,
	}
	err = a.webserver.Init()
	if err != nil {
		return err
	}

	// Init the sync client
	a.syncClient = &sync.Sync{
		State:      a.agentState,
		AppManager: a.appManager,
		Webserver:  a.webserver,
	}
	a.syncClient.Init()

	// Handle state changes and SIGUSR1 signals
	// Queue a sync on every new state received from the controller and when we get SIGUSR1 signals
	go a.handleUpdates(ctx)

	// Perform an initial state sync
	// Do this in a synchronous way to ensure the node starts up properly
	if err := a.syncClient.Run(); err != nil {
		panic(err)
	}

	/*
		// Ensure Nginx is running
		if err := webserver.Instance.EnsureServerRunning(); err != nil {
			panic(err)
		}
	*/

	// Prevent this from returning until the context is canceled (which never happens)
	<-ctx.Done()

	return nil
}

// Subscribes to changes to the state and to SIGUSR1 signals and triggers a new sync
func (a *Agent) handleUpdates(ctx context.Context) {
	// Channel receiving new states
	stateCh := make(chan int)
	a.agentState.Subscribe(stateCh)

	// Channel receiving SIGUSR1 signals
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGUSR1)

	// Cleanup
	defer func() {
		a.agentState.Unsubscribe(stateCh)
		close(stateCh)
		close(sigCh)
	}()

	// Wait for signals
	for {
		select {
		// On state updates
		case <-stateCh:
			a.logger.Println("Received new state, triggering a re-sync")
			// Force a sync, asynchronously
			go a.syncClient.QueueRun()
		// On signals
		case <-sigCh:
			a.logger.Println("Received SIGUSR1, triggering a re-sync")
			// Force a sync, asynchronously
			go a.syncClient.QueueRun()
		// Context termination
		case <-ctx.Done():
			a.logger.Println("Context canceled")
			return
		}
	}
}
