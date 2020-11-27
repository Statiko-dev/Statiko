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
	"crypto/tls"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/controller/cluster"
	"github.com/statiko-dev/statiko/controller/state"
	controllerutils "github.com/statiko-dev/statiko/controller/utils"
	"github.com/statiko-dev/statiko/shared/azurekeyvault"
	"github.com/statiko-dev/statiko/shared/fs"
	"github.com/statiko-dev/statiko/shared/httpsrvcore"
)

// APIServer is the API server
type APIServer struct {
	httpsrvcore.Core

	Store   fs.Fs
	State   *state.Manager
	Cluster *cluster.Cluster
	AKV     *azurekeyvault.Client
	TLSCert *tls.Certificate
}

// Init the object
func (s *APIServer) Init() {
	// Init the core object
	s.Core.Logger = log.New(buildinfo.LogDestination, "api: ", log.Ldate|log.Ltime|log.LUTC)
	s.Core.TLSCert = s.TLSCert
	s.Core.Port = viper.GetInt("controller.apiPort")
	s.Core.InitCore()

	// Enable CORS
	s.enableCORS()

	// Add routes and middlewares
	s.setupRoutes()
}

// Enable CORS in the router
func (s *APIServer) enableCORS() {
	corsConfig := cors.DefaultConfig()
	corsConfig.AddAllowHeaders("Authorization")
	corsConfig.AddExposeHeaders("Date")
	corsConfig.AllowOrigins = []string{"https://manage.statiko.dev"}
	if buildinfo.ENV != "production" {
		// For development
		corsConfig.AllowOrigins = append(corsConfig.AllowOrigins, "http://localhost:5000")
	}
	s.Core.Router.Use(cors.New(corsConfig))
}

// Sets up the routes
func (s *APIServer) setupRoutes() {
	// Add middlewaress
	s.Core.Router.Use(s.NodeName())

	// Add routes that don't require authentication
	// The middleware still checks for authentication, but it's optional
	{
		group := s.Core.Router.Group("/")
		group.Use(controllerutils.AuthGinMiddleware(false))

		group.GET("/info", s.InfoHandler)
	}

	// Routes that require authorization
	{
		group := s.Core.Router.Group("/")
		group.Use(controllerutils.AuthGinMiddleware(true))
		group.POST("/site", s.CreateSiteHandler)
		group.GET("/site", s.ListSiteHandler)
		group.GET("/site/:domain", s.ShowSiteHandler)
		group.DELETE("/site/:domain", s.DeleteSiteHandler)
		group.PATCH("/site/:domain", s.PatchSiteHandler)

		group.POST("/site/:domain/app", s.DeploySiteHandler)
		group.PUT("/site/:domain/app", s.DeploySiteHandler) // Alias

		group.GET("/clusterstatus", s.ClusterStatusHandler)

		group.GET("/state", s.GetStateHandler)
		group.POST("/state", s.PutStateHandler)
		group.PUT("/state", s.PutStateHandler) // Alias

		group.GET("/app", s.AppListHandler)
		group.POST("/app", s.AppUploadHandler)
		group.POST("/app/:name", s.AppUpdateHandler)
		group.DELETE("/app/:name", s.AppDeleteHandler)

		group.POST("/certificate", s.ImportCertificateHandler)
		group.GET("/certificate", s.ListCertificateHandler)
		group.DELETE("/certificate/:id", s.DeleteCertificateHandler)
		group.POST("/certificate/refresh", s.RefreshCertificateHandler)

		group.GET("/dhparams", s.DHParamsGetHandler)
		group.POST("/dhparams", s.DHParamsSetHandler)
	}
}
