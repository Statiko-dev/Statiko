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

package state

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/etcd-io/etcd/clientv3"
	"github.com/etcd-io/etcd/pkg/transport"
	"google.golang.org/grpc/connectivity"

	"github.com/ItalyPaleAle/statiko/appconfig"
)

// Maximum lock duration, in seconds
const etcdLockDuration = 20

type stateStoreEtcd struct {
	state           *NodeState
	client          *clientv3.Client
	stateKey        string
	stateLockKey    string
	syncLockKey     string
	secretKeyPrefix string
	lastRevisionPut int64
	updateCallback  func()
}

// Init initializes the object
func (s *stateStoreEtcd) Init() (err error) {
	s.lastRevisionPut = 0
	s.stateKey = appconfig.Config.GetString("state.etcd.keyPrefix") + "/state"
	s.stateLockKey = appconfig.Config.GetString("state.etcd.keyPrefix") + "/locks/state"
	s.syncLockKey = appconfig.Config.GetString("state.etcd.keyPrefix") + "/locks/sync"
	s.secretKeyPrefix = appconfig.Config.GetString("state.etcd.keyPrefix") + "/secrets/"

	// TLS configuration
	var tlsConf *tls.Config
	tlsConfCA := appconfig.Config.GetString("state.etcd.tlsConfiguration.ca")
	tlsConfClientCertificate := appconfig.Config.GetString("state.etcd.tlsConfiguration.clientCertificate")
	tlsConfClientKey := appconfig.Config.GetString("state.etcd.tlsConfiguration.clientKey")
	tlsSkipVerify := appconfig.Config.GetBool("state.etcd.tlsSkipVerify")
	if tlsSkipVerify || tlsConfCA != "" {
		tlsInfo := transport.TLSInfo{
			InsecureSkipVerify: tlsSkipVerify,
		}
		// Check if we have a CA certificate
		if tlsConfCA != "" {
			tlsInfo.TrustedCAFile = tlsConfCA

			// Check if we have a client certificate (and key) too
			if tlsConfClientCertificate != "" && tlsConfClientKey != "" {
				tlsInfo.CertFile = tlsConfClientCertificate
				tlsInfo.KeyFile = tlsConfClientKey
			}
		}

		var err error
		tlsConf, err = tlsInfo.ClientConfig()
		if err != nil {
			return err
		}
	}

	// Connect to the etcd cluster
	addr := strings.Split(appconfig.Config.GetString("state.etcd.address"), ",")
	s.client, err = clientv3.New(clientv3.Config{
		Endpoints:            addr,
		DialTimeout:          2 * time.Second, //5 * time.Second,
		DialKeepAliveTime:    5 * time.Second, //30 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		TLS:                  tlsConf,
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
				err := s.unserializeState(event.Kv.Value)
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

// AcquireStateLock acquires a lock on the state before making changes, across all nodes in the cluster
func (s *stateStoreEtcd) AcquireStateLock() (interface{}, error) {
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
		logger.Println("Acquiring etcd state lock")

		succeeded, err := s.tryLockAcquisition(s.stateLockKey, leaseID)
		if err != nil {
			return leaseID, err
		}

		// If this succeeded, we got the lock
		if succeeded {
			break
		} else {
			// Someone else has a lock, so sleep for 1 second
			logger.Printf("Another etcd node has a state lock - waiting (timeout in %d seconds)\n", i)
			time.Sleep(1000 * time.Millisecond)
			i--
		}
	}

	if i == 0 {
		return leaseID, errors.New("could not obtain a state lock - timeout occurred")
	}

	return leaseID, nil
}

// ReleaseStateLock releases the lock on the state
func (s *stateStoreEtcd) ReleaseStateLock(leaseID interface{}) error {
	logger.Println("Releasing etcd state lock")
	return s.releaseLock(leaseID)
}

// AcquireSyncLock acquires a lock on the sync semaphore, ensuring that only one node at a time can be syncing
func (s *stateStoreEtcd) AcquireSyncLock() (interface{}, error) {
	var leaseID clientv3.LeaseID

	// Get a lease
	ctx, cancel := s.getContext()
	lease, err := s.client.Grant(ctx, 10)
	cancel()
	if err != nil {
		return leaseID, err
	}
	leaseID = lease.ID

	// Keep the lease alive forever
	ctx, cancel = s.getContext()
	_, err = s.client.KeepAlive(context.TODO(), leaseID)
	cancel()
	if err != nil {
		return leaseID, err
	}

	// Try to acquire the lock, waiting indefinitely if needed
	for true {
		logger.Println("Acquiring etcd sync lock")

		succeeded, err := s.tryLockAcquisition(s.syncLockKey, leaseID)
		if err != nil {
			return leaseID, err
		}

		// If this succeeded, we got the lock
		if succeeded {
			break
		} else {
			// Someone else has a lock, so sleep for 5 second
			logger.Println("Another etcd node has a sync lock - waiting 5 seconds")
			time.Sleep(5000 * time.Millisecond)
		}
	}

	return leaseID, nil
}

// ReleaseSyncLock releases the lock on the sync semaphore
func (s *stateStoreEtcd) ReleaseSyncLock(leaseID interface{}) error {
	logger.Println("Releasing etcd sync lock")
	return s.releaseLock(leaseID)
}

// Releases a lock, both on state and on sync semaphore
func (s *stateStoreEtcd) releaseLock(leaseID interface{}) error {
	// Revoke the lease
	ctx, cancel := s.getContext()
	_, err := s.client.Revoke(ctx, leaseID.(clientv3.LeaseID))
	cancel()
	if err != nil {
		return err
	}

	return nil
}

// Tries to acquire a lock if no other node has it
func (s *stateStoreEtcd) tryLockAcquisition(lockKey string, leaseID clientv3.LeaseID) (bool, error) {
	lockValue := fmt.Sprintf("%s-%d", appconfig.Config.GetString("nodeName"), time.Now().UnixNano())
	ctx, cancel := s.getContext()
	txn := s.client.Txn(ctx)
	res, err := txn.If(
		clientv3.Compare(clientv3.Version(lockKey), "=", 0),
	).Then(
		clientv3.OpPut(lockKey, lockValue, clientv3.WithLease(leaseID)),
	).Commit()
	cancel()

	if err != nil {
		return false, err
	}

	return res.Succeeded, nil
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
	data, err = s.serializeState()
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

	// Check if the value exists
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 && resp.Kvs[0].Value != nil && len(resp.Kvs[0].Value) > 0 {
		// Parse the JSON from the state
		err = s.unserializeState(resp.Kvs[0].Value)
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

// Serialize the state to JSON
// Additionally, store all secrets longer than 64 bytes in a separate etcd key
func (s *stateStoreEtcd) serializeState() ([]byte, error) {
	// Create a copy of the state
	serialize := NodeState{
		Sites: s.state.Sites,
	}

	// Check if we have any secret
	if s.state.Secrets != nil && len(s.state.Secrets) > 0 {
		serialize.Secrets = make(map[string][]byte, len(s.state.Secrets))

		for k, v := range s.state.Secrets {
			// If the secret is up to 64 bytes, add it as is
			if len(v) <= 64 {
				serialize.Secrets[k] = v
				continue
			}

			// Value is longer than 64 bytes, so store it as a separate key
			// First, calculate its hash
			h := sha256.New()
			if _, err := h.Write(v); err != nil {
				return nil, err
			}
			hash := h.Sum(nil)

			// Convert to hex for the key name
			secretKey := s.secretKeyPrefix + hex.EncodeToString(hash)

			// Encode the value to b64
			secretData := base64.StdEncoding.EncodeToString(v)

			// If the secret doesn't exist, store it
			// Use a transaction that stores the value only if it doesn't exist already
			// Since the key of the secret is its hash, if the key already exists, then we have the same secret
			ctx, cancel := s.getContext()
			txn := s.client.Txn(ctx)
			res, err := txn.If(
				clientv3.Compare(clientv3.Version(secretKey), "=", 0),
			).Then(
				clientv3.OpPut(secretKey, secretData),
			).Commit()
			cancel()

			if err != nil {
				return nil, err
			}

			if res.Succeeded {
				logger.Println("Stored secret:", secretKey)
			}

			// Set the value of the secret to the hash, with the "etcd:" prefix
			serialize.Secrets[k] = append([]byte("etcd:"), hash...)
		}
	}

	// Convert to JSON
	var data []byte
	data, err := json.Marshal(serialize)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Unserialize the state from JSON
// Additionally, retrieve all secrets that were stored separately in etcd
func (s *stateStoreEtcd) unserializeState(data []byte) error {
	// First, unserialize the JSON data
	unserialized := &NodeState{}
	if err := json.Unmarshal(data, unserialized); err != nil {
		return err
	}

	// Iterate through the secrets, if any
	if unserialized.Secrets != nil && len(unserialized.Secrets) > 0 {
		for k, v := range unserialized.Secrets {
			// If the value isn't 37-byte and starting with "etcd:", it's definitely not a hash
			if len(v) != 37 || string(v[0:5]) != "etcd:" {
				continue
			}

			// Check if the secret is already in the current version of the state
			// Since secrets are referenced by its hash, they're immutable
			if s.state != nil && s.state.Secrets != nil {
				if _, ok := s.state.Secrets[k]; ok {
					unserialized.Secrets[k] = s.state.Secrets[k]
					continue
				}
			}

			// Need to retrieve the secret from etcd
			hash := v[5:]
			secretKey := s.secretKeyPrefix + hex.EncodeToString(hash)
			ctx, cancel := s.getContext()
			resp, err := s.client.Get(ctx, secretKey)
			cancel()
			if err != nil {
				return err
			}

			// Check if the secret exists
			if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
				// Decode the value from base64
				secret, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
				if err != nil {
					return err
				}
				unserialized.Secrets[k] = secret
			} else {
				// Doesn't exist, so print a warning and leave the secret empty
				logger.Println("[Warn] Secret not found:", secretKey)
			}
		}
	}

	// Set the state
	s.state = unserialized

	return nil
}
