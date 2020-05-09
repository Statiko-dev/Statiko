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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
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
func (w *ControllerEtcd) Init(store state.StateStore) {
	// Init variables
	keyPrefix := appconfig.Config.GetString("state.etcd.keyPrefix")
	w.leaderKey = keyPrefix + "/leader"
	w.jobsKeyPrefix = keyPrefix + "/jobs/"
	w.isLeader = false
	w.lastJobRevision = 0
	w.store = store.(*state.StateStoreEtcd)
	w.logger = log.New(os.Stdout, "worker/controller-etcd: ", log.Ldate|log.Ltime|log.LUTC)

	// Start the leadership manager if this node can become a leader
	if !appconfig.Config.GetBool("disallowLeadership") {
		err := w.leadershipManager()
		if err != nil {
			panic(fmt.Errorf("error while starting the leadership manager:\n%v", err))
		}
	}
}

// IsLeader returns true if this node is the leader of the cluster
func (w *ControllerEtcd) IsLeader() bool {
	return w.isLeader
}

// AddJob adds a job to the queue, returning its ID
func (w *ControllerEtcd) AddJob(job utils.JobData) (string, error) {
	// Ensure we have a leader
	hasLeader, err := w.hasLeader()
	if err != nil {
		return "", err
	}
	if !hasLeader {
		return "", errors.New("cluster does not have a leader")
	}

	// Get the job ID
	jobID := utils.CreateJobID(job)

	// Serialize the job
	message, err := json.Marshal(job)
	if err != nil {
		return "", err
	}

	// Write the job
	ctx, cancel := w.store.GetContext()
	_, err = w.store.GetClient().Put(ctx, w.jobsKeyPrefix+jobID, string(message))
	cancel()
	if err != nil {
		return "", err
	}

	w.logger.Printf("Added job %s: %s\n", jobID, string(message))

	return jobID, nil
}

// CompleteJob marks a job as complete
func (w *ControllerEtcd) CompleteJob(jobID string) error {
	// Delete the job
	ctx, cancel := w.store.GetContext()
	_, err := w.store.GetClient().Delete(ctx, w.jobsKeyPrefix+jobID)
	cancel()
	if err != nil {
		return err
	}

	w.logger.Println("Completed job", jobID)
	return nil
}

// WaitForJob waits until the job with the specified ID is done
func (w *ControllerEtcd) WaitForJob(jobID string, ch chan error) {
	// First, start a watcher to check when the key is deleted
	watchCtx, watchCancel := context.WithCancel(context.Background())
	watchCtx = clientv3.WithRequireLeader(watchCtx)
	go func() {
		rch := w.store.GetClient().Watch(watchCtx, w.jobsKeyPrefix+jobID)

		// Listen to events
		for resp := range rch {
			// Check for unrecoverable errors
			if resp.Err() != nil {
				ch <- fmt.Errorf("unrecoverable error from etcd watcher: %v", resp.Err())
				watchCancel()
				return
			}

			// Only look for deleted keys
			for _, event := range resp.Events {
				if event.Type == mvccpb.DELETE {
					ch <- nil
					watchCancel()
					return
				}
			}
		}
	}()

	// Check if the the key was already deleted
	reqCtx, reqCancel := w.store.GetContext()
	resp, err := w.store.GetClient().Get(reqCtx, w.jobsKeyPrefix+jobID)
	reqCancel()
	if err != nil {
		watchCancel()
		ch <- err
	} else if resp.Count == 0 {
		// Key was already deleted
		watchCancel()
		ch <- nil
	}
}

// Returns true if the cluster has a leader
func (w *ControllerEtcd) hasLeader() (bool, error) {
	// Get the current leader
	reqCtx, reqCancel := w.store.GetContext()
	resp, err := w.store.GetClient().Get(reqCtx, w.leaderKey)
	reqCancel()
	if err != nil {
		return false, err
	}

	return resp.Count > 0 && len(resp.Kvs[0].Value) > 0, nil
}

// Try to acquire leadership and watch for changes in leadership
func (w *ControllerEtcd) leadershipManager() error {
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
				ctx = clientv3.WithRequireLeader(ctx)
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

	// Receive jobs
	receiveJob := func(jobID string, message []byte) error {
		// Process the job
		job := utils.JobData{}
		err := json.Unmarshal(message, &job)
		if err != nil {
			return err
		}
		err = ProcessJob(job)
		if err != nil {
			return err
		}

		// Mark job as complete
		return w.CompleteJob(jobID)
	}

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
					// Process jobs
					jobID := strings.TrimPrefix(string(event.Kv.Key), w.jobsKeyPrefix)
					w.logger.Println("Received job", jobID)
					err := receiveJob(jobID, event.Kv.Value)
					if err != nil {
						w.logger.Printf("Error in job %s: %v\n", jobID, err)
						continue
					}
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
				// Process jobs
				jobID := strings.TrimPrefix(string(kv.Key), w.jobsKeyPrefix)
				w.logger.Println("Loaded job", jobID)
				err := receiveJob(jobID, kv.Value)
				if err != nil {
					w.logger.Printf("Error in job %s: %v\n", jobID, err)
					continue
				}
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
