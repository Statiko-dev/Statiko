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
	"encoding/binary"
	"errors"
	"io"
	"strings"

	"github.com/statiko-dev/statiko/appconfig"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// StateCommon is a base class used by both controller/state.Manager and agent/state.AgentState
type StateCommon struct{}

// CertificateSecretKey returns the key of secret for the certificate
func (s *StateCommon) CertificateSecretKey(typ pb.State_Site_TLS_Type, nameOrDomains []string) string {
	switch typ {
	case pb.State_Site_TLS_IMPORTED:
		if len(nameOrDomains) != 1 || len(nameOrDomains[0]) == 0 {
			return ""
		}
		return "cert/" + pb.State_Site_TLS_IMPORTED.String() + "/" + nameOrDomains[0]
	case pb.State_Site_TLS_ACME, pb.State_Site_TLS_SELF_SIGNED:
		domainKey := utils.SHA256String(strings.Join(nameOrDomains, ","))[:15]
		return "cert/" + typ.String() + "/" + domainKey
	default:
		return ""
	}
}

// GetSecretsCipher returns a cipher for AES-GCM-128 initialized
func (s *StateCommon) GetSecretsCipher() (cipher.AEAD, error) {
	// Get the symmetric encryption key
	encKey, err := s.GetSecretsEncryptionKey()
	if err != nil {
		return nil, err
	}

	// Init the AES-GCM cipher
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aesgcm, nil
}

// GetSecretsEncryptionKey returns the value of the secrets symmetric encryption key from the configuration file
func (s *StateCommon) GetSecretsEncryptionKey() ([]byte, error) {
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

// ListImportedCertificates_Internal returns a list of the names of all imported certificates (used internally)
func (s *StateCommon) ListImportedCertificates_Internal(secrets map[string][]byte) (res []string) {
	res = make([]string, 0)
	// Iterate through all secrets looking for those starting with "cert/IMPORTED/"
	for k := range secrets {
		if strings.HasPrefix(k, "cert/"+pb.State_Site_TLS_IMPORTED.String()+"/") {
			res = append(res, strings.TrimPrefix(k, "cert/"+pb.State_Site_TLS_IMPORTED.String()+"/"))
		}
	}
	return
}

// DecryptSecret decrypts a secret (used internally)
func (s *StateCommon) DecryptSecret(encValue []byte) (value []byte, err error) {
	// Get the cipher
	aesgcm, err := s.GetSecretsCipher()
	if err != nil {
		return nil, err
	}

	// Decrypt the secret
	// First 12 bytes of the value are the nonce
	return aesgcm.Open(nil, encValue[0:12], encValue[12:], nil)
}

// EncryptSecret encrypts a secret (used internally)
func (s *StateCommon) EncryptSecret(value []byte) (encValue []byte, err error) {
	// Get the cipher
	aesgcm, err := s.GetSecretsCipher()
	if err != nil {
		return nil, err
	}

	// Get a nonce
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the secret
	encrypted := aesgcm.Seal(nil, nonce, value, nil)

	// Prepend the nonce to the secret
	encValue = append(nonce, encrypted...)

	return encValue, nil
}

// UnserializeCertificate unserializes a TLS certificate (decrypted)
func (s *StateCommon) UnserializeCertificate(serialized []byte) (key []byte, cert []byte, err error) {
	keyLen := binary.LittleEndian.Uint32(serialized[0:4])
	certLen := binary.LittleEndian.Uint32(serialized[4:8])
	if keyLen < 1 || certLen < 1 || len(serialized) != int(8+keyLen+certLen) {
		return nil, nil, errors.New("invalid serialized data")
	}

	key = serialized[8:(keyLen + 8)]
	cert = serialized[(keyLen + 8):]
	err = nil
	return
}

// SerializeCertificate serializes a TLS certificate (decrypted)
func (s *StateCommon) SerializeCertificate(key []byte, cert []byte) ([]byte, error) {
	if len(key) > 204800 || len(cert) > 204800 {
		return nil, errors.New("key and/or certificate are too long")
	}
	keyLen := make([]byte, 4)
	certLen := make([]byte, 4)
	binary.LittleEndian.PutUint32(keyLen, uint32(len(key)))
	binary.LittleEndian.PutUint32(certLen, uint32(len(cert)))
	serialized := bytes.Buffer{}
	serialized.Write(keyLen)
	serialized.Write(certLen)
	serialized.Write(key)
	serialized.Write(cert)
	return serialized.Bytes(), nil
}
