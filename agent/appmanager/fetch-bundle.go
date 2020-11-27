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

package appmanager

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// FetchBundle downloads the application's bundle
func (m *Manager) FetchBundle(bundle string) error {
	// Get the archive
	found, data, metadata, err := m.Fs.Get(context.Background(), bundle)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("bundle not found in store")
	}
	defer data.Close()

	var hash []byte
	var signature []byte
	fileType := ""
	if metadata != nil && len(metadata) > 0 {
		// Get the hash from the blob's metadata, if any
		hashB64, ok := metadata["hash"]
		if ok && hashB64 != "" {
			hash, err = base64.StdEncoding.DecodeString(hashB64)
			if err != nil {
				return err
			}
			if len(hash) != 32 {
				hash = nil
			}
		}

		// Get the signature from the blob's metadata, if any
		// Skip if we don't have a codesign key
		if m.codesignKey != nil {
			signatureB64, ok := metadata["signature"]
			if ok && signatureB64 != "" {
				signature, err = base64.StdEncoding.DecodeString(signatureB64)
				if err != nil {
					return err
				}
				if len(signature) != 512 {
					signature = nil
				}
			}
		}

		// Check if there's a file type in the metadata
		typ, ok := metadata["type"]
		if ok && typ != "" {
			fileType = typ
		}
	}
	if signature == nil && m.ClusterOpts.Codesign.RequireCodesign {
		return errors.New("Bundle does not have a signature, but unsigned apps are not allowed by this node's configuration")
	}

	// The stream is split between two readers: one for the hashing, one for writing the stream to disk
	h := sha256.New()
	tee := io.TeeReader(data, h)

	// Write to disk (this also makes the stream proceed so the hash is calculated)
	out, err := os.Create(m.appRoot + "cache/" + bundle)
	if err != nil {
		return err
	}

	// The deferred function will delete the file if the signature is invalid
	deleteFile := false
	defer func(deleteFile *bool) {
		out.Close()

		if *deleteFile {
			m.log.Println("Deleting bundle " + bundle)
			os.Remove(m.appRoot + "cache/" + bundle)
		}
	}(&deleteFile)

	// Write stream to disk
	_, err = io.Copy(out, tee)
	if err != nil {
		return err
	}

	// Calculate the SHA256 hash
	hashed := h.Sum(nil)
	m.log.Printf("SHA256 checksum for bundle %s is %x\n", bundle, hashed)

	// Verify the hash and digital signature if present
	if hash == nil && signature == nil {
		m.log.Printf("[Warn] Bundle %s did not contain a signature; skipping integrity and origin check\n", bundle)
	}
	if hash != nil {
		if bytes.Compare(hash, hashed) != 0 {
			// File needs to be deleted if hash is invalid
			deleteFile = true
			m.log.Println("Hash mismatch for bundle", bundle)
			return fmt.Errorf("hash does not match: got %x, wanted %x", hashed, hash)
		}
	}
	if signature != nil {
		err = rsa.VerifyPKCS1v15(m.codesignKey, crypto.SHA256, hashed, signature)
		if err != nil {
			// File needs to be deleted if signature is invalid
			deleteFile = true
			m.log.Println("Signature mismatch for bundle", bundle)
			return err
		}
	}

	// Write the file type to disk
	if fileType != "" {
		err = ioutil.WriteFile(m.appRoot+"cache/.type."+bundle, []byte(fileType), 0644)
		if err != nil {
			// File needs to be deleted if we had an error
			deleteFile = true
			return err
		}
	}

	return nil
}
