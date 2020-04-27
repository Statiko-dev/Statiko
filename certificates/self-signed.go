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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"

	"github.com/statiko-dev/statiko/state"
)

// SelfSignedCertificateIssuer is the organization that issues self-signed certificates
const SelfSignedCertificateIssuer = "statiko self-signed"

// GetSelfSignedCertificate returns a self-signed certificate, with key and certificate PEM-encoded
func GetSelfSignedCertificate(site *state.SiteState) (key []byte, cert []byte, err error) {
	// List of domains
	domains := append([]string{site.Domain}, site.Aliases...)

	// Keys
	storePathKey := "cert/" + site.Domain + ".key.pem"
	storePathCert := "cert/" + site.Domain + ".cert.pem"

	// Check if we have certificates generated already in the state store
	key, err = state.Instance.GetSecret(storePathKey)
	if err != nil {
		key = nil
		return
	}
	cert, err = state.Instance.GetSecret(storePathCert)
	if err != nil {
		key = nil
		cert = nil
		return
	}

	var certObj *x509.Certificate

	// TODO: THIS SHOULD HAPPEN IN THE CLUSTER LEADER ONLY
	// Check if the certificate is not empty and if it's still valid
	if key == nil || len(key) == 0 || cert == nil || len(cert) == 0 {
		goto generate
	}

	// Check if the certificate is not valid anymore
	certObj, err = x509.ParseCertificate(cert)
	if err != nil {
		return
	}
	if InspectCertificate(site, certObj) != nil {
		goto generate
	}

	return

generate:
	// Generate a new certificate
	key, cert, err = generateSelfSignedCertificate(domains...)

	// Store the certificate
	err = state.Instance.SetSecret(storePathKey, key)
	if err != nil {
		return
	}
	err = state.Instance.SetSecret(storePathCert, cert)
	if err != nil {
		return
	}
	return
}

// generateSelfSignedCertificate generates a new self-signed TLS certificate (with a RSA 4096-bit key) and returns the private key and public certificate encoded as PEM
// The first domain is the primary one, used as value for the "Common Name" value too
// Each certificate is valid for 1 year
func generateSelfSignedCertificate(domains ...string) (keyPEM []byte, certPEM []byte, err error) {
	// Ensure we have at least 1 domain
	if len(domains) < 1 {
		err = errors.New("need to specify at least one domain name")
		return
	}

	// Generate a private key
	// The main() method has already invoked rand.Seed
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}

	// Build the X.509 certificate
	now := time.Now()
	tpl := x509.Certificate{}
	tpl.BasicConstraintsValid = false
	tpl.DNSNames = domains
	tpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}
	tpl.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment
	tpl.IsCA = false
	tpl.NotAfter = now.Add(43800 * time.Hour) // 5 year
	tpl.NotBefore = now
	tpl.SerialNumber = big.NewInt(1)
	tpl.SignatureAlgorithm = x509.SHA256WithRSA
	tpl.Subject = pkix.Name{
		Organization: []string{SelfSignedCertificateIssuer},
		CommonName:   domains[0],
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &privateKey.PublicKey, privateKey)
	if err != nil {
		return
	}

	// Encode the key in a PEM block
	buf := &bytes.Buffer{}
	err = pem.Encode(buf, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err != nil {
		return
	}
	keyPEM = buf.Bytes()

	// Encode the certificate in a PEM block
	buf = &bytes.Buffer{}
	err = pem.Encode(buf, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})
	if err != nil {
		return
	}
	certPEM = buf.Bytes()

	logger.Println("Generated self-signed certificate for", domains[0])

	return
}
