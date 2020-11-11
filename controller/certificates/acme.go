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

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/shared/utils"
)

// ACMEMinDays controls how many days from the expiration a new certificate is requested from ACME
const ACMEMinDays = 21

// GenerateACMECertificate requests a new certificate from the ACME provider
func (c *Certificates) GenerateACMECertificate(domains ...string) (keyPEM []byte, certPEM []byte, err error) {
	// Ensure we have at least 1 domain
	if len(domains) < 1 {
		err = errors.New("need to specify at least one domain name")
		return
	}

	// Get the email
	email := viper.GetString("acme.email")
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
		Key:   privateKey,
	}
	config := lego.NewConfig(&user)
	config.Certificate.KeyType = certcrypto.RSA4096
	config.CADirURL = viper.GetString("acme.endpoint")
	// Disable TLS validation when connecting to the ACME endpoint with the "ACME_INSECURE_TLS" env var for development
	if os.Getenv("ACME_INSECURE_TLS") != "" && buildinfo.ENV != "production" {
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

	// Disable the TLS-ALPN-01 challenge
	client.Challenge.Remove(challenge.TLSALPN01)

	// Enable the HTTP-01 challenge
	err = client.Challenge.SetHTTP01Provider(&StatikoProvider{
		State:          c.State,
		ACMETokenReady: c.ACMETokenReady,
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
	Key          crypto.PrivateKey
}

func (u *ACMEUser) GetEmail() string {
	return u.Email
}
func (u *ACMEUser) GetRegistration() *registration.Resource {
	return u.Registration
}
func (u *ACMEUser) GetPrivateKey() crypto.PrivateKey {
	return u.Key
}

// This function blocks until the node is ready to present the ACME token
// In the cluster, this waits until all nodes in the cluster are in sync and on the right version
type ACMETokenReadyFunc func() error

// StatikoProvider implements ChallengeProvider for `http-01` challenge.
type StatikoProvider struct {
	State          secretProvider
	ACMETokenReady ACMETokenReadyFunc
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

	// Wait for when the node/cluster is ready to present the token
	if w.ACMETokenReady != nil {
		// This is a blocking call
		err = w.ACMETokenReady()
		if err != nil {
			return err
		}
	}

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
