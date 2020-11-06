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
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/agent/client"
	"github.com/statiko-dev/statiko/agent/state"
	"github.com/statiko-dev/statiko/buildinfo"
)

// HTTPServer is the HTTP server
type HTTPServer struct {
	State *state.AgentState
	RPC   *client.RPCClient

	logger    *log.Logger
	router    *gin.Engine
	srv       *http.Server
	stopCh    chan int
	restartCh chan int
	doneCh    chan int
	running   bool
}

// Init the object
func (s *HTTPServer) Init() {
	s.running = false

	// Initialize the logger
	s.logger = log.New(os.Stdout, "httpserver: ", log.Ldate|log.Ltime|log.LUTC)

	// Channel used to stop and restart the server
	s.stopCh = make(chan int)
	s.restartCh = make(chan int)
	s.doneCh = make(chan int)

	// Create the router object
	// If we're in production mode, set Gin to "release" mode
	if buildinfo.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Start gin
	s.router = gin.Default()

	// Add routes and middlewares
	s.setupRoutes()
}

// IsRunning returns true if the HTTP server is running
func (s *HTTPServer) IsRunning() bool {
	return s.running
}

// Start the HTTP server; must be run in a goroutine with `go s.Start()`
func (s *HTTPServer) Start() {
	for {
		// Start the server in another channel
		go func() {
			// HTTP Server
			s.srv = &http.Server{
				Addr:              "0.0.0.0:" + viper.GetString("serverPort"),
				Handler:           s.router,
				ReadTimeout:       2 * time.Hour,
				ReadHeaderTimeout: 30 * time.Second,
				WriteTimeout:      2 * time.Hour,
				MaxHeaderBytes:    1 << 20,
			}

			s.running = true
			s.logger.Printf("Starting HTTP server on http://%s\n", s.srv.Addr)
			if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
				panic(err)
			}
		}()

		select {
		case <-s.stopCh:
			// We received an interrupt signal, shut down for good
			s.logger.Println("Shutting down the HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				s.logger.Printf("HTTP server shutdown error: %v\n", err)
			}
			s.logger.Println("HTTP server shut down")
			s.running = false
			s.doneCh <- 1
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.logger.Println("Restarting the HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				panic(err)
			}
			s.doneCh <- 1
			// Do not return, let the for loop repeat
		}
	}
}

// Restart the server
func (s *HTTPServer) Restart() {
	if s.running {
		s.restartCh <- 1
		<-s.doneCh
	}
}

// Stop the server
func (s *HTTPServer) Stop() {
	if s.running {
		s.stopCh <- 1
		<-s.doneCh
	}
}

// Sets up the routes
func (s *HTTPServer) setupRoutes() {
	// ACME challenge
	s.router.GET("/.well-known/acme-challenge/:token", s.ACMEChallengeHandler)
}
