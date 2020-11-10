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

package utils

import (
	"crypto/rsa"
	"encoding/base64"
	"math/big"
)

// ParseRSAPublicKey converts a public RSA key represented by base64-encoded modulus and exponent into a rsa.PublicKey object
func ParseRSAPublicKey(nStr string, eStr string) (*rsa.PublicKey, error) {
	pubKey := &rsa.PublicKey{}

	// Modulus
	nData, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, err
	}
	n := &big.Int{}
	n.SetBytes(nData)
	pubKey.N = n

	// Public exponent
	eData, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, err
	}
	e := big.Int{}
	e.SetBytes(eData)
	pubKey.E = int(e.Int64())

	return pubKey, nil
}
