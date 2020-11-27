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

	pb "github.com/statiko-dev/statiko/shared/proto"
)

type certRefreshCb func()

var ErrCertificateNotFound = errors.New("TLS certificate not found")

// GetCertificate returns a certificate object; for certs with data in the state, this returns the key and certificate too, decrypted and PEM-encoded
func (m *Manager) GetCertificate(certId string) (key []byte, cert []byte, err error) {
	// Get the state
	state := m.store.GetState()
	if state == nil {
		return nil, nil, errors.New("state not loaded")
	}

	// Retrieve the certificate
	obj := state.GetTLSCertificate(certId)
	if obj == nil {
		return nil, nil, ErrCertificateNotFound
	}

	// Check if we have the certificate data in the object; if so, decrypt it
	key, cert, err = m.decryptCertificate(obj)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}

// GetCertificateInfo returns the info (metadata) of a TLS certificate, but does not decrypt the content
func (m *Manager) GetCertificateInfo(certId string) (obj *pb.TLSCertificate, err error) {
	// Get the state
	state := m.store.GetState()
	if state == nil {
		return nil, errors.New("state not loaded")
	}

	// Retrieve the certificate
	obj = state.GetTLSCertificate(certId)
	if obj == nil {
		return nil, errors.New("TLS certificate not found")
	}

	return obj, nil
}

// SetCertificate sets a certificate in the state
// It requires an ID for the certificate, pre-generated; if it's already set, it will be replaced
// Additionally, if a key and certificate are passed, they will be encrypted
func (m *Manager) SetCertificate(obj *pb.TLSCertificate, certId string, key []byte, cert []byte) (err error) {
	// Check if the store is healthy
	// Note: this won't guarantee that the store will be healthy when we try to write in it
	healthy, err := m.StoreHealth()
	if !healthy {
		return err
	}

	// Ensure that we have a certificate object and ID
	if obj == nil {
		return errors.New("certificate object is empty")
	}
	if certId == "" {
		return errors.New("certificate ID is empty")
	}

	// If we have a certificate, encrypt the key then store both
	if len(key) > 0 && len(cert) > 0 {
		// Encrypt the key
		obj.Key, err = encryptData(key)
		if err != nil {
			return err
		}

		// Store the certificate
		obj.Certificate = cert
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

	// Update the state
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Certificates == nil {
		state.Certificates = make(map[string]*pb.TLSCertificate)
	}
	state.Certificates[certId] = obj

	// Do not increase the version or set the updated flag because no site has been modified at this point
	//state.Version++
	//m.setUpdated()

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
		if s.GeneratedTlsId == certId || s.ImportedTlsId == certId {
			return errors.New("certificate is in use by a site and cannot be deleted")
		}
	}

	// Delete the certificate
	if state.Certificates != nil {
		delete(state.Certificates, certId)
	}

	// Do not increase the version or set the updated flag because no site has been modified at this point
	//state.Version++
	//m.setUpdated()

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// ListCertificates returns a list of all certificates
func (m *Manager) ListCertificates() map[string]*pb.TLSCertificate {
	state := m.store.GetState()
	if state == nil {
		return nil
	}

	return state.Certificates
}

// ReplaceCertificate updates all sites that are using an old self-signed (or ACME) certificate to use a new one
func (m *Manager) ReplaceCertificate(oldCertId, newCertId string) error {
	if oldCertId == "" || newCertId == "" {
		return errors.New("parameters `oldCertId` and `newCertId` must not be empty")
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

	// Get the state
	state := m.store.GetState()
	if state == nil {
		return errors.New("state not loaded")
	}
	if state.Certificates == nil {
		return errors.New("old certificate not found or it's an imported one")
	}

	// Ensure that both the old and new certificates exist and are not imported
	oldCert, ok := state.Certificates[oldCertId]
	if !ok || oldCert == nil || oldCert.Type == pb.TLSCertificate_IMPORTED {
		return errors.New("old certificate not found or it's an imported one")
	}
	newCert, ok := state.Certificates[newCertId]
	if !ok || newCert == nil || newCert.Type == pb.TLSCertificate_IMPORTED {
		return errors.New("new certificate not found or it's an imported one")
	}

	// Iterate through the sites that are using the old certificate
	updated := false
	for _, s := range state.Sites {
		if s.GeneratedTlsId == oldCertId {
			s.GeneratedTlsId = newCertId
			updated = true
		}
	}

	// Delete the old certificate
	delete(state.Certificates, oldCertId)

	// If we updated a site, update the version send the "setUpdated" notification
	// Only if we updated a site, however, to avoid unnecessary re-syncs
	if updated {
		state.Version++
		m.setUpdated()
	}

	// Commit the state to the store
	if err := m.store.WriteState(); err != nil {
		return err
	}

	return nil
}

// Decrypts the certificate data from the object
func (m *Manager) decryptCertificate(obj *pb.TLSCertificate) (key []byte, cert []byte, err error) {
	// If there's no data, just return
	if len(obj.Key) == 0 || len(obj.Certificate) == 0 {
		return nil, nil, nil
	}

	// Decrypt the key
	key, err = decryptData(obj.Key)
	if err != nil {
		return nil, nil, err
	}
	if len(key) == 0 {
		return nil, nil, errors.New("invalid decrypted data")
	}

	// Certificate
	cert = obj.Certificate

	return key, cert, nil
}

// TriggerCertRefresh triggers the worker to refresh the certificates
func (m *Manager) TriggerCertRefresh() {
	if m.CertRefresh != nil {
		go m.CertRefresh()
	}
}
