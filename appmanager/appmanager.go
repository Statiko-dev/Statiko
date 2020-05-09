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
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	azpipeline "github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/gobuffalo/packd"
	"github.com/gobuffalo/packr/v2"
	"github.com/google/renameio"
	"gopkg.in/yaml.v2"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/azurekeyvault"
	"github.com/statiko-dev/statiko/certificates"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
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
	log         *log.Logger
	box         *packr.Box
}

// Init the object
func (m *Manager) Init() error {
	// Logger
	m.log = log.New(os.Stdout, "appmanager: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from env vars
	m.appRoot = appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(m.appRoot, "/") {
		m.appRoot += "/"
	}

	// Azure Storage authorization
	credential, err := utils.GetAzureStorageCredentials()
	if err != nil {
		return err
	}

	// Get Azure Storage configuration
	azureStorageAccount := appconfig.Config.GetString("azure.storage.account")
	azureStorageContainer := appconfig.Config.GetString("azure.storage.appsContainer")
	azureStorageSuffix, err := utils.GetAzureStorageEndpointSuffix()
	if err != nil {
		return err
	}
	m.azureStorageURL = fmt.Sprintf("https://%s.blob.%s/%s/", azureStorageAccount, azureStorageSuffix, azureStorageContainer)

	// Azure Storage pipeline
	m.azureStoragePipeline = azblob.NewPipeline(credential, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			MaxTries: 3,
		},
	})

	// Load the code signing key
	if err := m.LoadSigningKey(); err != nil {
		return err
	}

	// Packr
	m.box = packr.New("Default app", "../default-app/dist")

	return nil
}

// SyncState ensures that the state of the filesystem matches the desired one
func (m *Manager) SyncState(sites []state.SiteState) (updated bool, restartServer bool, err error) {
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

	// Misc files
	var u bool
	u, restartServer, err = m.SyncMiscFiles()
	if err != nil {
		return
	}
	updated = updated || u

	// Apps: ensure we have the right ones
	err = m.SyncApps(sites)
	if err != nil {
		return
	}

	// Sync site folders too
	u, err = m.SyncSiteFolders(sites)
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
		if state.Instance.GetSiteHealth(s.Domain) != nil {
			m.log.Println("Skipping because of unhealthy site:", s.Domain)
			continue
		}

		// /approot/sites/{site}
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain)
		if err != nil {
			m.log.Println("Error while creating folder for site:", s.Domain, err)
			state.Instance.SetSiteHealth(s.Domain, err)
			continue
		}
		updated = updated || u

		// /approot/sites/{site}/tls
		pathTLS := m.appRoot + "sites/" + s.Domain + "/tls"
		u, err = ensureFolderWithUpdated(pathTLS)
		if err != nil {
			m.log.Println("Error while creating tls folder for site:", s.Domain, err)
			state.Instance.SetSiteHealth(s.Domain, err)
			continue
		}
		updated = updated || u

		// Get the TLS certificate
		pathKey := pathTLS + "/key.pem"
		pathCert := pathTLS + "/certificate.pem"
		keyPEM, certPEM, err := certificates.GetCertificate(&s)
		if err != nil {
			m.log.Println("Error while getting TLS certificate for site:", s.Domain, err)
			state.Instance.SetSiteHealth(s.Domain, err)
			continue
		}
		u, err = m.writeFileIfChanged(pathKey, keyPEM)
		updated = updated || u
		u, err = m.writeFileIfChanged(pathCert, certPEM)
		updated = updated || u

		// Deploy the app; do this every time, regardless, since it doesn't disrupt the running server
		// /approot/sites/{site}/www
		// www is always a symbolic link, and if there's no app deployed, it goes to the default one
		bundle := "_default"
		if s.App != nil {
			bundle = s.App.Name + "-" + s.App.Version
		}
		if err := m.ActivateApp(bundle, s.Domain); err != nil {
			m.log.Println("Error while activating app for site:", s.Domain, err)
			state.Instance.SetSiteHealth(s.Domain, err)
			continue
		}

		expectFolders = append(expectFolders, s.Domain)
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
	jobs := make(chan state.SiteState, 4)
	res := make(chan int, len(sites))

	// Spin up 3 backround workers
	for w := 1; w <= 3; w++ {
		go m.workerStageApp(w, jobs, res)
	}

	// Iterate through the sites looking for apps
	requested := 0
	appIndexes := make(map[string]int)
	expectApps := make([]string, 1)
	expectApps[0] = "_default"
	fetchAppsList := make(map[string]int)
	for i, s := range sites {
		// Reset the error
		state.Instance.SetSiteHealth(s.Domain, nil)

		// Check if the jobs channel is full
		for len(jobs) == cap(jobs) {
			// Pause this thread until the channel is not at capacity anymore
			m.log.Println("Channel jobs is full, sleeping for a second")
			time.Sleep(time.Second)
		}

		// If there's no app, skip this
		if s.App == nil {
			continue
		}

		// Check if we have the app deployed
		folderName := s.App.Name + "-" + s.App.Version
		exists, err := utils.PathExists(m.appRoot + "apps/" + folderName)
		if err != nil {
			return err
		}
		if !exists {
			m.log.Println("Need to fetch", folderName)

			// Do not fetch this app if it's already being fetched
			if _, ok := fetchAppsList[folderName]; ok {
				m.log.Println("App", folderName, "is already being fetched")
			} else {
				// We need to deploy the app
				// Use the worker pool to handle concurrency
				fetchAppsList[folderName] = 1
				jobs <- s
				requested++
			}
		}

		// Add app to expected list and the index to the dictionary
		expectApps = append(expectApps, folderName)
		appIndexes[folderName] = i
	}

	// No more jobs; close the channel
	close(jobs)

	// Iterate through all the responses
	for i := 0; i < requested; i++ {
		<-res
	}
	close(res)

	// Look for extraneous folders in the /approot/apps directory
	// Note that we are not deleting the apps' bundles from the cache, however - just the staged folder
	// We are also scanning for manifest files here
	files, err := ioutil.ReadDir(m.appRoot + "apps/")
	if err != nil {
		return err
	}
	for _, f := range files {
		name := f.Name()
		// There should only be folders
		if f.IsDir() {
			// Folder name must be _default or one of the domains
			if !utils.StringInSlice(expectApps, name) {
				// Delete the folder
				m.log.Println("Removing extraneous folder", m.appRoot+"apps/"+name)
				if err := os.RemoveAll(m.appRoot + "apps/" + name); err != nil {
					// Do not return on error
					m.log.Println("Error ignored while removing extraneous folder", m.appRoot+"apps/"+name, err)
				}
			}

			// Check if there's a manifest file
			manifestFile := m.appRoot + "apps/" + name + "/" + appconfig.Config.GetString("manifestFile")
			exists, err := utils.FileExists(manifestFile)
			if err != nil {
				return err
			}
			if exists {
				readBytes, err := ioutil.ReadFile(manifestFile)
				if err != nil {
					return err
				}
				// Get the index of the item in the slice to update
				i, ok := appIndexes[name]
				if !ok {
					return errors.New("Cannot find index for app " + name)
				}
				manifest := &utils.AppManifest{}
				err = yaml.Unmarshal(readBytes, manifest)
				sites[i].App.Manifest = manifest
				if err != nil {
					return err
				}
			}
		} else {
			// There shouldn't be any file; delete extraneous stuff
			m.log.Println("Removing extraneous file", m.appRoot+"apps/"+name)
			if err := os.Remove(m.appRoot + "apps/" + name); err != nil {
				// Do not return on error
				m.log.Println("Error ignored while removing extraneous file", m.appRoot+"apps/"+name, err)
			}
		}
	}

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

	// Create /approot/misc
	if err := utils.EnsureFolder(m.appRoot + "misc"); err != nil {
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

	// Write the default website
	err := m.box.Walk(func(path string, file packd.File) error {
		defer file.Close()
		// Ensure the folder exists
		pos := strings.LastIndex(path, "/")
		if pos > 0 {
			if err := utils.EnsureFolder(m.appRoot + "apps/_default/" + path[:pos]); err != nil {
				return err
			}
		}

		// Write the file
		f, err := os.Create(m.appRoot + "apps/_default/" + path)
		defer f.Close()
		if err != nil {
			return err
		}
		io.Copy(f, file)
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// SyncMiscFiles synchronizes the misc folder
// This contains the DH parameters and the node manager's TLS
func (m *Manager) SyncMiscFiles() (bool, bool, error) {
	updated := false
	restartServer := false

	// Get the latest DH parameters and compare them with the ones on disk
	pem, _ := state.Instance.GetDHParams()
	u, err := m.writeFileIfChanged(m.appRoot+"misc/dhparams.pem", []byte(pem))
	if err != nil {
		return false, false, err
	}
	updated = updated || u

	// The TLS certificate for the node manager
	if appconfig.Config.GetBool("tls.node.enabled") {
		var (
			err               error
			certData, keyData []byte
		)

		// Check if we have a TLS certificate
		cert := appconfig.Config.GetString("tls.node.certificate")
		key := appconfig.Config.GetString("tls.node.key")
		for {
			if cert == "" || key == "" {
				break
			}
			certData, err = ioutil.ReadFile(cert)
			if err != nil {
				if os.IsNotExist(err) {
					certData = nil
					keyData = nil
					err = nil
					break
				} else {
					return false, false, err
				}
			}
			keyData, err = ioutil.ReadFile(key)
			if err != nil {
				if os.IsNotExist(err) {
					certData = nil
					keyData = nil
					err = nil
					break
				} else {
					return false, false, err
				}
			}

			break
		}

		// If we don't have a TLS certificate, generate a self-signed one
		if certData == nil || keyData == nil {
			s := state.SiteState{
				Domain:  appconfig.Config.GetString("nodeName"),
				Aliases: []string{},
				TLS: &state.SiteTLS{
					Type: state.TLSCertificateSelfSigned,
				},
			}
			keyData, certData, err = certificates.GetCertificate(&s)
			if err != nil {
				return false, false, fmt.Errorf("error while generating self-signed certificate for node manager: %v", err)
			}
		}

		// Write the certificate and key if they're different
		u, err = m.writeFileIfChanged(m.appRoot+"misc/node.cert.pem", certData)
		if err != nil {
			return false, false, err
		}
		restartServer = restartServer || u
		updated = updated || u
		u, err = m.writeFileIfChanged(m.appRoot+"misc/node.key.pem", keyData)
		if err != nil {
			return false, false, err
		}
		restartServer = restartServer || u
		updated = updated || u
	}

	return updated, restartServer, nil
}

// Writes a file on disk if its content differ from val
// Returns true if the file has been updated
func (m *Manager) writeFileIfChanged(filename string, val []byte) (bool, error) {
	// Read the existing file
	read, err := ioutil.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	if len(read) > 0 && bytes.Compare(read, val) == 0 {
		// Nothing to do here
		return false, nil
	}

	// Write the updated file
	err = ioutil.WriteFile(filename, val, 0644)
	if err != nil {
		return false, err
	}

	// File has been updated
	return true, nil
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
	keyName := appconfig.Config.GetString("azure.keyVault.codesignKey.name")
	keyVersion := appconfig.Config.GetString("azure.keyVault.codesignKey.version")

	// Request the key from Azure Key Vault
	m.log.Printf("Requesting code signing key %s (%s)\n", keyName, keyVersion)
	keyVersion, pubKey, err := azurekeyvault.GetInstance().GetPublicKey(keyName, keyVersion)
	if err != nil {
		return err
	}
	m.log.Println("Received code signing key with version", keyVersion)
	m.codeSignKey = pubKey

	// Check if we have a new key version
	if keyVersion != "" && keyVersion != "latest" {
		appconfig.Config.Set("azure.keyVault.codesignKey.version", keyVersion)
	}

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
func (m *Manager) workerStageApp(id int, jobs <-chan state.SiteState, res chan<- int) {
	for j := range jobs {
		m.log.Println("Worker", id, "started staging app "+j.App.Name+"-"+j.App.Version)
		err := m.StageApp(j.App.Name, j.App.Version)
		m.log.Println("Worker", id, "finished staging app "+j.App.Name+"-"+j.App.Version)

		// Handle errors
		if err != nil {
			m.log.Println("Error staging app "+j.App.Name+"-"+j.App.Version+":", err)

			// Store the error
			state.Instance.SetSiteHealth(j.Domain, err)
		}
		res <- 1
	}
}

// FetchBundle downloads the application's tar.bz2 bundle for a specific version
func (m *Manager) FetchBundle(bundle string, version string) error {
	archiveName := bundle + "-" + version + ".tar.bz2"

	// Get the archive
	u, err := url.Parse(m.azureStorageURL + archiveName)
	if err != nil {
		return err
	}
	blobURL := azblob.NewBlobURL(*u, m.azureStoragePipeline)
	resp, err := blobURL.Download(context.TODO(), 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false)
	if err != nil {
		if stgErr, ok := err.(azblob.StorageError); !ok {
			err = fmt.Errorf("Network error while downloading the archive: %s", err.Error())
		} else {
			err = fmt.Errorf("Azure Storage error while downloading the archive: %s", stgErr.Response().Status)
		}
		return err
	}
	body := resp.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
	defer body.Close()

	// Get the signature from the blob's metadata, if any
	var signature []byte
	metadata := resp.NewMetadata()
	if metadata != nil {
		signatureB64, ok := metadata["signature"]
		if ok && signatureB64 != "" {
			// Signature should be 512-byte long (+1 null terminator). If it's longer, Go will throw an error anyways (out of range)
			signature = make([]byte, 513)
			len, err := base64.StdEncoding.Decode(signature, []byte(signatureB64))
			if err != nil {
				return err
			}
			if len != 512 {
				return errors.New("Invalid signature length")
			}
		}
	}
	if signature == nil && appconfig.Config.GetBool("disallowUnsignedApps") {
		return errors.New("Bundle does not have a signature, but unsigned apps are not allowed by this node's configuration")
	}

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

	// Verify the digital signature if present
	if signature != nil {
		// (Need to grab the first 512 bytes from the signature only)
		err = rsa.VerifyPKCS1v15(m.codeSignKey, crypto.SHA256, hashed, signature[:512])
		if err != nil {
			m.log.Printf("Signature mismatch for bundle %s-%s\n", bundle, version)

			// File needs to be deleted if signature is invalid
			deleteFile = true

			return err
		}
	} else {
		m.log.Printf("WARN Bundle %s-%s did not contain a signature; skipping integrity and origin check\n", bundle, version)
	}

	return nil
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
