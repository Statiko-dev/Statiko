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
	"log"
	"os"

	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/utils"
)

// ControllerFile is a worker controller that uses file as backend
type ControllerFile struct {
	logger *log.Logger
	jobs   map[string]chan int
}

// Init the object
func (w *ControllerFile) Init(store state.StateStore) {
	// Init variables
	w.logger = log.New(os.Stdout, "worker/controller-file: ", log.Ldate|log.Ltime|log.LUTC)
	w.jobs = make(map[string]chan int)

	// Start workers that require leadership, with a background context that can't be canceled
	startLeaderWorkers(context.Background())
}

// IsLeader returns always true because we're not in a cluster
func (w *ControllerFile) IsLeader() bool {
	return true
}

// AddJob adds a job to the queue, returning its ID
func (w *ControllerFile) AddJob(job utils.JobData) (string, error) {
	// Get the job ID
	jobID := utils.CreateJobID(job)

	// Create a new job
	w.jobs[jobID] = make(chan int)

	// Start the worker right away
	go func() {
		err := ProcessJob(job)
		if err != nil {
			w.logger.Printf("Error in job %s: %v\n", jobID, err)
			return
		}

		// Mark job as complete
		err = w.CompleteJob(jobID)
		if err != nil {
			w.logger.Printf("Error in job %s: %v\n", jobID, err)
			return
		}
	}()

	w.logger.Println("Added job", jobID)

	return jobID, nil
}

// CompleteJob marks a job as complete
func (w *ControllerFile) CompleteJob(jobID string) error {
	// Close the channel, if any
	ch, ok := w.jobs[jobID]
	if ok {
		close(ch)
	}

	// Delete the job from the list
	delete(w.jobs, jobID)

	w.logger.Println("Completed job", jobID)
	return nil
}

// WaitForJob waits until the job with the specified ID is done
func (w *ControllerFile) WaitForJob(jobID string, ch chan error) {
	// Check if the the job was already deleted
	j, ok := w.jobs[jobID]
	if !ok {
		// Job was already deleted, so likely already completed
		ch <- nil
	} else {
		go func() {
			// Wait for a message in the channel
			<-j
			ch <- nil
		}()
	}
}
