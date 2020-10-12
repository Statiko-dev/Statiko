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
	// Register the node
	nodeId, ch, err := s.registerNode()
	if err != nil {
		return err
	}
	defer s.unregisterNode(nodeId)

	var responseCh chan *pb.NodeHealth
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

			// If there's no response channel, ignore this message
			if responseCh == nil {
				continue
			}

			// Try sending the response if the channel is not closed
			in.NodeId = nodeId
			select {
			case responseCh <- in:
			default:
			}
			responseCh = nil
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
		case responseCh = <-ch:
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
