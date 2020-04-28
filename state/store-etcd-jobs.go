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
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/google/uuid"
	"github.com/prometheus/common/log"
)

// AddJob adds a job to the queue
func (s *stateStoreEtcd) AddJob(job string) error {
	// Generate a random job id
	jobID := uuid.New().String()

	// Write the job
	ctx, cancel := s.getContext()
	_, err := s.client.Put(ctx, s.jobsKeyPrefix+jobID, jobID)
	cancel()
	if err != nil {
		return err
	}

	return nil
}

// Starts the worker that processes all job requests
func (s *stateStoreEtcd) startWorker() error {
	// Start leader election
	leaderChan := make(chan bool)
	go func() {
		err := s.runElection(leaderChan)
		if err != nil {
			panic(fmt.Errorf("error while running etcd election campaign:\n%v", err))
		}
	}()

	// Listen for changes in leadership
	go func() {
		var cancel context.CancelFunc
		var err error
		for _ = range leaderChan {
			// Cancel all existing contexts if any
			if cancel != nil {
				cancel()
				cancel = nil
			}

			// If we're leader now, start listening for jobs
			if s.isLeader {
				cancel, err = s.listenForJobs()
				if err != nil {
					panic(fmt.Errorf("error while listening for jobs:\n%v", err))
				}
			}
		}
	}()

	return nil
}

// Listens for jobs
func (s *stateStoreEtcd) listenForJobs() (context.CancelFunc, error) {
	s.lastJobRevision = 0

	// Start watching for jobs
	var ctx context.Context
	var cancel context.CancelFunc
	go func() {
		ctx, cancel = context.WithCancel(context.Background())
		rch := s.client.Watch(ctx, s.jobsKeyPrefix, clientv3.WithPrefix())

		// Listen to events
		// TODO: WATCH FOR CLOSED CHANNELS
		for resp := range rch {
			// Only look for new jobs
			for _, event := range resp.Events {
				if event.Kv.ModRevision > s.lastJobRevision && event.IsCreate() {
					// Process jobs here
					fmt.Println("Received job:", string(event.Kv.Key), string(event.Kv.Value))
					s.lastJobRevision = event.Kv.ModRevision
				}
			}
		}
	}()

	// Request the first list of jobs
	reqCtx, reqCancel := s.getContext()
	resp, err := s.client.Get(reqCtx, s.jobsKeyPrefix, clientv3.WithPrefix())
	reqCancel()
	if err != nil {
		cancel()
		return nil, err
	}
	s.lastJobRevision = resp.Header.GetRevision()
	if resp != nil && resp.Header.Size() > 0 && len(resp.Kvs) > 0 {
		for _, kv := range resp.Kvs {
			if kv.Value != nil && len(kv.Value) > 0 {
				// Process jobs here
				fmt.Println("Loaded job:", string(kv.Key), string(kv.Value))
			}
		}
	}

	return cancel, nil
}

// Runs a leader election
func (s *stateStoreEtcd) runElection(leaderChan chan bool) error {
	// Adapted from https://gist.github.com/thrawn01/c007e6a37b682d3899910e33243a3cdc
	var errChan chan error
	ctx := context.Background()

	setLeader := func(set bool) {
		// Only report changes in leadership
		if s.isLeader == set {
			return
		}
		if set {
			logger.Println("We are leaders now")
		} else {
			logger.Println("We lost leadership")
		}
		s.isLeader = set
		leaderChan <- set
	}

	createSession := func(id int64) (session *concurrency.Session, election *concurrency.Election, err error) {
		session, err = concurrency.NewSession(
			s.client,
			concurrency.WithTTL(etcdLockDuration),
			concurrency.WithContext(ctx),
			concurrency.WithLease(clientv3.LeaseID(id)),
		)
		if err != nil {
			session = nil
			return
		}
		election = concurrency.NewElection(session, s.electionKey)
		return
	}

	session, election, err := createSession(0)
	if err != nil {
		return err
	}

	go func() {
		defer close(leaderChan)

		for {
			observe := election.Observe(ctx)

			// Discover who if any, is leader of this election
			node, err := election.Leader(ctx)
			if err != nil {
				if err != concurrency.ErrElectionNoLeader {
					logger.Printf("[Error] Error while determining election leader: %s\n", err)
					goto reconnect
				}
			} else if string(node.Kvs[0].Value) == s.clusterMemberId {
				// Resuming an election from which we previously had leadership
				// If resign takes longer than our TTL then lease is expired and we are no
				// longer leader anyway.
				election = concurrency.ResumeElection(session, s.electionKey,
					string(node.Kvs[0].Key), node.Kvs[0].CreateRevision)
				err = election.Resign(ctx)
				if err != nil {
					logger.Printf("[Error] Error while resigning leadership after reconnect: %s\n", err)
					goto reconnect
				}
			}
			// Reset leadership if we had it previously
			setLeader(false)

			// Attempt to become leader
			errChan = make(chan error)
			go func() {
				// Make this a non blocking call so we can check for session close
				errChan <- election.Campaign(ctx, s.clusterMemberId)
			}()

			select {
			case err = <-errChan:
				if err != nil {
					if err == context.Canceled {
						return
					}
					// NOTE: Campaign currently does not return an error if session expires
					logger.Printf("[Error] Error while campaigning for leader: %s\n", err)
					session.Close()
					goto reconnect
				}
			case <-ctx.Done():
				session.Close()
				return
			case <-session.Done():
				goto reconnect
			}

			// If Campaign() returned without error, we are leader
			setLeader(true)

			// Observe changes to leadership
			for {
				select {
				case resp, ok := <-observe:
					if !ok {
						// NOTE: Observe will not close if the session expires, we must
						// watch for session.Done()
						session.Close()
						goto reconnect
					}
					if string(resp.Kvs[0].Value) == s.clusterMemberId {
						setLeader(true)
					} else {
						// We are not leader
						setLeader(false)
						break
					}
				case <-ctx.Done():
					if s.isLeader {
						// If resign takes longer than our TTL then lease is expired and we are no
						// longer leader anyway.
						ctx, cancel := s.getContext()
						if err = election.Resign(ctx); err != nil {
							log.Errorf("while resigning leadership during shutdown: %s", err)
						}
						cancel()
					}
					session.Close()
					return
				case <-session.Done():
					goto reconnect
				}
			}

		reconnect:
			setLeader(false)

			for {
				session, election, err = createSession(0)
				if err != nil {
					if err == context.Canceled {
						return
					}
					logger.Printf("[Error] Error while creating new session: %s", err)
					tick := time.NewTicker(1 * time.Second)
					select {
					case <-ctx.Done():
						tick.Stop()
						return
					case <-tick.C:
						tick.Stop()
					}
					continue
				}
				break
			}
		}
	}()

	// Wait until we have a leader before returning
	for {
		resp, err := election.Leader(ctx)
		if err != nil {
			if err != concurrency.ErrElectionNoLeader {
				return err
			}
			time.Sleep(time.Millisecond * 300)
			continue
		}
		// If we are not leader, notify the channel
		if string(resp.Kvs[0].Value) != s.clusterMemberId {
			leaderChan <- false
		}
		break
	}
	return nil
}
