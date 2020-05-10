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

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

// GetACMECertificate returns a certificate issued by ACME (e.g. Let's Encrypt), with key and certificate PEM-encoded
// If Let's Encrypt hasn't issued a certificate yet, this will return a self-signed TLS certificate, until the Let's Encrypt one is available
func GetACMECertificate(site *state.SiteState) (key []byte, cert []byte, err error) {
	// Secret keys
	storePathKey := "cert/acme/" + site.Domain + ".key.pem"
	storePathCert := "cert/acme/" + site.Domain + ".cert.pem"

	// Check if we have a certificate issued by Let's Encrypt already
	key, err = state.Instance.GetSecret(storePathKey)
	if err != nil {
		return nil, nil, err
	}
	cert, err = state.Instance.GetSecret(storePathCert)
	if err != nil {
		return nil, nil, err
	}
	if key != nil && len(key) > 0 && cert != nil && len(cert) > 0 {
		// If the certificate has expired, still return it, but in the meanwhile trigger a refresh job
		block, _ := pem.Decode(cert)
		if block == nil {
			err = errors.New("invalid certificate PEM block")
			return nil, nil, err
		}
		certObj, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		if certErr := InspectCertificate(site, certObj); certErr != nil {
			logger.Printf("Certificate from Let's Encrypt for site %s has an error; requesting a new one: %v\n", site.Domain, certErr)
			err := requestACMECertificate(site)
			if err != nil {
				return nil, nil, err
			}
		}

		// Return the certificate (even if invalid/expired)
		return key, cert, nil
	}

	// No certificate yet
	// Triggering a background job togenerate it, and for now returning a self-signed certificate
	logger.Println("Requesting certificate from Let's Encrypt for site", site.Domain)
	err = requestACMECertificate(site)
	if err != nil {
		return nil, nil, err
	}
	return GetSelfSignedCertificate(site)
}

func requestACMECertificate(site *state.SiteState) error {
	// List of domains
	domains := append([]string{site.Domain}, site.Aliases...)

	// Create a job
	job := utils.JobData{
		Type: utils.JobTypeACME,
		Data: strings.Join(domains, ","),
	}
	_, err := state.Worker.AddJob(job)
	if err != nil {
		return err
	}
	return nil
}
