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
	"github.com/statiko-dev/statiko/agent/httpserver"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/agent/sync"
	"github.com/statiko-dev/statiko/agent/webserver"
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/fs"
	"github.com/statiko-dev/statiko/shared/notifications"
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
	httpSrv     *httpserver.HTTPServer
	syncClient  *sync.Sync
	appManager  *appmanager.Manager
	webserver   *webserver.NginxConfig
	clusterOpts *pb.ClusterOptions
	akv         *azurekeyvault.Client
	stateCh     chan int
}

// Run the agent app
func (a *Agent) Run(ctx context.Context) (err error) {
	// Init logger
	a.logger = log.New(os.Stdout, "agent: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the configuration
	err = a.LoadConfig()
	if err != nil {
		return err
	}

	// Init and start the gRPC client
	a.rpcClient = &client.RPCClient{}
	a.rpcClient.Init()
	connectedCh, err := a.rpcClient.Connect()
	if err != nil {
		return err
	}

	// Channel receiving new states
	a.stateCh = make(chan int)
	defer close(a.stateCh)

	// Channel receiving SIGUSR1 signals
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGUSR1)
	defer close(sigCh)

	// Listen for the various signals
	for {
		select {
		// When the connection with the controller is established via gRPC and the node has registered itself successfully
		case <-connectedCh:
			a.logger.Println("Node registered and ready")
			a.stopServer()
			err := a.ready()
			if err != nil {
				return err
			}

		// On state updates, queue a sync
		case <-a.stateCh:
			a.logger.Println("Received new state, triggering a re-sync")
			// Force a sync, asynchronously
			go a.syncClient.QueueRun()

		// On SIGUSR1 signals, queue a sync
		case <-sigCh:
			a.logger.Println("Received SIGUSR1, triggering a re-sync")
			// Force a sync, asynchronously
			go a.syncClient.QueueRun()

		// Context termination
		case <-ctx.Done():
			a.logger.Println("Context canceled")
			// Disconnect
			a.stopServer()
			err := a.rpcClient.Disconnect()
			return err
		}
	}
}

// Function executed every time the node is ready: after the node has connected to the controller and registered itself
func (a *Agent) ready() (err error) {
	// Init the state object
	a.agentState = &state.AgentState{}
	a.agentState.Init()

	// Method for requesting the node health
	a.rpcClient.GetHealth = a.NodeHealth

	// Callback for receiving new states
	a.rpcClient.StateUpdate = func(state *pb.StateMessage) {
		a.agentState.ReplaceState(state)
	}

	// Request the options for the cluster
	a.clusterOpts, err = a.rpcClient.GetClusterOptions()
	if err != nil {
		return err
	}

	// Init the notifications client
	a.notifier = &notifications.Notifications{}
	err = a.notifier.Init(a.clusterOpts.Notifications)
	if err != nil {
		return err
	}

	// Init the HTTP server
	a.httpSrv = &httpserver.HTTPServer{
		State: a.agentState,
		RPC:   a.rpcClient,
	}
	a.httpSrv.Init()
	go a.httpSrv.Start()

	// Request the initial state
	state, err := a.rpcClient.GetState()
	if err != nil {
		return err
	}
	a.agentState.ReplaceState(state)

	// Subscribe to receive new state updates
	a.agentState.Subscribe(a.stateCh)

	// Init the store
	switch a.clusterOpts.Storage.(type) {
	case *pb.ClusterOptions_Local:
		a.store, err = fs.Get("local", a.clusterOpts.Storage)
	case *pb.ClusterOptions_Azure:
		a.store, err = fs.Get("azure", a.clusterOpts.Storage)
	case *pb.ClusterOptions_S3:
		a.store, err = fs.Get("s3", a.clusterOpts.Storage)
	}
	if err != nil {
		return err
	}

	// Init the Azure Key Vault client if we need it
	akvName := a.clusterOpts.AzureKeyVault.VaultName
	if akvName != "" {
		a.akv = &azurekeyvault.Client{
			VaultName: akvName,
		}
		err = a.akv.Init(a.clusterOpts.AzureKeyVault.Auth)
		if err != nil {
			return err
		}
	}

	// Init the certs object
	a.certs = &certificates.AgentCertificates{
		State: a.agentState,
		RPC:   a.rpcClient,
		AKV:   a.akv,
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
		Notifier:   a.notifier,
	}
	a.syncClient.SyncComplete = func(syncError error) {
		// Send the updated health to the controller
		go a.rpcClient.SendHealth()
	}
	a.syncClient.Init()

	// Perform an initial state sync
	// Do this in a synchronous way to ensure the node starts up properly
	err = a.syncClient.Run()
	if err != nil {
		return err
	}

	return nil
}

// Stop the HTTP server if it's running
func (a *Agent) stopServer() {
	if a.httpSrv != nil && a.httpSrv.IsRunning() {
		a.httpSrv.Stop()
		a.httpSrv = nil
	}
}

// NodeHealth returns the object with the health of the node
func (a *Agent) NodeHealth() (health *pb.NodeHealth) {
	// Health object
	health = &pb.NodeHealth{}

	// Node name
	health.NodeName = viper.GetString("nodeName")

	// State version
	health.Version = a.agentState.GetVersion()

	// Nginx status
	{
		nginxStatus, err := a.webserver.Status()
		if err != nil {
			// Log the error only
			a.logger.Println("caught error while requesting nginx status:", err)
			nginxStatus = false
		}
		health.WebServer = &pb.NodeHealth_WebServer{
			Healthy: nginxStatus,
		}
	}

	// Sync status
	{
		var lastSyncUnix int64
		lastSyncTime := a.syncClient.LastSync()
		if lastSyncTime != nil {
			lastSyncUnix = lastSyncTime.Unix()
		}
		var syncErrStr string
		syncErr := a.syncClient.SyncError()
		if syncErr != nil {
			syncErrStr = syncErr.Error()
		}
		health.Sync = &pb.NodeHealth_Sync{
			Running:   a.syncClient.IsRunning(),
			LastSync:  lastSyncUnix,
			SyncError: syncErrStr,
		}
	}

	// Sites
	{
		// Get all sites and their health
		sites := a.agentState.GetSites()
		sitesHealth := a.agentState.GetAllSitesHealth()
		health.Sites = make([]*pb.NodeHealth_Site, len(sites))

		// Build the result
		for i, s := range sites {
			health.Sites[i] = &pb.NodeHealth_Site{
				Domain: s.Domain,
			}
			if s.App != nil && s.App.Name != "" {
				health.Sites[i].App = s.App.Name
			}
			h, ok := sitesHealth[s.Domain]
			if ok && h != nil {
				health.Sites[i].Error = h.Error()
			}
		}
	}

	return
}
