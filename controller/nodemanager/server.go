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

package nodemanager

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/statiko-dev/statiko/controller/state"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// RPCServer is the gRPC server that is used to communicate with nodes
type RPCServer struct {
	State *state.Manager

	logger        *log.Logger
	stopCh        chan int
	restartCh     chan int
	doneCh        chan int
	runningCtx    context.Context
	runningCancel context.CancelFunc
	running       bool
	healthReq     *utils.Signaler
}

// Init the gRPC server
func (s *RPCServer) Init() {
	s.running = false

	// Initialize the logger
	s.logger = log.New(os.Stdout, "grpc: ", log.Ldate|log.Ltime|log.LUTC)

	// Channels used to stop and restart the server
	s.stopCh = make(chan int)
	s.restartCh = make(chan int)
	s.doneCh = make(chan int)

	// Channel used to request node health
	s.healthReq = &utils.Signaler{}
}

// Start the gRPC server; must be run in a goroutine with `go s.Start()`
func (s *RPCServer) Start() {
	for {
		// Create the context
		s.runningCtx, s.runningCancel = context.WithCancel(context.Background())

		// Create the server
		grpcServer := grpc.NewServer()
		pb.RegisterControllerServer(grpcServer, s)

		// Register reflection service on gRPC server
		reflection.Register(grpcServer)

		// Start the server in another channel
		go func() {
			// Listen
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", 2300))
			if err != nil {
				panic(err)
			}
			s.logger.Printf("Starting gRPC server on port %d\n", 2300)
			s.running = true
			grpcServer.Serve(listener)
		}()

		select {
		case <-s.stopCh:
			// We received an interrupt signal, shut down for good
			s.logger.Println("Shutting down the gRCP server")
			s.runningCancel()
			grpcServer.GracefulStop()
			s.running = false
			s.doneCh <- 1
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.logger.Println("Restarting the gRCP server")
			s.runningCancel()
			grpcServer.GracefulStop()
			s.doneCh <- 1
			// Do not return, let the for loop repeat
		}
	}
}

// Restart the server
func (s *RPCServer) Restart() {
	if s.running {
		s.restartCh <- 1
		<-s.doneCh
	}
}

// Stop the server
func (s *RPCServer) Stop() {
	if s.running {
		s.stopCh <- 1
		<-s.doneCh
	}
}
