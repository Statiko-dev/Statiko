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

package httpserver

import (
	"log"

	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/agent/client"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/shared/httpsrvcore"
	"github.com/statiko-dev/statiko/shared/utils"
)

// HTTPServer is the HTTP server
type HTTPServer struct {
	httpsrvcore.Core

	State *state.AgentState
	RPC   *client.RPCClient
}

// Init the object
func (s *HTTPServer) Init() {
	// Init the logger
	s.Core.Logger = log.New(buildinfo.LogDestination, "httpserver: ", log.Ldate|log.Ltime|log.LUTC)

	// Get the port to bind to
	// If the port is 0, then automatically select one that's available
	var err error
	port := viper.GetInt("serverPort")
	if port == 0 {
		port, err = utils.GetFreePort()
		if err != nil {
			s.Core.Logger.Fatal(err)
		}

		// Set the updated port in viper (the value is kept in memory anyways)
		viper.Set("serverPort", port)
	}

	// Init the core object
	s.Core.Port = port
	s.Core.InitCore()

	// Add routes and middlewares
	s.setupRoutes()
}

// Sets up the routes
func (s *HTTPServer) setupRoutes() {
	// ACME challenge
	s.Core.Router.GET("/.well-known/acme-challenge/:token", s.ACMEChallengeHandler)
}
