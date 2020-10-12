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
	"io"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetState is a simple RPC that returns the current state object
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.StateMessage, error) {
	state, err := s.State.DumpState()
	if err != nil {
		return nil, err
	}
	return state.StateMessage(), nil
}

// HealthChannel is a bi-directional stream that is used by the server to request the health of a node
func (s *RPCServer) HealthChannel(stream pb.Controller_HealthChannelServer) error {
	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	// Register the node
	nodeId, ch, err := s.registerNode()
	if err != nil {
		return err
	}
	defer s.unregisterNode(nodeId)

	var responseCh chan *pb.NodeHealth
	go func() {
		for responseCh = range ch {
			// Send a ping to request the health
			err := stream.Send(&pb.NodeHealthPing{})
			if err != nil {
				s.logger.Println("Error while sending health request:", err)
				cancel()
				return
			}
		}
	}()

	// Read incoming messages
	for {
		select {
		// Exit if context is done
		case <-ctx.Done():
			return ctx.Err()
		// The server is shutting down
		case <-s.runningCtx.Done():
			return nil
		// Receive a message
		default:
			// Receive a message (the next call is blocking)
			in, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}

			// If there's no response channel, ignore this
			if responseCh == nil {
				continue
			}

			// Send the response back to the channel, if any
			in.NodeId = nodeId

			// Try sending the response if the channel exist
			select {
			case responseCh <- in:
				responseCh = nil
			default:
				responseCh = nil
			}
		}
	}
}

// WatchState is a stream that notifies clients of state updates
func (s *RPCServer) WatchState(req *pb.WatchStateRequest, stream pb.Controller_WatchStateServer) error {
	ctx := stream.Context()
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
			stream.Send(state.StateMessage())
		// The server is shutting down
		case <-s.runningCtx.Done():
			return nil
		}
	}
}

// GetTLSCertificate is a simple RPC that returns a TLS certificate
func (s *RPCServer) GetTLSCertificate(ctx context.Context, in *pb.TLSCertificateRequest) (*pb.TLSCertificateMessage, error) {
	return nil, nil
}
