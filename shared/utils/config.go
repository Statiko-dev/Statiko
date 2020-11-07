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

package utils

import (
	"fmt"
	"os"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
)

// Dictionary with options for each configuration entry, allowing setting default values and mapping to env vars
type ConfigEntry struct {
	DefaultValue interface{}
	EnvVar       string
	Required     bool
}

// Loadconfig loads the configuration for a node (both controller and agent)
func LoadConfig(envPrefix string, nodeType string, entries ...map[string]ConfigEntry) error {
	// Check if we have an environment set
	env := os.Getenv(envPrefix + "ENV")
	if env == "" {
		// Check if we have something hardcoded at build-time
		if len(buildinfo.ENV) > 0 {
			env = buildinfo.ENV
		} else {
			// Fallback to "development"
			env = "development"
		}
	}
	logger.Printf("Environment: %s\n", env)

	// Load configuration
	logger.Println("Loading configuration")

	if nodeType != "" {
		// Look for a config file named <node-type>-config.(json|yaml|toml|...)
		viper.SetConfigName(nodeType + "-config")
	}

	// Check if we have a path for the config file
	configFilePath := os.Getenv(envPrefix + "CONFIG_FILE")
	if configFilePath != "" {
		viper.AddConfigPath(configFilePath)
	}

	// Look in /etc/statiko
	viper.AddConfigPath("/etc/statiko/")

	// In development, add also the current working directory
	if env != "production" {
		viper.AddConfigPath(".")
	}

	// For each entry, set the default value and map to an env var
	// Note also ENV and CONFIG_FILE which are used above, and not part of the config file
	for _, entry := range entries {
		for k, e := range entry {
			if e.DefaultValue != nil {
				viper.SetDefault(k, e.DefaultValue)
			}
			if e.EnvVar != "" {
				viper.BindEnv(k, (envPrefix + e.EnvVar))
			}
		}
	}

	// Load config file
	err := viper.ReadInConfig()
	if err != nil {
		// Config file not found
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Println("No configuration file found")
			return nil
		}

		// Another error
		logger.Fatalf("Error reading config file: %s \n", err)
		return err
	}
	logger.Printf("Config file used: %s\n", viper.ConfigFileUsed())

	// Check that required entries are set
	for _, entry := range entries {
		for k, e := range entry {
			if e.Required && !viper.IsSet(k) {
				return fmt.Errorf("configuration option %s is required", k)
			}
		}
	}

	return nil
}

// RepoConfigEntries returns the map of ConfigEntry's for the repo
func RepoConfigEntries() map[string]ConfigEntry {
	return map[string]ConfigEntry{
		"repo.type": {
			EnvVar:   "REPO_TYPE",
			Required: true,
		},
		"repo.local.path": {
			EnvVar: "REPO_LOCAL_PATH",
		},
		"repo.azure.account": {
			EnvVar: "REPO_AZURE_ACCOUNT",
		},
		"repo.azure.container": {
			EnvVar: "REPO_AZURE_CONTAINER",
		},
		"repo.azure.accessKey": {
			EnvVar: "REPO_AZURE_ACCESS_KEY",
		},
		"repo.azure.endpointSuffix": {
			EnvVar: "REPO_AZURE_ENDPOINT_SUFFIX",
		},
		"repo.azure.customEndpoint": {
			EnvVar: "REPO_AZURE_CUSTOM_ENDPOINT",
		},
		"repo.azure.noTLS": {
			EnvVar: "REPO_AZURE_NO_TLS",
		},
		"repo.azure.auth.tenantId": {
			EnvVar: "REPO_AZURE_AUTH_TENANT_ID",
		},
		"repo.azure.auth.clientId": {
			EnvVar: "REPO_AZURE_AUTH_CLIENT_ID",
		},
		"repo.azure.auth.clientSecret": {
			EnvVar: "REPO_AZURE_AUTH_CLIENT_SECRET",
		},
		"repo.s3.accessKeyId": {
			EnvVar: "REPO_S3_ACCESS_KEY_ID",
		},
		"repo.s3.bucket": {
			EnvVar: "REPO_S3_BUCKET",
		},
		"repo.s3.endpoint": {
			EnvVar:       "REPO_S3_ENDPOINT",
			DefaultValue: "s3.amazonaws.com",
		},
		"repo.s3.noTLS": {
			EnvVar: "REPO_S3_NO_TLS",
		},
		"repo.s3.secretAccessKey": {
			EnvVar: "REPO_S3_SECRET_ACCESS_KEY",
		},
	}
}
