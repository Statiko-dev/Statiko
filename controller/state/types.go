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

package state

import (
	"github.com/statiko-dev/statiko/utils"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// WorkerController is the interface for the worker controller
type WorkerController interface {
	Init(store StateStore)
	IsLeader() bool
	AddJob(job utils.JobData) (string, error)
	CompleteJob(jobID string) error
	WaitForJob(jobID string, ch chan error)
}

// StateStore is the interface for the state stores
type StateStore interface {
	Init() error
	AcquireLock(name string, timeout bool) (interface{}, error)
	ReleaseLock(leaseID interface{}) error
	GetState() *pb.State
	SetState(*pb.State) error
	WriteState() error
	ReadState() error
	Healthy() (bool, error)
	OnReceive(func())
}
