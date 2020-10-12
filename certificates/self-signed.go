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
	"strings"
	"time"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// SelfSignedCertificateIssuer is the organization that issues self-signed certificates
const SelfSignedCertificateIssuer = "statiko self-signed"

// SelfSignedMinDays controls how many days from the expiration self-signed certificates are renewed
const SelfSignedMinDays = 14

// GetSelfSignedCertificate returns a self-signed certificate, with key and certificate PEM-encoded
func (c *Certificates) GetSelfSignedCertificate(site *pb.State_Site) (key []byte, cert []byte, err error) {
	var block *pem.Block
	var certObj *x509.Certificate

	// List of domains
	domains := append([]string{site.Domain}, site.Aliases...)

requestcert:
	// Check if we have certificates generated already in the state store
	key, cert, err = c.AgentState.GetCertificate(pb.State_Site_TLS_SELF_SIGNED, domains)
	if err != nil {
		return nil, nil, err
	}

	// Check if the certificate is not empty
	if key == nil || len(key) == 0 || cert == nil || len(cert) == 0 {
		c.logger.Println("Generating missing self-signed certificate for site", site.Domain)
		goto newcert
	}

	// Check if the certificate is still valid
	block, _ = pem.Decode(cert)
	if block == nil {
		err = errors.New("invalid certificate PEM block")
		return nil, nil, err
	}
	certObj, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	if certErr := InspectCertificate(site, certObj); certErr != nil {
		c.logger.Printf("Regenerating invalid self-signed certificate for site %s: %v\n", site.Domain, certErr)
		goto newcert
	}

	return

newcert:
	// Request a new certificate
	job := utils.JobData{
		Type: utils.JobTypeTLSCertificate,
		Data: strings.Join(domains, ","),
	}
	jobID, err := state.Worker.AddJob(job)
	if err != nil {
		return nil, nil, err
	}

	// Wait for the job
	ch := make(chan error, 1)
	go state.Worker.WaitForJob(jobID, ch)
	err = <-ch
	close(ch)
	if err != nil {
		return nil, nil, err
	}

	// Wait 3 seconds to ensure changes are synced
	time.Sleep(3 * time.Second)

	// Get the certificate
	goto requestcert
}
