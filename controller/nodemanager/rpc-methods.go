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
func (s *RPCServer) GetState(ctx context.Context, req *pb.GetStateRequest) (*pb.State, error) {
	return s.State.DumpState()
}

// HealthChannel is a bi-directional stream that is used by the server to request the health of a node
func (s *RPCServer) HealthChannel(stream pb.Controller_HealthChannelServer) error {
	ctx, cancel := context.WithCancel(stream.Context())

	// Register the node
	nodeId, ch, err := s.registerNode()
	defer func() {
		cancel()
		if ch != nil {
			close(ch)
		}
	}()
	if err != nil {
		return err
	}

	// Send a ping to request the health
	var responseCh chan *NodeHealthResponse
	go func() {
		for responseCh = range ch {
			err := stream.Send(&pb.NodeHealthPing{})
			if err != nil {
				s.logger.Println("Error while sending health request:", err)
				cancel()
				return
			}
		}
		fmt.Println("Returning from internal goroutine")
	}()

	// Read incoming messages
	for {
		select {
		// Exit if context is done
		case <-ctx.Done():
			return ctx.Err()
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

			in.Reset()
			// Send the response back to the channel, if any
			res := &NodeHealthResponse{
				NodeHealth: in,
				NodeId:     nodeId,
			}

			// Try sending the response if the channel exist
			select {
			case responseCh <- res:
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
		case <-ctx.Done():
			return nil
		case <-stateCh:
			state, err := s.State.DumpState()
			if err != nil {
				return err
			}
			stream.Send(state)
		case <-s.runningCtx.Done():
			return nil
		}
	}
}
