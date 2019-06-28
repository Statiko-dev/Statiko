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
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"smplatform/appconfig"
	"smplatform/db"
	"smplatform/middlewares"
	"smplatform/routes"
	"smplatform/startup"
)

func main() {
	// Seed rand
	rand.Seed(time.Now().UnixNano())

	// If we're in production mode, set Gin to "release" mode
	if appconfig.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Start gin
	router := gin.Default()

	// HTTP Server
	server := &http.Server{
		Addr:           "0.0.0.0:" + appconfig.Config.GetString("port"),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Enable TLS if necessary
	protocol := "http"
	if appconfig.Config.GetBool("tls.enabled") {
		tlsCertFile := appconfig.Config.GetString("tls.certificate")
		tlsKeyFile := appconfig.Config.GetString("tls.key")
		cer, err := tls.LoadX509KeyPair(tlsCertFile, tlsKeyFile)
		if err != nil {
			panic(err)
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cer},
			MinVersion:   tls.VersionTLS12,
		}
		server.TLSConfig = tlsConfig
		protocol = "https"
	}

	// Connect to the database
	db.Init()

	// Perform some cleanup
	// First, remove all pending deployments from the database
	if err := startup.RemovePendingDeployments(); err != nil {
		panic(err)
	}

	// Second, sync the state
	if err := startup.SyncState(); err != nil {
		panic(err)
	}

	// Add routes
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.GET("/status", routes.StatusHandler)
	router.GET("/info", routes.InfoHandler)

	// Routes that require authorization
	{
		authorized := router.Group("/")
		authorized.Use(middlewares.Auth())
		authorized.POST("/adopt", routes.AdoptHandler)
		authorized.POST("/site", routes.CreateSiteHandler)
		authorized.GET("/site", routes.ListSiteHandler)
		authorized.GET("/site/:site", routes.ShowSiteHandler)
		authorized.DELETE("/site/:site", routes.DeleteSiteHandler)
		authorized.POST("/site/:site/deploy", routes.DeployHandler)
	}

	// Start the server
	fmt.Printf("Starting server on %s://%s\n", protocol, server.Addr)
	server.ListenAndServe()
}
