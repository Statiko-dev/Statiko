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
	"log"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"smplatform/appconfig"
	"smplatform/db"
	"smplatform/utils"

	"github.com/gobuffalo/packr/v2"
	"github.com/gofrs/uuid"
)

/* Constants */

var templateFiles = [4]string{"nginx.conf", "mime.types", "site.conf", "default-site.conf"}

/* NginxConfig struct */

// NginxConfig creates the configuration for nginx
type NginxConfig struct {
	appRoot       string
	nginxConfPath string
	logger        *log.Logger
	templates     map[string]*template.Template
}

// Init initializes the object and loads the templates from file
func (n *NginxConfig) Init() error {
	// Logger
	n.logger = log.New(os.Stdout, "nginx: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from env vars
	n.appRoot = appconfig.Config.GetString("appRoot")
	n.nginxConfPath = appconfig.Config.GetString("nginx.configPath")

	// Load the templates
	n.templates = make(map[string]*template.Template, len(templateFiles))
	if err := n.loadTemplates(); err != nil {
		return err
	}

	return nil
}

// ResetConfiguration resets the nginx configuration so there's only the default website
func (n *NginxConfig) ResetConfiguration() error {
	// Ensure the folder exists
	if err := utils.EnsureFolder(n.nginxConfPath); err != nil {
		return err
	}

	// Clear the contents of the folder
	if err := utils.RemoveContents(n.nginxConfPath); err != nil {
		return err
	}

	// Basic webserver configuration
	if err := n.writeConfigurationFile("nginx.conf", "nginx.conf", nil); err != nil {
		return err
	}

	if err := n.writeConfigurationFile("mime.types", "mime.types", nil); err != nil {
		return err
	}

	// Create the conf.d folder for websites
	if err := utils.EnsureFolder(n.nginxConfPath + "conf.d"); err != nil {
		return err
	}

	// Default website configuration
	if err := n.writeConfigurationFile("conf.d/_default.conf", "default-site.conf", nil); err != nil {
		return err
	}

	return nil
}

// SyncConfiguration ensures that the configuration for the webserver matches the desired state
func (n *NginxConfig) SyncConfiguration(sites []db.Site) error {
	// First, reset the configuration
	err := n.ResetConfiguration()
	if err != nil {
		return err
	}

	// Create the configuration file for each website
	for _, site := range sites {
		err = n.ConfigureSite(&site)
		if err != nil {
			return err
		}
	}

	return nil
}

// ConfigureSite creates the configuration for a website
func (n *NginxConfig) ConfigureSite(site *db.Site) error {
	if err := n.writeConfigurationFile("conf.d/"+site.SiteID.String()+".conf", "site.conf", site); err != nil {
		return err
	}

	return nil
}

// RemoveSite removes the configuration for a website
func (n *NginxConfig) RemoveSite(siteName string) error {
	if err := os.Remove(n.nginxConfPath + "conf.d/" + siteName + ".conf"); err != nil {
		return err
	}

	return nil
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

		// Converts UUIDs to strings
		"uuidToString": func(u uuid.UUID) string {
			return u.String()
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

// Write a configuration to disk
func (n *NginxConfig) writeConfigurationFile(path string, templateName string, itemData interface{}) error {
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
			Enabled     bool
			Certificate string
			Key         string
			Dhparams    string
		}
	}{
		Item:     itemData,
		AppRoot:  n.appRoot,
		Port:     appconfig.Config.GetString("port"),
		Protocol: protocol,
		TLS: struct {
			Enabled     bool
			Certificate string
			Key         string
			Dhparams    string
		}{
			Enabled:     appconfig.Config.GetBool("tls.enabled"),
			Certificate: appconfig.Config.GetString("tls.certificate"),
			Key:         appconfig.Config.GetString("tls.key"),
			Dhparams:    appconfig.Config.GetString("tls.dhparams"),
		},
	}

	// Get the template
	tpl := n.templates[templateName]

	// Create the file and execute the template
	f, err := os.Create(n.nginxConfPath + path)
	if err != nil {
		return err
	}
	if err := tpl.Execute(f, tplData); err != nil {
		return err
	}
	f.Close()

	return nil
}
