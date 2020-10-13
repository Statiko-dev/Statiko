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
	"log"
	"os"
	"sync"

	"github.com/google/uuid"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// registeredNode contains properties for each node that is registered
type registeredNode struct {
	// Channel to trigger a ping, which requests the node's health
	// This is a "chan chan", or a channel that is used to pass a response channel
	HealthChan chan chan *pb.NodeHealth

	// Node's state version
	Version uint64
}

// Cluster contains information on the nodes in the cluster and methods to interact with their state
type Cluster struct {
	logger      *log.Logger
	semaphore   *sync.Mutex
	nodes       map[string]*registeredNode
	verWatchers []chan uint64
	clusterVer  uint64
}

// Init the object
func (c *Cluster) Init() error {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "cluster: ", log.Ldate|log.Ltime|log.LUTC)

	// Other properties
	c.semaphore = &sync.Mutex{}
	c.nodes = make(map[string]*registeredNode)
	c.verWatchers = make([]chan uint64, 0)
	c.clusterVer = 0

	return nil
}

// RegisterNode registers a new node, returning its id
func (c *Cluster) RegisterNode() (string, chan chan *pb.NodeHealth, error) {
	// Create a channel that will trigger a ping
	ch := make(chan chan *pb.NodeHealth)

	// Register the node
	nodeIdUUID, err := uuid.NewRandom()
	if err != nil {
		close(ch)
		return "", nil, err
	}
	nodeId := nodeIdUUID.String()

	// Acquire a lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Add the node to the map
	// Set version as 0 for now
	c.nodes[nodeId] = &registeredNode{
		HealthChan: ch,
		Version:    0,
	}
	// Reset the clusterVer to 0 because at least one node has version 0
	c.clusterVer = 0

	c.logger.Println("Node registered:", nodeId)

	return nodeId, ch, nil
}

// UnregisterNode un-registers a node
func (c *Cluster) UnregisterNode(nodeId string) {
	// Acquire a lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Remove the node from the map
	obj, ok := c.nodes[nodeId]
	if ok && obj != nil {
		// Close the channel
		if obj.HealthChan != nil {
			close(obj.HealthChan)
		}
		delete(c.nodes, nodeId)
		c.logger.Println("Node un-registered:", nodeId)
	}
}
