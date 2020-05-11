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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-acme/lego/v3/certcrypto"
	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/lego"
	"github.com/go-acme/lego/v3/registration"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
)

// Processes the "acme" job
func processJobACME(data string) error {
	// List of domains
	domains := strings.Split(data, ",")
	if len(domains) < 1 {
		return errors.New("empty domain list")
	}

	// Secrets' keys
	storePathKey := "cert/acme/" + domains[0] + ".key.pem"
	storePathCert := "cert/acme/" + domains[0] + ".cert.pem"

	// Get the email
	email := appconfig.Config.GetString("acme.email")
	if email == "" {
		return errors.New("configuration option acme.email must be set")
	}

	// Get the private key, or generate one if doesn't exist
	privateKey, err := acmePrivateKey(email)
	if err != nil {
		return err
	}

	// Client configuration
	user := ACMEUser{
		Email: email,
		key:   privateKey,
	}
	config := lego.NewConfig(&user)
	config.Certificate.KeyType = certcrypto.RSA4096
	config.CADirURL = "https://localhost:14000/dir"
	config.HTTPClient.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}

	// Client
	client, err := lego.NewClient(config)
	if err != nil {
		return err
	}

	// New users will need to register
	reg, err := acmeRegistration(email, client)
	if err != nil {
		return err
	}
	user.Registration = reg

	// Enable the HTTP-01 challenge
	err = client.Challenge.SetHTTP01Provider(&StatikoProvider{})
	if err != nil {
		return err
	}

	// Request the certificate
	// This always generates a new key, even if we're renewing the certificate
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return err
	}

	// Retrieve and store the certificate and private key
	if certificates.Certificate == nil || len(certificates.Certificate) < 1 {
		return errors.New("received an empty certificate")
	}
	if certificates.PrivateKey == nil || len(certificates.PrivateKey) < 1 {
		return errors.New("received an empty private key")
	}
	err = state.Instance.SetSecret(storePathCert, certificates.Certificate)
	if err != nil {
		return err
	}
	err = state.Instance.SetSecret(storePathKey, certificates.PrivateKey)
	if err != nil {
		return err
	}

	// Queue a sync
	sync.QueueRun()

	return nil
}

// Returns the private key for ACME
func acmePrivateKey(email string) (*ecdsa.PrivateKey, error) {
	// Check if we have a key stored
	storePath := "acme/keys/" + utils.SHA256String(email)[:10] + ".pem"
	data, err := state.Instance.GetSecret(storePath)
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
	err = state.Instance.SetSecret(storePath, data)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// Returns the registration object for ACME
func acmeRegistration(email string, client *lego.Client) (*registration.Resource, error) {
	// Check if the user has registered already
	storePath := "acme/registrations/" + utils.SHA256String(email)[:10] + ".pem"
	data, err := state.Instance.GetSecret(storePath)
	if err != nil {
		return nil, err
	}
	if data == nil || len(data) == 0 {
		// No data, register a new user
		return acmeNewRegistration(email, client)
	}

	// Decode JSON
	reg := &registration.Resource{}
	err = json.Unmarshal(data, reg)
	if err != nil {
		return nil, err
	}

	if reg == nil || reg.URI == "" || !reg.Body.TermsOfServiceAgreed {
		// Register a new user
		return acmeNewRegistration(email, client)
	}

	return reg, nil
}

// Registers a new user for ACME
func acmeNewRegistration(email string, client *lego.Client) (*registration.Resource, error) {
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
	storePath := "acme/registrations/" + utils.SHA256String(email)[:10] + ".pem"
	err = state.Instance.SetSecret(storePath, data)
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
}

// Present makes the token available
func (w *StatikoProvider) Present(domain, token, keyAuth string) error {
	// Set the token as a secret
	err := state.Instance.SetSecret("acme/challenges/"+token, []byte(keyAuth))
	if err != nil {
		return err
	}

	// Sleep 3 seconds to aid with syncing across the cluster
	time.Sleep(3 * time.Second)

	return nil
}

// CleanUp removes the key created for the challenge
func (w *StatikoProvider) CleanUp(domain, token, keyAuth string) error {
	// Delete the secret
	err := state.Instance.DeleteSecret("acme/challenges/"+token, []byte(keyAuth))
	if err != nil {
		return err
	}
	return nil
}
