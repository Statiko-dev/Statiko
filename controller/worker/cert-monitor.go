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
	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/shared/notifications"
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

		// Run on ticker
		ticker := time.NewTicker(certMonitorInterval)
		defer ticker.Stop()

		// Run right away
		state.Instance.TriggerRefreshCerts()

		for {
			select {
			case <-ticker.C:
				err := certMonitorWorker()
				if err != nil {
					certMonitorLogger.Println("Worker error:", err)
				}
			case <-state.Instance.RefreshCerts:
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

	// If there's a sync running, wait for it to be done
	for sync.IsRunning() {
		certMonitorLogger.Println("Waiting for sync to be complete")
		time.Sleep(2 * time.Second)
	}

	// Init variablles
	now := time.Now()
	needsSync := false
	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	// Go through all sites
	sites := state.Instance.GetSites()

	// Scan all sites on disk
	for _, el := range sites {
		// Check if there's a TLS certificate for this site
		if el.TLS == nil || el.TLS.Type == "" {
			continue
		}

		// Read the TLS certificate
		certMonitorLogger.Println("Reading certificate for site", el.Domain)
		certData, err := ioutil.ReadFile(appRoot + "sites/" + el.Domain + "/tls/certificate.pem")
		if err != nil {
			if os.IsNotExist(err) {
				// This should never happen... regardless, just move on to the next site
				continue
			}
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

		// List of domains
		domains := append([]string{el.Domain}, el.Aliases...)

		// Check if we sent a notification for expiring certificates already
		sent, found := certMonitorNotifications[el.Domain]
		if !found {
			sent = len(certMonitorChecks)
		}

		// Check expiry date
		exp := cert.NotAfter
		switch el.TLS.Type {
		case state.TLSCertificateSelfSigned:
			// Certificate is self-signed, so let's just restart the server to have it regenerate if it's got less than N days left
			if exp.Before(now.Add(time.Duration(certificates.SelfSignedMinDays*24) * time.Hour)) {
				certMonitorLogger.Printf("Certificate for site %s is expiring in less than %d days; regenerating it\n", el.Domain, certificates.SelfSignedMinDays)

				// Queue a job
				job := utils.JobData{
					Type: utils.JobTypeTLSCertificate,
					Data: strings.Join(domains, ","),
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
		case state.TLSCertificateImported, state.TLSCertificateAzureKeyVault:
			// Imported certificate
			for i := 0; i < len(certMonitorChecks); i++ {
				// If the certificate has expired
				// Note: we are assuming 24-hour days, which isn't always correct but it's fine in this case
				if exp.Before(now.Add(time.Duration(certMonitorChecks[i]*24) * time.Hour)) {
					// If we haven't already sent this notification
					if i < sent {
						message := "Certificate for " + el.Domain + " "
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
						certMonitorNotifications[el.Domain] = i
						go notifications.SendNotification(message)
						break
					}
				}
			}
		case state.TLSCertificateACME:
			// Certificate issued by an ACME provider (e.g. Let's Encrypt)
			// We will need to request a new certificate if it's expiring
			// Additionally, if the certificate currently on disk was self-signed, it was just temporary, so we need to request the certificate
			expired := exp.Before(now.Add(time.Duration(certificates.ACMEMinDays*24) * time.Hour))
			selfSigned := len(cert.Issuer.Organization) > 0 &&
				cert.Issuer.Organization[0] == certificates.SelfSignedCertificateIssuer
			if expired || selfSigned {
				certMonitorLogger.Printf("Requesting a new certificate for site %s from ACME\n", el.Domain)

				// Queue a job
				job := utils.JobData{
					Type: utils.JobTypeACME,
					Data: strings.Join(domains, ","),
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
		}
	}

	// If we need to queue a sync
	if needsSync {
		sync.QueueRun()
	}

	certMonitorLogger.Println("Done")

	return nil
}
