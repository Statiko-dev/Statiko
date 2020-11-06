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
func LoadConfig(envPrefix string, nodeType string, entries map[string]ConfigEntry) error {
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

	// Look for a config file named <node-type>-config.(json|yaml|toml|...)
	viper.SetConfigName(nodeType + "-config")

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
	for k, e := range entries {
		if e.DefaultValue != nil {
			viper.SetDefault(k, e.DefaultValue)
		}
		if e.EnvVar != "" {
			viper.BindEnv(k, (envPrefix + e.EnvVar))
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
	for k, e := range entries {
		if e.Required && !viper.IsSet(k) {
			return fmt.Errorf("configuration option %s is required", k)
		}
	}

	return nil
}
