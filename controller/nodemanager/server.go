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
	"io"
	"log"
	"net"
	"os"

	"google.golang.org/grpc"

	"github.com/statiko-dev/statiko/controller/state"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// RPCServer is the gRPC server that is used to communicate with nodes
type RPCServer struct {
	State *state.Manager

	logger    *log.Logger
	stopCh    chan int
	restartCh chan int
	running   bool
}

// Init the gRPC server
func (s *RPCServer) Init() {
	s.running = false

	// Initialize the logger
	s.logger = log.New(os.Stdout, "rpc: ", log.Ldate|log.Ltime|log.LUTC)

	// Channel used to stop and restart the server
	s.stopCh = make(chan int)
	s.restartCh = make(chan int)
}

// Start the gRPC server; must be run in a goroutine with `go s.Start()`
func (s *RPCServer) Start() {
	for {
		// Create the server
		grpcServer := grpc.NewServer()
		pb.RegisterControllerServer(grpcServer, s)

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
			grpcServer.GracefulStop()
			s.running = false
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.logger.Println("Restarting the gRCP server")
			grpcServer.GracefulStop()
			// Do not return, let the for loop repeat
		}
	}
}

// Restart the server
func (s *RPCServer) Restart() {
	if s.running {
		s.restartCh <- 1
	}
}

// Stop the server
func (s *RPCServer) Stop() {
	if s.running {
		s.stopCh <- 1
	}
}

// GetState is a simple RPC that returns the current state object
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.State, error) {
	return s.State.DumpState()
}

// HealthUpdate is a bi-directional stream that is used by the server to request the health of a node
func (s *RPCServer) HealthUpdate(stream pb.Controller_HealthUpdateServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Println(in)
	}
}

// WatchState is a stream that notifies clients of state updates
func (s *RPCServer) WatchState(req *pb.WatchStateRequest, stream pb.Controller_WatchStateServer) error {
	fmt.Println(req)
	stream.Send(&pb.State{
		Secrets: map[string][]byte{"hello": []byte("world")},
	})
	return nil
}
