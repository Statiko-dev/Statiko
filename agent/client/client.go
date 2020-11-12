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
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"
	"time"

	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"

	"github.com/statiko-dev/statiko/buildinfo"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Timeout for all requests, in seconds
const requestTimeout = 15

// Interval between keepalive requests, in seconds
const keepaliveInterval = 600

// Callback invoked when a new state is received from the controller
type StateUpdateCallback func(*pb.StateMessage)

// Function providing the node's health
type NodeHealthCallback func() *pb.NodeHealth

// RPCClient is the gRPC client for communicating with the cluster manager
type RPCClient struct {
	GetHealth   NodeHealthCallback
	StateUpdate StateUpdateCallback

	client       pb.ControllerClient
	connection   *grpc.ClientConn
	logger       *log.Logger
	connectedCh  chan bool
	sendHealthCh chan bool
}

// Init the gRPC client
func (c *RPCClient) Init() {
	// Initialize the logger
	c.logger = log.New(buildinfo.LogDestination, "grpc: ", log.Ldate|log.Ltime|log.LUTC)
}

// Connect starts the connection to the gRPC server and starts all background streams
// Returns a channel that gets a message every time a connection is established
func (c *RPCClient) Connect() (connectedCh chan bool, err error) {
	c.logger.Println("Connecting to gRPC server at", viper.GetString("controller.address"))

	// Authentication info
	rpcCreds := &rpcAuth{}
	err = rpcCreds.Init()
	if err != nil {
		return nil, err
	}

	// TLS configuration
	tlsConf := &tls.Config{}

	// Option to use a custom CA certificate rather than the root one
	caFile := viper.GetString("controller.tls.ca")
	if caFile != "" {
		// Replace the RootCA with a new cert pool
		certs := x509.NewCertPool()
		crt, err := ioutil.ReadFile(caFile)
		if err != nil {
			return nil, err
		}
		certs.AppendCertsFromPEM(crt)
		tlsConf.RootCAs = certs
	}

	// Option to skip verifying TLS certificates
	// This is insecure and should be used in development only
	insecureTls := viper.GetBool("controller.tls.insecure")
	if insecureTls {
		tlsConf.InsecureSkipVerify = true
		c.logger.Println("[Warn] `controller.tls.insecure` is enabled: server TLS certificates aren't validated")
	}

	// Credentials for gRPC
	creds := credentials.NewTLS(tlsConf)

	// Establish the underlying connection
	connOpts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithTransportCredentials(creds),
		grpc.WithPerRPCCredentials(rpcCreds),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff:           backoff.DefaultConfig,
			MinConnectTimeout: time.Duration(requestTimeout) * time.Second,
		}),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    time.Duration(keepaliveInterval) * time.Second,
			Timeout: time.Duration(requestTimeout) * time.Second,
		}),
	}
	c.connection, err = grpc.DialContext(context.Background(), viper.GetString("controller.address"), connOpts...)
	if err != nil {
		return nil, err
	}

	// Client
	c.client = pb.NewControllerClient(c.connection)
	c.logger.Println("Connection with gRPC server established")

	// Channel that receives a message every time we establish a connection
	c.connectedCh = make(chan bool)

	// Channel used to send the health to the controller
	c.sendHealthCh = make(chan bool)

	// Start the background stream in another goroutine
	go func() {
		// Continue re-connecting automatically if the connection drops
		for c.connection != nil {
			c.logger.Println("Connecting to the channel")
			// Note that if the underlying connection is down, this call blocks until it comes back
			c.startStreamChannel()
		}
	}()

	return c.connectedCh, nil
}

// Disconnect closes the connection with the gRPC server
func (c *RPCClient) Disconnect() error {
	close(c.connectedCh)
	conn := c.connection
	c.connection = nil
	err := conn.Close()
	return err
}

// Reconnect re-connects to the gRPC server
// Returns a new channel that gets a message every time a connection is established
func (c *RPCClient) Reconnect() (chan bool, error) {
	if c.connection != nil {
		// Ignore errors here
		_ = c.Disconnect()
	}
	return c.Connect()
}

// Client returns the underlying client object
func (c *RPCClient) Client() pb.ControllerClient {
	return c.client
}

// GetState requests the latest state from the cluster manager
func (c *RPCClient) GetState() (*pb.StateMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Second)
	defer cancel()

	// Make the request
	in := &pb.GetStateRequest{}
	return c.client.GetState(ctx, in, grpc.WaitForReady(true))
}

// GetTLSCertificate requests a TLS certificate from the cluster manager
func (c *RPCClient) GetTLSCertificate(certificateId string) (*pb.TLSCertificateMessage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Second)
	defer cancel()

	// Make the request
	in := &pb.TLSCertificateRequest{
		CertificateId: certificateId,
	}
	return c.client.GetTLSCertificate(ctx, in, grpc.WaitForReady(true))
}

// GetClusterOptions requests the cluster options object
func (c *RPCClient) GetClusterOptions() (*pb.ClusterOptions, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Second)
	defer cancel()

	// Make the request
	in := &pb.ClusterOptionsRequest{}
	return c.client.GetClusterOptions(ctx, in, grpc.WaitForReady(true))
}

// GetACMEChallengeResponse requests the response to an ACME challenge
func (c *RPCClient) GetACMEChallengeResponse(token string, domain string) (*pb.ACMEChallengeResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(requestTimeout)*time.Second)
	defer cancel()

	// Make the request
	in := &pb.ACMEChallengeRequest{
		Token:  token,
		Domain: domain,
	}
	return c.client.GetACMEChallengeResponse(ctx, in, grpc.WaitForReady(true))
}

// Starts the stream channel with the server
func (c *RPCClient) startStreamChannel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get node name
	nodeName := viper.GetString("nodeName")

	// Connect to the stream RPC
	stream, err := c.client.Channel(ctx, grpc.WaitForReady(true))
	if err != nil {
		c.logger.Println("Error while connecting to the Channel stream:", err)
		return
	}
	defer stream.CloseSend()
	c.logger.Println("Channel connected")

	// Send the message to register the node
	err = stream.Send(&pb.ChannelClientStream{
		Type: pb.ChannelClientStream_REGISTER_NODE,
		Registration: &pb.ChannelClientStream_RegisterNode{
			NodeName: nodeName,
		},
	})
	if err != nil {
		c.logger.Println("Error while sending registration message:", err)
		return
	}

	// Channel to receive messages
	msgCh := serverStreamToChan(stream)

	// Flag: has received the "OK" message after registering
	registered := false

	// Send and receive messages
forloop:
	for {
		select {
		// New message
		case in := <-msgCh:
			if in.Error != nil {
				// Abort
				c.logger.Println("Error while reading message:", in.Error)
				// Pause for a few seconds to avoid DoS'ing the server, e.g. if there's an auth error
				time.Sleep(5 * time.Second)
				return
			}
			if in.Done {
				// Exit without error
				c.logger.Println("Stream reached EOF")
				return
			}
			if in.Message == nil {
				// Ignore empty messages
				continue forloop
			}

			// Check the type of message
			switch in.Message.Type {
			// Registered correctly
			case pb.ChannelServerStream_OK:
				c.logger.Println("Node registered correctly")
				registered = true
				// Notify the app that we have a successful connection
				c.connectedCh <- true

			// Server sent an error
			case pb.ChannelServerStream_ERROR:
				// Abort
				errStr := "(empty error message)"
				if in.Message != nil && in.Message.Error != "" {
					errStr = in.Message.Error
				}
				c.logger.Println("Received an error from the server:", errStr)
				return

			// New state
			case pb.ChannelServerStream_STATE_MESSAGE:
				// Error if we haven't registered yet
				if !registered {
					c.logger.Println("Received a state message, but have not received confirmation of node registration yet")
					return
				}
				// Ensure we have a state in the message
				if in.Message.State != nil {
					c.logger.Println("Received new state")
					// Invoke the callback with the new state
					if c.StateUpdate != nil {
						c.StateUpdate(in.Message.State)
					}
				}

			// Received a health ping
			case pb.ChannelServerStream_HEALTH_PING:
				c.SendHealth()

			// Invalid message
			default:
				c.logger.Printf("Server sent a message with an invalid type: %d", in.Message.Type)
				continue forloop
			}

		// Send the node's health to the controlller
		case <-c.sendHealthCh:
			c.logger.Println("Sending node health to controller")
			err = stream.Send(&pb.ChannelClientStream{
				Type:   pb.ChannelClientStream_HEALTH_MESSAGE,
				Health: c.GetHealth(),
			})
			if err != nil {
				// Abort
				c.logger.Println("Error while sending health:", err)
				return
			}

		// Context for canceling the operation
		case <-ctx.Done():
			c.logger.Println("Channel closed")
			return
		}
	}
}

// SendHealth sends the health of the node to the controller even if that wasn't requested
func (c *RPCClient) SendHealth() {
	c.sendHealthCh <- true
}
