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
	"os"
	"strings"
	"time"

	"github.com/google/renameio"
	"github.com/markbates/pkger"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"

	"github.com/statiko-dev/statiko/agent/certificates"
	"github.com/statiko-dev/statiko/agent/state"
	agentutils "github.com/statiko-dev/statiko/agent/utils"
	"github.com/statiko-dev/statiko/shared/fs"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
)

// Manager contains helper functions to manage apps and sites
type Manager struct {
	State        *state.AgentState
	Certificates *certificates.AgentCertificates
	Fs           fs.Fs
	ClusterOpts  *pb.ClusterOptions

	codesignKey *rsa.PublicKey
	appRoot     string
	log         *log.Logger
	manifests   map[string]*agentutils.AppManifest
}

// Init the object
func (m *Manager) Init() error {
	// Logger
	m.log = log.New(os.Stdout, "appmanager: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from config
	m.appRoot = viper.GetString("appRoot")
	if !strings.HasSuffix(m.appRoot, "/") {
		m.appRoot += "/"
	}

	// Init properties from the ClusterOpts
	m.codesignKey = m.ClusterOpts.CodesignKey()

	return nil
}

// SyncState ensures that the state of the filesystem matches the desired one
func (m *Manager) SyncState(sites []*pb.Site) (updated bool, err error) {
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
	u, err = m.SyncMiscFiles()
	if err != nil {
		return
	}
	updated = updated || u

	// Apps: ensure we have the right ones
	err = m.SyncApps(sites)
	if err != nil {
		return
	}

	// Lastly, sync the site folders
	u, err = m.SyncSiteFolders(sites)
	if err != nil {
		return
	}
	updated = updated || u

	return
}

// SyncSiteFolders ensures that we have the correct folders in the site directory, and TLS certificates are present
func (m *Manager) SyncSiteFolders(sites []*pb.Site) (bool, error) {
	updated := false

	var u bool
	var err error

	// Iterate through the sites list
	expectFolders := []string{}
	for _, s := range sites {
		// If the app failed to deploy, skip this
		if m.State.GetSiteHealth(s.Domain) != nil {
			m.log.Println("Skipping because of unhealthy site:", s.Domain)
			continue
		}

		// /approot/sites/{site}
		u, err = ensureFolderWithUpdated(m.appRoot + "sites/" + s.Domain)
		if err != nil {
			m.log.Println("Error while creating folder for site:", s.Domain, err)
			m.State.SetSiteHealth(s.Domain, err)
			continue
		}
		updated = updated || u

		// /approot/sites/{site}/tls
		pathTLS := m.appRoot + "sites/" + s.Domain + "/tls"
		u, err = ensureFolderWithUpdated(pathTLS)
		if err != nil {
			m.log.Println("Error while creating tls folder for site:", s.Domain, err)
			m.State.SetSiteHealth(s.Domain, err)
			continue
		}
		updated = updated || u

		// Get the TLS certificate
		pathKey := pathTLS + "/key.pem"
		pathCert := pathTLS + "/certificate.pem"
		// If we have an imported certificate, use that; otherwise, fallback to the generated one
		certificateId := s.GeneratedTlsId
		if s.ImportedTlsId != "" {
			certificateId = s.ImportedTlsId
		}
		keyPEM, certPEM, err := m.Certificates.GetCertificate(certificateId)
		if err != nil {
			m.log.Println("Error while getting TLS certificate for site:", s.Domain, err)
			m.State.SetSiteHealth(s.Domain, err)
			continue
		}
		u, err = writeFileIfChanged(pathKey, keyPEM)
		updated = updated || u
		u, err = writeFileIfChanged(pathCert, certPEM)
		updated = updated || u

		// Deploy the app; do this every time, regardless, since it doesn't disrupt the running server
		// /approot/sites/{site}/www
		// www is always a symbolic link, and if there's no app deployed, it goes to the default one
		bundle := "_default"
		if s.App != nil {
			bundle = s.App.Name
		}
		if err := m.ActivateApp(bundle, s.Domain); err != nil {
			m.log.Println("Error while activating app for site:", s.Domain, err)
			m.State.SetSiteHealth(s.Domain, err)
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
			// Folder name must be one of the domains
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
func (m *Manager) SyncApps(sites []*pb.Site) error {
	// Init/reset the manifest list
	m.manifests = make(map[string]*agentutils.AppManifest)

	// Channels used by the worker pool to fetch apps in parallel
	jobs := make(chan *pb.Site, 4)
	res := make(chan int, len(sites))

	// Spin up 3 backround workers
	for w := 1; w <= 3; w++ {
		go m.workerStageApp(w, jobs, res)
	}

	// Iterate through the sites looking for apps
	requested := 0
	appIndexes := make(map[string]int)
	expectApps := []string{"_default"}
	fetchAppsList := make(map[string]int)
	for i, s := range sites {
		// Reset the error
		m.State.SetSiteHealth(s.Domain, nil)

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
		exists, err := utils.PathExists(m.appRoot + "apps/" + s.App.Name)
		if err != nil {
			return err
		}
		if !exists {
			m.log.Println("Need to fetch", s.App.Name)

			// Do not fetch this app if it's already being fetched
			if _, ok := fetchAppsList[s.App.Name]; ok {
				m.log.Println("App", s.App.Name, "is already being fetched")
			} else {
				// We need to deploy the app
				// Use the worker pool to handle concurrency
				fetchAppsList[s.App.Name] = 1
				jobs <- s
				requested++
			}
		}

		// Add app to expected list and the index to the dictionary
		expectApps = append(expectApps, s.App.Name)
		appIndexes[s.App.Name] = i
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
			// Folder name must be _default or one of the apps
			if !utils.StringInSlice(expectApps, name) {
				// Delete the folder
				m.log.Println("Removing extraneous folder", m.appRoot+"apps/"+name)
				if err := os.RemoveAll(m.appRoot + "apps/" + name); err != nil {
					// Do not return on error
					m.log.Println("Error ignored while removing extraneous folder", m.appRoot+"apps/"+name, err)
				}
			}

			// Check if there's a manifest file
			manifestFile := m.appRoot + "apps/" + name + "/" + m.ClusterOpts.ManifestFile
			exists, err := utils.FileExists(manifestFile)
			if err != nil {
				return err
			}
			if exists {
				readBytes, err := ioutil.ReadFile(manifestFile)
				if err != nil {
					return err
				}
				manifest := &agentutils.AppManifest{}
				err = yaml.Unmarshal(readBytes, manifest)
				if err != nil {
					return err
				}
				manifest.Sanitize()
				m.manifests[name] = manifest
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
	bundle := "github.com/statiko-dev/statiko:/default-app/dist"
	err := pkger.Walk(bundle, func(path string, info os.FileInfo, err error) error {
		// Remove the prefix
		obj := path[len(bundle):]

		// Check type
		mode := info.Mode()
		switch {

		// Folder
		case mode.IsDir():
			// Ensure the folder exists
			err := utils.EnsureFolder(m.appRoot + "apps/_default" + obj)
			if err != nil {
				return err
			}
			return nil

		// Reguar file
		case mode.IsRegular():
			// Open the file to read
			in, err := pkger.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()

			// Write the file
			out, err := os.Create(m.appRoot + "apps/_default/" + obj)
			defer out.Close()
			if err != nil {
				return err
			}
			io.Copy(out, in)
			return nil

		// Anything else is not supported
		default:
			return errors.New("for default app, only regular files and directories are supported")
		}
	})
	if err != nil {
		return err
	}

	return nil
}

// SyncMiscFiles synchronizes the misc folder
// This contains the DH parameters for the server
func (m *Manager) SyncMiscFiles() (bool, error) {
	updated := false

	// Get the latest DH parameters and compare them with the ones on disk
	pem, _ := m.State.GetDHParams()
	u, err := writeFileIfChanged(m.appRoot+"misc/dhparams.pem", []byte(pem))
	if err != nil {
		return false, err
	}
	updated = updated || u

	return updated, nil
}

// ActivateApp points a site to an app, by creating the symbolic link
func (m *Manager) ActivateApp(app string, domain string) error {
	// Switch the www folder to an app staged
	if err := renameio.Symlink(m.appRoot+"apps/"+app, m.appRoot+"sites/"+domain+"/www"); err != nil {
		return err
	}
	return nil
}

// StageApp stages an app after unpacking the bundle
func (m *Manager) StageApp(bundle string) error {
	// Check if the app has been staged already
	stagingPath := m.appRoot + "apps/" + bundle
	exists, err := utils.PathExists(stagingPath)
	if err != nil {
		return err
	}
	if exists {
		// All done, we can just exit
		m.log.Println("App already staged: " + bundle)
		return nil
	}

	// Check if we need to download the bundle
	archivePath := m.appRoot + "cache/" + bundle
	exists, err = utils.PathExists(archivePath)
	if err != nil {
		return err
	}
	if !exists {
		// Bundle doesn't exist, so we need to download it
		m.log.Println("Fetching bundle: " + bundle)
		if err := m.FetchBundle(bundle); err != nil {
			return err
		}
	}

	// Get file type
	var fileType int
	exists, err = utils.FileExists(m.appRoot + "cache/.type." + bundle)
	if err != nil {
		return err
	}
	if exists {
		read, err := ioutil.ReadFile(m.appRoot + "cache/.type." + bundle)
		if err != nil {
			return err
		}
		if len(read) == 0 {
			return errors.New("File type object was empty")
		}
		fileType = utils.ArchiveTypeByExtension("." + string(read))
	} else {
		// Try getting the file type from the extension
		fileType = utils.ArchiveTypeByExtension(bundle)
	}

	// Uncompress the archive
	m.log.Println("Extracting " + archivePath)
	if err := utils.EnsureFolder(stagingPath); err != nil {
		return err
	}
	f, err := os.Open(archivePath)
	defer f.Close()
	if err != nil {
		return err
	}
	stat, err := f.Stat()
	if err != nil {
		return err
	}
	err = utils.ExtractArchive(stagingPath, f, stat.Size(), fileType)
	if err != nil {
		return err
	}

	// Check how many filles were extracted
	contents, err := ioutil.ReadDir(stagingPath)
	if err != nil {
		return err
	}
	if len(contents) == 0 {
		// If there's nothing in the extracted bundle, remove the folder and return an error
		if err := os.Remove(stagingPath); err != nil {
			return err
		}
		return errors.New("no files in the extracted folder")
	} else if len(contents) == 1 && contents[0].IsDir() {
		// If there's only one folder, move all files one directory up
		// First, rename to a temporary folder (app bundles can't begin with an underscore)
		// Then, delete the target folder and rename the extracted one
		if err := os.Rename(stagingPath+"/"+contents[0].Name(), m.appRoot+"apps/__"+bundle); err != nil {
			return err
		}
		if err := os.Remove(stagingPath); err != nil {
			return err
		}
		if err := os.Rename(m.appRoot+"apps/__"+bundle, stagingPath); err != nil {
			return err
		}
	}

	return nil
}

// Background worker for the StageApp function
func (m *Manager) workerStageApp(id int, jobs <-chan *pb.Site, res chan<- int) {
	for j := range jobs {
		m.log.Println("Worker", id, "started staging app "+j.App.Name)
		err := m.StageApp(j.App.Name)
		m.log.Println("Worker", id, "finished staging app "+j.App.Name)

		// Handle errors
		if err != nil {
			m.log.Println("Error staging app "+j.App.Name+":", err)

			// Store the error
			m.State.SetSiteHealth(j.Domain, err)
		}
		res <- 1
	}
}

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

// ManifestForApp returns the manifest for an app, if any
func (m *Manager) ManifestForApp(name string) *agentutils.AppManifest {
	return m.manifests[name]
}
