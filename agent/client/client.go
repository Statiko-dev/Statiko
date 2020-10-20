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

package client

import (
	"context"
	"io"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/keepalive"

	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/appconfig"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Timeout for all requests, in seconds
const requestTimeout = 15

// Interval between keepalive requests, in seconds
const keepaliveInterval = 600

// RPCClient is the gRPC client for communicating with the cluster manager
type RPCClient struct {
	AgentState *state.AgentState

	client     pb.ControllerClient
	connection *grpc.ClientConn
	logger     *log.Logger
}

// Init the gRPC client
func (c *RPCClient) Init() {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "grpc: ", log.Ldate|log.Ltime|log.LUTC)
}

// Connect starts the connection to the gRPC server and starts all background streams
func (c *RPCClient) Connect() (err error) {
	// Underlying connection
	connOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithInsecure(),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: time.Duration(requestTimeout) * time.Second,
		}),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    time.Duration(keepaliveInterval) * time.Second,
			Timeout: time.Duration(requestTimeout) * time.Second,
		}),
	}
	c.connection, err = grpc.Dial("localhost:2300", connOpts...)
	if err != nil {
		return err
	}

	// Client
	c.client = pb.NewControllerClient(c.connection)

	// Start the background streams in other goroutines
	go c.startStateWatcher()
	go c.startHealthChannel()

	return nil
}

// Disconnect closes the connection with the gRPC server
func (c *RPCClient) Disconnect() error {
	err := c.connection.Close()
	c.connection = nil
	return err
}

// Reconnect re-connects to the gRPC server
func (c *RPCClient) Reconnect() error {
	if c.connection != nil {
		// Ignore errors here
		_ = c.Disconnect()
	}
	return c.Connect()
}

// GetState requests the latest state from the cluster manager
func (c *RPCClient) GetState() (*pb.StateMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Second)
	defer cancel()

	// Make the request
	// The request object is empty
	in := &pb.GetStateRequest{}
	return c.client.GetState(ctx, in, grpc.WaitForReady(true))
}

// startStateWatcher starts the background stream to watch for state updates
func (c *RPCClient) startStateWatcher() {
	// Make the request
	// The request object is empty
	in := &pb.WatchStateRequest{}
	stream, err := c.client.WatchState(context.Background(), in, grpc.WaitForReady(true))
	if err != nil {
		c.logger.Println("State watcher error while connecting to the gRPC server:", err)
		return
	}
	defer stream.CloseSend()
	c.logger.Println("State watcher connected")

	// Watch for incoming messages
	for {
		state, err := stream.Recv()
		if err == io.EOF {
			// TODO: Reconnect
			c.logger.Println("State watcher disconnected from gRPC server")
			break
		}
		if err != nil {
			// TODO: Reconnect
			c.logger.Println("Error caught from the gRPC server while watching for state:", err)
			break
		}

		// Update the state in the manager
		c.AgentState.ReplaceState(state)
	}
}

// startHealthChannel starts the background stream to register this node and respond to health requests
func (c *RPCClient) startHealthChannel() {
	// Making the connection will register the node with the cluster manager
	stream, err := c.client.HealthChannel(context.Background(), grpc.WaitForReady(true))
	if err != nil {
		c.logger.Println("Health channel error while connecting to the gRPC server:", err)
		return
	}
	defer stream.CloseSend()
	c.logger.Println("Health channel connected")

	// Watch for incoming messages
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			// TODO: Reconnect
			c.logger.Println("Health channel disconnected from gRPC server")
			break
		}
		if err != nil {
			// TODO: Reconnect
			c.logger.Println("Error caught from the gRPC server in the health channel:", err)
			break
		}

		// We received a ping, which means we need to respond with our health
		// TODO THIS
		msg := &pb.NodeHealth{
			NodeName: appconfig.Config.GetString("nodeName"),
		}
		err = stream.Send(msg)
		if err != nil {
			// TODO: Reconnect
			c.logger.Println("Error while sending health:", err)
			break
		}
	}
}
