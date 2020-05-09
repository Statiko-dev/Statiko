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
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/api/middlewares"
	"github.com/statiko-dev/statiko/appconfig"
)

// APIServer is the API server
type APIServer struct {
	router *gin.Engine
	srv    *http.Server
}

func (s *APIServer) Init() {
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

	// Create the server object
	// HTTP Server
	s.srv = &http.Server{
		Addr:           "0.0.0.0:" + appconfig.Config.GetString("port"),
		Handler:        s.router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

// Start the API server
func (s *APIServer) Start() {
	// Handle graceful shutdown on SIGINT
	idleConnsClosed := make(chan struct{})
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		// We received an interrupt signal, shut down.
		if err := s.srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			fmt.Printf("HTTP server shutdown error: %v\n", err)
		}
		close(idleConnsClosed)
	}()

	// Start the server
	if appconfig.Config.GetBool("tls.node.enabled") {
		fmt.Printf("Starting server on https://%s\n", s.srv.Addr)
		tlsCertFile := appconfig.Config.GetString("appRoot") + "/misc/node.cert.pem"
		tlsKeyFile := appconfig.Config.GetString("appRoot") + "/misc/node.key.pem"
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		s.srv.TLSConfig = tlsConfig
		if err := s.srv.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != http.ErrServerClosed {
			panic(err)
		}
	} else {
		s.srv.TLSConfig = nil
		fmt.Printf("Starting server on http://%s\n", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}

	<-idleConnsClosed
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

	// Add routes that don't require authentication
	// The middleware still checks for authentication, but it's optional
	{
		group := s.router.Group("/")
		group.Use(middlewares.Auth(false))

		group.GET("/status", StatusHandler)
		group.GET("/status/:domain", StatusHandler)
		group.GET("/info", InfoHandler)
	}

	// Routes that require authorization
	{
		group := s.router.Group("/")
		group.Use(middlewares.Auth(true))
		group.POST("/site", CreateSiteHandler)
		group.GET("/site", ListSiteHandler)
		group.GET("/site/:domain", ShowSiteHandler)
		group.DELETE("/site/:domain", DeleteSiteHandler)
		group.PATCH("/site/:domain", PatchSiteHandler)

		group.POST("/site/:domain/app", DeploySiteHandler)
		group.PUT("/site/:domain/app", DeploySiteHandler) // Alias

		group.GET("/clusterstatus", ClusterStatusHandler)

		group.GET("/state", GetStateHandler)
		group.POST("/state", PutStateHandler)
		group.PUT("/state", PutStateHandler) // Alias

		group.POST("/uploadauth", UploadAuthHandler)
		group.GET("/keyvaultinfo", KeyVaultInfoHandler)

		group.POST("/sync", SyncHandler)
		group.POST("/dhparams", DHParamsHandler)
	}
}
