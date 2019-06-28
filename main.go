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

	// HTTP Server
	server := &http.Server{
		Addr:           "0.0.0.0:" + appconfig.Config.GetString("port"),
		Handler:        router,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Start the server
	if appconfig.Config.GetBool("tls.enabled") {
		fmt.Printf("Starting server on https://%s\n", server.Addr)
		tlsCertFile := appconfig.Config.GetString("tls.certificate")
		tlsKeyFile := appconfig.Config.GetString("tls.key")
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		server.TLSConfig = tlsConfig
		if err := server.ListenAndServeTLS(tlsCertFile, tlsKeyFile); err != nil {
			panic(err)
		}
	} else {
		fmt.Printf("Starting server on http://%s\n", server.Addr)
		if err := server.ListenAndServe(); err != nil {
			panic(err)
		}
	}
}
