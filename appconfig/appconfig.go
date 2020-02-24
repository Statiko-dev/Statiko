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

	"github.com/ItalyPaleAle/statiko/buildinfo"
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
	if ENV == "" {
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

	// Check if we have a path for the config file
	configFilePath := os.Getenv("CONFIG_FILE_PATH")
	if configFilePath != "" {
		viper.AddConfigPath(configFilePath)
	}

	// Look in /etc/statiko
	viper.AddConfigPath("/etc/statiko/")

	// In development, add also the current working directory
	if ENV != "production" {
		viper.AddConfigPath(".")
	}

	// Set defaults
	c.setDefaults()

	// Some settings can be set as env vars too
	c.bindEnvVars()

	// Load config file
	err := viper.ReadInConfig()
	if err != nil {
		c.logger.Fatalf("Error reading config file: %s \n", err)
		return err
	}
	c.logger.Printf("Config file used: %s\n", viper.ConfigFileUsed())

	return nil
}

// Set default options
func (c *appConfig) setDefaults() {
	// Default values
	// Default port is 2265
	viper.SetDefault("port", "2265")

	// Default node name is the hostname
	// Ignore errors here
	hostname, _ := os.Hostname()
	viper.SetDefault("nodeName", hostname)

	// Other default values
	viper.SetDefault("state.store", "file")
	viper.SetDefault("state.file.path", "/etc/statiko/state.json")
	viper.SetDefault("state.etcd.keyPrefix", "/statiko")
	viper.SetDefault("state.etcd.timeout", 10000)
	viper.SetDefault("appRoot", "/var/statiko")
	viper.SetDefault("nginx.configPath", "/etc/nginx/")
	viper.SetDefault("nginx.user", "www-data")
	viper.SetDefault("azure.storage.appsContainer", "apps")
	viper.SetDefault("azure.keyVault.codesignKey.name", "codesign")
	viper.SetDefault("azure.keyVault.codesignKey.version", "latest")
	viper.SetDefault("tls.dhparams", "/etc/statiko/dhparams.pem")
	viper.SetDefault("tls.node.enabled", true)
	viper.SetDefault("tls.node.certificate", "/etc/statiko/node-public.crt")
	viper.SetDefault("tls.node.key", "/etc/statiko/node-private.key")
	viper.SetDefault("manifestFile", "_statiko.yaml")
	viper.SetDefault("notifications.webhook.payloadKey", "value1")
}

// Bind environmental variables to config options
func (c *appConfig) bindEnvVars() {
	viper.BindEnv("auth.psk.key", "AUTH_KEY")
	viper.BindEnv("auth.azureAD.tenantId", "AUTH_AZUREAD_TENANT_ID")
	viper.BindEnv("auth.azureAD.clientId", "AUTH_AZUREAD_CLIENT_ID")
	viper.BindEnv("secretsEncryptionKey", "SECRETS_ENCRYPTION_KEY")
	viper.BindEnv("port", "PORT")
	viper.BindEnv("state.store", "STATE_STORE")
	viper.BindEnv("state.file.path", "STATE_FILE_PATH")
	viper.BindEnv("state.etcd.keyPrefix", "STATE_ETCD_KEY_PREFIX")
	viper.BindEnv("state.etcd.timeout", "STATE_ETCD_TIMEOUT")
	viper.BindEnv("state.etcd.address", "STATE_ETCD_ADDRESS")
	viper.BindEnv("state.etcd.tlsConfiguration.ca", "STATE_ETCD_TLS_CA")
	viper.BindEnv("state.etcd.tlsConfiguration.clientCertificate", "STATE_ETCD_TLS_CLIENT_CERTIFICATE")
	viper.BindEnv("state.etcd.tlsConfiguration.clientKey", "STATE_ETCD_TLS_CLIENT_KEY")
	viper.BindEnv("appRoot", "APP_ROOT")
	viper.BindEnv("nginx.user", "NGINX_USER")
	viper.BindEnv("nginx.commands.start", "NGINX_START")
	viper.BindEnv("nginx.commands.stop", "NGINX_STOP")
	viper.BindEnv("nginx.commands.restart", "NGINX_RESTART")
	viper.BindEnv("nginx.commands.status", "NGINX_STATUS")
	viper.BindEnv("azure.sp.tenantId", "AZURE_TENANT_ID")
	viper.BindEnv("azure.sp.clientId", "AZURE_CLIENT_ID")
	viper.BindEnv("azure.sp.clientSecret", "AZURE_CLIENT_SECRET")
	viper.BindEnv("azure.keyVault.name", "AZURE_KEYVAULT_NAME")
	viper.BindEnv("azure.keyVault.codesignKey.name", "CODESIGN_KEY_NAME")
	viper.BindEnv("azure.keyVault.codesignKey.version", "CODESIGN_KEY_VERSION")
	viper.BindEnv("azure.storage.account", "AZURE_STORAGE_ACCOUNT")
	viper.BindEnv("azure.storage.key", "AZURE_STORAGE_KEY")
	viper.BindEnv("azure.storage.appsContainer", "AZURE_STORAGE_APPS_CONTAINER")
	viper.BindEnv("nodeName", "NODE_NAME")
	viper.BindEnv("notifications.method", "NOTIFICATIONS_METHOD")
	viper.BindEnv("notifications.webhook.url", "NOTIFICATIONS_WEBHOOK_URL")
	viper.BindEnv("notifications.webhook.payloadKey", "NOTIFICATIONS_WEBHOOK_PAYLOAD_KEY")
}

// Get returns the value as interface{}
func (c *appConfig) Get(key string) interface{} {
	return viper.Get(key)
}

// GetString returns the value as string
func (c *appConfig) GetString(key string) string {
	return viper.GetString(key)
}

// GetString returns the value as slice of strings
func (c *appConfig) GetStringSlice(key string) []string {
	return viper.GetStringSlice(key)
}

// GetBool returns the value as bool
func (c *appConfig) GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetInt returns the value as int
func (c *appConfig) GetInt(key string) int {
	return viper.GetInt(key)
}

// Set a new value in the configuration
// Note that the value is only stored in memory and not written to disk
func (c *appConfig) Set(key string, value interface{}) {
	viper.Set(key, value)
}
