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
	"io/ioutil"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	agentutils "github.com/statiko-dev/statiko/agent/utils"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

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
