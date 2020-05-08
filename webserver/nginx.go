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
	"log"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"

	"github.com/gobuffalo/packr/v2"
)

// List of template files
var templateFiles = [4]string{"nginx.conf", "mime.types", "site.conf", "default-site.conf"}

// ConfigData is a map of each configuration file and its content
type ConfigData map[string][]byte

// NginxConfig creates the configuration for nginx
type NginxConfig struct {
	logger              *log.Logger
	templates           map[string]*template.Template
	clientCachingRegexp *regexp.Regexp
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

	// Compile the regular expression for matching ClientCaching values in apps' manifests
	n.clientCachingRegexp = regexp.MustCompile(`^[1-9][0-9]*(ms|s|m|h|d|w|M|y)$`)

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
		if state.Instance.GetSiteHealth(s.Domain) != nil {
			n.logger.Println("Skipping site with error (in DesiredConfiguration)", s.Domain)
			continue
		}

		key := "conf.d/" + s.Domain + ".conf"
		var val []byte
		val, err = n.createConfigurationFile("site.conf", &s)
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
		if state.Instance.GetSiteHealth(s.Domain) != nil {
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
		if written && strings.HasPrefix(key, "conf.d/") && key != "conf.d/_default.conf" {
			configOk, err := n.ConfigTest()
			if err != nil {
				return false, err
			}
			if !configOk {
				n.logger.Println("Error in configuration file", nginxConfPath+key, " - removing it")
				site := key[7:(len(key) - 5)]

				// Add an error to the site's object
				for _, v := range sites {
					if v.Domain == site {
						state.Instance.SetSiteHealth(v.Domain, errors.New("invalid nginx configuration - check manifest"))
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

// Status returns the status of the Nginx server
func (n *NginxConfig) Status() (bool, error) {
	result, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.status")).Output()
	if err != nil {
		n.logger.Printf("Error while checking Nginx server status: %s\n", err)
		return false, err
	}

	running := false
	resultStr := strings.TrimSpace(string(result))
	if resultStr == "1" {
		running = true
	}

	return running, nil
}

// ConfigTest runs the Nginx's config test command and returns whether the configuration is valid
func (n *NginxConfig) ConfigTest() (bool, error) {
	result, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.test")).Output()
	// Ignore the error "exit status 1", which means the configuration test failed
	if err != nil && err.Error() != "exit status 1" {
		n.logger.Printf("Error while testing Nginx server configuration: %s\n", err)
		return false, err
	}

	ok := false
	resultStr := strings.TrimSpace(string(result))
	if resultStr == "" && err == nil {
		// Even if there was nothing printed out, the exit status can't be 1
		ok = true
	}

	return ok, nil
}

// EnsureServerRunning starts the Nginx server if it's not running already
func (n *NginxConfig) EnsureServerRunning() error {
	// Check if Nginx is running
	running, err := n.Status()
	if err != nil {
		return err
	}
	if !running {
		n.logger.Println("Starting Nginx server")
		_, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.start")).Output()
		if err != nil {
			n.logger.Printf("Error while starting Nginx server: %s\n", err)
			return err
		}
	}

	return nil
}

// RestartServer restarts the Nginx server
func (n *NginxConfig) RestartServer() error {
	// Check if Nginx is running
	running, err := n.Status()
	if err != nil {
		return err
	}
	if running {
		// Reload the configuration
		n.logger.Println("Restarting Nginx server")
		_, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.restart")).Output()
		if err != nil {
			n.logger.Printf("Error while restarting Nginx server: %s\n", err)
			return err
		}
	} else {
		// Start Nginx
		n.logger.Println("Starting Nginx server")
		_, err := exec.Command("sh", "-c", appconfig.Config.GetString("nginx.commands.start")).Output()
		if err != nil {
			n.logger.Printf("Error while starting Nginx server: %s\n", err)
			return err
		}
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
func (n *NginxConfig) createConfigurationFile(templateName string, itemData *state.SiteState) ([]byte, error) {
	// Check if the current node is using HTTPS
	protocol := "http"
	if appconfig.Config.GetBool("tls.node.enabled") {
		protocol = "https"
	}

	// Ensure that itemData.App.Manifest is set
	if itemData != nil {
		if itemData.App == nil {
			itemData.App = &state.SiteApp{}
		}
		if itemData.App.Manifest == nil {
			itemData.App.Manifest = &utils.AppManifest{}
		}

		// Parse and validate the app's manifest
		itemData.App.Manifest.Locations = make(map[string]utils.ManifestRuleOptions)
		if itemData.App.Manifest.Rules != nil && len(itemData.App.Manifest.Rules) > 0 {
			for _, v := range itemData.App.Manifest.Rules {
				// Ensure that only one of the various match types (exact, prefix, match, file) is set
				if (v.Match != "" && (v.Exact != "" || v.File != "" || v.Prefix != "")) ||
					(v.Exact != "" && (v.Match != "" || v.File != "" || v.Prefix != "")) ||
					(v.File != "" && (v.Match != "" || v.Exact != "" || v.Prefix != "")) ||
					(v.Prefix != "" && (v.Match != "" || v.Exact != "" || v.File != "")) {
					n.logger.Println("Ignoring rule that has more than one match type")
					continue
				}

				// Get the location rule block
				location := ""
				if v.Exact != "" {
					location = "= " + v.Exact
				} else if v.Prefix != "" {
					if v.Prefix == "/" {
						location = "/"
					} else {
						location = "^~ " + v.Prefix
					}
				} else if v.Match != "" {
					if v.CaseSensitive {
						location = "~ " + v.Match
					} else {
						location = "~* " + v.Match
					}
				} else if v.File != "" {
					switch v.File {
					// Aliases
					case "_images":
						location = " ~* \\.(jpg|jpeg|png|gif|ico|svg|svgz|webp|tif|tiff|dng|psd|heif|bmp)$"
					case "_videos":
						location = "~* \\.(mp4|m4v|mkv|webm|avi|mpg|mpeg|ogg|wmv|flv|mov)$"
					case "_audios":
						location = "~* \\.(mp3|mp4|aac|m4a|flac|wav|ogg|wma)$"
					case "_fonts":
						location = "~* \\.(woff|woff2|eot|otf|ttf)$"
					default:
						// Replace all commas with |
						v.File = strings.ReplaceAll(v.File, ",", "|")
						// TODO: validate this with a regular expression
						location = "~* \\.(" + v.File + ")$"
					}
				} else {
					n.logger.Println("Ignoring rule that has no match type")
					continue
				}

				// Sanitize rule options
				options := n.sanitizeManifestRuleOptions(v.Options)

				// Add the element
				itemData.App.Manifest.Locations[location] = options
			}
		}

		// Ensure that Page404 and Page403 don't start with a /
		if len(itemData.App.Manifest.Page404) > 1 && itemData.App.Manifest.Page404[0] == '/' {
			itemData.App.Manifest.Page404 = itemData.App.Manifest.Page404[1:]
		}
		if len(itemData.App.Manifest.Page403) > 1 && itemData.App.Manifest.Page403[0] == '/' {
			itemData.App.Manifest.Page403 = itemData.App.Manifest.Page403[1:]
		}
	}

	// App root
	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	// Get parameters
	tplData := struct {
		Item         *state.SiteState
		AppRoot      string
		Port         string
		Protocol     string
		ManifestFile string
		User         string
		TLS          struct {
			Dhparams string
			Node     struct {
				Enabled     bool
				Certificate string
				Key         string
			}
		}
	}{
		Item:         itemData,
		AppRoot:      appRoot,
		Port:         appconfig.Config.GetString("port"),
		Protocol:     protocol,
		ManifestFile: appconfig.Config.GetString("manifestFile"),
		User:         appconfig.Config.GetString("nginx.user"),
		TLS: struct {
			Dhparams string
			Node     struct {
				Enabled     bool
				Certificate string
				Key         string
			}
		}{
			Dhparams: appRoot + "misc/dhparams.pem",
			Node: struct {
				Enabled     bool
				Certificate string
				Key         string
			}{
				Enabled:     appconfig.Config.GetBool("tls.node.enabled"),
				Certificate: appRoot + "misc/node.cert.pem",
				Key:         appRoot + "misc/node.key.pem",
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

// Validates and sanitizes an ManifestRuleOptions object in the manifest
func (n *NginxConfig) sanitizeManifestRuleOptions(v utils.ManifestRuleOptions) utils.ManifestRuleOptions {
	// If there's a ClientCaching value, ensure it's valid
	if v.ClientCaching != "" {
		if !n.clientCachingRegexp.MatchString(v.ClientCaching) {
			n.logger.Println("Ignoring invalid value for clientCaching:", v.ClientCaching)
			v.ClientCaching = ""
		}
	}

	// Escape values in headers
	if v.Headers != nil && len(v.Headers) > 0 {
		v.CleanHeaders = make(map[string]string, 0)
		for hk, hv := range v.Headers {
			// Filter out disallowed headers
			if !utils.HeaderIsAllowed(hk) {
				n.logger.Println("Ignoring invalid header:", hk)
				continue
			}
			// Escape the header
			v.CleanHeaders[escapeConfigString(hk)] = escapeConfigString(hv)
		}
	}

	// Validate the URL for proxying
	if v.Proxy != "" {
		parsed, err := url.ParseRequestURI(v.Proxy)
		if err != nil || parsed == nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			n.logger.Println("Ignoring invalid value for proxy:", v.Proxy)
			v.Proxy = ""
		}
	}

	return v
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

// Escapes characters in strings used in nginx's config files
func escapeConfigString(in string) (out string) {
	out = ""
	for _, char := range in {
		// Escape " and \
		if char == '"' || char == '\\' {
			out += "\\"
		}
		out += string(char)

		// We can't escape the $ sign, but we can use the $dollar variable
		// See: https://serverfault.com/a/854600/93929
		if char == '$' {
			out += "{dollar}"
		}
	}
	return
}
