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

package cluster

import (
	"context"
	"math"
	"time"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// Timeout for health requests in seconds
const healthRequestTimeout = 15

// ClusterHealthResponse is the health of every node in the cluster
type ClusterHealthResponse []*pb.NodeHealth

// RequestClusterHealth requests each node in the cluster to return their health
// This blocks until we have the health
func (c *Cluster) RequestClusterHealth() ClusterHealthResponse {
	// Channel to collect responses
	responseCh := make(chan *pb.NodeHealth)

	// Lock before iterating through the map
	c.semaphore.Lock()

	// Send a message to every channel in the map
	nodes := []string{}
	for nodeId, value := range c.nodes {
		nodes = append(nodes, nodeId)

		// Send the ping with the response channel
		value.HealthChan <- responseCh
	}

	// Unlock as we're done with the map
	c.semaphore.Unlock()

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
			result[i] = &pb.NodeHealth{
				XNodeId: nodeId,
				XError:  ctx.Err().Error(),
			}
			i++
		}
	}

	return result
}

// ReceivedVersion is called when a node reports a new version of their state
func (c *Cluster) ReceivedVersion(nodeId string, ver uint64) {
	// Acquire a lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Check if the object exists, then update the version for that node
	obj, ok := c.nodes[nodeId]
	if !ok || obj == nil {
		return
	}
	obj.Version = ver

	// Check the minimum version of each node in the cluster
	c.clusterVer = c.minNodeStateVersion()

	// Notify all watchers
	for i := 0; i < len(c.verWatchers); i++ {
		c.verWatchers[i] <- c.clusterVer
	}
}

// Internal function that returns the minimum version of the state in the cluster
func (c *Cluster) minNodeStateVersion() (ver uint64) {
	// Empty cluster has version 0
	if len(c.nodes) == 0 {
		return 0
	}

	// Find the minimum version of the state
	ver = math.MaxUint64
	for _, node := range c.nodes {
		if node != nil && node.Version < ver {
			ver = node.Version
		}
	}
	return
}
