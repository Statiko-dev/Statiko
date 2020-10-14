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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"os"

	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/lego"
	"github.com/go-acme/lego/v3/registration"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/state"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// ACMEMinDays controls how many days from the expiration a new certificate is requested from ACME
const ACMEMinDays = 21

// GetACMECertificate returns a certificate issued by ACME (e.g. Let's Encrypt), with key and certificate PEM-encoded
// If the ACME provider hasn't issued a certificate yet, this will return a self-signed TLS certificate, until the ACME one is available
func (c *Certificates) GetACMECertificate(site *pb.Site, certificateId string) (key []byte, cert []byte, err error) {
	var certObj *pb.TLSCertificate

	// Get the certificate object
	certObj, key, cert, err = c.State.GetCertificate(certificateId)
	if err != nil {
		return nil, nil, err
	}
	if certObj == nil {
		return nil, nil, errors.New("certificate not found")
	}

	// Check if we have a certificate issued by the ACME provider already
	if len(key) > 0 && len(cert) > 0 {
		// Get the x509 object
		certX509, err := GetX509(cert)
		if err != nil {
			return nil, nil, err
		}

		// Check if the certificate is self-signed
		if IsSelfSigned(certX509) {
			c.logger.Printf("Stored certificate for ACME is self-signed; will request a new one: %v\n", site.Domain)
			// TODO: THIS
			//state.Instance.TriggerRefreshCerts()
		} else if certErr := InspectCertificate(site, certObj, certX509); certErr != nil {
			// If the certificate has expired, still return it, but in the meanwhile trigger a refresh job
			c.logger.Printf("Certificate from ACME provider for site %s has an error; will request a new one: %v\n", site.Domain, certErr)
			// TODO: THIS
			//state.Instance.TriggerRefreshCerts()
		}

		// Return the certificate (even if invalid/expired)
		return key, cert, nil
	}

	// No certificate yet
	// Triggering a background job to generate it, and for now returning a self-signed certificate
	c.logger.Println("Requesting certificate from ACME provider for site", site.Domain)
	// TODO: THIS
	//state.Instance.TriggerRefreshCerts()
	return c.GetSelfSignedCertificate(site, certificateId)
}

// GenerateACMECertificate requests a new certificate from the ACME provider
func (c *Certificates) GenerateACMECertificate(domains ...string) (keyPEM []byte, certPEM []byte, err error) {
	// Ensure we have at least 1 domain
	if len(domains) < 1 {
		err = errors.New("need to specify at least one domain name")
		return
	}

	// Get the email
	email := appconfig.Config.GetString("acme.email")
	if email == "" {
		return nil, nil, errors.New("configuration option acme.email must be set")
	}

	// Get the private key, or generate one if doesn't exist
	privateKey, err := c.acmePrivateKey(email)
	if err != nil {
		return nil, nil, err
	}

	// Client configuration
	user := ACMEUser{
		Email: email,
		key:   privateKey,
	}
	config := lego.NewConfig(&user)
	config.Certificate.KeyType = certcrypto.RSA4096
	config.CADirURL = appconfig.Config.GetString("acme.endpoint")
	// Disable TLS validation when connecting to the ACME endpoint with the "ACME_INSECURE_TLS" env var for development
	if os.Getenv("ACME_INSECURE_TLS") != "" && appconfig.ENV != "production" {
		config.HTTPClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	// Client
	client, err := lego.NewClient(config)
	if err != nil {
		return nil, nil, err
	}

	// New users will need to register
	reg, err := c.acmeRegistration(email, client)
	if err != nil {
		return nil, nil, err
	}
	user.Registration = reg

	// Enable the HTTP-01 challenge
	err = client.Challenge.SetHTTP01Provider(&StatikoProvider{
		State:   c.State,
		Cluster: c.Cluster,
	})
	if err != nil {
		return nil, nil, err
	}

	// Request the certificate
	// This always generates a new key, even if we're renewing the certificate
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return nil, nil, err
	}

	// Retrieve the certificate and private key
	if len(certificates.PrivateKey) == 0 {
		return nil, nil, errors.New("received an empty private key")
	}
	if len(certificates.Certificate) == 0 {
		return nil, nil, errors.New("received an empty certificate")
	}

	return certificates.PrivateKey, certificates.Certificate, nil
}

// Returns the private key for ACME
func (c *Certificates) acmePrivateKey(email string) (*ecdsa.PrivateKey, error) {
	// Check if we have a key stored
	storePath := "acme/keys/" + utils.SHA256String(email)[:15] + ".pem"
	data, err := c.State.GetSecret(storePath)
	if err != nil {
		return nil, err
	}
	if data != nil && len(data) > 0 {
		return utils.UnserializeECDSAKey(data)
	}

	// If we're here, we need to generate one (and store it)
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	data, err = utils.SerializeECDSAKey(privateKey)
	if err != nil {
		return nil, err
	}
	err = c.State.SetSecret(storePath, data)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// Returns the registration object for ACME
func (c *Certificates) acmeRegistration(email string, client *lego.Client) (*registration.Resource, error) {
	// Check if the user has registered already
	storePath := "acme/registrations/" + utils.SHA256String(email)[:15] + ".pem"
	data, err := c.State.GetSecret(storePath)
	if err != nil {
		return nil, err
	}
	if data == nil || len(data) == 0 {
		// No data, register a new user
		return c.acmeNewRegistration(email, client)
	}

	// Decode JSON
	reg := &registration.Resource{}
	err = json.Unmarshal(data, reg)
	if err != nil {
		return nil, err
	}

	if reg == nil || reg.URI == "" || !reg.Body.TermsOfServiceAgreed {
		// Register a new user
		return c.acmeNewRegistration(email, client)
	}

	return reg, nil
}

// Registers a new user for ACME
func (c *Certificates) acmeNewRegistration(email string, client *lego.Client) (*registration.Resource, error) {
	// Register the user
	reg, err := client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: true,
	})
	if err != nil {
		return nil, err
	}

	// Store the registration object
	data, err := json.Marshal(reg)
	if err != nil {
		return nil, err
	}
	storePath := "acme/registrations/" + utils.SHA256String(email)[:15] + ".pem"
	err = c.State.SetSecret(storePath, data)
	if err != nil {
		return nil, err
	}

	return reg, nil
}

// ACMEUser implements registration.User
type ACMEUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *ACMEUser) GetEmail() string {
	return u.Email
}
func (u *ACMEUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// StatikoProvider implements ChallengeProvider for `http-01` challenge.
type StatikoProvider struct {
	State   *state.Manager
	Cluster *cluster.Cluster
}

// Present makes the token available
func (w *StatikoProvider) Present(domain, token, keyAuth string) error {
	// Message
	message := domain + "|" + keyAuth

	// Set the token as a secret
	err := w.State.SetSecret("acme/challenges/"+token, []byte(message))
	if err != nil {
		return err
	}

	// Wait until the cluster has synced
	// This is a blocking call
	ver := w.State.GetVersion()
	w.Cluster.WaitForVersion(ver)

	return nil
}

// CleanUp removes the key created for the challenge
func (w *StatikoProvider) CleanUp(domain, token, keyAuth string) error {
	// Delete the secret
	err := w.State.DeleteSecret("acme/challenges/" + token)
	if err != nil {
		return err
	}
	return nil
}
