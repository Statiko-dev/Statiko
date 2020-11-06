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

	"github.com/spf13/viper"

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
	store    fs.Fs
	state    *state.Manager
	notifier *notifications.Notifications
	cluster  *cluster.Cluster
	certs    *certificates.Certificates
	apiSrv   *api.APIServer
	rcpSrv   *rpcserver.RPCServer
	akv      *azurekeyvault.Client
	logger   *log.Logger
	worker   *worker.Worker
}

// Run the controller app
func (c *Controller) Run() (err error) {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "controller: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the configuration
	err = c.LoadConfig()
	if err != nil {
		return err
	}

	// Init the store
	fsType, fsOpts := controllerutils.GetClusterOptionsStorage()
	c.store, err = fs.Get(fsType, fsOpts)
	if err != nil {
		return err
	}

	// Init the state manager
	c.state = &state.Manager{}
	err = c.state.Init()
	if err != nil {
		return err
	}
	if !c.state.LoadCodesignKey() && viper.GetBool("codesign.required") {
		return errors.New("codesign.required is true, but no valid key found in codesign.publicKey")
	}

	// Init the notifications client
	notificationsOpts, err := controllerutils.GetClusterOptionsNotifications()
	if err != nil {
		return err
	}
	c.notifier = &notifications.Notifications{}
	err = c.notifier.Init(notificationsOpts)
	if err != nil {
		return err
	}

	// Init the cluster object
	c.cluster = &cluster.Cluster{
		State: c.state,
	}
	c.cluster.NodeActivity = func(count int, direction int) {
		// When we get the very first node, trigger a refresh of the certificates
		// This is because things like ACME require at least one node running to work
		if count == 1 && direction == 1 {
			c.state.CertRefresh()
		}
	}
	err = c.cluster.Init()
	if err != nil {
		return err
	}

	// Init the Azure Key Vault client if we need it
	akvName := viper.GetString("azureKeyVault.name")
	if akvName != "" {
		c.akv = &azurekeyvault.Client{
			VaultName: akvName,
		}
		err = c.akv.Init(controllerutils.GetClusterOptionsAzureSP("azureKeyVault"))
		if err != nil {
			return err
		}
	}

	// Init the certs object
	c.certs = &certificates.Certificates{
		State:   c.state,
		Cluster: c.cluster,
		AKV:     c.akv,
	}
	err = c.certs.Init()
	if err != nil {
		return err
	}

	// Start all background workers
	c.worker = &worker.Worker{
		State:        c.state,
		Certificates: c.certs,
		Notifier:     c.notifier,
	}
	c.worker.Start()

	// Handle graceful shutdown on SIGINT, SIGTERM and SIGQUIT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	// Init and start the gRPC server
	c.rcpSrv = &rpcserver.RPCServer{
		State:   c.state,
		Cluster: c.cluster,
		Certs:   c.certs,
		Fs:      c.store,
	}
	c.rcpSrv.Init()
	go c.rcpSrv.Start()
	if err != nil {
		return err
	}

	// Init and start the API server
	c.apiSrv = &api.APIServer{
		Store:   c.store,
		State:   c.state,
		Cluster: c.cluster,
		AKV:     c.akv,
	}
	c.apiSrv.Init()
	go c.apiSrv.Start()

	// Wait for the shutdown signal then stop the servers and the worker
	<-sigCh
	c.logger.Println("Received signal to terminate the app")
	c.apiSrv.Stop()
	c.rcpSrv.Stop()
	c.worker.Stop()

	return nil
}
