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

import "github.com/google/uuid"

// JobData is the struct of the
type JobData struct {
	Type string
	Data string
}

// Job type identifiers
const (
	JobTypeTLSCertificate = "tlscert"
	JobTypeACME           = "acme"
)

// Build job ID
func CreateJobID(job JobData) (jobID string) {
	switch job.Type {
	case JobTypeTLSCertificate, JobTypeACME:
		jobID = job.Type + "/" + SHA256String(job.Data)[:15]
	default:
		// Random
		jobID = job.Type + "/" + uuid.New().String()
	}
	return
}
