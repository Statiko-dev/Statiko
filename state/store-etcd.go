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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/etcd-io/etcd/clientv3"
	"google.golang.org/grpc/connectivity"

	"github.com/ItalyPaleAle/statiko/appconfig"
)

// Maximum lock duration, in seconds
const etcdLockDuration = 20

type stateStoreEtcd struct {
	state           *NodeState
	client          *clientv3.Client
	stateKey        string
	lockKey         string
	lastRevisionPut int64
	updateCallback  func()
}

// Init initializes the object
func (s *stateStoreEtcd) Init() (err error) {
	s.lastRevisionPut = 0
	s.stateKey = appconfig.Config.GetString("state.etcd.keyPrefix") + "/state"
	s.lockKey = appconfig.Config.GetString("state.etcd.keyPrefix") + "/lock"

	// Connect to the etcd cluster
	addr := strings.Split(appconfig.Config.GetString("state.etcd.address"), ",")
	s.client, err = clientv3.New(clientv3.Config{
		Endpoints:            addr,
		DialTimeout:          2 * time.Second, //5 * time.Second,
		DialKeepAliveTime:    5 * time.Second, //30 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("error while connecting to etcd cluster: %v", err)
	}

	// Watch for changes
	go s.watch()

	// Load the current state
	err = s.ReadState()
	if err != nil {
		return fmt.Errorf("error while requesting state from etcd cluster: %v", err)
	}

	return
}

// Returns a context that times out
func (s *stateStoreEtcd) getContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(appconfig.Config.GetInt("state.etcd.timeout")) * time.Millisecond
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	// Require a leader
	return clientv3.WithRequireLeader(ctx), cancelFunc
}

// Starts the watcher for changes in etcd
func (s *stateStoreEtcd) watch() {
	// Start watching for changes in the key
	ctx, cancel := context.WithCancel(context.Background())
	ctx = clientv3.WithRequireLeader(ctx)
	rch := s.client.Watch(ctx, s.stateKey)

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

				// Invoke the callback as the state has been replaced
				if s.updateCallback != nil {
					s.updateCallback()
				}
			} else if event.Kv.ModRevision < s.lastRevisionPut {
				// Ignoring the case ==, which means we just received the state we just committed
				logger.Println("Ignoring an older state received from etcd: version", event.Kv.ModRevision)
			}
		}
	}

	// Cancel the context
	cancel()

	return
}

// AcquireLock acquires a lock on the state before making changes, across all nodes in the cluster
func (s *stateStoreEtcd) AcquireLock() (interface{}, error) {
	var leaseID clientv3.LeaseID

	// Get a lease
	ctx, cancel := s.getContext()
	lease, err := s.client.Grant(ctx, etcdLockDuration)
	cancel()
	if err != nil {
		return leaseID, err
	}
	leaseID = lease.ID

	// Try to acquire the lock
	i := etcdLockDuration + 5
	for i > 0 {
		logger.Println("Acquiring etcd lock")

		lockValue := fmt.Sprintf("%s-%d", appconfig.Config.GetString("nodeName"), time.Now().UnixNano())
		ctx, cancel = s.getContext()
		txn := s.client.Txn(ctx)
		res, err := txn.If(
			clientv3.Compare(clientv3.Version(s.lockKey), "=", 0),
		).Then(
			clientv3.OpPut(s.lockKey, lockValue, clientv3.WithLease(leaseID)),
		).Commit()
		cancel()

		if err != nil {
			return leaseID, err
		}

		// If this succeeded, we got the lock
		if res.Succeeded {
			break
		} else {
			// Someone else has a lock, so sleep for 1 second
			logger.Printf("Another etcd node has a lock - waiting (timeout in %d seconds)\n", i)
			time.Sleep(1000 * time.Millisecond)
			i--
		}
	}

	if i == 0 {
		return leaseID, errors.New("could not obtain a lock - timeout occurred")
	}

	return leaseID, nil
}

// ReleaseLock releases the lock on the state
func (s *stateStoreEtcd) ReleaseLock(leaseID interface{}) error {
	// Revoke the lease
	ctx, cancel := s.getContext()
	_, err := s.client.Revoke(ctx, leaseID.(clientv3.LeaseID))
	cancel()
	if err != nil {
		return err
	}
	logger.Println("Released etcd lock")

	return nil
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
	ctx, cancel := s.getContext()
	res, err = s.client.Put(ctx, s.stateKey, string(data))
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
	ctx, cancel := s.getContext()
	resp, err := s.client.Get(ctx, s.stateKey)
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

// Healthy returns true if the connection with etcd is active
func (s *stateStoreEtcd) Healthy() (healthy bool, err error) {
	healthy = true

	// Get state of the connection
	state := s.client.ActiveConnection().GetState()
	if state != connectivity.Ready {
		err = errors.New("Connection with etcd in state: " + state.String())
		healthy = false

		// Reset also the lastRevisionPut index to potentially trigger an update when etcd comes back
		s.lastRevisionPut = 0
	}

	return
}

// OnStateUpdate stores the callback that is invoked when there's a new state from etcd
func (s *stateStoreEtcd) OnStateUpdate(callback func()) {
	s.updateCallback = callback
}
