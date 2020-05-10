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

package certificates

import (
	"crypto/x509"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/statiko-dev/statiko/azurekeyvault"
	"github.com/statiko-dev/statiko/state"
)

// GetCertificate returns the certificate for the site (with key and certificate PEM-encoded)
func GetCertificate(site *state.SiteState) (key []byte, cert []byte, err error) {
	var certObj *x509.Certificate

	// Check the type of the TLS certificate
	switch site.TLS.Type {
	case state.TLSCertificateImported:
		// Get the certificate
		key, cert, certObj, err = GetImportedCertificate(site)
		if err != nil {
			return
		}

		// Inspect the certificate, but consider errors as warnings only
		if insp := InspectCertificate(site, certObj); insp != nil {
			logger.Printf("[Warn] %v\n", insp)
		}
		return
	case state.TLSCertificateSelfSigned:
		key, cert, err = GetSelfSignedCertificate(site)
		return
	case state.TLSCertificateACME:
		key, cert, err = GetACMECertificate(site)
		return
	default:
		err = errors.New("invalid TLS certificate type")
		return
	}

	return
}

// GetImportedCertificate returns a certificate from Azure Key Vault
func GetImportedCertificate(site *state.SiteState) (key []byte, cert []byte, certObj *x509.Certificate, err error) {
	var name, version string
	if site.TLS.Certificate == nil || *site.TLS.Certificate == "" {
		err = errors.New("certificate name is empty")
		return
	}
	name = *site.TLS.Certificate
	if site.TLS.Version != nil && *site.TLS.Version != "" {
		version = *site.TLS.Version
	}
	version, cert, key, certObj, err = azurekeyvault.GetInstance().GetCertificate(name, version)
	if err != nil {
		return
	}

	logger.Printf("Retrieved TLS certificate from AKV: %s (%s)\n", name, version)

	return
}

// InspectCertificate loads a X.509 certificate and checks its details, such as expiration
func InspectCertificate(site *state.SiteState, cert *x509.Certificate) error {
	now := time.Now()

	// Check "NotAfter" (require at least 12 hours)
	if cert.NotAfter.Before(now.Add(12 * time.Hour)) {
		return fmt.Errorf("certificate has expired or has less than 12 hours of validity: %v", cert.NotAfter)
	}

	// Check "NotBefore"
	if !cert.NotBefore.Before(now) {
		return fmt.Errorf("certificate's NotBefore is in the future: %v", cert.NotBefore)
	}

	// Check if the list of domains matches, but not for imported certificates
	// We're not checking this for imported certificates because they might have wildcards and be valid for more domains
	if site.TLS.Type != state.TLSCertificateImported {
		domains := append([]string{site.Domain}, site.Aliases...)
		sort.Strings(domains)
		certDomains := append(make([]string, 0), cert.DNSNames...)
		sort.Strings(certDomains)
		if !reflect.DeepEqual(domains, certDomains) {
			return fmt.Errorf("list of domains in certificate does not match: %v", certDomains)
		}
	}

	return nil
}
