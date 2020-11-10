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
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
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

var appNameRegEx *regexp.Regexp

// SanitizeAppName validates and sanitizes the name of an app's bundle
// App bundles must be lowercase strings containing letters, numbers, dashes and dots; the first character must be a letter
func SanitizeAppName(name string) string {
	if appNameRegEx == nil {
		appNameRegEx = regexp.MustCompile("^([a-z][a-zA-Z0-9\\.\\-]*)$")
	}
	name = strings.ToLower(name)
	if !appNameRegEx.MatchString(name) {
		name = ""
	}

	return name
}

// IsTruthy returns true if a string (e.g. a querystring parameter) is a truthy value, as a string
func IsTruthy(val string) bool {
	return val == "1" || val == "true" || val == "t" || val == "y" || val == "yes"
}

// StringSliceDiff returns the difference between two string slices
func StringSliceDiff(a, b []string) []string {
	var result []string

	// For this algorithm to work, the slices must be sorted
	sort.Strings(a)
	sort.Strings(b)

	// Look for elements that are different
	i := 0
	j := 0
	for i < len(a) && j < len(b) {
		c := strings.Compare(a[i], b[j])
		if c == 0 {
			i++
			j++
		} else if c < 0 {
			result = append(result, a[i])
			i++
		} else {
			result = append(result, b[j])
			j++
		}
	}

	// Append whatever is left
	result = append(result, a[i:]...)
	result = append(result, b[j:]...)

	return result
}

// GetFreePort returns the number of a port that is not in use at the time it's checked
// Note that the port might become in use as soon as this function returns if other processes are asking for it!
func GetFreePort() (int, error) {
	// Use port ":0" to ask the kernel for a port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	err = listener.Close()
	return port, err
}
