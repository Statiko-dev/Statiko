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

package webserver

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"github.com/markbates/pkger"
	"github.com/spf13/viper"

	agentutils "github.com/statiko-dev/statiko/agent/utils"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
)

// DesiredConfiguration builds the list of files for the desired configuration for nginx
func (n *NginxConfig) DesiredConfiguration(sites []*pb.Site) (config ConfigData, err error) {
	config = make(ConfigData)

	// Basic webserver configuration
	config["nginx.conf"], err = n.createConfigurationFile("nginx.conf", nil)
	if err != nil {
		return
	}
	if config["nginx.conf"] == nil {
		err = errors.New("Invalid configuration generated for file nginx.conf")
		return
	}

	config["mime.types"], err = n.createConfigurationFile("mime.types", nil)
	if err != nil {
		return
	}
	if config["mime.types"] == nil {
		err = errors.New("Invalid configuration generated for file mime.types")
		return
	}

	// Configuration for each site
	for _, s := range sites {
		// If the site/app failed to deploy, skip this
		if n.State.GetSiteHealth(s.Domain) != nil {
			n.logger.Println("Skipping site with error (in DesiredConfiguration)", s.Domain)
			continue
		}

		key := "conf.d/" + s.Domain + ".conf"
		var val []byte
		val, err = n.createConfigurationFile("site.conf", s)
		if err != nil {
			return
		}
		if val == nil {
			err = errors.New("Invalid configuration generated for file " + key)
			return
		}
		config[key] = val
	}

	return
}

// ExistingConfiguration reads the list of files currently on disk, and deletes some extraneous ones already
func (n *NginxConfig) ExistingConfiguration(sites []*pb.Site) (ConfigData, bool, error) {
	nginxConfPath := viper.GetString("nginx.configPath")
	existing := make(ConfigData)
	updated := false

	// Start with the nginxConfigPath directory
	files, err := ioutil.ReadDir(nginxConfPath)
	if err != nil {
		return nil, false, err
	}
	for _, f := range files {
		name := f.Name()
		// There should be only 1 directory: conf.d
		if f.IsDir() {
			if name != "conf.d" {
				// Delete the folder
				updated = true
				n.logger.Println("Removing extraneous folder", nginxConfPath+name)
				if err := os.RemoveAll(nginxConfPath + name); err != nil {
					return nil, false, err
				}
			}
		} else {
			// There should only be two files: nginx.conf and mime.type
			if name == "nginx.conf" || name == "mime.types" {
				var err error
				existing[name], err = ioutil.ReadFile(nginxConfPath + name)
				if err != nil {
					return nil, false, err
				}
			} else {
				// Delete the extraneous file
				updated = true
				n.logger.Println("Removing extraneous file", nginxConfPath+name)
				if err := os.Remove(nginxConfPath + name); err != nil {
					return nil, false, err
				}
			}
		}
	}

	// List of files we expect in the conf.d directory
	for _, s := range sites {
		// If the site/app failed to deploy, skip this
		if n.State.GetSiteHealth(s.Domain) != nil {
			n.logger.Println("Skipping site with error (in ExistingConfiguration)", s.Domain)
			continue
		}

		existing["conf.d/"+s.Domain+".conf"] = nil
	}

	// Scan the conf.d directory
	files, err = ioutil.ReadDir(nginxConfPath + "conf.d")
	if err != nil {
		return nil, false, err
	}
	for _, f := range files {
		// There shouldn't be any directory
		if f.IsDir() {
			// Delete the folder
			updated = true
			name := f.Name()
			n.logger.Println("Removing extraneous folder", nginxConfPath+"conf.d/"+name)
			if err := os.RemoveAll(nginxConfPath + "conf.d/" + name); err != nil {
				return nil, false, err
			}
		} else {
			// Expect certain files only
			key := "conf.d/" + f.Name()
			_, ok := existing[key]
			if ok {
				// We are expecting this file: read it
				var err error
				existing[key], err = ioutil.ReadFile(nginxConfPath + key)
				if err != nil {
					return nil, false, err
				}
			} else {
				// Delete the extraneous file
				updated = true
				n.logger.Println("Removing extraneous file", nginxConfPath+key)
				if err := os.Remove(nginxConfPath + key); err != nil {
					return nil, false, err
				}
			}
		}
	}

	return existing, updated, nil
}

// SyncConfiguration ensures that the configuration for the webserver matches the desired state
func (n *NginxConfig) SyncConfiguration(sites []*pb.Site) (bool, error) {
	nginxConfPath := viper.GetString("nginx.configPath")
	updated := false

	// Generate the desired configuration
	desired, err := n.DesiredConfiguration(sites)
	if err != nil {
		return false, err
	}

	// Ensure that the required folders exist
	if err := utils.EnsureFolder(nginxConfPath); err != nil {
		return false, err
	}
	if err := utils.EnsureFolder(nginxConfPath + "conf.d"); err != nil {
		return false, err
	}

	// Scan the existing content
	existing, u, err := n.ExistingConfiguration(sites)
	updated = updated || u
	if err != nil {
		return false, err
	}

	// Iterate through the desired state looking for missing keys and different files
	// We're guaranteed that the existing state does not contain any extraenous file already
	for key, val := range desired {
		written := false
		// Check if the file exists already
		existingVal, ok := existing[key]
		if ok && existingVal != nil {
			// Compare the files
			if bytes.Compare(val, existingVal) != 0 {
				// Files are different
				updated = true
				written = true
				n.logger.Println("Replacing configuration file", nginxConfPath+key)
				if err := writeConfigFile(nginxConfPath+key, val); err != nil {
					return false, err
				}
			}
		} else {
			// File is missing
			updated = true
			written = true
			n.logger.Println("Creating configuration file", nginxConfPath+key)
			if err := writeConfigFile(nginxConfPath+key, val); err != nil {
				return false, err
			}
		}

		// If we wrote a config file for a site, test the nginx configuration to see if there's any error
		if written && strings.HasPrefix(key, "conf.d/") {
			configOk, err := n.ConfigTest()
			if err != nil {
				return false, err
			}
			if !configOk {
				n.logger.Println("Error in configuration file", nginxConfPath+key, " - removing it")
				site := key[7:(len(key) - 5)]

				// Add an error to the site's object
				for i := 0; i < len(sites); i++ {
					if sites[i].Domain == site {
						n.State.SetSiteHealth(sites[i].Domain, errors.New("invalid nginx configuration - check manifest"))
						break
					}
				}

				// Remove this site
				if err := os.Remove(nginxConfPath + key); err != nil {
					return false, err
				}
			}
		}
	}

	return updated, nil
}

// Read all templates
func (n *NginxConfig) loadTemplates() error {
	// Functions to add to the template
	funcMap := template.FuncMap{
		// Joins all values of a slice with a string separator
		"joinList": func(slice []string, separator string) string {
			return strings.Join(slice, separator)
		},
	}

	// Read all templates from the list
	for i := 0; i < len(templateFiles); i++ {
		f, err := pkger.Open("github.com/statiko-dev/statiko/agent:/webserver/nginx-template/" + templateFiles[i])
		if err != nil {
			return err
		}
		read, err := ioutil.ReadAll(f)
		f.Close()
		if err != nil {
			return err
		}
		tpl, err := template.New(templateFiles[i]).Funcs(funcMap).Parse(string(read))
		if err != nil {
			return err
		}
		n.templates[templateFiles[i]] = tpl
	}

	return nil
}

// Create a configuration file
func (n *NginxConfig) createConfigurationFile(templateName string, itemData *pb.Site) ([]byte, error) {
	// Ensure these aren't nil
	if itemData == nil {
		itemData = &pb.Site{}
	}
	if itemData.App == nil {
		itemData.App = &pb.Site_App{}
	}

	// App root
	appRoot := viper.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	// Get the app's manifest, if any
	manifest := n.AppManager.ManifestForApp(itemData.App.Name)
	// Ensure the object isn't nil
	if manifest == nil {
		manifest = &agentutils.AppManifest{}
	}

	// Get parameters
	tplData := struct {
		Item         *pb.Site
		Manifest     *agentutils.AppManifest
		AppRoot      string
		Port         string
		ManifestFile string
		User         string
		Dhparams     string
	}{
		Item:         itemData,
		Manifest:     manifest,
		AppRoot:      appRoot,
		Port:         viper.GetString("serverPort"),
		ManifestFile: n.ClusterOpts.ManifestFile,
		User:         viper.GetString("nginx.user"),
		Dhparams:     appRoot + "misc/dhparams.pem",
	}

	// Get the template
	tpl := n.templates[templateName]

	// Execute the template
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, tplData); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Writes data to a configuration file
func writeConfigFile(path string, val []byte) error {
	// Running f.Close() manually to avoid having too many open file descriptors
	f, err := os.Create(path)
	defer f.Close()
	if err != nil {
		return err
	}
	if err := f.Chmod(0644); err != nil {
		return err
	}
	_, err = f.Write(val)
	if err != nil {
		return err
	}
	return nil
}
