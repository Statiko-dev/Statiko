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
	"errors"
	"io"
	"io/ioutil"
	"os"

	"github.com/markbates/pkger"

	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

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
