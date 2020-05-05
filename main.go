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

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/statiko-dev/statiko/appconfig"
	"github.com/statiko-dev/statiko/middlewares"
	"github.com/statiko-dev/statiko/notifications"
	"github.com/statiko-dev/statiko/routes"
	"github.com/statiko-dev/statiko/sync"
	"github.com/statiko-dev/statiko/webserver"
	"github.com/statiko-dev/statiko/worker"
)

func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Init notifications client
	if err := notifications.InitNotifications(); err != nil {
		panic(err)
	}

	// Start all background workers
	worker.StartWorker()

	// If we're in production mode, set Gin to "release" mode
	if appconfig.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Start gin
	router := gin.Default()

	// Sync the state
	// Do this in a synchronous way to ensure the node starts up properly
	if err := sync.Run(); err != nil {
		panic(err)
	}

	// Ensure Nginx is running
	if err := webserver.Instance.EnsureServerRunning(); err != nil {
		panic(err)
	}

	// CORS
	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddExposeHeaders("Date")
	corsConfig.AllowOrigins = []string{"https://manage.statiko.dev"}
	if appconfig.ENV != "production" {
		// For development
		corsConfig.AllowOrigins = append(corsConfig.AllowOrigins, "http://localhost:5000")
	}
	router.Use(cors.New(corsConfig))

	// Add middlewares
	router.Use(middlewares.NodeName())

	// Add routes that don't require authentication
	// The middleware still checks for authentication, but it's optional
	{
		group := router.Group("/")
		group.Use(middlewares.Auth(false))

		group.GET("/status", routes.StatusHandler)
		group.GET("/status/:domain", routes.StatusHandler)
		group.GET("/info", routes.InfoHandler)
	}

	// Routes that require authorization
	{
		group := router.Group("/")
		group.Use(middlewares.Auth(true))
		group.POST("/site", routes.CreateSiteHandler)
		group.GET("/site", routes.ListSiteHandler)
		group.GET("/site/:domain", routes.ShowSiteHandler)
		group.DELETE("/site/:domain", routes.DeleteSiteHandler)
		group.PATCH("/site/:domain", routes.PatchSiteHandler)

		group.POST("/site/:domain/app", routes.DeploySiteHandler)
		group.PUT("/site/:domain/app", routes.DeploySiteHandler) // Alias

		group.GET("/state", routes.GetStateHandler)
		group.POST("/state", routes.PutStateHandler)
		group.PUT("/state", routes.PutStateHandler) // Alias

		group.POST("/uploadauth", routes.UploadAuthHandler)
		group.GET("/keyvaultinfo", routes.KeyVaultInfoHandler)

		group.POST("/sync", routes.SyncHandler)
		group.POST("/dhparams", routes.DHParamsHandler)
	}

	// HTTP Server
	server := &http.Server{
		Addr:           "0.0.0.0:" + appconfig.Config.GetString("port"),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Handle graceful shutdown on SIGINT
	idleConnsClosed := make(chan struct{})
	go func() {
		s := make(chan os.Signal, 1)
		signal.Notify(s, syscall.SIGINT, syscall.SIGTERM)
		<-s

		// We received an interrupt signal, shut down.
		if err := server.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			fmt.Printf("HTTP server shutdown error: %v\n", err)
		}
		close(idleConnsClosed)
	}()

	// Start the server
	if appconfig.Config.GetBool("tls.node.enabled") {
		fmt.Printf("Starting server on https://%s\n", server.Addr)
		tlsCertFile := appconfig.Config.GetString("tls.node.certificate")
		tlsKeyFile := appconfig.Config.GetString("tls.node.key")
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server.TLSConfig = tlsConfig
		if err := server.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != http.ErrServerClosed {
			panic(err)
		}
	} else {
		fmt.Printf("Starting server on http://%s\n", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			panic(err)
		}
	}

	<-idleConnsClosed
}
