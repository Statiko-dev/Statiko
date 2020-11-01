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

	"github.com/statiko-dev/statiko/agent/client"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/certutils"
)

// AgentCertificates is the class that retrieves TLS certificates for the state object
type AgentCertificates struct {
	State  *state.AgentState
	RPC    *client.RPCClient
	AKV    *azurekeyvault.Client
	logger *log.Logger
}

// Init the object
func (c *AgentCertificates) Init() error {
	// Initialize the logger
	c.logger = log.New(os.Stdout, "certificates: ", log.Ldate|log.Ltime|log.LUTC)

	return nil
}

// GetCertificate returns a certificate key and cert by ID
func (c *AgentCertificates) GetCertificate(certificateId string) (key []byte, cert []byte, err error) {
	if certificateId == "" {
		return nil, nil, errors.New("parameter certificateId must not be empty")
	}

	// Check if the certificate is in the cache (or if it's from an external source like Azure Key Vault)
	key, cert, err = certutils.GetCertificate(certificateId, c.State, c.AKV)
	if err == nil && len(key) > 0 && len(cert) > 0 {
		return key, cert, nil
	}

	// Check if we have an error or if the cert was just not found
	if err != certutils.NotFoundErr {
		return nil, nil, err
	}

	// Try requesting the certificate from the agent
	msg, err := c.RPC.GetTLSCertificate(certificateId)
	if err != nil {
		return nil, nil, err
	}
	if msg == nil || len(msg.KeyPem) == 0 || len(msg.CertificatePem) == 0 {
		return nil, nil, certutils.NotFoundErr
	}

	// We have a certificate! Store it in the cache before returning
	key = []byte(msg.KeyPem)
	cert = []byte(msg.CertificatePem)
	err = c.State.SetCertificate(certificateId, key, cert)
	if err != nil {
		return nil, nil, err
	}

	return key, cert, nil
}
