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
	"io"
	"sync"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetState is a simple RPC that returns the current state object
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.StateMessage, error) {
	if req.NodeName == "" {
		return nil, errors.New("Argument NodeName is empty")
	}

	// Get the state message
	state, err := s.State.DumpState()
	if err != nil {
		return nil, err
	}
	msg := state.StateMessage(req.NodeName)

	// Check if the message's agent options is nil - if it is, need to generate a new object
	if msg.AgentOptions == nil {
		// TODO: GENERATE TLS CERT
		msg.AgentOptions = &pb.AgentOptions{}
	}

	return msg, nil
}

// HealthChannel is a bi-directional stream that is used by the server to request the health of a node
func (s *RPCServer) HealthChannel(stream pb.Controller_HealthChannelServer) error {
	// Register the node
	nodeId, ch, err := s.Cluster.RegisterNode()
	if err != nil {
		return err
	}
	defer s.Cluster.UnregisterNode(nodeId)

	responseChs := make([]chan *pb.NodeHealth, 0)
	semaphore := sync.Mutex{}
	go func() {
		// Receive messages in background
		for {
			// This call is blocking
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				s.logger.Println("Error while reading health message:", err)
				break
			}

			// Store the state version the node is on
			s.Cluster.ReceivedVersion(nodeId, in.Version)

			// If there's no response channel, stop processing here
			if responseChs == nil {
				continue
			}

			// Try sending the response to each channel if they're not closed
			semaphore.Lock()
			in.XNodeId = nodeId
			for i := 0; i < len(responseChs); i++ {
				if responseChs == nil {
					continue
				}
				select {
				case responseChs[i] <- in:
				default:
				}
			}
			responseChs = make([]chan *pb.NodeHealth, 0)
			semaphore.Unlock()
		}
	}()

	for {
		select {
		// Exit if context is done
		case <-stream.Context().Done():
			fmt.Println("stream.Context().Done()")
			return nil
		// The server is shutting down
		case <-s.runningCtx.Done():
			fmt.Println("runningCtx.Done()")
			return nil
		// Need to send a ping to request the health
		case rch := <-ch:
			semaphore.Lock()
			responseChs = append(responseChs, rch)
			semaphore.Unlock()
			err := stream.Send(&pb.NodeHealthPing{})
			if err != nil {
				s.logger.Println("Error while sending health request:", err)
				return err
			}
		}
	}
}

// WatchState is a stream that notifies clients of state updates
func (s *RPCServer) WatchState(req *pb.WatchStateRequest, stream pb.Controller_WatchStateServer) error {
	ctx := stream.Context()

	// Subscribe to the state
	stateCh := make(chan int)
	s.State.Subscribe(stateCh)
	defer func() {
		s.State.Unsubscribe(stateCh)
		close(stateCh)
	}()

	// Send updates when requested
	for {
		select {
		// RPC done
		case <-ctx.Done():
			return nil
		// Send the new state to the clients
		case <-stateCh:
			state, err := s.State.DumpState()
			if err != nil {
				return err
			}
			stream.Send(state.StateMessage(req.NodeName))
		// The server is shutting down
		case <-s.runningCtx.Done():
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
