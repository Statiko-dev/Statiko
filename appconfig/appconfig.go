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

package appconfig

import (
	"log"
	"os"

	"github.com/spf13/viper"

	"smplatform/buildinfo"
)

type appConfig struct {
	logger *log.Logger
}

// ENV is used to help switch settings based on where the application is being run
var ENV string

// Load itializes the object
func (c *appConfig) Load() error {
	// Logger
	c.logger = log.New(os.Stdout, "appconfig: ", log.Ldate|log.Ltime|log.LUTC)

	// Set environment
	ENV = os.Getenv("GO_ENV")
	if len(ENV) < 1 {
		// Check if we have something hardcoded at build-time
		if len(buildinfo.ENV) > 0 {
			ENV = buildinfo.ENV
		} else {
			// Fallback to "development"
			ENV = "development"
		}
	}
	c.logger.Printf("Environment: %s\n", ENV)

	// Load configuration
	c.logger.Println("Loading configuration")

	// Look for a config file named node-config.(json|yaml|toml|...)
	viper.SetConfigName("node-config")

	// Look in /etc/smplatform
	viper.AddConfigPath("/etc/smplatform/")

	// In development, add also the current working directory
	if ENV != "production" {
		viper.AddConfigPath(".")
	}

	// Default values
	// Default port is 3000
	viper.SetDefault("port", "3000")

	// Some settings can be set as env vars too
	viper.BindEnv("auth", "PLATFORM_AUTH")
	viper.BindEnv("db", "PLATFORM_DB")
	viper.BindEnv("port", "PORT")
	viper.BindEnv("appRoot", "PLATFORM_APP_ROOT")
	viper.BindEnv("nginx.commands.start", "PLATFORM_NGINX_START")
	viper.BindEnv("nginx.commands.stop", "PLATFORM_NGINX_STOP")
	viper.BindEnv("nginx.commands.restart", "PLATFORM_NGINX_RESTART")
	viper.BindEnv("azureKeyVault.name", "AZURE_KEYVAULT_NAME")
	viper.BindEnv("azureKeyVault.servicePrincipal.tenantId", "AZURE_TENANT_ID")
	viper.BindEnv("azureKeyVault.servicePrincipal.clientId", "AZURE_CLIENT_ID")
	viper.BindEnv("azureKeyVault.servicePrincipal.clientSecret", "AZURE_CLIENT_SECRET")
	viper.BindEnv("azureStorage.account", "AZURE_STORAGE_ACCOUNT")
	viper.BindEnv("azureStorage.container", "AZURE_STORAGE_CONTAINER")
	viper.BindEnv("azureStorage.sasToken", "AZURE_STORAGE_SAS_TOKEN")

	// Load config file
	err := viper.ReadInConfig()
	if err != nil {
		c.logger.Fatalf("Error reading config file: %s \n", err)
		return err
	}
	c.logger.Printf("Config file used: %s\n", viper.ConfigFileUsed())

	return nil
}

// Get returns the value as interface{}
func (c *appConfig) Get(key string) interface{} {
	return viper.Get(key)
}

// GetString returns the value as string
func (c *appConfig) GetString(key string) string {
	return viper.GetString(key)
}

// GetBool returns the value as boolean
func (c *appConfig) GetBool(key string) bool {
	return viper.GetBool(key)
}
