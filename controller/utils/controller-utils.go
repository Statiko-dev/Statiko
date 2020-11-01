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
	"github.com/statiko-dev/statiko/appconfig"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// GetClusterOptionsAzureSP returns the pb.ClusterOptions_AzureKeyVault option for a namespace
func GetClusterOptionsAzureSP(namespace string) *pb.ClusterOptions_AzureServicePrincipal {
	tenantId := appconfig.Config.GetString(namespace + ".auth.tenantId")
	clientId := appconfig.Config.GetString(namespace + ".auth.clientId")
	clientSecret := appconfig.Config.GetString(namespace + ".auth.clientSecret")
	if tenantId == "" || clientId == "" || clientSecret == "" {
		return nil
	}
	return &pb.ClusterOptions_AzureServicePrincipal{
		TenantId:     tenantId,
		ClientId:     clientId,
		ClientSecret: clientSecret,
	}
}
