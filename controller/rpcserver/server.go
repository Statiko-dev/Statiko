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

package rpcserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/state"
	controllerutils "github.com/statiko-dev/statiko/controller/utils"
	"github.com/statiko-dev/statiko/shared/fs"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// RPCServer is the gRPC server that is used to communicate with nodes
type RPCServer struct {
	State   *state.Manager
	Cluster *cluster.Cluster
	Certs   *certificates.Certificates
	Fs      fs.Fs
	TLSCert *tls.Certificate

	logger        *log.Logger
	stopCh        chan int
	restartCh     chan int
	doneCh        chan int
	runningCtx    context.Context
	runningCancel context.CancelFunc
	running       bool
	grpcServer    *grpc.Server
}

// Init the gRPC server
func (s *RPCServer) Init() {
	s.running = false

	// Initialize the logger
	s.logger = log.New(buildinfo.LogDestination, "grpc: ", log.Ldate|log.Ltime|log.LUTC)

	// Channels used to stop and restart the server
	s.stopCh = make(chan int)
	s.restartCh = make(chan int)
	s.doneCh = make(chan int)
}

// Start the gRPC server; must be run in a goroutine with `go s.Start()`
func (s *RPCServer) Start() {
	// Port for the gRPC server
	port := viper.GetInt("controller.grpcPort")

	// Continue until the server is stopped
	for {
		// Create the context
		s.runningCtx, s.runningCancel = context.WithCancel(context.Background())

		// Methods that don't require auth
		noAuth := []string{
			"/statiko.Controller/GetClusterOptions",
		}

		// Create the server
		s.grpcServer = grpc.NewServer(
			grpc.Creds(credentials.NewServerTLSFromCert(s.TLSCert)),
			grpc.UnaryInterceptor(controllerutils.AuthGRPCUnaryInterceptor(noAuth)),
			grpc.StreamInterceptor(controllerutils.AuthGRPCStreamInterceptor),
		)
		pb.RegisterControllerServer(s.grpcServer, s)

		// Register reflection service on gRPC server in development
		if buildinfo.ENV != "production" {
			reflection.Register(s.grpcServer)
		}

		// Start the server in another channel
		go func() {
			// Listen
			listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
			if err != nil {
				s.runningCancel()
				s.logger.Fatal(err)
			}
			s.logger.Printf("Starting gRPC server on port %d\n", port)
			s.running = true
			s.grpcServer.Serve(listener)
		}()

		select {
		case <-s.stopCh:
			// We received an interrupt signal, shut down for good
			s.logger.Println("Shutting down the gRCP server")
			s.gracefulStop()
			s.running = false
			s.doneCh <- 1
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.logger.Println("Restarting the gRCP server")
			s.gracefulStop()
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

// Internal function that gracefully stops the gRPC server, with a timeout
func (s *RPCServer) gracefulStop() {
	const shutdownTimeout = 15
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownTimeout)*time.Second)
	defer cancel()

	// Cancel the context
	s.runningCancel()

	// Try gracefulling closing the gRPC server
	closed := make(chan int)
	go func() {
		s.grpcServer.GracefulStop()
		if closed != nil {
			// Use a select just in case the channel was closed
			select {
			case closed <- 1:
			default:
			}
		}
	}()

	select {
	// Closed - all good
	case <-closed:
		close(closed)
	// Timeout
	case <-ctx.Done():
		// Force close
		s.logger.Printf("Shutdown timeout of %d seconds reached - force shutdown\n", shutdownTimeout)
		s.grpcServer.Stop()
		close(closed)
		closed = nil
	}
	s.logger.Println("gRPC server shut down")
}
