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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/statiko-dev/statiko/appconfig"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
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
	nodeHealth         *pb.NodeHealth
	logger             *log.Logger
}

// Init loads the state from the store
func (m *Manager) Init() (err error) {
	// Initialize the logger
	m.logger = log.New(os.Stdout, "state: ", log.Ldate|log.Ltime|log.LUTC)

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

// setUpdated sets the updated time in the object
func (m *Manager) setUpdated() {
	now := time.Now()
	m.updated = &now
}

// LastUpdated returns the time the state was updated last
func (m *Manager) LastUpdated() *time.Time {
	return m.updated
}

// GetSites returns the list of all sites
func (m *Manager) GetSites() []*pb.State_Site {
	state := m.store.GetState()
	if state != nil {
		return state.Sites
	}

	return nil
}

// GetSite returns the site object for a specific domain (including aliases)
func (m *Manager) GetSite(domain string) *pb.State_Site {
	state := m.store.GetState()
	if state != nil {
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

// OnStateUpdate stores the callback to invoke if the state is updated because of an external event
func (m *Manager) OnStateUpdate(callback func()) {
	m.store.OnStateUpdate(callback)
}

// GetDHParams returns the PEM-encoded DH parameters and their date
func (m *Manager) GetDHParams() (string, *time.Time) {
	// Check if we DH parameters; if not, return the default ones
	state := m.store.GetState()
	if state != nil && state.DhParams == nil || state.DhParams.Date == 0 || state.DhParams.Pem == "" {
		return defaultDHParams, nil
	}

	// Return the saved DH parameters
	date := time.Unix(state.DhParams.Date, 0)
	return state.DhParams.Pem, &date
}

// SetDHParams stores new PEM-encoded DH parameters
func (m *Manager) SetDHParams(val string) error {
	if val == "" || val == defaultDHParams {
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

	// Store the value
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Secrets == nil {
		state.Secrets = make(map[string][]byte)
	}
	state.Secrets[key] = append(nonce, encValue...)

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
	secretKey := m.certificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return nil, nil, errors.New("invalid name or domains")
	}

	// Retrieve the secret
	serialized, err := m.GetSecret(secretKey)
	if err != nil || serialized == nil || len(serialized) < 8 {
		return nil, nil, err
	}

	// Un-serialize the secret
	keyLen := binary.LittleEndian.Uint32(serialized[0:4])
	certLen := binary.LittleEndian.Uint32(serialized[4:8])
	if keyLen < 1 || certLen < 1 || len(serialized) != int(8+keyLen+certLen) {
		return nil, nil, errors.New("invalid serialized data")
	}

	key = serialized[8:(keyLen + 8)]
	cert = serialized[(keyLen + 8):]
	err = nil
	return
}

// SetCertificate stores a PEM-encoded certificate pair (key and certificate) as a secret
func (m *Manager) SetCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string, key []byte, cert []byte) (err error) {
	// Key of the secret
	secretKey := m.certificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return errors.New("invalid name or domains")
	}

	// Serialize the certificates
	if len(key) > 204800 || len(cert) > 204800 {
		return errors.New("key and/or certificate are too long")
	}
	keyLen := make([]byte, 4)
	certLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLen, uint32(len(key)))
	binary.LittleEndian.PutUint32(certLen, uint32(len(cert)))
	serialized := bytes.Buffer{}
	serialized.Write(keyLen)
	serialized.Write(certLen)
	serialized.Write(key)
	serialized.Write(cert)

	// Store the secret
	err = m.SetSecret(secretKey, serialized.Bytes())
	if err != nil {
		return err
	}

	m.logger.Printf("Stored %s certificate for %v with key %s\n", typ, nameOrDomains, secretKey)
	return nil
}

// RemoveCertificate removes a certificate from the store
func (m *Manager) RemoveCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string) (err error) {
	// Key of the secret
	secretKey := m.certificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return errors.New("invalid name or domains")
	}

	return m.DeleteSecret(secretKey)
}

// ListImportedCertificates returns a list of the names of all imported certificates
func (m *Manager) ListImportedCertificates() (res []string) {
	res = make([]string, 0)
	// Iterate through all secrets looking for those starting with "cert/imported/"
	state := m.store.GetState()
	for k := range state.Secrets {
		if strings.HasPrefix(k, "cert/imported/") {
			res = append(res, strings.TrimPrefix(k, "cert/imported/"))
		}
	}
	return
}

// certificateSecretKey returns the key of secret for the certificate
func (m *Manager) certificateSecretKey(typ pb.State_Site_TLS_Type, nameOrDomains []string) string {
	switch typ {
	case pb.State_Site_TLS_IMPORTED:
		if len(nameOrDomains) != 1 || len(nameOrDomains[0]) == 0 {
			return ""
		}
		return "cert/" + pb.State_Site_TLS_IMPORTED.String() + "/" + nameOrDomains[0]
	case pb.State_Site_TLS_ACME, pb.State_Site_TLS_SELF_SIGNED:
		domainKey := utils.SHA256String(strings.Join(nameOrDomains, ","))[:15]
		return "cert/" + typ.String() + "/" + domainKey
	default:
		return ""
	}
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

// GetNodeHealth gets the node status object
func (m *Manager) GetNodeHealth() *pb.NodeHealth {
	return m.nodeHealth
}
