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

package appmanager2

import (
	"bytes"
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
	"net/url"
	"os"
	"time"

	azpipeline "github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gobuffalo/packr/v2"

	"smplatform/appconfig"
	"smplatform/azurekeyvault"
	"smplatform/state"
	"smplatform/utils"
)

// Manager contains helper functions to manage apps and sites
type Manager struct {
	// Root folder for the platform
	appRoot string

	// Azure Storage client
	azureStoragePipeline azpipeline.Pipeline
	azureStorageURL      string

	// Internals
	codeSignKey *rsa.PublicKey
	akv         *azurekeyvault.Certificate
	log         *log.Logger
	box         *packr.Box
	ctx         context.Context
}

// Init the object
func (m *Manager) Init() error {
	// Logger
	m.log = log.New(os.Stdout, "webapp-manager: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from env vars
	m.appRoot = appconfig.Config.GetString("appRoot")

	// Get Azure Storage configuration
	azureStorageAccount := appconfig.Config.GetString("azureStorage.account")
	azureStorageKey := appconfig.Config.GetString("azureStorage.key")
	azureStorageContainer := appconfig.Config.GetString("azureStorage.container")
	m.azureStorageURL = fmt.Sprintf("https://%s.blob.core.windows.net/%s/", azureStorageAccount, azureStorageContainer)

	// Azure Storage pipeline
	m.ctx = context.Background()
	credential, err := azblob.NewSharedKeyCredential(azureStorageAccount, azureStorageKey)
	if err != nil {
		return err
	}
	m.azureStoragePipeline = azblob.NewPipeline(credential, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			MaxTries: 3,
		},
	})

	// Initialize the Azure Key Vault client
	m.akv = &azurekeyvault.Certificate{
		Ctx:       m.ctx,
		VaultName: appconfig.Config.GetString("azureKeyVault.name"),
	}
	if err := m.akv.Init(); err != nil {
		return err
	}

	// Load the code signing key
	if err := m.LoadSigningKey(); err != nil {
		return err
	}

	// Packr
	m.box = packr.New("Default app", "default-app")

	return nil
}

// SyncState ensures that the state of the filesystem matches the desired one
func (m *Manager) SyncState(sites []state.SiteState) (bool, error) {
	updated := false

	// To start, ensure the basic folders exist
	if err := m.InitAppRoot(); err != nil {
		return false, err
	}

	// Default app (writing this to disk always, regardless)
	if err := m.WriteDefaultApp(); err != nil {
		return false, err
	}

	// Start with apps: ensure we have the right ones
	if err := m.SyncApps(sites); err != nil {
		return false, err
	}

	// Sync site folders too
	u, err := m.SyncSiteFolders(sites)
	if err != nil {
		return false, err
	}
	updated = updated || u

	return updated, nil
}

// Creates a folder if it doesn't exist already
func ensureFolderWithUpdated(path string) (updated bool, err error) {
	updated = false
	exists := false
	exists, err = utils.FolderExists(path)
	if err != nil {
		return
	}
	if !exists {
		err = utils.EnsureFolder(path)
		if err != nil {
			return
		}
		updated = true
	}
	return
}

// SyncSiteFolders ensures that we have the correct folders in the site directory, and TLS certificates are present
func (m *Manager) SyncSiteFolders(sites []state.SiteState) (bool, error) {
	updated := false

	var u bool
	var err error

	// Folder for the default site
	// /approot/sites/_default
	u, err = ensureFolderWithUpdated(m.appRoot + "sites/_default")
	if err != nil {
		return false, err
	}
	updated = updated || u

	// Activate the default site
	// /approot/sites/_default/www
	if err := m.ActivateApp("_default", "_default"); err != nil {
		return false, err
	}

	// Iterate through the sites list
	expectFolders := make([]string, 1)
	expectFolders[0] = "_default"
	for _, s := range sites {
		expectFolders = append(expectFolders, s.Domain)

		// /approot/sites/{site}
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain)
		if err != nil {
			return false, err
		}
		updated = updated || u

		// /approot/sites/{site}/tls
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain + "/tls")
		if err != nil {
			return false, err
		}
		updated = updated || u

		// Check if the TLS certs are in place, if needed
		if s.TLSCertificate != nil {
			needTLS := false
			// If we just created the folder, the certs definitely don't exist yet
			if u {
				needTLS = true
			} else {
				var e bool

				e, err = utils.PathExists(m.appRoot + "sites/" + s.Domain + "/tls/certificate.pem")
				if err != nil {
					return false, err
				}
				needTLS = needTLS || !e

				e, err = utils.PathExists(m.appRoot + "sites/" + s.Domain + "/tls/key.pem")
				if err != nil {
					return false, err
				}
				needTLS = needTLS || !e

				e, err = utils.PathExists(m.appRoot + "sites/" + s.Domain + "/tls/dhparams.pem")
				if err != nil {
					return false, err
				}
				needTLS = needTLS || !e
			}

			// Fetch the TLS certificates if we need to
			if needTLS {
				if err := m.GetTLSCertificate(s.Domain, *s.TLSCertificate); err != nil {
					return false, err
				}
				updated = true
			}

			// Deploy the app; do this any time, regardless, since it doesn't disrupt the running server
			// /approot/sites/{site}/www
			// www is always a symbolic link, and if there's no app deployed, it goes to the default one
			bundle := "_default"
			if s.App != nil {
				bundle = s.App.Name + "-" + s.App.Version
			}
			if err := m.ActivateApp(bundle, s.Domain); err != nil {
				return false, err
			}
		}
	}

	// Look for extraneous folders in the /approot/sites directory
	files, err := ioutil.ReadDir(m.appRoot + "sites/")
	if err != nil {
		return false, err
	}
	for _, f := range files {
		name := f.Name()
		// There should only be folders
		if f.IsDir() {
			// Folder name must be _default or one of the domains
			if !utils.StringInSlice(expectFolders, name) {
				// Delete the folder
				updated = true
				m.log.Println("Removing extraneous folder", m.appRoot+"sites/"+name)
				if err := os.RemoveAll(m.appRoot + "sites/" + name); err != nil {
					return false, err
				}
			}
		} else {
			// There shouldn't be any file; delete extraneous stuff
			updated = true
			m.log.Println("Removing extraneous file", m.appRoot+"sites/"+name)
			if err := os.Remove(m.appRoot + "sites/" + name); err != nil {
				return false, err
			}
		}
	}

	return updated, nil
}

// SyncApps ensures that we have the correct apps
func (m *Manager) SyncApps(sites []state.SiteState) error {
	// Channels used by the worker pool to fetch apps in parallel
	jobs := make(chan *state.SiteApp, 4)
	errs := make(chan error, 4)

	// Spin up 3 backround workers
	for w := 1; w <= 3; w++ {
		go m.workerStageApp(w, jobs, errs)
	}

	// Iterate through the sites looking for apps
	requested := 0
	for _, s := range sites {
		app := s.App

		// Check if the jobs channel is full
		for len(jobs) == cap(jobs) {
			// Pause this thread until the channel is not at capacity anymore
			m.log.Println("Channel jobs is full, sleeping for a second")
			time.Sleep(time.Second)
		}

		// Check if the err channel is full
		if len(errs) == cap(errs) {
			// Process all errors, if any
			m.log.Println("Channel errs is full, processing errors")
			for i := 0; i < requested; i++ {
				e := <-errs
				if e != nil {
					return e
				}
			}

			// We've processed all errors
			requested = 0
		}

		// If there's no app, skip this
		if app == nil {
			continue
		}

		// Check if we have the app deployed
		exists, err := utils.PathExists(m.appRoot + "apps/" + app.Name + "-" + app.Version)
		if err != nil {
			return err
		}
		if !exists {
			m.log.Println("Need to fetch ", app.Name, app.Version)
			// We need to deploy the app
			// Use the worker pool to handle concurrency
			jobs <- app
			requested++
		}
	}

	// No more jobs; close the channel
	close(jobs)

	// Iterate through all the errors, if any
	for i := 0; i < requested; i++ {
		e := <-errs
		if e != nil {
			return e
		}
	}
	close(errs)

	return nil
}

// InitAppRoot creates a new, empty app root folder
func (m *Manager) InitAppRoot() error {
	// Ensure the app root folder exists
	if err := utils.EnsureFolder(m.appRoot); err != nil {
		return err
	}

	// Create /approot/cache
	if err := utils.EnsureFolder(m.appRoot + "cache"); err != nil {
		return err
	}

	// Create /approot/apps
	if err := utils.EnsureFolder(m.appRoot + "apps"); err != nil {
		return err
	}

	// Create /approot/sites
	if err := utils.EnsureFolder(m.appRoot + "sites"); err != nil {
		return err
	}

	return nil
}

// WriteDefaultApp creates the files for the default app on disk
func (m *Manager) WriteDefaultApp() error {
	// Ensure /approot/apps/_default exists
	if err := utils.EnsureFolder(m.appRoot + "apps/_default"); err != nil {
		return err
	}

	// Reset the folder
	if err := utils.RemoveContents(m.appRoot + "apps/_default/"); err != nil {
		return err
	}

	// Write the default website page
	welcome, err := m.box.Find("smplatform-welcome.html")
	if err != nil {
		return err
	}
	f, err := os.Create(m.appRoot + "apps/_default/smplatform-welcome.html")
	defer f.Close()
	if err != nil {
		return err
	}
	f.Write(welcome)

	return nil
}

// ActivateApp points a site to an app, by creating the symbolic link
func (m *Manager) ActivateApp(app string, domain string) error {
	// Switch the www folder to an app staged
	if err := utils.SymlinkAtomic(m.appRoot+"apps/"+app, m.appRoot+"sites/"+domain+"/www"); err != nil {
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
	exists, err := utils.PathExists(stagingPath)
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
	exists, err = utils.PathExists(archivePath)
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
	if err := utils.EnsureFolder(stagingPath); err != nil {
		return err
	}
	reader, err := os.Open(archivePath)
	defer reader.Close()
	if err != nil {
		return err
	}
	if err := utils.UntarBZ2(stagingPath, reader); err != nil {
		return err
	}

	return nil
}

// Background worker for the StageApp function
func (m *Manager) workerStageApp(id int, jobs <-chan *state.SiteApp, errs chan<- error) {
	for j := range jobs {
		m.log.Println("Worker", id, "started staging app "+j.Name+"-"+j.Version)
		err := m.StageApp(j.Name, j.Version)
		m.log.Println("Worker", id, "finished staging app "+j.Name+"-"+j.Version)
		errs <- err
	}
}

// FetchBundle downloads the application's tar.bz2 bundle for a specific version
func (m *Manager) FetchBundle(bundle string, version string) error {
	archiveName := bundle + "-" + version + ".tar.bz2"

	// Get the signature
	u, err := url.Parse(m.azureStorageURL + archiveName + ".sig")
	if err != nil {
		return err
	}
	blobURL := azblob.NewBlobURL(*u, m.azureStoragePipeline)
	resp, err := blobURL.Download(m.ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return err
	}

	// Signature is encoded as base64
	signatureB64 := &bytes.Buffer{}
	reader := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
	_, err = signatureB64.ReadFrom(reader)
	defer reader.Close()
	if err != nil {
		return err
	}

	// Hash should be 512-byte long (+1 null terminator). If it's longer, Go will throw an error anyways (out of range)
	signature := make([]byte, 513)
	len, err := base64.StdEncoding.Decode(signature, signatureB64.Bytes())
	if err != nil {
		return err
	}
	if len != 512 {
		return errors.New("Invalid signature length")
	}

	// Get the archive
	u, err = url.Parse(m.azureStorageURL + archiveName)
	if err != nil {
		return err
	}
	blobURL = azblob.NewBlobURL(*u, m.azureStoragePipeline)
	resp, err = blobURL.Download(m.ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		return err
	}
	body := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})

	// The stream is split between two readers: one for the hashing, one for writing the stream to disk
	h := sha256.New()
	tee := io.TeeReader(body, h)

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

// GetTLSCertificate requests the TLS certificate for a site
func (m *Manager) GetTLSCertificate(domain string, tlsCertificate string) error {
	// Check if we have the file in cache
	cachePathCert := m.appRoot + "cache/" + tlsCertificate + ".cert.pem"
	cachePathKey := m.appRoot + "cache/" + tlsCertificate + ".key.pem"
	cachePathDhparams := m.appRoot + "cache/" + tlsCertificate + ".dhparams.pem"
	existsCert, err := utils.PathExists(cachePathCert)
	if err != nil {
		return err
	}
	existsKey, err := utils.PathExists(cachePathKey)
	if err != nil {
		return err
	}
	existsDhparams, err := utils.PathExists(cachePathDhparams)
	if err != nil {
		return err
	}

	// Destinations
	pathCert := m.appRoot + "sites/" + domain + "/tls/certificate.pem"
	pathKey := m.appRoot + "sites/" + domain + "/tls/key.pem"
	pathDhparams := m.appRoot + "sites/" + domain + "/tls/dhparams.pem"

	if existsCert && existsKey && existsDhparams {
		// Load certificate from cache
		m.log.Println("Loading TLS certificate from cache: " + tlsCertificate)
		err := utils.CopyFile(cachePathCert, pathCert)
		if err != nil {
			return err
		}
		err = utils.CopyFile(cachePathKey, pathKey)
		if err != nil {
			return err
		}
		err = utils.CopyFile(cachePathDhparams, pathDhparams)
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
		u, err := url.Parse(m.azureStorageURL + "dhparams/" + tlsCertificate + ".pem")
		if err != nil {
			return err
		}
		blobURL := azblob.NewBlobURL(*u, m.azureStoragePipeline)
		resp, err := blobURL.Download(m.ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
		if err != nil {
			return err
		}
		body := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
		dhparams, err := ioutil.ReadAll(body)
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
