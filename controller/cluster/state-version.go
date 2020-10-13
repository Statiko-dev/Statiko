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

// WaitForVersion blocks until all nodes in the cluster report being on at least state version ver (or higher)
func (c *Cluster) WaitForVersion(ver uint64) {
	// First, if the cluster is already on the desired version, return
	// This requires a lock
	c.semaphore.Lock()
	if c.clusterVer >= ver {
		return
	}

	// Add a watcher; this still requires a lock
	ch := make(chan uint64)
	defer close(ch)
	c.verWatchers = append(c.verWatchers, ch)

	// Unlock now
	c.semaphore.Unlock()

	// Wait until the desired version is announced in the channel
	// This is a blocking call
	for v := range ch {
		if v >= ver {
			break
		}
	}

	// Remove the watcher
	// This requires another lock
	c.semaphore.Lock()
	defer c.semaphore.Unlock()

	// Keep all channels except the one we were using
	watchers := make([]chan uint64, len(c.verWatchers)-1)
	i := 0
	for _, w := range c.verWatchers {
		if w != ch {
			watchers[i] = w
		}
	}
	c.verWatchers = watchers
}
