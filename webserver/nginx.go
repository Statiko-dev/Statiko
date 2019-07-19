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

package webserver

import (
	"bytes"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"smplatform/appconfig"
	"smplatform/state"
	"smplatform/utils"

	"github.com/gobuffalo/packr/v2"
)

// List of template files
var templateFiles = [4]string{"nginx.conf", "mime.types", "site.conf", "default-site.conf"}

// ConfigData is a map of each configuration file and its content
type ConfigData map[string][]byte

// NginxConfig creates the configuration for nginx
type NginxConfig struct {
	logger    *log.Logger
	templates map[string]*template.Template
}

// Init initializes the object and loads the templates from file
func (n *NginxConfig) Init() error {
	// Logger
	n.logger = log.New(os.Stdout, "nginx: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the templates
	n.templates = make(map[string]*template.Template, len(templateFiles))
	if err := n.loadTemplates(); err != nil {
		return err
	}

	return nil
}

// DesiredConfiguration builds the list of files for the desired configuration for nginx
func (n *NginxConfig) DesiredConfiguration(sites []state.SiteState) (config ConfigData, err error) {
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

	// Default website configuration
	config["conf.d/_default.conf"], err = n.createConfigurationFile("default-site.conf", nil)
	if err != nil {
		return
	}
	if config["conf.d/_default.conf"] == nil {
		err = errors.New("Invalid configuration generated for file conf.d/_default.conf")
		return
	}

	// Configuration for each site
	for _, s := range sites {
		// If the site/app failed to deploy, skip this
		if s.Error != nil {
			n.logger.Println("Skipping site with error", s.Domain)
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
func (n *NginxConfig) ExistingConfiguration(sites []state.SiteState) (ConfigData, bool, error) {
	nginxConfPath := appconfig.Config.GetString("nginx.configPath")
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
	existing["conf.d/_default.conf"] = nil
	for _, s := range sites {
		// If the site/app failed to deploy, skip this
		if s.Error != nil {
			n.logger.Println("Skipping site with error", s.Domain)
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
func (n *NginxConfig) SyncConfiguration(sites []state.SiteState) (bool, error) {
	nginxConfPath := appconfig.Config.GetString("nginx.configPath")
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
		// Check if the file exists already
		existingVal, ok := existing[key]
		if ok && existingVal != nil {
			// Compare the files
			if bytes.Compare(val, existingVal) != 0 {
				// Files are different
				updated = true
				n.logger.Println("Replacing configuration file", nginxConfPath+key)
				if err := writeConfigFile(nginxConfPath+key, val); err != nil {
					return false, err
				}
			}
		} else {
			// File is missing
			updated = true
			n.logger.Println("Creating configuration file", nginxConfPath+key)
			if err := writeConfigFile(nginxConfPath+key, val); err != nil {
				return false, err
			}
		}
	}

	return updated, nil
}

// RestartServer restarts the Nginx server
func (n *NginxConfig) RestartServer() error {
	n.logger.Println("Restarting server")
	_, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.restart")).Output()
	if err != nil {
		n.logger.Panicf("Error while restarting server: %s\n", err)
		return err
	}

	return nil
}

// Read all templates
func (n *NginxConfig) loadTemplates() error {
	// Packr
	box := packr.New("Nginx templates", "nginx-template")

	// Functions to add to the template
	funcMap := template.FuncMap{
		// Joins all values of a slice with a string separator
		"joinList": func(slice []string, separator string) string {
			return strings.Join(slice, separator)
		},
	}

	// Read all templates from the list
	for i := 0; i < len(templateFiles); i++ {
		str, err := box.FindString(templateFiles[i])
		if err != nil {
			return err
		}
		tpl, err := template.New(templateFiles[i]).Funcs(funcMap).Parse(str)
		if err != nil {
			return err
		}
		n.templates[templateFiles[i]] = tpl
	}

	return nil
}

// Create a configuration file
func (n *NginxConfig) createConfigurationFile(templateName string, itemData interface{}) ([]byte, error) {
	protocol := "http"
	if appconfig.Config.GetBool("tls.enabled") {
		protocol = "https"
	}

	// Get parameters
	tplData := struct {
		Item     interface{}
		AppRoot  string
		Port     string
		Protocol string
		TLS      struct {
			Dhparams string
			Node     struct {
				Enabled     bool
				Certificate string
				Key         string
			}
		}
	}{
		Item:     itemData,
		AppRoot:  appconfig.Config.GetString("appRoot"),
		Port:     appconfig.Config.GetString("port"),
		Protocol: protocol,
		TLS: struct {
			Dhparams string
			Node     struct {
				Enabled     bool
				Certificate string
				Key         string
			}
		}{
			Dhparams: appconfig.Config.GetString("tls.dhparams"),
			Node: struct {
				Enabled     bool
				Certificate string
				Key         string
			}{
				Enabled:     appconfig.Config.GetBool("tls.node.enabled"),
				Certificate: appconfig.Config.GetString("tls.node.certificate"),
				Key:         appconfig.Config.GetString("tls.node.key"),
			},
		},
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
