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
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"

	"github.com/statiko-dev/statiko/appconfig"
)

// Encrypts a byte slice with AES-CBC
func encryptData(plaintext []byte) (out []byte, err error) {
	// Get the cipher
	block, err := getSecretsCipher()
	if err != nil {
		return nil, err
	}

	// Get a random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Pad the plaintext
	plaintext = pkcs7pad(plaintext)

	// Encrypt the data and place the IV at the beginning
	encrypter := cipher.NewCBCEncrypter(block, iv)
	out = make([]byte, len(plaintext)+aes.BlockSize)
	copy(out[0:aes.BlockSize], iv)
	encrypter.CryptBlocks(out[aes.BlockSize:], plaintext)

	return out, nil
}

// Decrypts a byte slice with AES-CBC
func decryptData(ciphertext []byte) (out []byte, err error) {
	// Ensure the message is longer than 32 bytes (first 16 bytes are IV, and then there's the ciphertext)
	// Also, the length must be a multiple of 16 bytes
	if len(ciphertext) < (aes.BlockSize*2) || (len(ciphertext)%aes.BlockSize) != 0 {
		return nil, errors.New("ciphertext's length is invalid")
	}

	// Get the cipher
	block, err := getSecretsCipher()
	if err != nil {
		return nil, err
	}

	// Decrypt the data
	// The first 16 bytes are the IV
	decrypter := cipher.NewCBCDecrypter(block, ciphertext[0:aes.BlockSize])
	out = make([]byte, len(ciphertext)-aes.BlockSize)
	decrypter.CryptBlocks(out, ciphertext[aes.BlockSize:])

	// Remove the padding
	out = pkcs7trim(out)
	return out, nil
}

// Padds a message using PKCS#7
func pkcs7pad(message []byte) []byte {
	padding := aes.BlockSize - (len(message) % aes.BlockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(message, padtext...)
}

// Removes the padding from a message using PKCS#7
func pkcs7trim(message []byte) []byte {
	padding := message[len(message)-1]
	return message[:len(message)-int(padding)]
}

// Returns a block cipher for AES-CBC-128 initialized
func getSecretsCipher() (cipher.Block, error) {
	// Get the symmetric encryption key
	encKey, err := getSecretsEncryptionKey()
	if err != nil {
		return nil, err
	}

	// Init the AES cipher
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}

	return block, nil
}

// Returns the value of the secrets symmetric encryption key from the configuration file
func getSecretsEncryptionKey() ([]byte, error) {
	// Get the key
	encKeyB64 := appconfig.Config.GetString("secretsEncryptionKey")
	if len(encKeyB64) != 24 {
		return nil, errors.New("empty or invalid 'secretsEncryptionKey' value in configuration file")
	}

	// Decode base64
	encKey, err := base64.StdEncoding.DecodeString(encKeyB64)
	if err != nil {
		return nil, err
	}
	if len(encKey) != 16 {
		return nil, errors.New("invalid length of 'secretsEncryptionKey'")
	}

	return encKey, nil
}
