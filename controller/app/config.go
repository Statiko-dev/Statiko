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
func (c *Controller) loadConfig() error {
	// Default node name is the hostname
	// Ignore errors here
	var hostname interface{}
	hostnameStr, _ := os.Hostname()
	if hostnameStr != "" {
		hostname = hostnameStr
	}

	// List of config options for controller nodes
	// This excludes all `repo.*` options that are returned by `utils.RepoConfigEntries`
	entries := map[string]utils.ConfigEntry{
		// Email address used to register with the ACME (e.g. Let's Encrypt) service
		"acme.email": {
			EnvVar: "ACME_EMAIL",
		},
		// Endpoint of the ACME service
		// The defaullt value is to use the public Let's Encrypt service
		// This can be overridden if using a private ACME certificate authority (e.g. using Boulder) or for testing
		"acme.endpoint": {
			EnvVar:       "ACME_ENDPOINT",
			DefaultValue: "https://acme-v02.api.letsencrypt.org/directory",
		},
		"auth.auth0.clientId": {
			EnvVar: "AUTH_AUTH0_CLIENT_ID",
		},
		"auth.auth0.domain": {
			EnvVar: "AUTH_AUTH0_DOMAIN",
		},
		"auth.auth0.enabled": {
			EnvVar: "AUTH_AUTH0_ENABLED",
		},
		"auth.azureAD.clientId": {
			EnvVar: "AUTH_AZUREAD_CLIENT_ID",
		},
		"auth.azureAD.enabled": {
			EnvVar: "AUTH_AZUREAD_ENABLED",
		},
		"auth.azureAD.tenantId": {
			EnvVar: "AUTH_AZUREAD_TENANT_ID",
		},
		"auth.psk.enabled": {
			EnvVar: "AUTH_PSK_ENABLED",
		},
		"auth.psk.key": {
			EnvVar: "AUTH_PSK_KEY",
		},
		"azureKeyVault.name": {
			EnvVar: "AZURE_KEY_VAULT_NAME",
		},
		"azureKeyVault.auth.tenantId": {
			EnvVar: "AZURE_KEY_VAULT_AUTH_TENANT_ID",
		},
		"azureKeyVault.auth.clientId": {
			EnvVar: "AZURE_KEY_VAULT_AUTH_CLIENT_ID",
		},
		"azureKeyVault.auth.clientSecret": {
			EnvVar: "AZURE_KEY_VAULT_AUTH_CLIENT_SECRET",
		},
		"codesign.publicKey": {
			EnvVar: "CODESIGN_PUBLIC_KEY",
		},
		"codesign.required": {
			EnvVar:       "CODESIGN_REQUIRED",
			DefaultValue: false,
		},
		"controller.apiPort": {
			EnvVar:       "CONTROLLER_API_PORT",
			DefaultValue: 2265,
		},
		"controller.grpcPort": {
			EnvVar:       "CONTROLLER_GRPC_PORT",
			DefaultValue: 2300,
		},
		"controller.tls.acme": {
			EnvVar:       "CONTROLLER_TLS_ACME",
			DefaultValue: false,
		},
		"controller.tls.azureKeyVault": {
			EnvVar: "CONTROLLER_TLS_AZURE_KEY_VAULT",
		},
		"controller.tls.certificate": {
			EnvVar:       "CONTROLLER_TLS_CERTIFICATE",
			DefaultValue: "/etc/statiko/controller.pem",
		},
		"controller.tls.key": {
			EnvVar:       "CONTROLLER_TLS_KEY",
			DefaultValue: "/etc/statiko/controller-key.pem",
		},
		// Auth token used by agents to communicate with the controlller
		"controller.token": {
			EnvVar:   "CONTROLLER_TOKEN",
			Required: true,
		},
		"dhparams.bits": {
			EnvVar:       "DHPARAMS_BITS",
			DefaultValue: 4096,
		},
		"dhparams.maxAge": {
			EnvVar:       "DHPARAMS_MAX_AGE",
			DefaultValue: 120,
		},
		"manifestFile": {
			EnvVar:       "MANIFEST_FILE",
			DefaultValue: "_statiko.yaml",
		},
		// Hostname of the controller node
		// This should be a Fully-Qualified Domain Name (FQDN) as it's used to generate TLS certificates for the node if needed
		"nodeName": {
			EnvVar:       "NODE_NAME",
			DefaultValue: hostname,
			Required:     true,
		},
		"notifications.method": {
			EnvVar: "NOTIFICATIONS_METHOD",
		},
		"notifications.webhook.payloadKey": {
			EnvVar:       "NOTIFICATIONS_WEBHOOK_PAYLOAD_KEY",
			DefaultValue: "message",
		},
		"notifications.webhook.url": {
			EnvVar: "NOTIFICATIONS_WEBHOOK_URL",
		},
		"secretsEncryptionKey": {
			EnvVar: "SECRETS_ENCRYPTION_KEY",
		},
		/*"state.etcd.address": {
			EnvVar: "STATE_ETCD_ADDRESS",
		},
		"state.etcd.keyPrefix": {
			EnvVar:       "STATE_ETCD_KEY_PREFIX",
			DefaultValue: "/statiko",
		},
		"state.etcd.timeout": {
			EnvVar:       "STATE_ETCD_TIMEOUT",
			DefaultValue: 10000,
		},
		"state.etcd.tlsConfiguration.ca": {
			EnvVar: "STATE_ETCD_TLS_CA",
		},
		"state.etcd.tlsConfiguration.clientCertificate": {
			EnvVar: "STATE_ETCD_TLS_CLIENT_CERTIFICATE",
		},
		"state.etcd.tlsConfiguration.clientKey": {
			EnvVar: "STATE_ETCD_TLS_CLIENT_KEY",
		},
		"state.etcd.tlsSkipVerify": {
			EnvVar: "STATE_ETCD_TLS_SKIP_VERIFY",
		},*/
		"state.file.path": {
			EnvVar:       "STATE_FILE_PATH",
			DefaultValue: "/etc/statiko/state",
		},
		"state.store": {
			EnvVar:       "STATE_STORE",
			DefaultValue: "file",
			Required:     true,
		},
		"temporarySites.domain": {
			EnvVar: "TEMPORARY_SITES_DOMAIN",
		},
	}

	// Load the config
	return utils.LoadConfig("STATIKO_", "controller", entries, utils.RepoConfigEntries())
}
