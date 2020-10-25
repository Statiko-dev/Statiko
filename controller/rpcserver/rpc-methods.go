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
	"errors"
	"fmt"
	"sync"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetState is a simple RPC that returns the current state object
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.StateMessage, error) {
	// Get the state message
	state, err := s.State.DumpState()
	if err != nil {
		return nil, err
	}
	return state.StateMessage(), nil
}

// Channel is a bi-directional stream that is used for:
// 1. Registering a node
// 2. Allowing the server to request the health of a node
// 3. Notify nodes of state updates
func (s *RPCServer) Channel(stream pb.Controller_ChannelServer) error {
	s.logger.Println("Client connected")
	defer s.logger.Println("Client disconnected")

	// Channel used for responding to health pings
	ch := make(chan chan *pb.NodeHealth)

	// Wait for the node to register itself
	nodeName, err := s.channelRegisterNode(stream, ch)
	if err != nil {
		return err
	}
	// Unregister the node when the channel ends
	defer s.Cluster.UnregisterNode(nodeName)

	// Subscribe to the state
	stateCh := make(chan int)
	s.State.Subscribe(stateCh)
	defer func() {
		s.State.Unsubscribe(stateCh)
		close(stateCh)
	}()

	// Collect the responses when requesting nodes' health
	responseChs := make([]chan *pb.NodeHealth, 0)
	semaphore := sync.Mutex{}

	// Channel to receive messages
	msgCh := clientStreamToChan(stream)

	// Send and receive messages
forloop:
	for {
		select {
		// New message
		case in := <-msgCh:
			if in.Error != nil {
				// Abort
				s.logger.Println("Error while reading message:", in.Error)
				return in.Error
			}
			if in.Done {
				// Exit without error
				s.logger.Println("Stream reached EOF")
				return nil
			}
			if in.Message == nil {
				// Ignore empty messages
				continue forloop
			}

			// Check the type of message
			switch in.Message.Type {
			// New health message received
			case pb.ChannelClientStream_HEALTH_MESSAGE:
				// Store the state version the node is on
				s.Cluster.ReceivedVersion(nodeName, in.Message.Health.Version)

				// If there's no response channel, stop processing here
				if responseChs == nil {
					continue forloop
				}

				// Try sending the response to each channel if they're not closed
				semaphore.Lock()
				for i := 0; i < len(responseChs); i++ {
					if responseChs == nil {
						continue
					}
					select {
					case responseChs[i] <- in.Message.Health:
					default:
					}
				}
				responseChs = make([]chan *pb.NodeHealth, 0)
				semaphore.Unlock()

			// Invalid message
			default:
				s.logger.Printf("node %s sent a message with an invalid type: %d", nodeName, in.Message.Type)
				continue forloop
			}

		// Need to send a ping to request the health
		// Note that this is triggered only after the registration is complete
		case rch := <-ch:
			semaphore.Lock()
			responseChs = append(responseChs, rch)
			semaphore.Unlock()
			err := stream.Send(&pb.ChannelServerStream{
				Type: pb.ChannelServerStream_HEALTH_PING,
			})
			if err != nil {
				s.logger.Println("Error while sending health request:", err)
				return err
			}

		// Send the new state to the clients
		case <-stateCh:
			state, err := s.State.DumpState()
			if err != nil {
				return err
			}
			stream.Send(&pb.ChannelServerStream{
				Type:  pb.ChannelServerStream_STATE_MESSAGE,
				State: state.StateMessage(),
			})

		// Exit if context is done
		case <-stream.Context().Done():
			fmt.Println("stream.Context().Done()")
			return nil

		// The server is shutting down
		case <-s.runningCtx.Done():
			fmt.Println("runningCtx.Done()")
			return nil
		}
	}
}

// GetTLSCertificate is a simple RPC that returns a TLS certificate
func (s *RPCServer) GetTLSCertificate(ctx context.Context, in *pb.TLSCertificateRequest) (*pb.TLSCertificateMessage, error) {
	// Get the certificate ID
	certId := in.CertificateId
	if certId == "" {
		return nil, errors.New("empty certificate ID")
	}

	// Get the certificate
	key, cert, err := s.Certs.GetCertificate(certId)
	if err != nil {
		return nil, err
	}
	if len(key) == 0 || len(cert) == 0 {
		return nil, errors.New("certificate not found")
	}

	// Response
	return &pb.TLSCertificateMessage{
		CertificatePem: string(cert),
		KeyPem:         string(key),
	}, nil
}
