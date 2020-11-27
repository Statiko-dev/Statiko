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

package httpsrvcore

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/buildinfo"
)

// Core is an abstract HTTP server
type Core struct {
	Router *gin.Engine
	Logger *log.Logger
	Port   int
	// Optional TLS certificate - if not set, server will not use TLS
	TLSCert *tls.Certificate

	srv       *http.Server
	stopCh    chan int
	restartCh chan int
	doneCh    chan int
	running   bool
}

// InitCore inits the core object
func (s *Core) InitCore() {
	s.running = false

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
	s.Router = gin.Default()
}

// IsRunning returns true if the HTTP server is running
func (s *Core) IsRunning() bool {
	return s.running
}

// Start the HTTP server; must be run in a goroutine with `go s.Start()`
func (s *Core) Start() {
	for {
		// Start the server in another channel
		go func() {
			// HTTP Server
			s.srv = &http.Server{
				Addr:              "0.0.0.0:" + strconv.Itoa(s.Port),
				Handler:           s.Router,
				ReadHeaderTimeout: 30 * time.Second,
				WriteTimeout:      2 * time.Hour,
				MaxHeaderBytes:    1 << 20,
			}

			// Check if we have a TLS certificate to enable TLS
			if s.TLSCert != nil {
				// TLS certificate and key
				tlsConfig := &tls.Config{
					MinVersion:   tls.VersionTLS12,
					Certificates: []tls.Certificate{*s.TLSCert},
				}
				s.srv.TLSConfig = tlsConfig

				// Start the server
				// Pass empty strings for the TLS certificate because it's already set in the tls.Config object
				s.running = true
				s.Logger.Printf("Starting API server on https://%s\n", s.srv.Addr)
				if err := s.srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
					s.Logger.Fatal(err)
				}
			} else {
				s.running = true
				s.Logger.Printf("Starting server on http://%s\n", s.srv.Addr)
				err := s.srv.ListenAndServe()
				if err != http.ErrServerClosed {
					s.Logger.Fatal(err)
				}
			}
		}()

		select {
		case <-s.stopCh:
			// We received an interrupt signal, shut down for good
			s.Logger.Println("Shutting down the server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := s.srv.Shutdown(ctx); err != nil {
				s.Logger.Printf("Server shutdown error: %v\n", err)
			}
			cancel()
			s.Logger.Println("server shut down")
			s.running = false
			s.doneCh <- 1
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.Logger.Println("Restarting the HTTP server")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := s.srv.Shutdown(ctx); err != nil {
				s.Logger.Fatal(err)
			}
			cancel()
			s.doneCh <- 1
			// Do not return, let the for loop repeat
		}
	}
}

// Restart the server
func (s *Core) Restart() {
	if s.running {
		s.restartCh <- 1
		<-s.doneCh
	}
}

// Stop the server
func (s *Core) Stop() {
	if s.running {
		s.stopCh <- 1
		<-s.doneCh
	}
}
