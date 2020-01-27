/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"smplatform/appconfig"
	"smplatform/notifications"
	"smplatform/utils"
)

// Logger for this file
var certsLogger *log.Logger

// Notifications sent
var certsNotifications map[string]int

// Send notifications when the certificate is expiring in N days
var checks []int

// In background, periodically check for expired certificates
func startCertsWorker() {
	// Set variables
	certsInterval := time.Duration(24 * time.Hour) // Run every 24 hours
	certsLogger = log.New(os.Stdout, "[certs]", log.Flags())
	certsNotifications = make(map[string]int)

	// Notification days
	checks = []int{-2, -1, 0, 1, 2, 3, 7, 14, 30}

	ticker := time.NewTicker(certsInterval)
	go func() {
		// Run right away
		err := certsWorker()
		if err != nil {
			certsLogger.Println("certs worker error:", err)
		}

		// Run on ticker
		for range ticker.C {
			err := certsWorker()
			if err != nil {
				certsLogger.Println("certs worker error:", err)
			}
		}
	}()
}

// Look up all certificates to look for those expiring
func certsWorker() error {
	certsLogger.Println("Starting certs worker")

	now := time.Now()

	// Scan all sites on disk
	appRoot := appconfig.Config.GetString("appRoot")
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
		certsLogger.Println("Reading certificate for site", site)
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

		// Check if we sent a notification for expiring certificates already
		sent, found := certsNotifications[site]
		if !found {
			sent = len(checks)
		}

		// Check expiry date
		// Note: we are assuming 24-hour days, which isn't always correct but it's fine in this case
		exp := cert.NotAfter
		for i := 0; i < len(checks); i++ {
			// If the certificate has expired
			if exp.Before(now.Add(time.Duration(checks[i]*24) * time.Hour)) {
				// If we haven't already sent this notification
				if i < sent {
					message := "Certificate for " + site + " "
					if checks[i] == -2 {
						message += "has expired over 2 days ago"
					} else if checks[i] == -1 {
						message += "has expired 1 day ago"
					} else if checks[i] == 0 {
						message += "has expired today"
					} else if checks[i] == 1 {
						message += "is expiring today"
					} else {
						message += fmt.Sprintf("expires in %d days", checks[i])
					}
					certsNotifications[site] = i
					go notifications.SendNotification(message)
					break
				}
			}
		}
	}

	return nil
}
