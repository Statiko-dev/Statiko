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

package proto

import (
	"crypto/x509"
	"sort"

	"github.com/statiko-dev/statiko/utils"
)

// This file contains additional methods added to the protobuf object

// StateMessage returns the StateMessage object from a given StateStore
func (x *StateStore) StateMessage() *StateMessage {
	return &StateMessage{
		Version:  x.Version,
		Sites:    x.Sites,
		DhParams: x.DhParams,
	}
}

// GetSite searches the list of sites to return the one matching the requested domain (including aliases)
func (x *StateStore) GetSite(domain string) *Site {
	sites := x.GetSites()
	if sites == nil {
		return nil
	}

	for _, s := range sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			return s
		}
	}

	return nil
}

// GetSite searches the list of sites to return the one matching the requested domain (including aliases)
func (x *StateMessage) GetSite(domain string) *Site {
	sites := x.GetSites()
	if sites == nil {
		return nil
	}

	for _, s := range sites {
		if s.Domain == domain || (len(s.Aliases) > 0 && utils.StringInSlice(s.Aliases, domain)) {
			return s
		}
	}

	return nil
}

// GetTLSCertificate returns a certificate by its ID
func (x *StateStore) GetTLSCertificate(id string) *TLSCertificate {
	if x.Certificates == nil {
		x.Certificates = make(map[string]*TLSCertificate)
	}

	cert, found := x.Certificates[id]
	if !found {
		cert = nil
	}
	return cert
}

// Validate a TLSCertificate object; this can modify the object
func (x *TLSCertificate) Validate() bool {
	switch x.Type {
	case TLSCertificate_IMPORTED, TLSCertificate_SELF_SIGNED, TLSCertificate_ACME:
		// Must have the certificate data only
		if len(x.Key) > 0 && len(x.Certificate) > 0 {
			x.Name = ""
			return true
		}
	case TLSCertificate_AZURE_KEY_VAULT:
		// Must have certificate name only
		if x.Name != "" {
			x.Key = nil
			x.Certificate = nil
			return true
		}
	}

	// If we're here, the validation failled
	return false
}

// SetCertificateProperties sets the properties of the certificate in the object
func (x *TLSCertificate) SetCertificateProperties(certX509 *x509.Certificate) {
	if x == nil {
		return
	}

	// List of domains
	// Copy the object before sorting it
	x.XDomains = append([]string{}, certX509.DNSNames...)
	sort.Strings(x.XDomains)

	// Not before and expiry (as UNIX timestamp)
	if !certX509.NotBefore.IsZero() {
		x.XNbf = certX509.NotBefore.Unix()
	}
	if !certX509.NotAfter.IsZero() {
		x.XExp = certX509.NotAfter.Unix()
	}
}
