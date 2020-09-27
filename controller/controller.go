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

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/controller/api"
	"github.com/statiko-dev/statiko/controller/nodemanager"
	"github.com/statiko-dev/statiko/controller/state"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/shared/fs"
	//"github.com/statiko-dev/statiko/controller/worker"
)

// Controller is the class that manages the controller app
type Controller struct {
	store    fs.Fs
	state    *state.Manager
	notifier *notifications.Notifications
	apiSrv   *api.APIServer
	rcpSrv   *nodemanager.RPCServer
	logger   *log.Logger
}

// Init the controller object
func (c *Controller) Init() (err error) {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "controller: ", log.Ldate|log.Ltime|log.LUTC)

	// Init the store
	fsType := appconfig.Config.GetString("repo.type")
	c.store, err = fs.Get(fsType)
	if err != nil {
		return err
	}

	// Init the state manager
	c.state = &state.Manager{}
	err = c.state.Init()
	if err != nil {
		return err
	}

	// Init the notifications client
	c.notifier = &notifications.Notifications{}
	err = c.notifier.Init()
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
	c.rcpSrv = &nodemanager.RPCServer{
		State: c.state,
	}
	c.rcpSrv.Init()
	go c.rcpSrv.Start()
	if err != nil {
		return err
	}

	// Init and start the API server
	c.apiSrv = &api.APIServer{
		Store: c.store,
		State: c.state,
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
