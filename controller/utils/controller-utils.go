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
	"errors"
	"strings"

	"github.com/spf13/viper"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetClusterOptionsAzureSP returns the pb.ClusterOptions_AzureKeyVault option for a namespace
func GetClusterOptionsAzureSP(namespace string) *pb.ClusterOptions_AzureServicePrincipal {
	tenantId := viper.GetString(namespace + ".auth.tenantId")
	clientId := viper.GetString(namespace + ".auth.clientId")
	clientSecret := viper.GetString(namespace + ".auth.clientSecret")
	if tenantId == "" || clientId == "" || clientSecret == "" {
		return nil
	}
	return &pb.ClusterOptions_AzureServicePrincipal{
		TenantId:     tenantId,
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}
}

// GetClusterOptionsStorage returns the type of storage and storage options object
// The storage option is an object of one of: *pb.ClusterOptions_Local, *pb.ClusterOptions_Azure, *pb.ClusterOptions_S3
func GetClusterOptionsStorage() (string, interface{}) {
	switch viper.GetString("repo.type") {
	case "file", "local":
		return "file", &pb.ClusterOptions_StorageLocal{
			Path: viper.GetString("repo.local.path"),
		}
	case "azure", "azureblob":
		o := &pb.ClusterOptions_StorageAzure{
			Account:        viper.GetString("repo.azure.account"),
			Container:      viper.GetString("repo.azure.container"),
			AccessKey:      viper.GetString("repo.azure.accessKey"),
			EndpointSuffix: viper.GetString("repo.azure.endpointSuffix"),
			CustomEndpoint: viper.GetString("repo.azure.customEndpoint"),
			NoTls:          viper.GetBool("repo.azure.noTLS"),
		}
		// Check if we have a SP for authentication
		auth := GetClusterOptionsAzureSP("repo.azure")
		if auth != nil {
			o.Auth = auth
		}
		return "azure", o
	case "s3", "minio":
		return "s3", &pb.ClusterOptions_StorageS3{
			AccessKeyId:     viper.GetString("repo.s3.accessKeyId"),
			SecretAccessKey: viper.GetString("repo.s3.secretAccessKey"),
			Bucket:          viper.GetString("repo.s3.bucket"),
			Endpoint:        viper.GetString("repo.s3.endpoint"),
			NoTls:           viper.GetBool("repo.s3.noTLS"),
		}
	}
	return "", nil
}

// GetClusterOptionsNotifications returns the options for notifications
func GetClusterOptionsNotifications() ([]*pb.ClusterOptions_NotificationsOpts, error) {
	// For now, we support only one set of notification options

	// Check if notifications are enabled
	method := strings.ToLower(viper.GetString("notifications.method"))
	if method == "" || method == "off" || method == "no" || method == "0" {
		return nil, nil
	}

	// Create the object depending on the method
	res := make([]*pb.ClusterOptions_NotificationsOpts, 1)
	switch method {
	case "webhook":
		res[0] = &pb.ClusterOptions_NotificationsOpts{
			Opts: &pb.ClusterOptions_NotificationsOpts_Webhook{
				Webhook: &pb.ClusterOptions_NotificationsWebhook{
					Url:        viper.GetString("notifications.webhook.url"),
					PayloadKey: viper.GetString("notifications.webhook.payloadKey"),
				},
			},
		}
	default:
		return nil, errors.New("invalid notification method: " + method)
	}

	return res, nil
}
