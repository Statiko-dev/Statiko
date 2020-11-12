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
	"io"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Struct containing a message and error
type serverStreamMessage struct {
	Message *pb.ChannelServerStream
	Error   error
	Done    bool
}

// Returns a channel that contains the mesages received by the gRPC server stream
func serverStreamToChan(stream pb.Controller_ChannelClient) <-chan *serverStreamMessage {
	ch := make(chan *serverStreamMessage)
	go func() {
		for {
			// This call is blocking
			in, err := stream.Recv()

			if err == io.EOF {
				// End of the stream
				ch <- &serverStreamMessage{
					Done: true,
				}
				return
			} else if err != nil {
				// Errors
				ch <- &serverStreamMessage{
					Error: err,
				}
				return
			}

			// Send the message
			ch <- &serverStreamMessage{
				Message: in,
			}
		}
	}()
	return ch
}
