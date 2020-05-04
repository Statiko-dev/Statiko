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
	"context"
	"fmt"
	"log"
	"os"

	"github.com/coreos/etcd/clientv3"
	"github.com/google/uuid"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
)

// ControllerEtcd is a worker controller that uses etcd as backend
type ControllerEtcd struct {
	leaderKey       string
	jobsKeyPrefix   string
	isLeader        bool
	lastJobRevision int64
	store           *state.StateStoreEtcd
	logger          *log.Logger
}

// Init the object
func (w *ControllerEtcd) Init(store *state.StateStoreEtcd) {
	// Init variables
	keyPrefix := appconfig.Config.GetString("state.etcd.keyPrefix")
	w.leaderKey = keyPrefix + "/leader"
	w.jobsKeyPrefix = keyPrefix + "/jobs/"
	w.isLeader = false
	w.lastJobRevision = 0
	w.store = store
	w.logger = log.New(os.Stdout, "worker/controller-etcd: ", log.Ldate|log.Ltime|log.LUTC)

	// Start the listener
	err := w.startListener()
	if err != nil {
		panic(fmt.Errorf("error while starting the listener:\n%v", err))
	}
}

// AddJob adds a job to the queue
func (w *ControllerEtcd) AddJob(job string) error {
	// Generate a random job id
	jobID := uuid.New().String()

	// Write the job
	ctx, cancel := w.store.GetContext()
	_, err := w.store.GetClient().Put(ctx, w.jobsKeyPrefix+jobID, job)
	cancel()
	if err != nil {
		return err
	}

	return nil
}

// Starts the listener that processes all job requests
func (w *ControllerEtcd) startListener() error {
	// Acquire leadership
	leaderChan := make(chan bool)
	go func() {
		err := w.acquireLeadership(leaderChan)
		if err != nil {
			panic(fmt.Errorf("error while electing leader:\n%v", err))
		}
	}()

	// Listen for changes in leadership
	go func() {
		var ctx context.Context
		var cancel context.CancelFunc
		var err error
		for _ = range leaderChan {
			// Cancel all existing contexts if any
			if cancel != nil {
				cancel()
				cancel = nil
			}

			// If we're leader now, start listening for jobs
			if w.isLeader {
				ctx, cancel = context.WithCancel(context.Background())
				err = w.listenForJobs(ctx)
				if err != nil {
					panic(fmt.Errorf("error while listening for jobs:\n%v", err))
				}

				// Start workers that require leadership
				startLeaderWorkers(ctx)
			}
		}

		w.logger.Println("Terminating all leader workers")
		cancel()
	}()

	return nil
}

// Listens for jobs
func (w *ControllerEtcd) listenForJobs(ctx context.Context) error {
	w.lastJobRevision = 0

	// Start watching for jobs
	go func() {
		rch := w.store.GetClient().Watch(ctx, w.jobsKeyPrefix, clientv3.WithPrefix())

		// Listen to events
		for resp := range rch {
			// Check for unrecoverable errors
			if resp.Err() != nil {
				panic(fmt.Errorf("unrecoverable error from etcd watcher: %v", resp.Err()))
			}

			// Only look for new jobs
			for _, event := range resp.Events {
				if event.Kv.ModRevision > w.lastJobRevision && event.IsCreate() {
					// Process jobs here
					fmt.Println("Received job:", string(event.Kv.Key), string(event.Kv.Value))
					w.lastJobRevision = event.Kv.ModRevision
				}
			}
		}
	}()

	// Request the first list of jobs
	reqCtx, reqCancel := w.store.GetContext()
	resp, err := w.store.GetClient().Get(reqCtx, w.jobsKeyPrefix, clientv3.WithPrefix())
	reqCancel()
	if err != nil {
		return err
	}
	w.lastJobRevision = resp.Header.GetRevision()
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		for _, kv := range resp.Kvs {
			if kv.Value != nil && len(kv.Value) > 0 {
				// Process jobs here
				fmt.Println("Loaded job:", string(kv.Key), string(kv.Value))
			}
		}
	}

	return nil
}

// Acquires leadership
func (w *ControllerEtcd) acquireLeadership(leaderChan chan bool) error {
	ctx := context.Background()

	setLeader := func(set bool) {
		// Only report changes in leadership
		if w.isLeader == set {
			return
		}
		if set {
			w.logger.Println("We are leaders now")
		} else {
			w.logger.Println("We lost leadership")
		}
		w.isLeader = set
		leaderChan <- set
	}
	setLeader(false)

	// Try acquiring leadership
	// We are not using the etcdv3 leader election APIs because they are too advanced for our needs and they had some inconsistencies that make them challenging to use for us
	// In particular, on reconnections they cause nodes to receive multiple messages at the same time, making them acquire leadership briefly to then lose it again
	// For our needs, this solution below is enough and it limits the number of leadership changes
	acquireLeadership := func() error {
		innerCtx, cancel := w.store.GetContext()
		lease, err := w.store.GetClient().Grant(innerCtx, state.EtcdLockDuration)
		cancel()
		if err != nil {
			return err
		}

		// Losing a connection will cause the loss of leadership. Even if we re-connected, we would still lose the lease. That's fine, we'll have a new "election"
		innerCtx, cancel = w.store.GetContext()
		txn := w.store.GetClient().Txn(innerCtx)
		res, err := txn.If(
			clientv3.Compare(clientv3.Version(w.leaderKey), "=", 0),
		).Then(
			clientv3.OpPut(w.leaderKey, w.store.GetMemberId(), clientv3.WithLease(lease.ID)),
		).Commit()
		cancel()
		if err != nil {
			return err
		}
		if !res.Succeeded {
			// We don't have leadership
			setLeader(false)
			return nil
		}

		// We acquired the lock, so we're leaders. Maintain the key alive
		_, err = w.store.GetClient().KeepAlive(ctx, lease.ID)
		if err != nil {
			return err
		}

		return nil
	}

	// Watch for changes of leadership
	go func() {
		rch := w.store.GetClient().Watch(ctx, w.leaderKey)

		// Listen to events
		for resp := range rch {
			// Check for unrecoverable errors
			if resp.Err() != nil {
				panic(fmt.Errorf("unrecoverable error from etcd watcher: %v", resp.Err()))
			}

			// Always get the last message only
			event := resp.Events[len(resp.Events)-1]

			// If there's no leader, try acquiring the leadership
			if len(event.Kv.Value) == 0 {
				err := acquireLeadership()
				if err != nil {
					w.logger.Println("Error while trying to acquire leadership", err)
				}
			} else {
				// If we have a leader, check if that's us
				leader := string(event.Kv.Value)
				setLeader(leader == w.store.GetMemberId())
			}
		}
	}()

	// Try acquiring the leadership now
	err := acquireLeadership()
	if err != nil {
		return err
	}

	return nil
}
