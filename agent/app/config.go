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
		"controllerAddress": {
			EnvVar:   "CONTROLLER_ADDRESS",
			Required: true,
		},
		"nodeName": {
			EnvVar:       "NODE_NAME",
			DefaultValue: hostname,
			Required:     true,
		},
		"serverPort": {
			EnvVar:       "SERVER_PORT",
			DefaultValue: 2424,
		},
		"appRoot": {
			EnvVar:       "APP_ROOT",
			DefaultValue: "/var/statiko/",
		},
		"nginx.commands.restart": {
			EnvVar:       "NGINX_RESTART",
			DefaultValue: "systemctl is-active --quiet nginx && systemctl reload nginx || systemctl restart nginx",
		},
		"nginx.commands.start": {
			EnvVar:       "NGINX_START",
			DefaultValue: "systemctl start nginx",
		},
		"nginx.commands.status": {
			EnvVar:       "NGINX_STATUS",
			DefaultValue: "systemctl is-active --quiet nginx && echo 1 || echo 0",
		},
		"nginx.commands.test": {
			EnvVar:       "NGINX_TEST",
			DefaultValue: "nginx -t -q",
		},
		"nginx.configPath": {
			EnvVar:       "NGINX_CONFIG_PATH",
			DefaultValue: "/etc/nginx/",
		},
		"nginx.user": {
			EnvVar:       "NGINX_USER",
			DefaultValue: "www-data",
		},
	}

	// Load the config
	return utils.LoadConfig("STATIKO_", "agent", entries)
}
