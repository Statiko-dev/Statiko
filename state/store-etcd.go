/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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

package state

import (
	"context"
	"encoding/json"
	"time"

	"github.com/etcd-io/etcd/clientv3"

	"smplatform/appconfig"
)

type stateStoreEtcd struct {
	state           *NodeState
	client          *clientv3.Client
	ctx             context.Context
	lastRevisionPut int64
}

// Init initializes the object
func (s *stateStoreEtcd) Init() (err error) {
	s.ctx = context.Background()
	s.lastRevisionPut = 0

	// Connect to the etcd cluster
	addr := appconfig.Config.GetStringSlice("etcd.addresses")
	s.client, err = clientv3.New(clientv3.Config{
		Endpoints:   addr,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return
	}

	// Watch for changes
	go s.watch()

	// Load the current state
	err = s.ReadState()

	return
}

// Returns a context that times out
func (s *stateStoreEtcd) getContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(appconfig.Config.GetInt("etcd.timeout")) * time.Millisecond
	return context.WithTimeout(s.ctx, timeout)
}

// Starts the watcher for changes in etcd
func (s *stateStoreEtcd) watch() {
	// Start watching for changes in the key
	key := appconfig.Config.GetString("etcd.key")
	rch := s.client.Watch(s.ctx, key)

	for resp := range rch {
		for _, event := range resp.Events {
			// Semaphore if the other goroutine is still at work
			i := 0
			for s.lastRevisionPut == -1 && i < 100 {
				i++
				time.Sleep(10 * time.Millisecond)
			}

			// Skip if we just stored this value
			if event.Kv.ModRevision > s.lastRevisionPut {
				logger.Println("Received new state from etcd: version", event.Kv.ModRevision)
				oldState := s.state
				s.state = &NodeState{}
				err := json.Unmarshal(event.Kv.Value, s.state)
				if err != nil {
					logger.Println("Error while parsing state", err)
					s.state = oldState
				}
			} else {
				logger.Println("Ignoring an older state received from etcd: version", event.Kv.ModRevision)
			}
		}
	}

	return
}

// GetState returns the full state
func (s *stateStoreEtcd) GetState() *NodeState {
	return s.state
}

// StoreState replaces the current state
func (s *stateStoreEtcd) SetState(state *NodeState) (err error) {
	s.state = state
	return
}

// WriteState stores the state in etcd
func (s *stateStoreEtcd) WriteState() (err error) {
	logger.Println("Writing state in etcd")

	// Convert to JSON
	var data []byte
	data, err = json.Marshal(s.state)
	if err != nil {
		return
	}

	// Set revision to -1 as a semaphore
	s.lastRevisionPut = -1

	// Store in etcd
	var res *clientv3.PutResponse
	key := appconfig.Config.GetString("etcd.key")
	ctx, cancel := s.getContext()
	res, err = s.client.Put(ctx, key, string(data))
	cancel()
	if err != nil {
		s.lastRevisionPut = 0
		return
	}
	s.lastRevisionPut = res.Header.GetRevision()
	logger.Println("Stored state in etcd: version", s.lastRevisionPut)
	return
}

// ReadState reads the state from etcd
func (s *stateStoreEtcd) ReadState() (err error) {
	logger.Println("Reading state from etcd")

	// Read the state
	key := appconfig.Config.GetString("etcd.key")
	ctx, cancel := s.getContext()
	resp, err := s.client.Get(ctx, key)
	cancel()
	if err != nil {
		return
	}

	// Check if the file exists
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		// Parse the JSON from the state
		s.state = &NodeState{}
		err = json.Unmarshal(resp.Kvs[0].Value, s.state)
	} else {
		logger.Println("Will create new state")

		// File doesn't exist, so load an empty state
		sites := make([]SiteState, 0)
		s.state = &NodeState{
			Sites: sites,
		}

		// Write the empty state to disk
		err = s.WriteState()
	}

	return
}
