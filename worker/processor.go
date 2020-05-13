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

package worker

import (
	"errors"
	"strings"

	"github.com/statiko-dev/statiko/certificates"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

// ProcessJob processes a job
func ProcessJob(job utils.JobData) error {
	switch job.Type {
	case utils.JobTypeTLSCertificate:
		return processCertJobs("tlscert", job.Data)
	case utils.JobTypeACME:
		return processCertJobs("acme", job.Data)
	}
	return errors.New("invalid job type")
}

// Processes the "tlscert" and "acme" jobs
func processCertJobs(jobType string, data string) error {
	// List of domains
	domains := strings.Split(data, ",")
	if len(domains) < 1 {
		return errors.New("empty domain list")
	}

	// Specialize for the job
	var genFunc func(...string) ([]byte, []byte, error)
	var certType string
	switch jobType {
	case "tlscert":
		genFunc = certificates.GenerateTLSCert
		certType = "selfsigned"
	case "acme":
		genFunc = certificates.GenerateACMECertificate
		certType = "acme"
	}

	// Generate the TLS certificate
	key, cert, err := genFunc(domains...)
	if err != nil {
		return err
	}

	// Store the certificate
	err = state.Instance.SetCertificate(certType, domains, key, cert)
	if err != nil {
		return err
	}

	// If certificate is of type acme, delete the old self-signed certificate
	if certType == "acme" {
		err = state.Instance.DeleteSecret(state.Instance.CertificateSecretKey("selfsigned", domains))
		if err != nil {
			return err
		}
	}

	return nil
}
