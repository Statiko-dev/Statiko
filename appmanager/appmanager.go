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

package appmanager

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
	"github.com/google/renameio"

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
func (m *Manager) SyncState(sites []state.SiteState) (updated bool, err error) {
	updated = false

	// To start, ensure the basic folders exist
	err = m.InitAppRoot()
	if err != nil {
		return
	}

	// Default app (writing this to disk always, regardless)
	err = m.WriteDefaultApp()
	if err != nil {
		return
	}

	// Start with apps: ensure we have the right ones
	err = m.SyncApps(sites)
	if err != nil {
		return
	}

	// Sync site folders too
	u, err := m.SyncSiteFolders(sites)
	if err != nil {
		return
	}
	updated = updated || u

	return
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
		// If the app failed to deploy, skip this
		if s.Error != nil {
			m.log.Println("Skipping because of errors:", s.Domain)
			continue
		}

		expectFolders = append(expectFolders, s.Domain)

		// /approot/sites/{site}
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain)
		if err != nil {
			m.log.Println("Error while creating folder for site:", s.Domain, err)
			s.Error = err
			state.Instance.UpdateSite(&s, true)
			continue
		}
		updated = updated || u

		// /approot/sites/{site}/tls
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain + "/tls")
		if err != nil {
			m.log.Println("Error while creating tls folder for site:", s.Domain, err)
			s.Error = err
			state.Instance.UpdateSite(&s, true)
			continue
		}
		updated = updated || u

		// Check if the TLS certs are in place, if needed
		if s.TLSCertificate != nil {
			certVersion := ""
			if s.TLSCertificateVersion != nil {
				certVersion = *s.TLSCertificateVersion
			}

			// Call GetTLSCertificate to ensure that the certificate exists in the cache
			version, err := m.GetTLSCertificate(s.Domain, *s.TLSCertificate, certVersion)
			if err != nil {
				m.log.Println("Error while getting TLS certificate for site:", s.Domain, err)
				s.Error = err
				state.Instance.UpdateSite(&s, true)
				continue
			}
			if u || certVersion == "" || certVersion != version {
				certVersion = version
				s.TLSCertificateVersion = &version
				state.Instance.UpdateSite(&s, false)

				updated = true
			}

			// Update the TLS certificate link if necessary
			u, err = m.LinkTLSCertificate(s.Domain, *s.TLSCertificate, certVersion)
			if err != nil {
				m.log.Println("Error while linking tls certificate for site:", s.Domain, err)
				s.Error = err
				state.Instance.UpdateSite(&s, true)
				continue
			}
			updated = updated || u
		}

		// Deploy the app; do this every time, regardless, since it doesn't disrupt the running server
		// /approot/sites/{site}/www
		// www is always a symbolic link, and if there's no app deployed, it goes to the default one
		bundle := "_default"
		if s.App != nil {
			bundle = s.App.Name + "-" + s.App.Version
		}
		if err := m.ActivateApp(bundle, s.Domain); err != nil {
			m.log.Println("Error while activating app for site:", s.Domain, err)
			s.Error = err
			state.Instance.UpdateSite(&s, true)
			continue
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
					// Do not return on error
					m.log.Println("Error ignored while removing extraneous folder", m.appRoot+"sites/"+name, err)
				}
			}
		} else {
			// There shouldn't be any file; delete extraneous stuff
			updated = true
			m.log.Println("Removing extraneous file", m.appRoot+"sites/"+name)
			if err := os.Remove(m.appRoot + "sites/" + name); err != nil {
				// Do not return on error
				m.log.Println("Error ignored while removing extraneous file", m.appRoot+"sites/"+name, err)
			}
		}
	}

	return updated, nil
}

// SyncApps ensures that we have the correct apps
func (m *Manager) SyncApps(sites []state.SiteState) error {
	// Channels used by the worker pool to fetch apps in parallel
	jobs := make(chan *state.SiteState, 4)
	res := make(chan int, len(sites))

	// Spin up 3 backround workers
	for w := 1; w <= 3; w++ {
		go m.workerStageApp(w, jobs, res)
	}

	// Iterate through the sites looking for apps
	requested := 0
	for _, s := range sites {
		// Reset the error
		if s.Error != nil {
			s.Error = nil
			state.Instance.UpdateSite(&s, true)
		}

		app := s.App

		// Check if the jobs channel is full
		for len(jobs) == cap(jobs) {
			// Pause this thread until the channel is not at capacity anymore
			m.log.Println("Channel jobs is full, sleeping for a second")
			time.Sleep(time.Second)
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
			jobs <- &s
			requested++
		}
	}

	// No more jobs; close the channel
	close(jobs)

	// Iterate through all the responses
	for i := 0; i < requested; i++ {
		<-res
	}
	close(res)

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
	if err := renameio.Symlink(m.appRoot+"apps/"+app, m.appRoot+"sites/"+domain+"/www"); err != nil {
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
func (m *Manager) workerStageApp(id int, jobs <-chan *state.SiteState, res chan<- int) {
	for j := range jobs {
		m.log.Println("Worker", id, "started staging app "+j.App.Name+"-"+j.App.Version)
		err := m.StageApp(j.App.Name, j.App.Version)
		m.log.Println("Worker", id, "finished staging app "+j.App.Name+"-"+j.App.Version)

		// Handle errors
		if err != nil {
			m.log.Println("Error staging app "+j.App.Name+"-"+j.App.Version+":", err)

			// Store it in the site object
			j.Error = err
			state.Instance.UpdateSite(j, true)
		}
		res <- 1
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

// Creates a symbolic link dst pointing to src, if it doesn't exist or if it's pointing to the wrong destination
func createLinkIfNeeded(src string, dst string) (updated bool, err error) {
	err = nil
	updated = false

	// First, check if dst exists
	var exists bool
	exists, err = utils.PathExists(dst)
	if err != nil {
		return
	}

	if exists {
		// Check if the link points to the right place
		var link string
		link, err = os.Readlink(dst)
		if err != nil {
			return
		}

		if link != src {
			updated = true
		} else {
			// Nothing to do
			return
		}
	} else {
		updated = true
	}

	// If we need to create a link
	if updated {
		err = renameio.Symlink(src, dst)
		// No need to return on error; that will happen right away anyways
	}

	return
}

// LinkTLSCertificate creates a symlink to the certificate for the domain
func (m *Manager) LinkTLSCertificate(domain string, tlsCertificate string, tlsCertificateVersion string) (updated bool, err error) {
	err = nil
	updated = false

	// Destinations
	pathCert := m.appRoot + "sites/" + domain + "/tls/certificate.pem"
	pathKey := m.appRoot + "sites/" + domain + "/tls/key.pem"

	// Cached file location
	cachePathCert := m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".cert.pem"
	cachePathKey := m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".key.pem"

	// For both the certificate and key, check if the link is correct, otherwise fix it
	var u bool

	u, err = createLinkIfNeeded(cachePathCert, pathCert)
	if err != nil {
		return
	}
	updated = updated || u

	u, err = createLinkIfNeeded(cachePathKey, pathKey)
	if err != nil {
		return
	}
	updated = updated || u

	if updated {
		m.log.Println("Updated symlink for certificate and key: " + tlsCertificate)
	}

	return
}

func (m *Manager) checkCertificateInCache(tlsCertificate string, tlsCertificateVersion string) (exists bool, err error) {
	cachePathCert := m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".cert.pem"
	cachePathKey := m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".key.pem"

	exists, err = utils.PathExists(cachePathCert)
	if err != nil {
		return
	}
	exists, err = utils.PathExists(cachePathKey)
	if err != nil {
		return
	}
	return
}

// GetTLSCertificate requests a TLS certificate
func (m *Manager) GetTLSCertificate(domain string, tlsCertificate string, tlsCertificateVersion string) (string, error) {
	var cachePathCert, cachePathKey string
	var err error

	// Check if we have the files in cache
	exists := false
	if len(tlsCertificateVersion) > 0 {
		// Check if all the files exist already
		exists, err = m.checkCertificateInCache(tlsCertificate, tlsCertificateVersion)
		if err != nil {
			return tlsCertificateVersion, err
		}
	}

	// Certificate is not in cache, need to request it
	if !exists {
		// Fetch the certificate and key as PEM
		m.log.Println("Request TLS certificate from key vault: " + tlsCertificate)
		if err := m.akv.GetKeyVaultClient(); err != nil {
			return tlsCertificateVersion, err
		}
		var cert []byte
		var key []byte
		tlsCertificateVersion, cert, key, err = m.akv.GetCertificate(tlsCertificate, tlsCertificateVersion)
		if err != nil {
			return tlsCertificateVersion, err
		}

		cachePathCert = m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".cert.pem"
		cachePathKey = m.appRoot + "cache/" + tlsCertificate + "-" + tlsCertificateVersion + ".key.pem"

		// Write to cache
		if err := writeData(cert, cachePathCert); err != nil {
			return tlsCertificateVersion, err
		}
		if err := writeData(key, cachePathKey); err != nil {
			return tlsCertificateVersion, err
		}
	}

	return tlsCertificateVersion, nil
}
