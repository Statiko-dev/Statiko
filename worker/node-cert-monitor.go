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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/certificates"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
)

// Logger for this file
var nodeCertMonitorLogger *log.Logger

// In background, periodically renew the TLS certificate of the API server if needed
func startNodeCertMonitorWorker(ctx context.Context) {
	// Set variables
	nodeCertMonitorInterval := time.Duration(24 * time.Hour) // Run every 24 hours
	nodeCertMonitorLogger = log.New(os.Stdout, "worker/node-cert-monitor: ", log.Ldate|log.Ltime|log.LUTC)

	// If TLS is disabled, exit right away
	if !appconfig.Config.GetBool("tls.node.enabled") {
		return
	}

	go func() {
		// Wait for startup
		waitForStartup()

		// Run right away
		stop, err := nodeCertMonitorWorker()
		if err != nil {
			nodeCertMonitorLogger.Println("Worker error:", err)
		}
		if stop {
			nodeCertMonitorLogger.Println("Worker quits as not needed anymore")
			return
		}

		// Run on ticker
		ticker := time.NewTicker(nodeCertMonitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stop, err := nodeCertMonitorWorker()
				if err != nil {
					nodeCertMonitorLogger.Println("Worker error:", err)
				}
				if stop {
					nodeCertMonitorLogger.Println("Worker quits as not needed anymore")
					return
				}
			case <-ctx.Done():
				nodeCertMonitorLogger.Println("Worker's context canceled")
				return
			}
		}
	}()
}

// Check if the certificate of the API server is expiring, then renew it
// The first returned value is a boolean that, when true, causes the worker to just stop
func nodeCertMonitorWorker() (bool, error) {
	nodeCertMonitorLogger.Println("Starting node-cert-monitor worker")

	now := time.Now()
	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	// Read the TLS certificate
	nodeCertMonitorLogger.Println("Reading certificate")
	certData, err := ioutil.ReadFile(appRoot + "misc/node.cert.pem")
	if err != nil {
		return false, err
	}
	p, _ := pem.Decode(certData)
	if p == nil {
		return false, errors.New("Could not parse PEM data")
	}
	cert, err := x509.ParseCertificate(p.Bytes)
	if err != nil {
		return false, err
	}

	// Is this certificate self-signed?
	selfSigned := false
	if len(cert.Issuer.Organization) > 0 && cert.Issuer.Organization[0] == certificates.SelfSignedCertificateIssuer {
		selfSigned = true
	}

	// Check if the certificate has expired
	stop := false
	exp := cert.NotAfter
	if appconfig.Config.GetBool("tls.node.acme") {
		// Certificate issued by an ACME provider (e.g. Let's Encrypt)
		// We will need to request a new certificate if it's expiring
		// Additionally, if the certificate currently on disk was self-signed, it was just temporary, so we need to request the certificate
		expired := exp.Before(now.Add(time.Duration(certificates.ACMEMinDays*24) * time.Hour))
		if expired || selfSigned {
			nodeCertMonitorLogger.Println("Requesting a new certificate for node from ACME")

			// Queue a job
			job := utils.JobData{
				Type: utils.JobTypeACME,
				Data: utils.NodeAddress(),
			}
			jobID, err := state.Worker.AddJob(job)
			if err != nil {
				return false, err
			}

			// Wait for the job
			ch := make(chan error, 1)
			go state.Worker.WaitForJob(jobID, ch)
			err = <-ch
			close(ch)
			if err != nil {
				return false, err
			}

			// We'll queue a sync
			sync.QueueRun()
		} else {
			nodeCertMonitorLogger.Println("Certificate for node is still valid")
		}
	} else if selfSigned {
		// Certificate is self-signed, so let's just restart the server to have it regenerate if it's got less than N days left
		if exp.Before(now.Add(time.Duration(certificates.SelfSignedMinDays*24) * time.Hour)) {
			nodeCertMonitorLogger.Printf("Self-signed certificate for node is expiring in less than %d days; regenerating it\n", certificates.SelfSignedMinDays)

			// Queue a job
			job := utils.JobData{
				Type: utils.JobTypeTLSCertificate,
				Data: utils.NodeAddress(),
			}
			jobID, err := state.Worker.AddJob(job)
			if err != nil {
				return false, err
			}

			// Wait for the job
			ch := make(chan error, 1)
			go state.Worker.WaitForJob(jobID, ch)
			err = <-ch
			close(ch)
			if err != nil {
				return false, err
			}

			// We'll queue a sync
			sync.QueueRun()
		} else {
			nodeCertMonitorLogger.Println("Self-signed certificate for node is still valid")
		}
	} else {
		// Imported certificate
		// Check if it has already expired
		if exp.Before(now) {
			// Since the expired certificate was imported, nothing we can do here besides sending a notification, then exiting
			nodeCertMonitorLogger.Println("Imported certificate for node has expired; sending notification")
			go notifications.SendNotification("TLS certificate has expired for node " + appconfig.Config.GetString("nodeName"))
			stop = true
		} else {
			nodeCertMonitorLogger.Println("Certificate for node is still valid")
		}
	}

	nodeCertMonitorLogger.Println("Done")

	return stop, nil
}
