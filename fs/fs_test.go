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

package fs

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"log"
	"os"
	"testing"
)

var (
	dir      string
	obj      Fs
	metadata map[string]string
)

const testFileName = "../.e2e-test/fixtures/simone-mascellari-h1SSRcFuHMk-unsplash.jpg"
const testFileDigest = "cae18d3ba38520dbd850f24b19739651a57e2a8bda4199f039b870173463c420"

// TestMain initializes all tests for this package
func TestMain(m *testing.M) {
	// Temp dir
	var err error
	dir, err = ioutil.TempDir("", "statikotest")
	if err != nil {
		log.Fatal(err)
	}

	// Metadata
	metadata = make(map[string]string)
	metadata["foo"] = "bar"
	metadata["hello"] = "world"

	// Run tests
	rc := m.Run()

	// Cleanup
	err = os.RemoveAll(dir)
	if err != nil {
		log.Fatal(err)
	}

	// Exit
	os.Exit(rc)
}

func openTestFile() io.ReadCloser {
	// Stream to test file
	in, err := os.Open(testFileName)
	if err != nil {
		log.Fatal(err)
	}
	return in
}

func calculateDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
