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
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"math/big"
	"math/rand"
	"net/http"

	"github.com/statiko-dev/statiko/appconfig"
)

// RequestJSON fetches a JSON document from the web
func RequestJSON(client *http.Client, url string, target interface{}) error {
	var err error

	// Request the file
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 399 {
		b, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(b))
	}

	// Decode the JSON into the target
	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		return err
	}
	return nil
}

// StringInSlice checks if a string is contained inside a slice of strings
func StringInSlice(list []string, a string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

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

// Letters used for random string generation
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandString returns a random string of n bytes
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}
	return string(b)
}

// SHA256String returns the SHA256 of a string, as a hex-encoded string
func SHA256String(str string) string {
	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:])
}

// SerializeECDSAKey serializes an ecdsa private key
// Source https://stackoverflow.com/a/41315404/192024
func SerializeECDSAKey(privateKey *ecdsa.PrivateKey) ([]byte, error) {
	x509Encoded, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, err
	}
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	return pemEncoded, nil
}

// UnserializeECDSAKey unserializes an ecdsa private key
// Source https://stackoverflow.com/a/41315404/192024
func UnserializeECDSAKey(pemEncoded []byte) (*ecdsa.PrivateKey, error) {
	// Private
	block, _ := pem.Decode(pemEncoded)
	if block == nil {
		return nil, errors.New("invalid private key block")
	}
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

// NodeAddress returns the address of the node
func NodeAddress() string {
	// Address of the node; fallback to the node name if empty
	address := appconfig.Config.GetString("tls.node.address")
	if address == "" {
		address = appconfig.Config.GetString("nodeName")
	}
	return address
}
