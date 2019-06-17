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

package main

import (
	"crypto/tls"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo/servers"

	"smplatform/actions"
	"smplatform/appconfig"
)

func getServer() (servers.Server, error) {
	server := &http.Server{}

	var buffaloServer servers.Server
	if appconfig.Config.GetBool("tls.enabled") {
		tlsCertFile := appconfig.Config.GetString("tls.certificate")
		tlsKeyFile := appconfig.Config.GetString("tls.key")
		cer, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			return nil, err
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cer},
			MinVersion:   tls.VersionTLS12,
		}
		server.TLSConfig = tlsConfig

		buffaloServer = servers.WrapTLS(server, tlsCertFile, tlsKeyFile)
	} else {
		buffaloServer = servers.Wrap(server)
	}

	return buffaloServer, nil
}

// main is the starting point for your Buffalo application.
// You can feel free and add to this `main` method, change
// what it does, etc...
// All we ask is that, at some point, you make sure to
// call `app.Serve()`, unless you don't want to start your
// application that is. :)
func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Load app's config
	if err := appconfig.Config.Init(); err != nil {
		return
	}

	// Create a http server and wrap it to become a Buffalo server
	buffaloServer, err := getServer()
	if err != nil {
		log.Fatal(err)
		return
	}

	// Start app
	app := actions.App()
	if err := app.Serve(buffaloServer); err != nil {
		log.Fatal(err)
		return
	}
}
