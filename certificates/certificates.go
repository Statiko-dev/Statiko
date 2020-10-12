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
	"log"
	"os"

	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/certificates/azurekeyvault"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Certificates is the class that manages TLS certificates
type Certificates struct {
	AgentState *state.AgentState

	logger *log.Logger
}

// Init the object
func (c *Certificates) Init() {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "certificates: ", log.Ldate|log.Ltime|log.LUTC)
}

// GetCertificate returns the certificate for the site (with key and certificate PEM-encoded)
func (c *Certificates) GetCertificate(site *pb.State_Site) (key []byte, cert []byte, err error) {
	if site == nil || site.Tls == nil {
		return nil, nil, errors.New("empty TLS configuration")
	}

	var certObj *x509.Certificate

	// Check the type of the TLS certificate
	switch site.Tls.Type {
	case pb.State_Site_TLS_AZURE_KEY_VAULT:
		// Get the certificate
		key, cert, certObj, err = c.GetAKVCertificate(site)
		if err != nil {
			return
		}

		// Inspect the certificate, but consider errors as warnings only
		if insp := InspectCertificate(site, certObj); insp != nil {
			c.logger.Printf("[Warn] %v\n", insp)
		}
		return
	case pb.State_Site_TLS_IMPORTED:
		// Get the certificate
		key, cert, certObj, err = c.GetImportedCertificate(site)
		if err != nil {
			return
		}

		// Inspect the certificate, but consider errors as warnings only
		if insp := InspectCertificate(site, certObj); insp != nil {
			c.logger.Printf("[Warn] %v\n", insp)
		}
		return
	case pb.State_Site_TLS_SELF_SIGNED:
		key, cert, err = c.GetSelfSignedCertificate(site)
		return
	case pb.State_Site_TLS_ACME:
		key, cert, err = c.GetACMECertificate(site)
		return
	default:
		err = errors.New("invalid TLS certificate type")
		return
	}
}

// GetImportedCertificate returns a certificate stored in the state store
func (c *Certificates) GetImportedCertificate(site *pb.State_Site) (key []byte, cert []byte, certObj *x509.Certificate, err error) {
	if site == nil || site.Tls == nil || site.Tls.Certificate == "" {
		return nil, nil, nil, errors.New("empty TLS certificate name")
	}

	// Get the certificate from the store
	key, cert, err = c.AgentState.GetCertificate(pb.State_Site_TLS_IMPORTED, []string{site.Tls.Certificate})
	if err != nil {
		return nil, nil, nil, err
	}

	// Get the certificate's x509 object
	block, _ := pem.Decode(cert)
	if block == nil {
		err = errors.New("invalid certificate PEM block")
		return nil, nil, nil, err
	}
	certObj, err = x509.ParseCertificate(block.Bytes)
	return
}

// GetAKVCertificate returns a certificate from Azure Key Vault
func (c *Certificates) GetAKVCertificate(site *pb.State_Site) (key []byte, cert []byte, certObj *x509.Certificate, err error) {
	var name, version string
	if site.Tls.Certificate == "" {
		err = errors.New("certificate name is empty")
		return
	}
	version, cert, key, certObj, err = azurekeyvault.GetInstance().GetCertificate(site.Tls.Certificate, site.Tls.Version)
	if err != nil {
		return
	}

	c.logger.Printf("Retrieved TLS certificate from AKV: %s (%s)\n", name, version)

	return
}
