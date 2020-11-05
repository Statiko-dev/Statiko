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
	"time"

	"github.com/google/uuid"

	"github.com/statiko-dev/statiko/controller/certificates"
	"github.com/statiko-dev/statiko/shared/certutils"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Init the cert monitor worker
func (w *Worker) initCertMonitorWorker() {
	// Logger for this worker
	w.certMonitorLogger = log.New(os.Stdout, "worker/cert-monitor: ", log.Ldate|log.Ltime|log.LUTC)

	// Notifications sent
	w.certMonitorNotifications = make(map[string]int)

	// Send notifications when the certificate is expiring in N days
	w.certMonitorChecks = []int{-2, -1, 0, 1, 2, 3, 7, 14, 30}

	// Set the callback to refresh the certs
	w.certMonitorRefreshCh = make(chan int)
	w.State.CertRefresh = func() {
		w.certMonitorRefreshCh <- 1
	}
}

// In background, periodically check for expired certificates
func (w *Worker) startCertMonitorWorker(ctx context.Context) {
	// Set variables
	certMonitorInterval := time.Duration(24 * time.Hour) // Run every 24 hours

	go func() {
		// Run on ticker
		ticker := time.NewTicker(certMonitorInterval)
		defer ticker.Stop()

		// Run right away
		w.State.TriggerCertRefresh()

		for {
			select {
			case <-ticker.C:
				err := w.certMonitorWorker()
				if err != nil {
					w.certMonitorLogger.Println("Worker error:", err)
				}
			case <-w.certMonitorRefreshCh:
				err := w.certMonitorWorker()
				if err != nil {
					w.certMonitorLogger.Println("Worker error:", err)
				}
			case <-ctx.Done():
				w.certMonitorLogger.Println("Worker's context canceled")
				return
			}
		}
	}()
}

// Look up all certificates to look for those expiring
func (w *Worker) certMonitorWorker() error {
	w.certMonitorLogger.Println("Starting cert-monitor worker")

	// Go through all sites
	sites := w.State.GetSites()
	for _, el := range sites {
		// List of domains
		domains := append([]string{el.Domain}, el.Aliases...)

		// Check if there's a generated TLS certificate for this site
		if el.GeneratedTlsId != "" {
			// Errors are already logged
			_ = w.certMonitorInspectCert(el.GeneratedTlsId, true, el.EnableAcme, domains)
		}

		// Check if there's an imported TLS certificate for this site
		if el.ImportedTlsId != "" {
			// Errors are already logged
			_ = w.certMonitorInspectCert(el.ImportedTlsId, false, false, domains)
		}
	}

	w.certMonitorLogger.Println("Done")

	return nil
}

// Inspects a certificate
// For generated certs, if they're expired, it will re-generate them
// For imported certs, will only send a notification
func (w *Worker) certMonitorInspectCert(certId string, generated bool, acme bool, domains []string) error {
	now := time.Now()

	// Load the certificate and parse the PEM
	_, certPem, err := w.Certificates.GetCertificate(certId)
	if err != nil || len(certPem) == 0 {
		if err == certutils.NotFoundErr || len(certPem) == 0 {
			w.certMonitorLogger.Printf("Certificate %s not found\n", certId)
		} else {
			w.certMonitorLogger.Printf("Error while obtaining certificate %s: %s\n", certId, err)
		}
		return err
	}
	cert, err := certificates.GetX509(certPem)
	if err != nil {
		w.certMonitorLogger.Printf("Could not parse PEM data for certificate %s: %s", certId, err)
		return err
	}

	// For generated certificates, if they're about to expire re-generate them
	if generated {
		// Check if the certificate is self-signed
		selfSigned := len(cert.Issuer.Organization) > 0 &&
			cert.Issuer.Organization[0] == certificates.SelfSignedCertificateIssuer

		// Interval before we need to request new certs
		// ACME and self-signed certs have a different time before we need to update them
		interval := certificates.SelfSignedMinDays
		if acme {
			interval = certificates.ACMEMinDays
		}
		expired := cert.NotAfter.Before(now.Add(time.Duration((interval * 24)) * time.Hour))

		var (
			certObj         *pb.TLSCertificate
			keyPem, certPem []byte
		)

		// If the certificate has expired, or if it's self-signed but we want to use ACME, request a new certificate
		if acme && (expired || selfSigned) {
			w.certMonitorLogger.Printf("Requesting a new certificate for site %s from ACME\n", domains[0])

			// Get the certificate from ACME (this can be a blocking call)
			keyPem, certPem, err = w.Certificates.GenerateACMECertificate(domains...)
			if err != nil {
				w.certMonitorLogger.Printf("Error while requesting certificate from ACME for site %s: %s\n", domains[0], err)
				return err
			}
		} else if expired {
			// Self-signed certificate has expired, need to re-generate it
			w.certMonitorLogger.Printf("Certificate for site %s is expiring in less than %d days; regenerating it\n", domains[0], certificates.SelfSignedMinDays)

			// Get a new self-signed cert
			keyPem, certPem, err = certificates.GenerateTLSCert(domains...)
			if err != nil {
				w.certMonitorLogger.Printf("Error while generating a new certificate for site %s: %s\n", domains[0], err)
				return err
			}
		}

		// If we have a new certificate, store it
		if certObj != nil {
			// Get the X509 object
			certX509, err := certificates.GetX509(certPem)
			if err != nil {
				w.certMonitorLogger.Printf("Could not parse PEM data for the new certificate for site %s: %s", domains[0], err)
				return err
			}

			// Save the new certificate in the state
			certObj = &pb.TLSCertificate{
				Type: pb.TLSCertificate_SELF_SIGNED,
			}
			if acme {
				certObj.Type = pb.TLSCertificate_ACME
			}
			certObj.SetCertificateProperties(certX509)

			// Generate a certificate ID
			u, err := uuid.NewRandom()
			if err != nil {
				w.certMonitorLogger.Println("Error while generating a UUID:", err)
				return err
			}
			newCertId := u.String()

			// Set the certificate
			err = w.State.SetCertificate(certObj, certId, keyPem, certPem)
			if err != nil {
				w.certMonitorLogger.Printf("Could not store the new certificate for site %s: %s", domains[0], err)
				return err
			}

			// Replace the certificate
			err = w.State.ReplaceCertificate(certId, newCertId)
			if err != nil {
				w.certMonitorLogger.Printf("Could not replace the certificate for site %s: %s", domains[0], err)
				return err
			}
		}
	} else {
		// For imported certificates, send a notification if the cert has expired

		// Check if we sent a notification for expiring certificates already
		sent, found := w.certMonitorNotifications[domains[0]]
		if !found {
			sent = len(w.certMonitorChecks)
		}

		// Go through all checks
		for i := 0; i < len(w.certMonitorChecks); i++ {
			// If the certificate has expired
			// Note: we are assuming 24-hour days, which isn't always correct but it's fine in this case
			expired := cert.NotAfter.Before(now.Add(time.Duration((w.certMonitorChecks[i] * 24)) * time.Hour))
			if expired {
				// If we haven't already sent this notification
				if i < sent {
					message := "Certificate for " + domains[0] + " "
					switch w.certMonitorChecks[i] {
					case -2:
						message += "has expired over 2 days ago"
					case -1:
						message += "has expired 1 day ago"
					case 0:
						message += "has expired today"
					case 1:
						message += "is expiring today"
					default:
						message += fmt.Sprintf("expires in %d days", w.certMonitorChecks[i])
					}
					w.certMonitorNotifications[domains[0]] = i
					go w.Notifier.SendNotification(message)
					break
				}
			}
		}
	}

	return nil
}
