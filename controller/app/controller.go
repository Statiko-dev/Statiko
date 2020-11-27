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

package app

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/controller/api"
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/rpcserver"
	"github.com/statiko-dev/statiko/controller/state"
	controllerutils "github.com/statiko-dev/statiko/controller/utils"
	"github.com/statiko-dev/statiko/controller/worker"
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/fs"
	"github.com/statiko-dev/statiko/shared/notifications"
)

// Controller is the class that manages the controller app
type Controller struct {
	Store       fs.Fs
	State       *state.Manager
	Notifier    *notifications.Notifications
	Cluster     *cluster.Cluster
	Certs       *certificates.Certificates
	APISrv      *api.APIServer
	RPCSrv      *rpcserver.RPCServer
	AKV         *azurekeyvault.Client
	Worker      *worker.Worker
	NodeCerts   *certificates.Certificates
	NodeCertSrv *certificates.ACMEServer

	logger *log.Logger

	// For testing
	StartedCb func()
	NoWorker  bool
	ACMEDelay time.Duration
}

// Run the controller app
func (c *Controller) Run(ctx context.Context) (err error) {
	// Initialize the logger
	c.logger = log.New(buildinfo.LogDestination, "controller: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the configuration
	err = c.loadConfig()
	if err != nil {
		return err
	}

	// Init the store
	fsType, fsOpts, _ := controllerutils.GetClusterOptionsStore()
	c.Store, err = fs.Get(fsType, fsOpts)
	if err != nil {
		return err
	}

	// Init the state manager
	c.State = &state.Manager{}
	err = c.State.Init()
	if err != nil {
		return err
	}
	if !c.State.LoadCodesignKey() && viper.GetBool("codesign.required") {
		return errors.New("codesign.required is true, but no valid key found in codesign.publicKey")
	}

	// Init the notifications client
	notificationsOpts, err := controllerutils.GetClusterOptionsNotifications()
	if err != nil {
		return err
	}
	c.Notifier = &notifications.Notifications{}
	err = c.Notifier.Init(notificationsOpts)
	if err != nil {
		return err
	}

	// Init the cluster object
	c.Cluster = &cluster.Cluster{
		State: c.State,
	}
	c.Cluster.NodeActivity = func(count int, direction int) {
		// When we get the very first node, trigger a refresh of the certificates
		// This is because things like ACME require at least one node running to work
		if count == 1 && direction == 1 {
			c.State.CertRefresh()
		}
	}
	err = c.Cluster.Init()
	if err != nil {
		return err
	}

	// Init the Azure Key Vault client if we need it
	akvName := viper.GetString("azureKeyVault.name")
	if akvName != "" {
		c.AKV = &azurekeyvault.Client{
			VaultName: akvName,
		}
		err = c.AKV.Init(controllerutils.GetClusterOptionsAzureSP("azureKeyVault"))
		if err != nil {
			return err
		}
	}

	// Init the certs object
	tokenReady := func() error {
		// Wait until the cluster has synced
		ver := c.State.GetVersion()
		// This is a blocking call
		err := c.Cluster.WaitForVersion(ver)

		// In testing, we can add a delay here
		if c.ACMEDelay > 0 {
			time.Sleep(c.ACMEDelay)
		}

		return err
	}
	c.Certs = &certificates.Certificates{
		State:          c.State,
		ACMETokenReady: tokenReady,
		AKV:            c.AKV,
	}
	err = c.Certs.Init()
	if err != nil {
		return err
	}

	// Init and start the API and gRPC servers, in background goroutines
	err = c.startServer()
	if err != nil {
		return err
	}

	// Start all background workers
	// In testing mode, we can disable that
	if !c.NoWorker {
		// If we're using ACME, init another web server that is used to present ACME challenges
		// This is separate from the API server because it has to be on port 80
		// Also, this is disabled when the worker is disabled because only the worker can trigger new ACME cert generations
		if viper.GetBool("controller.tls.acme") {
			semaphore := &sync.Mutex{}
			nodeTokenReady := func() error {
				semaphore.Lock()
				// Init and start the server if it hasn't been started yet
				// Note that this server is not stopped when the API server is, so we only initialize it once
				if c.NodeCertSrv == nil {
					c.NodeCertSrv = &certificates.ACMEServer{
						State: c.State,
					}
					c.NodeCertSrv.Init()
					go c.NodeCertSrv.Start()
				}
				semaphore.Unlock()
				return nil
			}
			// This is separate from c.Certificates because this is specific for the controller
			c.NodeCerts = &certificates.Certificates{
				State:          c.State,
				ACMETokenReady: nodeTokenReady,
				AKV:            c.AKV,
			}
		}

		// Init the worker
		c.Worker = &worker.Worker{
			State:          c.State,
			Certificates:   c.Certs,
			NodeCerts:      c.NodeCerts,
			Notifier:       c.Notifier,
			RestartServers: c.restartServer,
		}
		c.Worker.Start()
	}

	// For testing
	if c.StartedCb != nil {
		c.StartedCb()
	}

	// Wait for the shutdown signal or context canceled then stop the servers and the worker
	c.waitForShutdown(ctx)

	return nil
}

// Starts the servers, in background goroutines
func (c *Controller) startServer() error {
	// Get the TLS certificate for the controller node
	cert, err := c.GetControllerCertificate()
	if err != nil {
		return err
	}

	// Init and start the gRPC server
	c.RPCSrv = &rpcserver.RPCServer{
		State:   c.State,
		Cluster: c.Cluster,
		Certs:   c.Certs,
		Fs:      c.Store,
		TLSCert: cert,
	}
	c.RPCSrv.Init()
	go c.RPCSrv.Start()

	// Init and start the API server
	c.APISrv = &api.APIServer{
		Store:   c.Store,
		State:   c.State,
		Cluster: c.Cluster,
		AKV:     c.AKV,
		TLSCert: cert,
	}
	c.APISrv.Init()
	go c.APISrv.Start()

	return nil
}

// Blocking call that returns only after the servers are shut down
// It shuts down servers when the context is canceled
// This also handles graceful shutdowns on SIGINT, SIGTERM, SIGQUIT
func (c *Controller) waitForShutdown(ctx context.Context) {
	// Handle graceful shutdown on SIGINT, SIGTERM and SIGQUIT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	// Wait for the shutdown signal or context canceled then stop the servers and the worker
	// This is blocking
	select {
	case <-sigCh:
		c.logger.Println("Received signal to terminate the app")
		break
	case <-ctx.Done():
		c.logger.Println("Context canceled: terminating the app")
		break
	}
	c.APISrv.Stop()
	c.RPCSrv.Stop()
	if c.Worker != nil {
		c.Worker.Stop()
	}
}

// Triggers a restart of the servers (and updates the TLS certificate they're using)
func (c *Controller) restartServer() error {
	// Get the TLS certificate for the controller node and update it in the objects
	cert, err := c.GetControllerCertificate()
	if err != nil {
		return err
	}
	c.RPCSrv.TLSCert = cert
	c.APISrv.TLSCert = cert

	// Restart the servers
	// These calls block until the servers have restarted
	c.RPCSrv.Restart()
	c.APISrv.Restart()

	return nil
}
