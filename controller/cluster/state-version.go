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
)

var ErrNoNodes = errors.New("no node connected")

// WaitForVersion blocks until all nodes in the cluster report being on at least state version ver (or higher)
func (c *Cluster) WaitForVersion(ver uint64) error {
	// This requires a lock
	c.semaphore.Lock()

	// If there are no nodes, return with an error
	if c.NodeCount() == 0 {
		c.semaphore.Unlock()
		return ErrNoNodes
	}

	// If the cluster is already on the desired version, return
	if c.clusterVer >= ver {
		c.semaphore.Unlock()
		return nil
	}

	// Add a watcher; this still requires a lock
	ch := make(chan uint64)
	c.verWatchers = append(c.verWatchers, ch)

	// Unlock now
	c.semaphore.Unlock()

	// Wait until the desired version is announced in the channel
	// This is a blocking call
	// TODO: NEEDS A TIMEOUT HERE
	for v := range ch {
		if v >= ver {
			break
		}
	}

	// Remove the watcher
	// This requires another lock
	c.semaphore.Lock()

	// Keep all channels except the one we were using
	watchers := make([]chan uint64, len(c.verWatchers)-1)
	i := 0
	for _, w := range c.verWatchers {
		if w != ch {
			watchers[i] = w
		}
	}
	c.verWatchers = watchers

	// Unlock
	c.semaphore.Unlock()

	// If nodes disconnected during the loop, we might be here but not with the right version
	if c.clusterVer < ver {
		return ErrNoNodes
	}

	return nil
}
