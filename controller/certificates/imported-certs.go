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
	"strings"

	"github.com/statiko-dev/statiko/controller/certificates/azurekeyvault"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetImportedCertificate returns a certificate from the internal store
func (c *Certificates) GetImportedCertificate(site *pb.Site, certificateId string) (key []byte, cert []byte, err error) {
	var (
		certObj  *pb.TLSCertificate
		certX509 *x509.Certificate
	)

	// Get the certificate object
	certObj, key, cert, err = c.State.GetCertificate(certificateId)
	if err != nil {
		return nil, nil, err
	}
	if certObj == nil {
		return nil, nil, errors.New("certificate not found")
	}

	// Ensure we have a certificate
	if len(key) == 0 || len(cert) == 0 {
		return nil, nil, errors.New("certificate is empty")
	}

	// Get the x509 object
	certX509, err = GetX509(cert)
	if err != nil {
		return nil, nil, err
	}

	// Inspect the certificate, but consider errors as warnings only
	if insp := InspectCertificate(site, certObj, certX509); insp != nil {
		c.logger.Printf("[Warn] %v\n", insp)
	}

	return
}

// GetAKVCertificate returns a certificate from Azure Key Vault
func (c *Certificates) GetAKVCertificate(certificateId string) (key []byte, cert []byte, err error) {
	// Get the name and version
	pos := strings.Index(certificateId, "/")
	var name, version string
	if pos == -1 {
		name = certificateId[4:]
	} else {
		name = certificateId[4:pos]
		version = certificateId[(pos + 1):]
	}

	// Get the certificate and key
	version, cert, key, _, err = azurekeyvault.GetInstance().GetCertificate(name, version)
	if err != nil {
		return nil, nil, err
	}
	c.logger.Printf("Retrieved TLS certificate from AKV: %s (%s)\n", name, version)

	return key, cert, err
}
