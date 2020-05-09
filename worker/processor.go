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

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

// ProcessJob processes a job
func ProcessJob(job utils.JobData) error {
	switch job.Type {
	case utils.JobTypeTLSCertificate:
		return processJobTLSCert(job.Data)
	}
	return errors.New("invalid job type")
}

// Processes the "tlscert" job
func processJobTLSCert(data string) error {
	// List of domains
	domains := strings.Split(data, ",")
	if len(domains) < 1 {
		return errors.New("empty domain list")
	}

	// Generate the TLS certificate
	key, cert, err := utils.GenerateTLSCert(domains...)
	if err != nil {
		return err
	}

	// Store the certificate
	storePathKey := "cert/selfsigned/" + domains[0] + ".key.pem"
	storePathCert := "cert/selfsigned/" + domains[0] + ".cert.pem"
	err = state.Instance.SetSecret(storePathKey, key)
	if err != nil {
		return err
	}
	err = state.Instance.SetSecret(storePathCert, cert)
	if err != nil {
		return err
	}

	return nil
}
