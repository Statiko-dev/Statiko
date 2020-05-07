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

package statuscheck

import (
	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
	"github.com/statiko-dev/statiko/webserver"
)

func GetNodeHealth() *utils.NodeStatus {
	// Health object
	health := &utils.NodeStatus{}

	// Node name
	health.NodeName = appconfig.Config.GetString("nodeName")

	// Nginx server status
	// Ignore errors in this command
	nginxStatus, _ := webserver.Instance.Status()
	health.Nginx = utils.NginxStatus{
		Running: nginxStatus,
	}

	// Sync status
	syncError := sync.SyncError()
	syncErrorStr := ""
	if syncError != nil {
		syncErrorStr = syncError.Error()
	}
	health.Sync = utils.NodeSync{
		Running:   sync.IsRunning(),
		LastSync:  sync.LastSync(),
		SyncError: syncErrorStr,
	}

	// Store status
	storeHealth, _ := state.Instance.StoreHealth()
	health.Store = utils.NodeStore{
		Healthy: storeHealth,
	}

	// Test if the actual apps are responding (just to be sure), but only every 5 minutes
	health.Health = GetHealthCache()

	return health
}
