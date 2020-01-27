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
	"context"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
	"smplatform/middlewares"
	"smplatform/notifications"
	"smplatform/routes"
	"smplatform/sync"
	"smplatform/webserver"
	"smplatform/worker"
)

func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// Init notifications client
	if err := notifications.InitNotifications(); err != nil {
		panic(err)
	}

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

	// Start all background workers
	worker.StartWorker()

	// Ensure Nginx is running
	if err := webserver.Instance.EnsureServerRunning(); err != nil {
		panic(err)
	}

	// Add middlewares
	router.Use(middlewares.NodeName())

	// Add routes
	router.GET("/status", routes.StatusHandler)
	router.GET("/status/:domain", routes.StatusHandler)
	router.GET("/info", routes.InfoHandler)

	// Routes that require authorization
	{
		authorized := router.Group("/")
		authorized.Use(middlewares.Auth())
		authorized.POST("/site", routes.CreateSiteHandler)
		authorized.GET("/site", routes.ListSiteHandler)
		authorized.GET("/site/:domain", routes.ShowSiteHandler)
		authorized.DELETE("/site/:domain", routes.DeleteSiteHandler)
		authorized.PATCH("/site/:domain", routes.PatchSiteHandler)

		authorized.POST("/site/:domain/app", routes.DeploySiteHandler)
		authorized.PUT("/site/:domain/app", routes.DeploySiteHandler) // Alias

		authorized.GET("/state", routes.GetStateHandler)
		authorized.POST("/state", routes.PutStateHandler)
		authorized.PUT("/state", routes.PutStateHandler) // Alias

		authorized.POST("/uploadauth", routes.UploadAuthHandler)
		authorized.GET("/keyvaultname", routes.KeyVaultNameHandler)
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
		signal.Notify(s, os.Interrupt, syscall.SIGTERM)
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
