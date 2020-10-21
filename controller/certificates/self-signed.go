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
	"errors"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// SelfSignedCertificateIssuer is the organization that issues self-signed certificates
const SelfSignedCertificateIssuer = "statiko self-signed"

// SelfSignedMinDays controls how many days from the expiration self-signed certificates are renewed
const SelfSignedMinDays = 14

// GetSelfSignedCertificate returns a self-signed certificate, with key and certificate PEM-encoded
// TODO: REMOVE THIS
func (c *Certificates) GetSelfSignedCertificate(site *pb.Site, certificateId string) (key []byte, cert []byte, err error) {
	var certObj *pb.TLSCertificate

	// Get the certificate object
	certObj, key, cert, err = c.State.GetCertificate(certificateId)
	if err != nil {
		return nil, nil, err
	}
	if certObj == nil {
		return nil, nil, errors.New("certificate not found")
	}

	// If we have an existing certificate, check if it's still valid
	if len(key) > 0 && len(cert) > 0 {
		// Get the x509 object
		certX509, err := GetX509(cert)
		if err != nil {
			return nil, nil, err
		}

		// If the certificate is valid, use that
		certErr := InspectCertificate(site, certObj, certX509)
		if certErr == nil {
			return key, cert, nil
		}
		c.logger.Printf("Regenerating invalid self-signed certificate for site %s: %v\n", site.Domain, certErr)
	} else {
		c.logger.Println("Generating missing self-signed certificate for site", site.Domain)
	}

	// If we're here, we need to generate a new sellf-signed certificate
	// That's either because we didn't have one to begin with, or because we had one but it had expired or it was invalid
	domains := append([]string{site.Domain}, site.Aliases...)
	key, cert, err = GenerateTLSCert(domains...)

	// Save the certificate
	err = c.State.SetCertificate(certObj, certificateId, key, cert)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}
