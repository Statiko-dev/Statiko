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
	"errors"
	"log"
	"sync"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/controller/state"
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

// Callback invoked when a node joins or leaves the cluster
// It receives as arguments the current number of nodes and the direction of the change:
// A direction of +1 means that a node joined
// A direction of -1 means that a node left
type nodeActivityCb func(count int, direction int)

// Cluster contains information on the nodes in the cluster and methods to interact with their state
type Cluster struct {
	State        *state.Manager
	NodeActivity nodeActivityCb

	logger      *log.Logger
	semaphore   *sync.Mutex
	nodes       map[string]*registeredNode
	verWatchers []chan uint64
	clusterVer  uint64
}

// Init the object
func (c *Cluster) Init() error {
	// Initialize the logger
	c.logger = log.New(buildinfo.LogDestination, "cluster: ", log.Ldate|log.Ltime|log.LUTC)

	// Other properties
	c.semaphore = &sync.Mutex{}
	c.nodes = make(map[string]*registeredNode)
	c.verWatchers = make([]chan uint64, 0)
	c.clusterVer = 0

	return nil
}

// NodeCount returns the number of nodes currently connected
// Note that nodes could connect and disconnect at any time, so this number might be inaccurate immediately
func (c *Cluster) NodeCount() int {
	return len(c.nodes)
}

// RegisterNode registers a new node, returning its id
func (c *Cluster) RegisterNode(nodeName string, ch chan chan *pb.NodeHealth) error {
	// Acquire a lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Check if a node with the same name is already registered
	n, ok := c.nodes[nodeName]
	if ok && n != nil {
		return errors.New("a node is already registered with the same name")
	}

	// Add the node to the map
	// Set version as 0 for now
	c.nodes[nodeName] = &registeredNode{
		HealthChan: ch,
		Version:    0,
	}
	// Reset the clusterVer to 0 because at least one node has version 0
	c.clusterVer = 0

	c.logger.Println("Node registered:", nodeName)

	// Invoke the callback
	if c.NodeActivity != nil {
		c.NodeActivity(len(c.nodes), 1)
	}

	return nil
}

// UnregisterNode un-registers a node
func (c *Cluster) UnregisterNode(nodeName string) {
	// Acquire a lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Remove the node from the map
	obj, ok := c.nodes[nodeName]
	if ok && obj != nil {
		// Close the channel
		if obj.HealthChan != nil {
			close(obj.HealthChan)
		}
		delete(c.nodes, nodeName)
		c.logger.Println("Node un-registered:", nodeName)

		// Invoke the callback
		if c.NodeActivity != nil {
			c.NodeActivity(len(c.nodes), -1)
		}
	}
}
