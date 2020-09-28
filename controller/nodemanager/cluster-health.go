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
	"time"

	"github.com/google/uuid"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// Timeout for health requests in seconds
const healthRequestTimeout = 10

// NodeHealthResponse is the object part of the response containing the health of a single node
// It extends pb.NodeHealth by adding the NodeId and an error
type NodeHealthResponse struct {
	*pb.NodeHealth

	NodeId string `json:"nodeId"`
	Error  string `json:"error,omitempty"`
}

// ClusterHealthResponse is the health of every node in the cluster
type ClusterHealthResponse []*NodeHealthResponse

// RequestClusterHealth requests each node in the cluster to return their health
func (s *RPCServer) RequestClusterHealth() ClusterHealthResponse {
	// Channel to collect responses
	responseCh := make(chan *NodeHealthResponse)

	// Send a message to every channel in the map
	nodes := []string{}
	s.nodeChs.Range(func(key, value interface{}) bool {
		// Get the node ID
		nodeId, ok := key.(string)
		if !ok {
			return true
		}
		nodes = append(nodes, nodeId)

		// Send the message
		ch, ok := value.(chan chan *NodeHealthResponse)
		if !ok {
			return true
		}
		ch <- responseCh
		return true
	})

	// Set a timeout for collecting responses
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(healthRequestTimeout)*time.Second)
	defer cancel()

	// Wait for all the responses
	i := 0
	result := make(ClusterHealthResponse, len(nodes))
	for i < len(nodes) {
		select {
		// Received a response from a node
		case msg := <-responseCh:
			result[i] = msg
			i++
		// Timeout
		case <-ctx.Done():
			break
		}
	}

	// Look for missing responses if any
	if i < len(nodes) {
		// Get a slice of al the keys in the result
		resultKeys := make([]string, len(nodes)-i)
		diff := utils.StringSliceDiff(nodes, resultKeys)
		for _, nodeId := range diff {
			result[i] = &NodeHealthResponse{
				NodeId: nodeId,
				Error:  ctx.Err().Error(),
			}
			i++
		}
	}

	return result
}

// Used internally to register a node
func (s *RPCServer) registerNode() (string, chan chan *NodeHealthResponse, error) {
	// Create a channel that will trigger a ping
	// This is a "chan chan", or a channel that is used to pass a response channel
	ch := make(chan chan *NodeHealthResponse)

	// Register the node
	nodeIdUUID, err := uuid.NewRandom()
	if err != nil {
		close(ch)
		return "", nil, err
	}
	nodeId := nodeIdUUID.String()

	// Store the node in the map
	s.nodeChs.Store(nodeId, ch)

	return nodeId, ch, nil
}

// Used internally to un-register a node
func (s *RPCServer) unregisterNode(nodeId string) {
	// Remove the node from the map
	loaded, ok := s.nodeChs.LoadAndDelete(nodeId)
	if ok && loaded != nil {
		// Close the channel
		ch, ok := loaded.(chan chan *NodeHealthResponse)
		if ok {
			close(ch)
		}
	}
}
