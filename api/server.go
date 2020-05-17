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
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/api/middlewares"
	"github.com/statiko-dev/statiko/api/routes"
	"github.com/statiko-dev/statiko/appconfig"
)

// APIServer is the API server
type APIServer struct {
	router    *gin.Engine
	srv       *http.Server
	restartCh chan int
	running   bool
}

func (s *APIServer) Init() {
	s.running = false

	// Channel used to restart the server
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

// Start the API server
func (s *APIServer) Start() {
	// Handle graceful shutdown on SIGINT, SIGTERM and SIGQUIT
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
	)

	appRoot := appconfig.Config.GetString("appRoot")
	if !strings.HasSuffix(appRoot, "/") {
		appRoot += "/"
	}

	for {
		// Start the server in background
		go func() {
			// HTTP Server
			s.srv = &http.Server{
				Addr:           "0.0.0.0:" + appconfig.Config.GetString("port"),
				Handler:        s.router,
				ReadTimeout:    10 * time.Second,
				WriteTimeout:   10 * time.Second,
				MaxHeaderBytes: 1 << 20,
			}

			s.running = true

			if appconfig.Config.GetBool("tls.node.enabled") {
				logger.Printf("Starting server on https://%s\n", s.srv.Addr)
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
				logger.Printf("Starting server on http://%s\n", s.srv.Addr)
				if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
					panic(err)
				}
			}
		}()

		select {
		case <-sigCh:
			// We received an interrupt signal, shut down for good
			logger.Println("Received signal to terminate the app; shutting down the API server")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := s.srv.Shutdown(ctx); err != nil {
				logger.Printf("HTTP server shutdown error: %v\n", err)
			}
			s.running = false
			return
		case <-s.restartCh:
			// We received a signal to restart the server
			logger.Println("Restarting the API server")
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
	s.router.Use(middlewares.NodeName())

	// ACME challenge
	s.router.GET("/.well-known/acme-challenge/:token", routes.ACMEChallengeHandler)

	// Add routes that don't require authentication
	// The middleware still checks for authentication, but it's optional
	{
		group := s.router.Group("/")
		group.Use(middlewares.Auth(false))

		group.GET("/status", routes.StatusHandler)
		group.GET("/status/:domain", routes.StatusHandler)
		group.GET("/info", routes.InfoHandler)
	}

	// Routes that require authorization
	{
		group := s.router.Group("/")
		group.Use(middlewares.Auth(true))
		group.POST("/site", routes.CreateSiteHandler)
		group.GET("/site", routes.ListSiteHandler)
		group.GET("/site/:domain", routes.ShowSiteHandler)
		group.DELETE("/site/:domain", routes.DeleteSiteHandler)
		group.PATCH("/site/:domain", routes.PatchSiteHandler)

		group.POST("/site/:domain/app", routes.DeploySiteHandler)
		group.PUT("/site/:domain/app", routes.DeploySiteHandler) // Alias

		group.GET("/clusterstatus", routes.ClusterStatusHandler)

		group.GET("/state", routes.GetStateHandler)
		group.POST("/state", routes.PutStateHandler)
		group.PUT("/state", routes.PutStateHandler) // Alias

		group.POST("/uploadauth", routes.UploadAuthHandler)
		group.GET("/keyvaultinfo", routes.KeyVaultInfoHandler)

		group.POST("/certificate", routes.ImportCertificateHandler)
		group.GET("/certificate", routes.ListCertificateHandler)
		group.DELETE("/certificate/:name", routes.DeleteCertificateHandler)
		group.POST("/dhparams", routes.DHParamsHandler)

		group.POST("/sync", routes.SyncHandler)
	}
}
