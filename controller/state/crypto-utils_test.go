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
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestCrypto(t *testing.T) {
	tests := []string{
		strings.Repeat("a", 20), // > 1 block
		strings.Repeat("a", 16), // Exactly 1 block
		strings.Repeat("a", 6),  // < 1 block
		strings.Repeat("a", 32), // Exactly 2 blocks
		"hello world",
		"", // Empty
	}

	// Set the key
	key := []byte(strings.Repeat("a", 22) + "==")
	viper.Set("secretsEncryptionKey", key)

	for i := 0; i < len(tests); i++ {
		msg := tests[i]

		// Encrypt
		ciphertext, err := encryptData([]byte(msg))
		assert.NoError(t, err)
		assert.NotEmpty(t, ciphertext)

		// Decrypt
		plaintext, err := decryptData(ciphertext)
		assert.NoError(t, err)
		assert.Equal(t, msg, string(plaintext))
	}
}
