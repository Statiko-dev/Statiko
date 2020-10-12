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
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/shared/defaults"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"

	"github.com/google/uuid"
)

const (
	StoreTypeFile = "file"
	StoreTypeEtcd = "etcd"
)

// Manager is the state manager class
type Manager struct {
	DHParamsGenerating bool
	updated            *time.Time
	store              StateStore
	storeType          string
	logger             *log.Logger
	signaler           *utils.Signaler
	semaphore          *sync.Mutex
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

// setUpdated sets the updated time in the object
func (m *Manager) setUpdated() {
	now := time.Now()
	m.updated = &now
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
func (m *Manager) GetSites() []*pb.Site {
	state := m.store.GetState()
	if state != nil {
		return state.Sites
	}

	return nil
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *pb.Site {
	state := m.store.GetState()
	if state != nil {
		return nil
	}

	return state.GetSite(domain)
}

// AddSite adds a site to the store
func (m *Manager) AddSite(site *pb.Site) error {
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

	// Get the current state
	state := m.store.GetState()

	// Ensure that the TLS certificate referenced exists
	if site.Tls != "" {
		if _, ok := state.Certificates[site.Tls]; !ok {
			return errors.New("the site references a TLS certificate that doesn't exist")
		}
	}

	// Add the site and increase the version
	state.Sites = append(state.Sites, site)
	state.Version++
	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// UpdateSite updates a site with the same Domain
func (m *Manager) UpdateSite(site *pb.Site, setUpdated bool) error {
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

	// Get the current state
	state := m.store.GetState()

	// Ensure that the TLS certificate referenced exists
	if site.Tls != "" {
		if _, ok := state.Certificates[site.Tls]; !ok {
			return errors.New("the site references a TLS certificate that doesn't exist")
		}
	}

	// Replace in the memory state
	found := false
	for i, s := range state.Sites {
		if s.Domain == site.Domain {
			// Replace the element
			state.Sites[i] = site

			// Increase the version since we made a change
			state.Version++

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
			// If the TLS certificate is self-signed or from ACME, remove it too
			if s.Tls != "" {
				cert := state.Certificates[s.Tls]
				if cert != nil && (cert.Type == pb.TLSCertificate_SELF_SIGNED || cert.Type == pb.TLSCertificate_ACME) {
					delete(state.Certificates, s.Tls)
				}
			}

			// Remove the element
			state.Sites[i] = state.Sites[len(state.Sites)-1]
			state.Sites = state.Sites[:len(state.Sites)-1]

			// Increase the version since we made a change
			state.Version++

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

	// Store the value and increase the version
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	now := time.Now()
	state.DhParams = &pb.DHParams{
		Pem:  val,
		Date: now.Unix(),
	}
	state.Version++

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

	// Get the cipher
	aesgcm, err := m.getSecretsCipher()
	if err != nil {
		return nil, err
	}

	// Decrypt the secret
	// First 12 bytes of the value are the nonce
	value, err := aesgcm.Open(nil, encValue[0:12], encValue[12:], nil)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// SetSecret sets the value for a secret (encrypted in the state)
func (m *Manager) SetSecret(key string, value []byte) error {
	// Get the cipher
	aesgcm, err := m.getSecretsCipher()
	if err != nil {
		return err
	}

	// Get a nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// Encrypt the secret
	encValue := aesgcm.Seal(nil, nonce, value, nil)

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

	// Store the value and increase the version
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Secrets == nil {
		state.Secrets = make(map[string][]byte)
	}
	state.Secrets[key] = append(nonce, encValue...)
	state.Version++

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

	// Delete the key and increase the version
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Secrets != nil {
		state.Version++
		delete(state.Secrets, key)
	}

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// GetCertificate returns a certificate object; for certs with data in the state, this returns the key and certificate too, decrypted and PEM-encoded
func (m *Manager) GetCertificate(domain string) (obj *pb.TLSCertificate, key []byte, cert []byte, err error) {
	// Get the state
	state := m.store.GetState()
	if state == nil {
		return nil, nil, nil, errors.New("state not loaded")
	}

	// Get the site and ensure it has a TLS certificate
	site := state.GetSite(domain)
	if site == nil {
		return nil, nil, nil, errors.New("site not found")
	}
	if site.Tls == "" {
		return nil, nil, nil, errors.New("site does not have a TLS certificate")
	}

	// Retrieve the certificate
	obj = state.GetTlsCertificate(site.Tls)
	if obj == nil {
		return nil, nil, nil, errors.New("TLS certificate not found")
	}

	// Check if we have the certificate data in the object; if so, decrypt it
	key, cert, err = m.decryptCertificate(obj)
	if err != nil {
		return nil, nil, nil, err
	}

	return obj, key, cert, nil
}

// SetCertificate sets a certificate in the state
// If the ID parameter is set, it will replace existing certificates with the same ID; otherwise, a new ID will be generated
// Additionally, if a key and certificate are passed, they will be encrypted
func (m *Manager) SetCertificate(obj *pb.TLSCertificate, certId string, key []byte, cert []byte) (err error) {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Ensure that we have a certificate object
	if obj == nil {
		return errors.New("certificate object is empty")
	}

	// If we don't have a certificate ID, generate a new one
	if certId == "" {
		u, err := uuid.NewRandom()
		if err != nil {
			return err
		}
		certId = u.String()
	}

	// If we have a key and certificate, enncrypt them
	if key != nil && len(key) > 0 && cert != nil && len(cert) > 0 {
		// Serialize the certificates
		if len(key) > 204800 || len(cert) > 204800 {
			return errors.New("key and/or certificate are too long")
		}
		keyLen := make([]byte, 4)
		certLen := make([]byte, 4)
		binary.LittleEndian.PutUint32(keyLen, uint32(len(key)))
		binary.LittleEndian.PutUint32(certLen, uint32(len(cert)))
		// Add 8 because of the 2 lengths that are 32-bit ints
		serialized := make([]byte, 8+len(key)+len(cert))
		copy(serialized[0:4], keyLen)
		copy(serialized[4:8], certLen)
		copy(serialized[8:(8+len(key))], key)
		copy(serialized[(8+len(key)):(8+len(key)+len(cert))], cert)

		// Get the cipher
		aesgcm, err := m.getSecretsCipher()
		if err != nil {
			return err
		}

		// Get a nonce
		nonce := make([]byte, 12)
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return err
		}

		// Encrypt the secret
		obj.Data = aesgcm.Seal(nil, nonce, serialized, nil)
	}

	// Now, validate the object (we first had to generate the data if needed)
	if !obj.Validate() {
		return errors.New("invalid TLS certificate object passed")
	}

	// Lock
	leaseID, err := m.store.AcquireLock("state", true)
	if err != nil {
		return err
	}
	defer m.store.ReleaseLock(leaseID)

	// Update the state and increase the version
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Certificates == nil {
		state.Certificates = make(map[string]*pb.TLSCertificate)
	}
	state.Certificates[certId] = obj
	state.Version++

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	m.logger.Printf("Stored certificate %s\n", certId)
	return nil
}

// DeleteCertificate deletes a certificate object
func (m *Manager) DeleteCertificate(certId string) (err error) {
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

	// Get the state and check if the certificate is in use by any site
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	for _, s := range state.Sites {
		if s.Tls == certId {
			return errors.New("certificate is in use by a site and cannot be deleted")
		}
	}

	// Delete the certificate and increase the version
	if state.Certificates != nil {
		delete(state.Certificates, certId)
		state.Version++
	}

	m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// ListCertificates returns a list of all certificates' IDs
func (m *Manager) ListCertificates() []string {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	// Get the keys
	res := make([]string, len(state.Certificates))
	i := 0
	for k, _ := range state.Certificates {
		res[i] = k
		i++
	}

	return res
}

// Returns a cipher for AES-GCM-128 initialized
func (m *Manager) getSecretsCipher() (cipher.AEAD, error) {
	// Get the symmetric encryption key
	encKey, err := m.getSecretsEncryptionKey()
	if err != nil {
		return nil, err
	}

	// Init the AES-GCM cipher
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aesgcm, nil
}

// Returns the value of the secrets symmetric encryption key from the configuration file
func (m *Manager) getSecretsEncryptionKey() ([]byte, error) {
	// Get the key
	encKeyB64 := appconfig.Config.GetString("secretsEncryptionKey")
	if len(encKeyB64) != 24 {
		return nil, errors.New("empty or invalid 'secretsEncryptionKey' value in configuration file")
	}

	// Decode base64
	encKey, err := base64.StdEncoding.DecodeString(encKeyB64)
	if err != nil {
		return nil, err
	}
	if len(encKey) != 16 {
		return nil, errors.New("invalid length of 'secretsEncryptionKey'")
	}

	return encKey, nil
}

// Decrypts the certificate data from the object
func (m *Manager) decryptCertificate(obj *pb.TLSCertificate) (key []byte, cert []byte, err error) {
	// If there's no data, just return
	if obj.Data == nil || len(obj.Data) < 13 {
		return nil, nil, nil
	}

	// Get the cipher
	aesgcm, err := m.getSecretsCipher()
	if err != nil {
		return nil, nil, err
	}

	// Decrypt the secret
	// First 12 bytes of the value are the nonce
	serialized, err := aesgcm.Open(nil, obj.Data[0:12], obj.Data[12:], nil)
	if err != nil {
		return nil, nil, err
	}
	if serialized == nil || len(serialized) < 13 {
		return nil, nil, errors.New("invalid decrypted data")
	}

	// Un-serialize the secret
	keyLen := binary.LittleEndian.Uint32(serialized[0:4])
	certLen := binary.LittleEndian.Uint32(serialized[4:8])
	if keyLen < 1 || certLen < 1 || len(serialized) != int(8+keyLen+certLen) {
		return nil, nil, errors.New("invalid serialized data")
	}

	key = serialized[8:(keyLen + 8)]
	cert = serialized[(keyLen + 8):]

	return key, cert, nil
}
