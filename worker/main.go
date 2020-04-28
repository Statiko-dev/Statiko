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

package worker

import (
	"github.com/statiko-dev/statiko/state"
)

// StartWorker starts all the background workers
func StartWorker() {
	//startCertMonitorWorker()
	//startDHParamsWorker()
	startSharedWorkers()
	startController()
}

// Start the controller that manages the workers that only run in the cluster's leader node
func startController() {
	// Get the store
	store := state.Instance.GetStore()
	switch state.Instance.GetStoreType() {
	case state.StoreTypeEtcd:
		controller := &ControllerEtcd{}
		controller.Init(store.(*state.StateStoreEtcd))
	}
}

// Start the workers that run on all nodes
func startSharedWorkers() {
	startHealthWorker()
}
