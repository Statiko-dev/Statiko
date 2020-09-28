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
	"errors"
	"log"
	"os"
	"time"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/shared/defaults"
	pb "github.com/statiko-dev/statiko/shared/proto"
	common "github.com/statiko-dev/statiko/shared/state"
	"github.com/statiko-dev/statiko/utils"
)

const (
	StoreTypeFile = "file"
	StoreTypeEtcd = "etcd"
)

// Manager is the state manager class
type Manager struct {
	common.StateCommon

	DHParamsGenerating bool
	updated            *time.Time
	store              StateStore
	storeType          string
	logger             *log.Logger
	signaler           *utils.Signaler
}

// Init loads the state from the store
func (m *Manager) Init() (err error) {
	// Initialize the logger
	m.logger = log.New(os.Stdout, "state: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the signaler
	m.signaler = &utils.Signaler{}

	// Get store type
	typ := appconfig.Config.GetString("state.store")
	switch typ {
	case "file":
		m.store = &StateStoreFile{}
		m.storeType = StoreTypeFile
	case "etcd":
		m.store = &StateStoreEtcd{}
		m.storeType = StoreTypeEtcd
	default:
		err = errors.New("invalid value for configuration `state.store`; valid options are `file` or `etcd`")
		return
	}
	err = m.store.Init()
	if err != nil {
		return err
	}
	m.store.OnReceive(func() {
		m.setUpdated()
	})

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
	return m.store.AcquireLock(name, timeout)
}

// ReleaseSyncLock releases the lock on the sync semaphore
func (m *Manager) ReleaseLock(leaseID interface{}) error {
	return m.store.ReleaseLock(leaseID)
}

// DumpState exports the entire state
func (m *Manager) DumpState() (*pb.State, error) {
	return m.store.GetState(), nil
}

// ReplaceState replaces the full state for the node with the provided one
func (m *Manager) ReplaceState(state *pb.State) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Ensure that if TLS certs are not imported or from Azure Key Vault, their name and version isn't included
	for _, s := range state.Sites {
		if s.Tls != nil && s.Tls.Type != pb.State_Site_TLS_IMPORTED && s.Tls.Type != pb.State_Site_TLS_AZURE_KEY_VAULT {
			s.Tls.Certificate = ""
			s.Tls.Version = ""
		}
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

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

// setUpdated sets the updated time in the object and broadcasts the message
func (m *Manager) setUpdated() {
	now := time.Now()
	m.updated = &now

	// Broadcast the update
	m.signaler.Broadcast()
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

// GetSites returns the list of all sites
func (m *Manager) GetSites() []*pb.State_Site {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	return state.Sites
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *pb.State_Site {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	return state.GetSite(domain)
}

// AddSite adds a site to the store
func (m *Manager) AddSite(site *pb.State_Site) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Add the site
	state := m.store.GetState()
	state.Sites = append(state.Sites, site)
	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// UpdateSite updates a site with the same Domain
func (m *Manager) UpdateSite(site *pb.State_Site, setUpdated bool) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Replace in the memory state
	found := false
	state := m.store.GetState()
	for i, s := range state.Sites {
		if s.Domain == site.Domain {
			// Replace the element
			state.Sites[i] = site

			found = true
			break
		}
	}

	if !found {
		return errors.New("site not found")
	}

	// Check if we need to set the object as updated
	if setUpdated {
		m.setUpdated()
	}

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// DeleteSite remvoes a site from the store
func (m *Manager) DeleteSite(domain string) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Update the state
	found := false
	state := m.store.GetState()
	for i, s := range state.Sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			// Remove the element
			state.Sites[i] = state.Sites[len(state.Sites)-1]
			state.Sites = state.Sites[:len(state.Sites)-1]

			found = true
			break
		}
	}

	if !found {
		return errors.New("site not found")
	}

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// StoreHealth returns true if the store is healthy
func (m *Manager) StoreHealth() (healthy bool, err error) {
	return m.store.Healthy()
}

// GetDHParams returns the PEM-encoded DH parameters and their date
func (m *Manager) GetDHParams() (string, *time.Time) {
	// Check if we DH parameters; if not, return the default ones
	state := m.store.GetState()
	if state != nil && (state.DhParams == nil || state.DhParams.Date == 0 || state.DhParams.Pem == "") {
		return defaults.DefaultDHParams, nil
	}

	// Return the saved DH parameters
	date := time.Unix(state.DhParams.Date, 0)
	return state.DhParams.Pem, &date
}

// SetDHParams stores new PEM-encoded DH parameters
func (m *Manager) SetDHParams(val string) error {
	if val == "" || val == defaults.DefaultDHParams {
		return errors.New("val is empty or invalid")
	}

	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Store the value
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	now := time.Now()
	state.DhParams = &pb.State_DHParams{
		Pem:  val,
		Date: now.Unix(),
	}

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// GetSecret returns the value for a secret (encrypted in the state)
func (m *Manager) GetSecret(key string) ([]byte, error) {
	// Check if we have a secret for this key
	state := m.store.GetState()
	if state == nil {
		return nil, errors.New("state not loaded")
	}
	if state.Secrets == nil {
		state.Secrets = make(map[string][]byte)
	}
	encValue, found := state.Secrets[key]
	if !found || encValue == nil || len(encValue) < 12 {
		return nil, nil
	}

	// Decrypt the secret
	return m.DecryptSecret(encValue)
}

// SetSecret sets the value for a secret (encrypted in the state)
func (m *Manager) SetSecret(key string, value []byte) error {
	// Encrypt the secret
	encValue, err := m.EncryptSecret(value)
	if err != nil {
		return err
	}

	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Store the value
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Secrets == nil {
		state.Secrets = make(map[string][]byte)
	}
	state.Secrets[key] = encValue

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// DeleteSecret deletes a secret
func (m *Manager) DeleteSecret(key string) error {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Delete the key
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Secrets != nil {
		delete(state.Secrets, key)
	}

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// GetCertificate returns a certificate pair (key and certificate) stored as secrets, PEM-encoded
func (m *Manager) GetCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string) (key []byte, cert []byte, err error) {
	// Key of the secret
	secretKey := m.CertificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return nil, nil, errors.New("invalid name or domains")
	}

	// Retrieve the secret
	serialized, err := m.GetSecret(secretKey)
	if err != nil || serialized == nil || len(serialized) < 8 {
		return nil, nil, err
	}

	// Un-serialize the secret
	return m.UnserializeCertificate(serialized)
}

// SetCertificate stores a PEM-encoded certificate pair (key and certificate) as a secret
func (m *Manager) SetCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string, key []byte, cert []byte) (err error) {
	// Key of the secret
	secretKey := m.CertificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return errors.New("invalid name or domains")
	}

	// Serialize the certificates
	serialized, err := m.SerializeCertificate(key, cert)
	if err != nil {
		return err
	}

	// Store the secret
	err = m.SetSecret(secretKey, serialized)
	if err != nil {
		return err
	}

	m.logger.Printf("Stored %s certificate for %v with key %s\n", typ, nameOrDomains, secretKey)
	return nil
}

// RemoveCertificate removes a certificate from the store
func (m *Manager) RemoveCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string) (err error) {
	// Key of the secret
	secretKey := m.CertificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return errors.New("invalid name or domains")
	}

	return m.DeleteSecret(secretKey)
}

// ListImportedCertificates returns a list of the names of all imported certificates
func (m *Manager) ListImportedCertificates() (res []string) {
	state := m.store.GetState()
	if state == nil {
		return []string{}
	}
	return m.ListImportedCertificates_Internal(state.Secrets)
}
