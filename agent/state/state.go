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

package state

import (
	"errors"
	"log"
	"os"
	"time"

	"github.com/statiko-dev/statiko/shared/defaults"
	pb "github.com/statiko-dev/statiko/shared/proto"
	common "github.com/statiko-dev/statiko/shared/state"
	"github.com/statiko-dev/statiko/utils"
)

// AgentState contains the state for the agent and methods to access it
type AgentState struct {
	common.StateCommon

	logger     *log.Logger
	state      *pb.State
	updated    *time.Time
	signaler   *utils.Signaler
	siteHealth map[string]error
}

// Init the object
func (a *AgentState) Init() {
	// Initialize the logger
	a.logger = log.New(os.Stdout, "state: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the signaler
	a.signaler = &utils.Signaler{}

	// Init the siteHealth map
	a.siteHealth = make(map[string]error)
}

// DumpState exports the entire state
func (a *AgentState) DumpState() *pb.State {
	return a.state
}

// ReplaceState replaces the full state for the node with the provided one
func (a *AgentState) ReplaceState(state *pb.State) {
	// Set the new state object
	a.state = state

	// Mark the state as updated and broadcast the updated message
	a.setUpdated()
}

// setUpdated sets the updated time in the object and broadcasts the message
func (a *AgentState) setUpdated() {
	now := time.Now()
	a.updated = &now

	// Broadcast the update
	a.signaler.Broadcast()
}

// LastUpdated returns the time the state was updated last
func (a *AgentState) LastUpdated() *time.Time {
	return a.updated
}

// Subscribe will add a channel as a subscriber to when new state is availalble
func (a *AgentState) Subscribe(ch chan int) {
	a.signaler.Subscribe(ch)
}

// Unsubscribe removes a channel from the list of subscribers to the state
func (a *AgentState) Unsubscribe(ch chan int) {
	a.signaler.Unsubscribe(ch)
}

// GetSites returns the list of all sites
func (a *AgentState) GetSites() []*pb.State_Site {
	if a.state == nil {
		return nil
	}
	return a.state.GetSites()
}

// GetSite returns the site object for a specific domain (including aliases)
func (a *AgentState) GetSite(domain string) *pb.State_Site {
	if a.state == nil {
		return nil
	}
	return a.state.GetSite(domain)
}

// GetDHParams returns the PEM-encoded DH parameters and their date
func (a *AgentState) GetDHParams() (string, *time.Time) {
	// Check if we DH parameters; if not, return the default ones
	if a.state != nil && (a.state.DhParams == nil || a.state.DhParams.Date == 0 || a.state.DhParams.Pem == "") {
		return defaults.DefaultDHParams, nil
	}

	// Return the saved DH parameters
	date := time.Unix(a.state.DhParams.Date, 0)
	return a.state.DhParams.Pem, &date
}

// GetSecret returns the value for a secret (encrypted in the state)
func (a *AgentState) GetSecret(key string) ([]byte, error) {
	// Check if we have a secret for this key
	if a.state == nil {
		return nil, errors.New("state not loaded")
	}
	if a.state.Secrets == nil {
		a.state.Secrets = make(map[string][]byte)
	}
	encValue, found := a.state.Secrets[key]
	if !found || encValue == nil || len(encValue) < 12 {
		return nil, nil
	}

	// Decrypt the secret
	return a.DecryptSecret(encValue)
}

// GetCertificate returns a certificate pair (key and certificate) stored as secrets, PEM-encoded
func (a *AgentState) GetCertificate(typ pb.State_Site_TLS_Type, nameOrDomains []string) (key []byte, cert []byte, err error) {
	// Key of the secret
	secretKey := a.CertificateSecretKey(typ, nameOrDomains)
	if secretKey == "" {
		return nil, nil, errors.New("invalid name or domains")
	}

	// Retrieve the secret
	serialized, err := a.GetSecret(secretKey)
	if err != nil || serialized == nil || len(serialized) < 8 {
		return nil, nil, err
	}

	// Un-serialize the secret
	return a.UnserializeCertificate(serialized)
}

// ListImportedCertificates returns a list of the names of all imported certificates
func (a *AgentState) ListImportedCertificates() (res []string) {
	if a.state == nil {
		return []string{}
	}
	return a.ListImportedCertificates_Internal(a.state.Secrets)
}

// GetSiteHealth returns the health of a site
func (a *AgentState) GetSiteHealth(domain string) error {
	return a.siteHealth[domain]
}

// GetAllSiteHealth returns the health of all objects
func (a *AgentState) GetAllSiteHealth() map[string]error {
	// Deep-clone the object
	r := make(map[string]error, len(a.siteHealth))
	for k, v := range a.siteHealth {
		r[k] = v
	}
	return r
}

// SetSiteHealth sets the health of a site
func (a *AgentState) SetSiteHealth(domain string, err error) {
	a.siteHealth[domain] = err
}
