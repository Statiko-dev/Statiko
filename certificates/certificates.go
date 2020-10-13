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
	"log"
	"os"

	"github.com/statiko-dev/statiko/certificates/azurekeyvault"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/state"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Certificates is the class that manages TLS certificates
type Certificates struct {
	State   *state.Manager
	Cluster *cluster.Cluster
	logger  *log.Logger
}

// Init the object
func (c *Certificates) Init() {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "certificates: ", log.Ldate|log.Ltime|log.LUTC)
}

// GetCertificate returns the certificate for the site (with key and certificate PEM-encoded)
func (c *Certificates) GetCertificate(site *pb.Site) (key []byte, cert []byte, err error) {
	if site == nil || site.Tls == "" {
		return nil, nil, errors.New("empty TLS configuration")
	}

	// Request the certificate information (and possibly data) from the state
	var (
		certObj  *pb.TLSCertificate
		certX509 *x509.Certificate
	)
	certObj, key, cert, err = c.State.GetCertificate(site.Tls)
	if err != nil {
		return nil, nil, err
	}
	if certObj == nil {
		return nil, nil, errors.New("certificate not found")
	}

	// Check the type of the TLS certificate
	switch certObj.Type {
	case pb.TLSCertificate_AZURE_KEY_VAULT:
		// Get the certificate
		key, cert, certX509, err = c.GetAKVCertificate(certObj)
		if err != nil {
			return nil, nil, err
		}

		// Inspect the certificate, but consider errors as warnings only
		if insp := InspectCertificate(site, certObj, certX509); insp != nil {
			c.logger.Printf("[Warn] %v\n", insp)
		}
		return
	case pb.TLSCertificate_IMPORTED:
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
	case pb.TLSCertificate_SELF_SIGNED:
		key, cert, err = c.GetSelfSignedCertificate(site, certObj, key, cert)
		return
	case pb.TLSCertificate_ACME:
		key, cert, err = c.GetACMECertificate(site, certObj, key, cert)
		return
	default:
		err = errors.New("invalid TLS certificate type")
		return
	}
}

// GetAKVCertificate returns a certificate from Azure Key Vault
func (c *Certificates) GetAKVCertificate(certObj *pb.TLSCertificate) (key []byte, cert []byte, certX509 *x509.Certificate, err error) {
	var name, version string
	version, cert, key, certX509, err = azurekeyvault.GetInstance().GetCertificate(certObj.Name, certObj.Version)
	if err != nil {
		return
	}

	c.logger.Printf("Retrieved TLS certificate from AKV: %s (%s)\n", name, version)

	return
}
