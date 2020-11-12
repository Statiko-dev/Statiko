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

package app

import (
	"os"

	"github.com/statiko-dev/statiko/shared/utils"
)

// loadConfig loads the configuration
func (a *Agent) loadConfig() error {
	// Default node name is the hostname
	// Ignore errors here
	var hostname interface{}
	hostnameStr, _ := os.Hostname()
	if hostnameStr != "" {
		hostname = hostnameStr
	}

	// List of config options for agent nodes
	entries := map[string]utils.ConfigEntry{
		// Address (ip or hostname, and port) of the controller gRPC server
		"controller.address": {
			EnvVar:   "CONTROLLER_ADDRESS",
			Required: true,
		},
		// Client secret for authenticating with the controller gRPC server using Azure AD, if supported
		"controller.auth.azureClientSecret": {
			EnvVar: "CONTROLLER_AUTH_AZURE_CLIENT_SECRET",
		},
		// PSK for authenticating with the controller gRPC server, if supported
		"controller.auth.psk": {
			EnvVar: "CONTROLLER_AUTH_PSK",
		},
		// Skip verifying TLS certificates presented by the gRPC server
		// This is a potentially insecure flag that should only be used for testing
		"controller.tls.insecure": {
			EnvVar:       "CONTROLLER_TLS_INSECURE",
			DefaultValue: false,
		},
		// If set, uses this CA certificate (PEM-encoded) to verify the TLS certificate presented by the gRPC server
		// If not set, it uses all certificates in the system's certificate store
		"controller.tls.ca": {
			EnvVar: "CONTROLLER_TLS_CA",
		},
		// Name of the agent node (by default, the hostname)
		"nodeName": {
			EnvVar:       "NODE_NAME",
			DefaultValue: hostname,
			Required:     true,
		},
		// Port where the internal HTTP server listens to.
		// if 0 (the default), an available port will be auto-selected
		"serverPort": {
			EnvVar:       "SERVER_PORT",
			DefaultValue: 0,
		},
		// Folder where to store the apps and other data for the Statiko agent
		"appRoot": {
			EnvVar:       "APP_ROOT",
			DefaultValue: "/var/statiko/",
		},
		// Command to restart nginx
		"nginx.commands.restart": {
			EnvVar:       "NGINX_RESTART",
			DefaultValue: "systemctl is-active --quiet nginx && systemctl reload nginx || systemctl restart nginx",
		},
		// Command to start nginx
		"nginx.commands.start": {
			EnvVar:       "NGINX_START",
			DefaultValue: "systemctl start nginx",
		},
		// Command that returns the status of the nginx web server
		// This should never return an error, and should print "1" if nginx is running, or "0" otherwise
		"nginx.commands.status": {
			EnvVar:       "NGINX_STATUS",
			DefaultValue: "systemctl is-active --quiet nginx && echo 1 || echo 0",
		},
		// Command that performs the configuration test for nginx
		"nginx.commands.test": {
			EnvVar:       "NGINX_TEST",
			DefaultValue: "nginx -t -q",
		},
		// Folder where the configuration for nginx is stored
		"nginx.configPath": {
			EnvVar:       "NGINX_CONFIG_PATH",
			DefaultValue: "/etc/nginx/",
		},
		// User that runs the nginx daemon (for setting the owner of the webroot)
		/*"nginx.user": {
			EnvVar:       "NGINX_USER",
			DefaultValue: "www-data",
		},*/
	}

	// Load the config
	return utils.LoadConfig("STATIKO_", "agent", entries)
}
