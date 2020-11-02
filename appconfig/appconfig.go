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

package appconfig

import (
	"log"
	"os"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
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
	viper.SetDefault("acme.endpoint", "https://acme-v02.api.letsencrypt.org/directory")
	viper.SetDefault("appRoot", "/var/statiko/")
	viper.SetDefault("codesign.required", false)
	viper.SetDefault("disallowLeadership", false)
	viper.SetDefault("manifestFile", "_statiko.yaml")
	viper.SetDefault("repo.s3.endpoint", "s3.amazonaws.com")
	viper.SetDefault("state.etcd.keyPrefix", "/statiko")
	viper.SetDefault("state.file.path", "/etc/statiko/state")
	viper.SetDefault("state.etcd.timeout", 10000)
	viper.SetDefault("state.store", "file")
	viper.SetDefault("tls.dhparams.maxAge", 120)
	viper.SetDefault("tls.dhparams.bits", 4096)
	viper.SetDefault("tls.node.acme", false)
	viper.SetDefault("tls.node.certificate", "/etc/statiko/node-public.crt")
	viper.SetDefault("tls.node.enabled", true)
	viper.SetDefault("tls.node.key", "/etc/statiko/node-private.key")
	viper.SetDefault("notifications.webhook.payloadKey", "value1")
}

// Bind environmental variables to config options
func (c *appConfig) bindEnvVars() {
	// Note also GO_ENV and CONFIG_FILE_PATH which are used above, and not part of the config file
	viper.BindEnv("acme.email", "STATIKO_ACME_EMAIL")
	viper.BindEnv("acme.endpoint", "STATIKO_ACME_ENDPOINT")
	viper.BindEnv("appRoot", "STATIKO_APP_ROOT")
	viper.BindEnv("auth.auth0.clientId", "STATIKO_AUTH_AUTH0_CLIENT_ID")
	viper.BindEnv("auth.auth0.domain", "STATIKO_AUTH_AUTH0_DOMAIN")
	viper.BindEnv("auth.auth0.enabled", "STATIKO_AUTH_AUTH0_ENABLED")
	viper.BindEnv("auth.azureAD.clientId", "STATIKO_AUTH_AZUREAD_CLIENT_ID")
	viper.BindEnv("auth.azureAD.enabled", "STATIKO_AUTH_AZUREAD_ENABLED")
	viper.BindEnv("auth.azureAD.tenantId", "STATIKO_AUTH_AZUREAD_TENANT_ID")
	viper.BindEnv("auth.psk.enabled", "STATIKO_AUTH_PSK_ENABLED")
	viper.BindEnv("auth.psk.key", "STATIKO_AUTH_PSK_KEY")
	viper.BindEnv("azure.clientId", "STATIKO_AZURE_CLIENT_ID")
	viper.BindEnv("azure.clientSecret", "STATIKO_AZURE_CLIENT_SECRET")
	viper.BindEnv("azure.tenantId", "STATIKO_AZURE_TENANT_ID")
	viper.BindEnv("azureKeyVault.name", "STATIKO_AZURE_KEY_VAULT_NAME")
	viper.BindEnv("azureKeyVault.auth.tenantId", "STATIKO_AZURE_KEY_VAULT_AUTH_TENANT_ID")
	viper.BindEnv("azureKeyVault.auth.clientId", "STATIKO_AZURE_KEY_VAULT_AUTH_CLIENT_ID")
	viper.BindEnv("azureKeyVault.auth.clientSecret", "STATIKO_AZURE_KEY_VAULT_AUTH_CLIENT_SECRET")
	viper.BindEnv("codesign.publicKey", "STATIKO_CODESIGN_PUBLIC_KEY")
	viper.BindEnv("codesign.required", "STATIKO_CODESIGN_REQUIRED")
	viper.BindEnv("disallowLeadership", "STATIKO_DISALLOW_LEADERSHIP")
	viper.BindEnv("manifestFile", "STATIKO_MANIFEST_FILE")
	viper.BindEnv("nodeName", "STATIKO_NODE_NAME")
	viper.BindEnv("notifications.method", "STATIKO_NOTIFICATIONS_METHOD")
	viper.BindEnv("notifications.webhook.payloadKey", "STATIKO_NOTIFICATIONS_WEBHOOK_PAYLOAD_KEY")
	viper.BindEnv("notifications.webhook.url", "STATIKO_NOTIFICATIONS_WEBHOOK_URL")
	viper.BindEnv("port", "STATIKO_PORT")
	viper.BindEnv("repo.type", "STATIKO_REPO_TYPE")
	viper.BindEnv("repo.local.path", "STATIKO_REPO_LOCAL_PATH")
	viper.BindEnv("repo.azure.account", "STATIKO_REPO_AZURE_ACCOUNT")
	viper.BindEnv("repo.azure.container", "STATIKO_REPO_AZURE_CONTAINER")
	viper.BindEnv("repo.azure.accessKey", "STATIKO_REPO_AZURE_ACCESS_KEY")
	viper.BindEnv("repo.azure.endpointSuffix", "STATIKO_REPO_AZURE_ENDPOINT_SUFFIX")
	viper.BindEnv("repo.azure.customEndpoint", "STATIKO_REPO_AZURE_CUSTOM_ENDPOINT")
	viper.BindEnv("repo.azure.noTLS", "STATIKO_REPO_AZURE_NO_TLS")
	viper.BindEnv("repo.s3.accessKeyId", "STATIKO_REPO_S3_ACCESS_KEY_ID")
	viper.BindEnv("repo.s3.bucket", "STATIKO_REPO_S3_BUCKET")
	viper.BindEnv("repo.s3.endpoint", "STATIKO_REPO_S3_ENDPOINT")
	viper.BindEnv("repo.s3.noTLS", "STATIKO_REPO_S3_NO_TLS")
	viper.BindEnv("repo.s3.secretAccessKey", "STATIKO_REPO_S3_SECRET_ACCESS_KEY")
	viper.BindEnv("secretsEncryptionKey", "STATIKO_SECRETS_ENCRYPTION_KEY")
	viper.BindEnv("state.etcd.address", "STATIKO_STATE_ETCD_ADDRESS")
	viper.BindEnv("state.etcd.keyPrefix", "STATIKO_STATE_ETCD_KEY_PREFIX")
	viper.BindEnv("state.etcd.timeout", "STATIKO_STATE_ETCD_TIMEOUT")
	viper.BindEnv("state.etcd.tlsConfiguration.ca", "STATIKO_STATE_ETCD_TLS_CA")
	viper.BindEnv("state.etcd.tlsConfiguration.clientCertificate", "STATIKO_STATE_ETCD_TLS_CLIENT_CERTIFICATE")
	viper.BindEnv("state.etcd.tlsConfiguration.clientKey", "STATIKO_STATE_ETCD_TLS_CLIENT_KEY")
	viper.BindEnv("state.etcd.tlsSkipVerify", "STATIKO_STATE_ETCD_TLS_SKIP_VERIFY")
	viper.BindEnv("state.file.path", "STATIKO_STATE_FILE_PATH")
	viper.BindEnv("state.store", "STATIKO_STATE_STORE")
	viper.BindEnv("temporarySites.domain", "STATIKO_TEMPORARY_SITES_DOMAIN")
	viper.BindEnv("tls.dhparams.bits", "STATIKO_TLS_DHPARAMS_BITS")
	viper.BindEnv("tls.dhparams.maxAge", "STATIKO_TLS_DHPARAMS_MAX_AGE")
	viper.BindEnv("tls.node.acme", "STATIKO_TLS_NODE_ACME")
	viper.BindEnv("tls.node.address", "STATIKO_TLS_NODE_ADDRESS")
	viper.BindEnv("tls.node.certificate", "STATIKO_TLS_NODE_CERTIFICATE")
	viper.BindEnv("tls.node.enabled", "STATIKO_TLS_NODE_ENABLED")
	viper.BindEnv("tls.node.key", "STATIKO_TLS_NODE_KEY")
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
