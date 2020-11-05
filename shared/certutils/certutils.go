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

package certutils

import (
	"errors"
	"log"
	"os"
	"strings"

	"github.com/statiko-dev/statiko/shared/azurekeyvault"
)

var NotFoundErr = errors.New("certificate not found")
var logger *log.Logger

// Init the package
func init() {
	logger = log.New(os.Stdout, "certutils: ", log.Ldate|log.Ltime|log.LUTC)
}

// Interface for both State (for the controller) and AgentState (for the agent)
type stateStoreCert interface {
	GetCertificate(certId string) (key []byte, cert []byte, err error)
}

// GetCertificate returns a certificate key and cert by ID from the certificate cache
func GetCertificate(certificateId string, state stateStoreCert, akv *azurekeyvault.Client) (key []byte, cert []byte, err error) {
	// Check if the certificate type
	switch {

	// Certificate from Azure Key Vault
	case strings.HasPrefix(certificateId, "akv:"):
		if akv == nil {
			return nil, nil, errors.New("requesting a certificate from Azure Key Vault, but AKV client is nil")
		}

		// Request the certificate
		key, cert, err = GetAKVCertificate(certificateId, akv)
		if err != nil {
			return nil, nil, err
		}
		if len(cert) == 0 || len(key) == 0 {
			return nil, nil, NotFoundErr
		}

	// Certificate is in the state store
	default:
		// Get the certificate
		key, cert, err = state.GetCertificate(certificateId)
		if err != nil {
			return nil, nil, err
		}
		if len(key) == 0 || len(cert) == 0 {
			return nil, nil, NotFoundErr
		}
	}

	return key, cert, nil
}

// GetAKVCertificate returns a certificate from Azure Key Vault
func GetAKVCertificate(certificateId string, akv *azurekeyvault.Client) (key []byte, cert []byte, err error) {
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
	version, cert, key, _, err = akv.GetCertificate(name, version)
	if err != nil {
		return nil, nil, err
	}
	logger.Printf("Retrieved TLS certificate from AKV: %s (%s)\n", name, version)

	return key, cert, err
}
