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
	"bytes"
	"io/ioutil"
	"strings"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

var (
	appRoot string
)

// SyncCertificates ensures that all self-signed certificates are written on disk and synced in the state
func SyncCertificates(sites []state.SiteState) (updated bool, err error) {
	updated = false

	appRoot = appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	// Iterate through all sites and look for those requiring a self-signed certificate
	for _, s := range sites {
		// Skip those who use an imported certificate
		if s.TLSCertificateType == state.TLSCertificateImported {
			continue
		}

		u, err := processSite(&s)
		if err != nil {
			logger.Println("[Error] Error while processing certificates for site", s.Domain, err)
			return false, err
		}

		updated = updated || u
	}

	return
}

// Processes certificates for a site
func processSite(site *state.SiteState) (updated bool, err error) {
	updated = false

	logger.Println("Processing certificate for site", site.Domain)

	domains := append([]string{site.Domain}, site.Aliases...)
	cachePathKey := appRoot + "cache/" + site.Domain + ".selfsigned.key.pem"
	cachePathCert := appRoot + "cache/" + site.Domain + ".selfsigned.cert.pem"
	storePathKey := "cert/" + site.Domain + ".key.pem"
	storePathCert := "cert/" + site.Domain + ".cert.pem"

	// Check if we have certificates generated already in the state store
	keyPEM, err := state.Instance.GetSecret(storePathKey)
	if err != nil {
		return
	}
	certPEM, err := state.Instance.GetSecret(storePathCert)
	if err != nil {
		return
	}

	// If we don't have certs, generate them
	// Likewise, check if the certificate is still valid: needs to have at least 7 days of validity, and must be for the same domains. If invalid, still generate a new one
	if keyPEM == nil || len(keyPEM) == 0 || certPEM == nil || len(certPEM) == 0 || !checkSelfSignedTLSCertificate(certPEM, domains) {
		logger.Println("Need to generate a self-signed certificate for site", site.Domain)

		keyPEM, certPEM, err = generateSelfSignedCertificate(domains...)
		if err != nil {
			return
		}

		// Add to the state store
		err = state.Instance.SetSecret(storePathKey, keyPEM)
		if err != nil {
			return
		}
		err = state.Instance.SetSecret(storePathCert, certPEM)
		if err != nil {
			return
		}

		// Write to cache
		err = utils.WriteData(keyPEM, cachePathKey)
		if err != nil {
			return
		}
		err = utils.WriteData(certPEM, cachePathCert)
		if err != nil {
			return
		}

		updated = true
	} else {
		// Check if we already have the certificate in cache
		var exists bool
		exists, err = utils.CertificateExists(cachePathCert, cachePathKey)
		if err != nil {
			return
		}

		// If it exists, ensure it's the same file
		if exists {
			var readKey, readCert []byte
			readKey, err = ioutil.ReadFile(cachePathKey)
			if err != nil {
				return
			}
			readCert, err = ioutil.ReadFile(cachePathCert)
			if err != nil {
				return
			}

			// Check if the files are the same
			if bytes.Compare(readKey, keyPEM) != 0 || bytes.Compare(readCert, certPEM) != 0 {
				logger.Println("Writing updated self-signed certificate in cache for site", site.Domain)

				// Write the correct files
				err = utils.WriteData(keyPEM, cachePathKey)
				if err != nil {
					return
				}
				err = utils.WriteData(certPEM, cachePathCert)
				if err != nil {
					return
				}

				updated = true
			}
		} else {
			logger.Println("Writing new self-signed certificate in cache for site", site.Domain)

			// Write the certificates in cache
			err = utils.WriteData(keyPEM, cachePathKey)
			if err != nil {
				return
			}
			err = utils.WriteData(certPEM, cachePathCert)
			if err != nil {
				return
			}

			updated = true
		}
	}

	return
}
