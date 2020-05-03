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
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/google/uuid"
	"google.golang.org/grpc/connectivity"

	"github.com/statiko-dev/statiko/appconfig"
)

// Maximum lock duration, in seconds
const EtcdLockDuration = 20

// TTL for node registration
const etcdNodeRegistrationTTL = 30

type StateStoreEtcd struct {
	state  *NodeState
	client *clientv3.Client

	stateKey         string
	dhparamsKey      string
	locksKeyPrefix   string
	nodesKeyPrefix   string
	secretsKeyPrefix string

	clusterMemberId string
	lastRevisionPut int64
	updateCallback  func()
}

// Init initializes the object
func (s *StateStoreEtcd) Init() (err error) {
	s.lastRevisionPut = 0

	// Keys and prefixes
	keyPrefix := appconfig.Config.GetString("state.etcd.keyPrefix")
	s.stateKey = keyPrefix + "/state"
	s.dhparamsKey = keyPrefix + "/dhparams"
	s.locksKeyPrefix = keyPrefix + "/locks/"
	s.nodesKeyPrefix = keyPrefix + "/nodes/"
	s.secretsKeyPrefix = keyPrefix + "/secrets/"

	// Random ID for this cluster member
	s.clusterMemberId = uuid.New().String()

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
		DialTimeout:          5 * time.Second,
		DialKeepAliveTime:    5 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		TLS:                  tlsConf,
	})
	if err != nil {
		return fmt.Errorf("error while connecting to etcd cluster: %v", err)
	}

	// Register the node
	err = s.registerNode()
	if err != nil {
		return fmt.Errorf("error while registering node: %v", err)
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

// GetContext returns a context that times out
func (s *StateStoreEtcd) GetContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(appconfig.Config.GetInt("state.etcd.timeout")) * time.Millisecond
	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	// Require a leader in the etcd cluster
	return clientv3.WithRequireLeader(ctx), cancelFunc
}

// GetMemberId returns the ID of this member in the cluster
func (s *StateStoreEtcd) GetMemberId() string {
	return s.clusterMemberId
}

// GetClient returns the etcd client with the active connection
func (s *StateStoreEtcd) GetClient() *clientv3.Client {
	return s.client
}

// Registers the node within the cluster
func (s *StateStoreEtcd) registerNode() error {
	// Get a lease
	ctx, cancel := s.GetContext()
	lease, err := s.client.Grant(ctx, etcdNodeRegistrationTTL)
	cancel()
	if err != nil {
		return err
	}

	// Put the node name
	ctx, cancel = s.GetContext()
	_, err = s.client.Put(
		ctx,
		s.nodesKeyPrefix+s.clusterMemberId,
		appconfig.Config.GetString("nodeName"),
		clientv3.WithLease(lease.ID),
	)
	cancel()
	if err != nil {
		return err
	}

	// Maintain the key for as long as the node is up
	_, err = s.client.KeepAlive(context.Background(), lease.ID)
	if err != nil {
		return err
	}

	logger.Println("Registered node with etcd, with member ID", s.clusterMemberId)

	return nil
}

// Starts the watcher for changes in etcd
func (s *StateStoreEtcd) watch() {
	// Start watching for changes in the key
	ctx, cancel := context.WithCancel(context.Background())
	ctx = clientv3.WithRequireLeader(ctx)
	rch := s.client.Watch(ctx, s.stateKey)

	for resp := range rch {
		// Check for unrecoverable errors
		if resp.Err() != nil {
			panic(fmt.Errorf("unrecoverable error from etcd watcher: %v", resp.Err()))
		}

		// Always get the last message only
		event := resp.Events[len(resp.Events)-1]

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

	// Cancel the context
	cancel()

	return
}

// AcquireLock acquires a lock, with an optional timeout
func (s *StateStoreEtcd) AcquireLock(name string, timeout bool) (interface{}, error) {
	var leaseID clientv3.LeaseID

	// Get a lease
	ctx, cancel := s.GetContext()
	lease, err := s.client.Grant(ctx, EtcdLockDuration)
	cancel()
	if err != nil {
		return leaseID, err
	}
	leaseID = lease.ID

	// Try to acquire the lock
	i := EtcdLockDuration * 2
	for i > 0 {
		logger.Println("Acquiring lock in etcd:", name)

		succeeded, err := s.tryLockAcquisition(s.locksKeyPrefix+name, leaseID)
		if err != nil {
			return leaseID, err
		}

		// If this succeeded, we got the lock
		if succeeded {
			break
		} else {
			// Someone else has a lock, so sleep for 1 second
			if timeout {
				logger.Printf("Another node has a lock - waiting (timeout in %d seconds)\n", i)
				i--
			} else {
				logger.Println("Another node has a lock - waiting")
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}

	if i == 0 {
		return leaseID, errors.New("could not obtain a state lock - timeout occurred")
	}

	logger.Println("Acquired etcd lock with ID:", leaseID)

	return leaseID, nil
}

// ReleaseLock releases a lock
func (s *StateStoreEtcd) ReleaseLock(leaseID interface{}) error {
	logger.Println("Releasing etcd lock with ID:", leaseID)

	// Revoke the lease
	ctx, cancel := s.GetContext()
	_, err := s.client.Revoke(ctx, leaseID.(clientv3.LeaseID))
	cancel()
	if err != nil {
		return err
	}

	return nil
}

// Tries to acquire a lock if no other node has it
func (s *StateStoreEtcd) tryLockAcquisition(lockKey string, leaseID clientv3.LeaseID) (bool, error) {
	lockValue := fmt.Sprintf("%s-%d", appconfig.Config.GetString("nodeName"), time.Now().UnixNano())
	ctx, cancel := s.GetContext()
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
func (s *StateStoreEtcd) GetState() *NodeState {
	return s.state
}

// StoreState replaces the current state
func (s *StateStoreEtcd) SetState(state *NodeState) (err error) {
	s.state = state
	return
}

// WriteState stores the state in etcd
func (s *StateStoreEtcd) WriteState() (err error) {
	logger.Println("Writing state in etcd")

	// Convert to JSON
	var data []byte
	data, err = s.serializeState()
	if err != nil {
		return
	}

	// Set revision to -1 as a semaphore
	s.lastRevisionPut = -1

	// Store in etcd only if it has changed
	ctx, cancel := s.GetContext()
	txn := s.client.Txn(ctx)
	res, err := txn.If(
		clientv3.Compare(clientv3.Value(s.stateKey), "!=", string(data)),
	).Then(
		clientv3.OpPut(s.stateKey, string(data)),
	).Commit()
	cancel()
	if err != nil {
		s.lastRevisionPut = 0
		return err
	}
	// If it has changed
	if res.Succeeded {
		s.lastRevisionPut = res.Header.GetRevision()
		logger.Println("Stored state in etcd: version", s.lastRevisionPut)
	}
	return
}

// ReadState reads the state from etcd
func (s *StateStoreEtcd) ReadState() (err error) {
	logger.Println("Reading state from etcd")

	// Read the state
	ctx, cancel := s.GetContext()
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
func (s *StateStoreEtcd) Healthy() (healthy bool, err error) {
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
func (s *StateStoreEtcd) OnStateUpdate(callback func()) {
	s.updateCallback = callback
}

// ClusterMembers returns the list of members in the cluster
func (s *StateStoreEtcd) ClusterMembers() (map[string]string, error) {
	// Gets the list of members
	ctx, cancel := s.GetContext()
	resp, err := s.client.Get(ctx, s.nodesKeyPrefix, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return nil, err
	}

	// Check if the secret exists
	var members map[string]string
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		members = make(map[string]string, len(resp.Kvs))
		for _, kv := range resp.Kvs {
			key := strings.TrimPrefix(string(kv.Key), s.nodesKeyPrefix)
			members[key] = string(kv.Value)
		}
	} else {
		return nil, errors.New("Received empty list of cluster members")
	}

	return members, nil
}

// Serialize the state to JSON
// Additionally, store all secrets longer than 64 bytes in a separate etcd key
// Store the DH parameters file in a separate etcd key too
func (s *StateStoreEtcd) serializeState() ([]byte, error) {
	// Create a copy of the state
	serialize := NodeState{
		Sites: s.state.Sites,
	}

	// Check if we have any secret
	if s.state.Secrets != nil && len(s.state.Secrets) > 0 {
		for k, v := range s.state.Secrets {
			// Encode the value to b64
			secretData := base64.StdEncoding.EncodeToString(v)

			// Store the secret if different
			ctx, cancel := s.GetContext()
			txn := s.client.Txn(ctx)
			_, err := txn.If(
				clientv3.Compare(clientv3.Value(s.secretsKeyPrefix+k), "!=", secretData),
			).Then(
				clientv3.OpPut(s.secretsKeyPrefix+k, secretData),
			).Commit()
			cancel()
			if err != nil {
				return nil, err
			}
		}
	}

	// Check if we have DH params
	if s.state.DHParams != nil && s.state.DHParams.PEM != "" && s.state.DHParams.Date != nil {
		// Encode to JSON
		dhparams, err := json.Marshal(s.state.DHParams)
		if err != nil {
			return nil, err
		}

		// Store the value if different
		ctx, cancel := s.GetContext()
		txn := s.client.Txn(ctx)
		_, err = txn.If(
			clientv3.Compare(clientv3.Value(s.dhparamsKey), "!=", string(dhparams)),
		).Then(
			clientv3.OpPut(s.dhparamsKey, string(dhparams)),
		).Commit()
		cancel()
		if err != nil {
			return nil, err
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
// Additionally, retrieve all elements that were stored separately in etcd
func (s *StateStoreEtcd) unserializeState(data []byte) error {
	// First, unserialize the JSON data
	unserialized := &NodeState{}
	if err := json.Unmarshal(data, unserialized); err != nil {
		return err
	}

	// Retrieve the list of secrets
	ctx, cancel := s.GetContext()
	resp, err := s.client.Get(ctx, s.secretsKeyPrefix, clientv3.WithPrefix())
	cancel()
	if err != nil {
		return err
	}
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		unserialized.Secrets = make(map[string][]byte)
		for _, kv := range resp.Kvs {
			if kv.Value != nil && len(kv.Value) > 0 {
				// Decode the value from base64
				secret, err := base64.StdEncoding.DecodeString(string(kv.Value))
				if err != nil {
					return err
				}
				unserialized.Secrets[string(kv.Key)] = secret
			}
		}
	}

	// Retrieve DH parameters, if any
	ctx, cancel = s.GetContext()
	resp, err = s.client.Get(ctx, s.dhparamsKey)
	cancel()
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		err := json.Unmarshal(resp.Kvs[0].Value, unserialized.DHParams)
		if err != nil {
			return err
		}
	}

	// Set the state
	s.state = unserialized

	return nil
}
