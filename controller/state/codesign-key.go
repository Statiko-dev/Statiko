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
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"io/ioutil"
	"strings"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/shared/utils"
)

// GetCodesignKey returns the codesign key as a rsa.PublicKey object
func (m *Manager) GetCodesignKey() *rsa.PublicKey {
	return m.codesignKey
}

// LoadCodesignKey loads the codesign key
func (m *Manager) LoadCodesignKey() bool {
	// Check if we have a key, then parse it
	pemKey := viper.GetString("codesign.publicKey")
	if pemKey == "" {
		return false
	}

	// Check if the key is the path to a file
	if !strings.HasPrefix(pemKey, "-----BEGIN") {
		exists, err := utils.FileExists(pemKey)
		if err != nil || !exists {
			m.logger.Println("codesign file referenced doesn't exist")
			return false
		}

		// Read the file
		read, err := ioutil.ReadFile(pemKey)
		if err != nil || read == nil || len(read) < 1 {
			m.logger.Println("coudl not load codesign file")
			return false
		}
		pemKey = string(read)
	}

	// Load the PEM key
	block, _ := pem.Decode([]byte(pemKey))
	if block == nil || len(block.Bytes) == 0 {
		m.logger.Println("could not PEM block")
		return false
	}

	switch block.Type {
	case "RSA PUBLIC KEY":
		// PKCS#1
		var err error
		m.codesignKey, err = x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil || m.codesignKey == nil {
			m.codesignKey = nil
			m.logger.Println("could not parse PKCS#1 certificate")
			return false
		}
	case "PUBLIC KEY":
		// PKIX
		pub, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil || pub == nil {
			m.logger.Println("could not parse PKIX certificate")
			return false
		}
		var ok bool
		m.codesignKey, ok = pub.(*rsa.PublicKey)
		if !ok {
			m.codesignKey = nil
			m.logger.Println("could not get RSA public key from certificate")
			return false
		}
	default:
		m.logger.Println("invalid type in PEM block")
		return false
	}

	m.logger.Println("Loaded code signing key")
	return true
}
