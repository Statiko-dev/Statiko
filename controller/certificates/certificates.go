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
	"log"
	"os"
	"strings"

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
func (c *Certificates) Init() error {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "certificates: ", log.Ldate|log.Ltime|log.LUTC)

	return nil
}

// GetCertificate returns a certificate key and cert by ID
func (c *Certificates) GetCertificate(certificateId string) (key []byte, cert []byte, err error) {
	// Check if the certificate type
	switch {
	// Certificate from Azure Key Vault
	case strings.HasPrefix(certificateId, "akv:"):
		// Request the certificate
		key, cert, err = c.GetAKVCertificate(certificateId)
		if err != nil {
			return nil, nil, err
		}
		if len(cert) == 0 || len(key) == 0 {
			return nil, nil, errors.New("certificate not found")
		}
	// Imported certificate in the state store
	default:
		// Get the certificate
		var certObj *pb.TLSCertificate
		certObj, key, cert, err = c.State.GetCertificate(certificateId)
		if err != nil {
			return nil, nil, err
		}
		if certObj == nil {
			return nil, nil, errors.New("certificate not found")
		}
	}

	return key, cert, nil
}
