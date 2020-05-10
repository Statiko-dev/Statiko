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

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

// GetSelfSignedCertificate returns a self-signed certificate, with key and certificate PEM-encoded
func GetSelfSignedCertificate(site *state.SiteState) (key []byte, cert []byte, err error) {
	var block *pem.Block
	var certObj *x509.Certificate

	// List of domains
	domains := append([]string{site.Domain}, site.Aliases...)

	// Keys
	storePathKey := "cert/selfsigned/" + site.Domain + ".key.pem"
	storePathCert := "cert/selfsigned/" + site.Domain + ".cert.pem"

requestcert:
	// Check if we have certificates generated already in the state store
	key, err = state.Instance.GetSecret(storePathKey)
	if err != nil {
		return nil, nil, err
	}
	cert, err = state.Instance.GetSecret(storePathCert)
	if err != nil {
		return nil, nil, err
	}

	// Check if the certificate is not empty
	if key == nil || len(key) == 0 || cert == nil || len(cert) == 0 {
		logger.Println("Generating missing certificate for site", site.Domain)
		goto newcert
	}

	// Check if the certificate is not valid anymore
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
		logger.Printf("Regenerating invalid certificate for site %s: %v\n", site.Domain, certErr)
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
