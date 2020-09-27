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

package api

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/controller/state"
	"github.com/statiko-dev/statiko/shared/fs"
)

// APIServer is the API server
type APIServer struct {
	Store fs.Fs
	State *state.Manager

	logger    *log.Logger
	router    *gin.Engine
	srv       *http.Server
	stopCh    chan int
	restartCh chan int
	running   bool
}

// Init the object
func (s *APIServer) Init() {
	s.running = false

	// Initialize the logger
	s.logger = log.New(os.Stdout, "api: ", log.Ldate|log.Ltime|log.LUTC)

	// Channel used to stop and restart the server
	s.stopCh = make(chan int)
	s.restartCh = make(chan int)

	// Create the router object
	// If we're in production mode, set Gin to "release" mode
	if appconfig.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Start gin
	s.router = gin.Default()

	// Enable CORS
	s.enableCORS()

	// Add routes and middlewares
	s.setupRoutes()
}

// IsRunning returns true if the API server is running
func (s *APIServer) IsRunning() bool {
	return s.running
}

// Start the API server; must be run in a goroutine with `go s.Start()`
func (s *APIServer) Start() {
	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	for {
		// Start the server in another channel
		go func() {
			// HTTP Server
			s.srv = &http.Server{
				Addr:              "0.0.0.0:" + appconfig.Config.GetString("port"),
				Handler:           s.router,
				ReadTimeout:       2 * time.Hour,
				ReadHeaderTimeout: 30 * time.Second,
				WriteTimeout:      2 * time.Hour,
				MaxHeaderBytes:    1 << 20,
			}

			s.running = true

			if appconfig.Config.GetBool("tls.node.enabled") {
				s.logger.Printf("Starting API server on https://%s\n", s.srv.Addr)
				tlsCertFile := appRoot + "misc/node.cert.pem"
				tlsKeyFile := appRoot + "misc/node.key.pem"
				tlsConfig := &tls.Config{
					MinVersion: tls.VersionTLS12,
				}
				s.srv.TLSConfig = tlsConfig
				if err := s.srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != http.ErrServerClosed {
					panic(err)
				}
			} else {
				s.srv.TLSConfig = nil
				s.logger.Printf("Starting API server on http://%s\n", s.srv.Addr)
				if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
					panic(err)
				}
			}
		}()

		select {
		case <-s.stopCh:
			// We received an interrupt signal, shut down for good
			s.logger.Println("Shutting down the API server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				s.logger.Printf("HTTP server shutdown error: %v\n", err)
			}
			s.running = false
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			s.logger.Println("Restarting the API server")
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				panic(err)
			}
			// Do not return, let the for loop repeat
		}
	}
}

// Restart the server
func (s *APIServer) Restart() {
	if s.running {
		s.restartCh <- 1
	}
}

// Stop the server
func (s *APIServer) Stop() {
	if s.running {
		s.stopCh <- 1
	}
}

// Enable CORS in the router
func (s *APIServer) enableCORS() {
	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddExposeHeaders("Date")
	corsConfig.AllowOrigins = []string{"https://manage.statiko.dev"}
	if appconfig.ENV != "production" {
		// For development
		corsConfig.AllowOrigins = append(corsConfig.AllowOrigins, "http://localhost:5000")
	}
	s.router.Use(cors.New(corsConfig))
}

// Sets up the routes
func (s *APIServer) setupRoutes() {
	// Add middlewares
	s.router.Use(s.NodeName())

	// ACME challenge
	//s.router.GET("/.well-known/acme-challenge/:token", routes.ACMEChallengeHandler)

	// Add routes that don't require authentication
	// The middleware still checks for authentication, but it's optional
	{
		group := s.router.Group("/")
		group.Use(s.Auth(false))

		//group.GET("/status", s.StatusHandler)
		//group.GET("/status/:domain", s.StatusHandler)
		group.GET("/info", s.InfoHandler)
	}

	// Routes that require authorization
	{
		group := s.router.Group("/")
		group.Use(s.Auth(true))
		group.POST("/site", s.CreateSiteHandler)
		group.GET("/site", s.ListSiteHandler)
		group.GET("/site/:domain", s.ShowSiteHandler)
		group.DELETE("/site/:domain", s.DeleteSiteHandler)
		group.PATCH("/site/:domain", s.PatchSiteHandler)

		group.POST("/site/:domain/app", s.DeploySiteHandler)
		group.PUT("/site/:domain/app", s.DeploySiteHandler) // Alias

		//group.GET("/clusterstatus", s.ClusterStatusHandler)

		group.GET("/state", s.GetStateHandler)
		group.POST("/state", s.PutStateHandler)
		group.PUT("/state", s.PutStateHandler) // Alias

		group.GET("/app", s.AppListHandler)
		group.POST("/app", s.AppUploadHandler)
		group.POST("/app/:name", s.AppUpdateHandler)
		group.DELETE("/app/:name", s.AppDeleteHandler)

		group.POST("/certificate", s.ImportCertificateHandler)
		group.GET("/certificate", s.ListCertificateHandler)
		group.DELETE("/certificate/:name", s.DeleteCertificateHandler)

		group.GET("/dhparams", s.DHParamsGetHandler)
		group.POST("/dhparams", s.DHParamsSetHandler)
	}
}
