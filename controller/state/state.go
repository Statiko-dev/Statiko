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
	"crypto/rsa"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

const (
	StoreTypeFile = "file"
	//StoreTypeEtcd = "etcd"
)

// Manager is the state manager class
type Manager struct {
	DHParamsGenerating bool
	CertRefresh        certRefreshCb

	updated     *time.Time
	store       StateStore
	storeType   string
	logger      *log.Logger
	signaler    *utils.Signaler
	semaphore   *sync.Mutex
	codesignKey *rsa.PublicKey
}

// Init loads the state from the store
func (m *Manager) Init() (err error) {
	// Initialize the logger
	m.logger = log.New(buildinfo.LogDestination, "state: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the signaler
	m.signaler = &utils.Signaler{}

	// Get store type
	typ := viper.GetString("state.store")
	switch typ {
	case "file":
		m.store = &StateStoreFile{}
		m.storeType = StoreTypeFile
	/*case "etcd":
	m.store = &StateStoreEtcd{}
	m.storeType = StoreTypeEtcd*/
	default:
		//err = errors.New("invalid value for configuration `state.store`; valid options are `file` or `etcd`")
		err = errors.New("invalid value for configuration `state.store`; valid options are `file`")
		return
	}
	err = m.store.Init()
	if err != nil {
		return err
	}
	m.store.OnReceive(func() {
		m.setUpdated()
	})

	// Check if there's a version
	state := m.store.GetState()
	if state.Version < 1 {
		state.Version = 1
	}

	return
}

// GetStoreType returns the type of the store in use
func (m *Manager) GetStoreType() string {
	return m.storeType
}

// GetStore returns the instance of the store in use
func (m *Manager) GetStore() StateStore {
	return m.store
}

// AcquireLock acquires a lock on the sync semaphore, ensuring that only one node at a time can be syncing
func (m *Manager) AcquireLock(name string, timeout bool) (interface{}, error) {
	m.semaphore.Lock()
	return m.store.AcquireLock(name, timeout)
}

// ReleaseSyncLock releases the lock on the sync semaphore
func (m *Manager) ReleaseLock(leaseID interface{}) error {
	m.semaphore.Unlock()
	return m.store.ReleaseLock(leaseID)
}

// DumpState exports the entire state
func (m *Manager) DumpState() (*pb.StateStore, error) {
	return m.store.GetState(), nil
}

// ReplaceState replaces the full state for the node with the provided one
func (m *Manager) ReplaceState(state *pb.StateStore) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Validate TLS certs
	certs := make(map[string]*pb.TLSCertificate)
	for k, e := range state.Certificates {
		// Validate the certificate; note that the Validate method might modify the object
		if e == nil || !e.Validate() {
			continue
		}
		certs[k] = e
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Set version based on the current version
	currentState := m.store.GetState()
	state.Version = currentState.Version + 1

	// Replace the state
	if err := m.store.SetState(state); err != nil {
		return err
	}
	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// setUpdated sets the updated time in the object and sends the signal with the state update
func (m *Manager) setUpdated() {
	// Update the time in the object
	now := time.Now()
	m.updated = &now

	// Broadcast the signal
	m.logger.Println("Broadcasting new state message")
	_ = m.signaler.Broadcast()
}

// LastUpdated returns the time the state was updated last
func (m *Manager) LastUpdated() *time.Time {
	return m.updated
}

// Subscribe will add a channel as a subscriber to when new state is availalble
func (m *Manager) Subscribe(ch chan int) {
	m.signaler.Subscribe(ch)
}

// Unsubscribe removes a channel from the list of subscribers to the state
func (m *Manager) Unsubscribe(ch chan int) {
	m.signaler.Unsubscribe(ch)
}

// GetVersion returns the version of the state
func (m *Manager) GetVersion() uint64 {
	state := m.store.GetState()
	if state == nil {
		return 0
	}

	return state.Version
}

// StoreHealth returns true if the store is healthy
func (m *Manager) StoreHealth() (healthy bool, err error) {
	return m.store.Healthy()
}
