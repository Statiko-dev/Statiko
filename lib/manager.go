/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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

package lib

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"smplatform/appconfig"
)

// Manager contains helper functions to manage apps and sites
type Manager struct {
	// Root folder for the platform
	appRoot string

	// Nginx configuration directory
	nginxConfPath string

	// Azure Storage account and SAS token
	azureStorageBaseURL  string
	azureStorageSASToken string

	// Internals
	httpClient  *http.Client
	codeSignKey *rsa.PublicKey
	akv         *AzureKeyVaultCertificate
	log         *log.Logger
}

// Init the object
func (m *Manager) Init() error {
	// Logger
	m.log = log.New(os.Stdout, "webapp-manager: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from env vars
	m.appRoot = appconfig.Config.GetString("appRoot")
	m.nginxConfPath = appconfig.Config.GetString("nginx.configPath")

	// Get Azure Storage configuration
	m.azureStorageBaseURL = fmt.Sprintf("https://%s.blob.core.windows.net/%s/",
		appconfig.Config.GetString("azureStorage.account"),
		appconfig.Config.GetString("azureStorage.container"))
	m.azureStorageSASToken = appconfig.Config.GetString("azureStorage.sasToken")

	// HTTP client with a timeout
	m.httpClient = &http.Client{Timeout: 30 * time.Second}

	// Initialize the Azure Key Vault client
	ctx := context.Background()
	m.akv = &AzureKeyVaultCertificate{
		Ctx:       ctx,
		VaultName: appconfig.Config.GetString("azureKeyVault.name"),
	}
	if err := m.akv.Init(); err != nil {
		return err
	}

	// Load the code signing key
	if err := m.LoadSigningKey(); err != nil {
		return err
	}

	return nil
}

// InitAppRoot creates a new, empty app root folder
func (m *Manager) InitAppRoot() error {
	// Ensure the app root folder exists
	if err := EnsureFolder(m.appRoot); err != nil {
		return err
	}

	// If folder isn't empty, clean it
	if err := RemoveContents(m.appRoot); err != nil {
		return err
	}

	// Create /approot/cache
	if err := EnsureFolder(m.appRoot + "cache"); err != nil {
		return err
	}

	// Create /approot/apps
	if err := EnsureFolder(m.appRoot + "apps"); err != nil {
		return err
	}

	// Create /approot/apps/_default
	if err := EnsureFolder(m.appRoot + "apps/_default"); err != nil {
		return err
	}

	// Create /approot/sites
	if err := EnsureFolder(m.appRoot + "sites"); err != nil {
		return err
	}

	return nil
}

// CreateFolders creates the required folders in the file system
func (m *Manager) CreateFolders(site string) error {
	// /approot/sites/{site}
	if err := EnsureFolder(m.appRoot + "sites/" + site); err != nil {
		return err
	}

	// /approot/sites/{site}/tls
	if err := EnsureFolder(m.appRoot + "sites/" + site + "/tls"); err != nil {
		return err
	}

	// Clean the tls directory
	if err := RemoveContents(m.appRoot + "sites/" + site + "/tls"); err != nil {
		return err
	}

	// /approot/sites/{site}/www
	// www is always a symbolic link, to the default app and then switched to app staged
	if err := m.ActivateApp("_default", site); err != nil {
		return err
	}

	return nil
}

// ActivateApp points a site to an app, by creating the symbolic link
func (m *Manager) ActivateApp(app string, site string) error {
	// Switch the www folder to an app staged
	if err := SymlinkAtomic(m.appRoot+"apps/"+app, m.appRoot+"sites/"+site+"/www"); err != nil {
		return err
	}
	return nil
}

// RemoveFolders deletes the folders for the site
func (m *Manager) RemoveFolders(site string) error {
	// /approot/sites/{site}
	if err := os.RemoveAll(m.appRoot + "sites/" + site); err != nil {
		return err
	}

	return nil
}

// LoadSigningKey loads the code signing public key
func (m *Manager) LoadSigningKey() error {
	// Load certificate from cache
	m.log.Println("Loading code signing key at " + appconfig.Config.GetString("codesignKey"))
	dataPEM, err := ioutil.ReadFile(appconfig.Config.GetString("codesignKey"))
	if err != nil {
		return err
	}

	// Parse the key
	block, _ := pem.Decode(dataPEM)
	if block == nil || block.Type != "PUBLIC KEY" {
		return errors.New("Cannot decode PEM block containing public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	m.codeSignKey = pub.(*rsa.PublicKey)

	return nil
}

// StageApp stages an app after unpacking the bundle
func (m *Manager) StageApp(app string, version string) error {
	// Check if the app has been staged already
	stagingPath := m.appRoot + "apps/" + app + "-" + version
	exists, err := PathExists(stagingPath)
	if err != nil {
		return err
	}
	if exists {
		// All done, we can just exit
		m.log.Println("App already staged: " + app + "-" + version)
		return nil
	}

	// Check if we need to download the bundle
	archivePath := m.appRoot + "cache/" + app + "-" + version + ".tar.bz2"
	exists, err = PathExists(archivePath)
	if err != nil {
		return err
	}
	if !exists {
		// Bundle doesn't exist, so we need to download it
		m.log.Println("Fetching bundle: " + app + "-" + version)
		if err := m.FetchBundle(app, version); err != nil {
			return err
		}
	}

	// Uncompress the archive
	m.log.Println("Extracting " + archivePath)
	if err := EnsureFolder(stagingPath); err != nil {
		return err
	}
	reader, err := os.Open(archivePath)
	defer reader.Close()
	if err != nil {
		return err
	}
	if err := UntarBZ2(stagingPath, reader); err != nil {
		return err
	}

	return nil
}

// FetchBundle downloads the application's tar.bz2 bundle for a specific version
func (m *Manager) FetchBundle(bundle string, version string) error {
	archiveName := bundle + "-" + version + ".tar.bz2"

	// Get the signature
	URL := m.azureStorageBaseURL + archiveName + ".sig" + m.azureStorageSASToken
	resp, err := m.httpClient.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 399 {
		b, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(b))
	}
	// Signature is encoded as base64
	signatureB64, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// Hash should be 512-byte long (+1 null terminator). If it's longer, Go will throw an error anyways (out of range)
	signature := make([]byte, 513)
	len, err := base64.StdEncoding.Decode(signature, signatureB64)
	if err != nil {
		return err
	}
	if len != 512 {
		return errors.New("Invalid signature length")
	}

	// Get the archive
	URL = m.azureStorageBaseURL + archiveName + m.azureStorageSASToken
	resp, err = m.httpClient.Get(URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 399 {
		b, _ := ioutil.ReadAll(resp.Body)
		return errors.New(string(b))
	}

	// The stream is split between two readers: one for the hashing, one for writing the stream to disk
	h := sha256.New()
	tee := io.TeeReader(resp.Body, h)

	// Write to disk (this also makes the stream proceed so the hash is calculated)
	out, err := os.Create(m.appRoot + "cache/" + archiveName)
	if err != nil {
		return err
	}

	// The deferred function will delete the file if the signature is invalid
	deleteFile := false
	defer func(deleteFile *bool) {
		out.Close()

		if *deleteFile {
			m.log.Println("Deleting archive " + archiveName)
			os.Remove(m.appRoot + "cache/" + archiveName)
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

	// Verify the digital signature
	// (Need to grab the first 512 bytes from the signature only)
	err = rsa.VerifyPKCS1v15(m.codeSignKey, crypto.SHA256, hashed, signature[:512])
	if err != nil {
		m.log.Printf("Signature mismatch for bundle %s-%s\n", bundle, version)

		// File needs to be deleted if signature is invalid
		deleteFile = true

		return err
	}

	return nil
}

// Write a byte array to disk
func writeData(data []byte, path string) error {
	f, err := os.Create(path)
	defer f.Close()

	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}

	return f.Close()
}

// GetTLSCertificate requests the TLS certificate for an app
func (m *Manager) GetTLSCertificate(site string, tlsCertificate string) error {
	// Check if we have the file in cache
	cachePathCert := m.appRoot + "cache/" + tlsCertificate + ".cert.pem"
	cachePathKey := m.appRoot + "cache/" + tlsCertificate + ".key.pem"
	cachePathDhparams := m.appRoot + "cache/" + tlsCertificate + ".dhparams.pem"
	existsCert, err := PathExists(cachePathCert)
	if err != nil {
		return err
	}
	existsKey, err := PathExists(cachePathKey)
	if err != nil {
		return err
	}
	existsDhparams, err := PathExists(cachePathDhparams)
	if err != nil {
		return err
	}

	// Destinations
	pathCert := m.appRoot + "sites/" + site + "/tls/certificate.pem"
	pathKey := m.appRoot + "sites/" + site + "/tls/key.pem"
	pathDhparams := m.appRoot + "sites/" + site + "/tls/dhparams.pem"

	if existsCert && existsKey && existsDhparams {
		// Load certificate from cache
		m.log.Println("Loading TLS certificate from cache: " + tlsCertificate)
		err := CopyFile(cachePathCert, pathCert)
		if err != nil {
			return err
		}
		err = CopyFile(cachePathKey, pathKey)
		if err != nil {
			return err
		}
		err = CopyFile(cachePathDhparams, pathDhparams)
		if err != nil {
			return err
		}
	} else {
		// Fetch the certificate and key as PEM
		m.log.Println("Request TLS certificate from key vault: " + tlsCertificate)
		if err := m.akv.GetKeyVaultClient(); err != nil {
			return err
		}
		cert, key, err := m.akv.GetCertificate(tlsCertificate)
		if err != nil {
			return err
		}

		// Write to file
		if err := writeData(cert, pathCert); err != nil {
			return err
		}
		if err := writeData(cert, cachePathCert); err != nil {
			return err
		}
		if err := writeData(key, pathKey); err != nil {
			return err
		}
		if err := writeData(key, cachePathKey); err != nil {
			return err
		}

		// Obtain the dhparams file, which is on the storage account
		// We pre-generated this as it can take a very long time, and it needs to be the same in every server
		// Request the file
		m.log.Println("Request dhparams file from object storage: " + tlsCertificate)
		resp, err := m.httpClient.Get(m.azureStorageBaseURL + "dhparams/" + tlsCertificate + ".pem" + m.azureStorageSASToken)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 399 {
			b, _ := ioutil.ReadAll(resp.Body)
			return errors.New(string(b))
		}
		dhparams, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if err := writeData(dhparams, pathDhparams); err != nil {
			return err
		}
		if err := writeData(dhparams, cachePathDhparams); err != nil {
			return err
		}
	}

	return nil
}
