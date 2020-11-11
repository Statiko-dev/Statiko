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
	"encoding/pem"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/statiko-dev/statiko/shared/certutils"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Interface of a class that provides methods to work with secrets, such as state.Manager
type secretProvider interface {
	// GetSecret returns a secret's data
	GetSecret(key string) ([]byte, error)
	// SetSecret stores a secret
	SetSecret(key string, value []byte, updates bool) error
	// DeleteSecret deletes a secret
	DeleteSecret(key string, updates bool) error
}

// Interface of a class that implements secretProvider and certutils.stateStoreCert
type stateObj interface {
	secretProvider
	certutils.StateStoreCert
}

// GetX509 returns a X509 object for a PEM-encoded certificate
func GetX509(cert []byte) (certX509 *x509.Certificate, err error) {
	// Get the certificate's x509 object
	block, _ := pem.Decode(cert)
	if block == nil {
		err = errors.New("invalid certificate PEM block")
		return nil, err
	}
	certX509, err = x509.ParseCertificate(block.Bytes)
	return
}

// DomainList returns the list of domains in a certificate
func DomainList(certX509 *x509.Certificate) []string {
	if certX509 == nil || len(certX509.DNSNames) == 0 {
		return nil
	}

	// Clone the object
	return append(make([]string, 0), certX509.DNSNames...)
}

// IsSelfSigned returns true if the certificate is self-signed
func IsSelfSigned(certX509 *x509.Certificate) bool {
	return len(certX509.Issuer.Organization) > 0 &&
		certX509.Issuer.Organization[0] == SelfSignedCertificateIssuer
}

// InspectCertificate loads a X.509 certificate and checks its details, such as expiration
func InspectCertificate(site *pb.Site, obj *pb.TLSCertificate, certX509 *x509.Certificate) error {
	now := time.Now()

	// Check "NotAfter" (require at least 12 hours)
	if certX509.NotAfter.Before(now.Add(12 * time.Hour)) {
		return fmt.Errorf("certificate has expired or has less than 12 hours of validity: %v", certX509.NotAfter)
	}

	// Check "NotBefore"
	if !certX509.NotBefore.Before(now) {
		return fmt.Errorf("certificate's NotBefore is in the future: %v", certX509.NotBefore)
	}

	// Check if the list of domains matches, but only for self-signed or ACME certificates
	// We're not checking this for imported certificates because they might have wildcards and be valid for more domains
	if obj.Type == pb.TLSCertificate_ACME || obj.Type == pb.TLSCertificate_SELF_SIGNED {
		domains := append([]string{site.Domain}, site.Aliases...)
		sort.Strings(domains)
		// Clone the object so we don't cause issues while sorting it
		certDomains := append(make([]string, 0), certX509.DNSNames...)
		sort.Strings(certDomains)
		if !reflect.DeepEqual(domains, certDomains) {
			return fmt.Errorf("list of domains in certificate does not match: %v", certDomains)
		}
	}

	return nil
}
