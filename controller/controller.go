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

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/controller/api"
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/rpcserver"
	"github.com/statiko-dev/statiko/controller/state"
	controllerutils "github.com/statiko-dev/statiko/controller/utils"
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/fs"
	"github.com/statiko-dev/statiko/shared/notifications"
	//"github.com/statiko-dev/statiko/controller/worker"
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
}

// Run the controller app
func (c *Controller) Run() (err error) {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "controller: ", log.Ldate|log.Ltime|log.LUTC)

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
	if !c.state.LoadCodesignKey() && appconfig.Config.GetBool("codesign.required") {
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
	err = c.cluster.Init()
	if err != nil {
		return err
	}

	// Init the Azure Key Vault client if we need it
	akvName := appconfig.Config.GetString("azureKeyVault.name")
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
	// TODO: NEEDS UPDATING
	//worker.StartWorker()

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

	// Wait for the shutdown signal then stop the servers
	<-sigCh
	c.logger.Println("Received signal to terminate the app")
	c.apiSrv.Stop()
	c.rcpSrv.Stop()

	return nil
}
