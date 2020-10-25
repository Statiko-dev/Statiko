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
	"github.com/statiko-dev/statiko/utils"
)

// Internal type used for certificates stored in the cache
type certCacheItem struct {
	Key         []byte
	Certificate []byte
}

// AgentState contains the state for the agent and methods to access it
type AgentState struct {
	logger     *log.Logger
	state      *pb.StateMessage
	updated    *time.Time
	signaler   *utils.Signaler
	certCache  map[string]certCacheItem
	siteHealth map[string]error
}

// Init the object
func (a *AgentState) Init() {
	// Initialize the logger
	a.logger = log.New(os.Stdout, "agent-state: ", log.Ldate|log.Ltime|log.LUTC)

	// Initialize the signaler
	a.signaler = &utils.Signaler{}

	// Init the certificate cache
	a.certCache = map[string]certCacheItem{}

	// Init the siteHealth map
	a.siteHealth = map[string]error{}
}

// DumpState exports the entire state
func (a *AgentState) DumpState() *pb.StateMessage {
	return a.state
}

// ReplaceState replaces the full state for the node with the provided one
// It also broadcasts the notification to all subscribers
func (a *AgentState) ReplaceState(state *pb.StateMessage) {
	// Set the new state object
	a.state = state

	// Mark the state as updated
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
func (a *AgentState) GetSites() []*pb.Site {
	if a.state == nil {
		return nil
	}
	return a.state.GetSites()
}

// GetSite returns the site object for a specific domain (including aliases)
func (a *AgentState) GetSite(domain string) *pb.Site {
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

// GetCertificate returns a certificate pair (key and certificate) from the cache
func (a *AgentState) GetCertificate(certificateId string) (key []byte, cert []byte, err error) {
	if certificateId == "" {
		return nil, nil, errors.New("certificate ID is empty")
	}

	// Get from the cache
	obj, found := a.certCache[certificateId]
	if !found {
		return nil, nil, nil
	}

	return obj.Key, obj.Certificate, nil
}

// SetCertificate adds a certificate pair (key and certificate) to the cache
func (a *AgentState) SetCertificate(certificateId string, key []byte, cert []byte) (err error) {
	if certificateId == "" {
		return errors.New("certificate ID is empty")
	}

	// If we're setting a new certificate
	if len(key) > 0 && len(cert) > 0 {
		a.certCache[certificateId] = certCacheItem{
			Key:         key,
			Certificate: cert,
		}
	} else {
		// Delete the certificate
		delete(a.certCache, certificateId)
	}

	return nil
}

// GetSiteHealth returns the health of a site
func (a *AgentState) GetSiteHealth(domain string) error {
	return a.siteHealth[domain]
}

// GetAllSitesHealth returns the health of all objects
func (a *AgentState) GetAllSitesHealth() map[string]error {
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
