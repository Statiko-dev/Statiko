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

import "github.com/statiko-dev/statiko/utils"

// This file contains additional methods added to the protobuf object

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

// GetTlsCertificate returns a certificate by its ID
func (x *StateStore) GetTlsCertificate(id string) *TLSCertificate {
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
	case TLSCertificate_IMPORTED:
		// Must have the certificate data and name only
		if x.Data != nil && len(x.Data) > 0 && x.Name != "" {
			x.Version = ""
			return true
		}
	case TLSCertificate_SELF_SIGNED:
		// Must have the certificate data only
		if x.Data != nil && len(x.Data) > 0 {
			x.Name = ""
			x.Version = ""
			return true
		}
	case TLSCertificate_ACME:
		// Must have the certificate data only
		if x.Data != nil && len(x.Data) > 0 {
			x.Name = ""
			x.Version = ""
			return true
		}
	case TLSCertificate_AZURE_KEY_VAULT:
		// Must have certificate name, and optionally version
		if x.Name != "" {
			x.Data = nil
			return true
		}
	}

	// If we're here, the validation failled
	return false
}
