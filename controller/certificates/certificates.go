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
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/certutils"
)

// Certificates is the class that manages TLS certificates
type Certificates struct {
	State          stateObj
	ACMETokenReady ACMETokenReadyFunc
	AKV            *azurekeyvault.Client
}

// Init the object
func (c *Certificates) Init() error {
	return nil
}

// GetCertificate returns a certificate key and cert by ID
func (c *Certificates) GetCertificate(certificateId string) (key []byte, cert []byte, err error) {
	// Return from the certutils package
	return certutils.GetCertificate(certificateId, c.State, c.AKV)
}
