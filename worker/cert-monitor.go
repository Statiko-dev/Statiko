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
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"time"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/state"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/utils"
)

// Logger for this file
var certMonitorLogger *log.Logger

// Notifications sent
var certMonitorNotifications map[string]int

// Send notifications when the certificate is expiring in N days
var certMonitorChecks []int

// In background, periodically check for expired certificates
func startCertMonitorWorker(ctx context.Context) {
	// Set variables
	certMonitorInterval := time.Duration(24 * time.Hour) // Run every 24 hours
	certMonitorLogger = log.New(os.Stdout, "worker/cert-monitor: ", log.Ldate|log.Ltime|log.LUTC)
	certMonitorNotifications = make(map[string]int)

	// Notification days
	certMonitorChecks = []int{-2, -1, 0, 1, 2, 3, 7, 14, 30}

	go func() {
		// Wait for startup
		waitForStartup()

		// Run right away
		err := certMonitorWorker()
		if err != nil {
			certMonitorLogger.Println("Worker error:", err)
		}

		// Run on ticker
		ticker := time.NewTicker(certMonitorInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := certMonitorWorker()
				if err != nil {
					certMonitorLogger.Println("Worker error:", err)
				}
			case <-ctx.Done():
				certMonitorLogger.Println("Worker's context canceled")
				return
			}
		}
	}()
}

// Look up all certificates to look for those expiring
func certMonitorWorker() error {
	certMonitorLogger.Println("Starting cert-monitor worker")

	now := time.Now()
	needsSync := false

	// Scan all sites on disk
	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}
	sites, err := ioutil.ReadDir(appRoot + "sites/")
	if err != nil {
		return err
	}
	for _, el := range sites {
		site := el.Name()

		// Check if there's a TLS certificate for this site
		hasCert, err := utils.FileExists(appRoot + "sites/" + site + "/tls/certificate.pem")
		if err != nil {
			return err
		}
		if !hasCert {
			continue
		}

		// Read the TLS certificate
		certMonitorLogger.Println("Reading certificate for site", site)
		certData, err := ioutil.ReadFile(appRoot + "sites/" + site + "/tls/certificate.pem")
		if err != nil {
			return err
		}
		p, _ := pem.Decode(certData)
		if p == nil {
			return errors.New("Could not parse PEM data")
		}
		cert, err := x509.ParseCertificate(p.Bytes)
		if err != nil {
			return err
		}

		// Is this certificate self-signed?
		selfSigned := false
		if len(cert.Issuer.Organization) > 0 && cert.Issuer.Organization[0] == utils.SelfSignedCertificateIssuer {
			selfSigned = true
		}

		// Check if we sent a notification for expiring certificates already
		sent, found := certMonitorNotifications[site]
		if !found {
			sent = len(certMonitorChecks)
		}

		// Check expiry date
		exp := cert.NotAfter
		if selfSigned {
			// Certificate is self-signed, so let's just restart the server to have it regenerate if it's got less than 7 days left
			if exp.Before(now.Add(time.Duration(time.Duration(utils.SelfSignedMinDays*24) * time.Hour))) {
				certMonitorLogger.Println("Certificate for site", site, "is expiring in less than 7 days; regenerating it")

				// Queue a job
				job := utils.JobData{
					Type: utils.JobTypeTLSCertificate,
					Data: strings.Join(cert.DNSNames, ","),
				}
				jobID, err := state.Worker.AddJob(job)
				if err != nil {
					return err
				}

				// Wait for the job
				ch := make(chan error, 1)
				go state.Worker.WaitForJob(jobID, ch)
				err = <-ch
				close(ch)
				if err != nil {
					return err
				}

				// We'll queue a sync
				needsSync = true
			}
		} else {
			// Note: we are assuming 24-hour days, which isn't always correct but it's fine in this case
			for i := 0; i < len(certMonitorChecks); i++ {
				// If the certificate has expired
				if exp.Before(now.Add(time.Duration(certMonitorChecks[i]*24) * time.Hour)) {
					// If we haven't already sent this notification
					if i < sent {
						message := "Certificate for " + site + " "
						if certMonitorChecks[i] == -2 {
							message += "has expired over 2 days ago"
						} else if certMonitorChecks[i] == -1 {
							message += "has expired 1 day ago"
						} else if certMonitorChecks[i] == 0 {
							message += "has expired today"
						} else if certMonitorChecks[i] == 1 {
							message += "is expiring today"
						} else {
							message += fmt.Sprintf("expires in %d days", certMonitorChecks[i])
						}
						certMonitorNotifications[site] = i
						go notifications.SendNotification(message)
						break
					}
				}
			}
		}
	}

	// If we need to queue a sync
	if needsSync {
		sync.QueueRun()
	}

	certMonitorLogger.Println("Done")

	return nil
}
