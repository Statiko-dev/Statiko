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
	"errors"
	"io"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Performs the node registration
func (s *RPCServer) channelRegisterNode(stream pb.Controller_ChannelServer, ch chan chan *pb.NodeHealth) (string, error) {
	// This call is blocking
	in, err := stream.Recv()
	if err != nil {
		// Error also for io.EOF error
		s.logger.Println("Error while reading message:", err)
		return "", err
	}

	// Message must be a registration request
	if in.Type != pb.ChannelClientStream_REGISTER_NODE {
		s.logger.Println("Node sent a message that was not a registration request")
		return "", errors.New("need to send registration request first")
	}

	// Ensure the registration request contains a node name
	if in.Registration == nil || in.Registration.NodeName == "" {
		s.logger.Println("Node sent a registration request message without a node name")
		return "", errors.New("invalid registration message")
	}

	// Register the node
	// This also checks if a node with the same name already exists
	err = s.Cluster.RegisterNode(in.Registration.NodeName, ch)
	if err != nil {
		s.logger.Println("Error while registering a node:", err)
		return "", err
	}

	// Send the OK acknowledgement
	err = stream.SendMsg(&pb.ChannelServerStream{
		Type: pb.ChannelServerStream_OK,
	})

	return in.Registration.NodeName, nil
}

// Struct containing a message and error
type clientStreamMessage struct {
	Message *pb.ChannelClientStream
	Error   error
	Done    bool
}

// Returns a channel that contains the mesages received by the gRPC client stream
func clientStreamToChan(stream pb.Controller_ChannelServer) <-chan *clientStreamMessage {
	ch := make(chan *clientStreamMessage)
	go func() {
		for {
			// This call is blocking
			in, err := stream.Recv()

			if err == io.EOF {
				// End of the stream
				ch <- &clientStreamMessage{
					Done: true,
				}
				return
			} else if err != nil {
				// Errors
				ch <- &clientStreamMessage{
					Error: err,
				}
				return
			}

			// Send the message
			ch <- &clientStreamMessage{
				Message: in,
			}
		}
	}()
	return ch
}
