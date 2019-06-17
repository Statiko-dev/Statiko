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

package actions

import (
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/buffalo-pop/pop/popmw"
	contenttype "github.com/gobuffalo/mw-contenttype"
	paramlogger "github.com/gobuffalo/mw-paramlogger"
	"github.com/gobuffalo/x/sessions"
	"github.com/rs/cors"

	"smplatform/appconfig"
	"smplatform/models"
)

// Swagger
/*import (
	// Docs for swagger
	_ "smplatform/docs"

	buffaloSwagger "github.com/swaggo/buffalo-swagger"
	"github.com/swaggo/buffalo-swagger/swaggerFiles"
)*/

var app *buffalo.App

// App is where all routes and middleware for buffalo
// should be defined. This is the nerve center of your
// application.
//
// Routing, middleware, groups, etc... are declared TOP -> DOWN.
// This means if you add a middleware to `app` *after* declaring a
// group, that group will NOT have that new middleware. The same
// is true of resource declarations as well.
//
// It also means that routes are checked in the order they are declared.
// `ServeFiles` is a CATCH-ALL route, so it should always be
// placed last in the route declarations, as it will prevent routes
// declared after it to never be called.
//
// @title Platform node APIs
// @version 1.0
func App() *buffalo.App {
	if app == nil {
		// Buffalo app
		app = buffalo.New(buffalo.Options{
			Addr:         "0.0.0.0:" + appconfig.Config.GetString("port"),
			Env:          appconfig.ENV,
			SessionStore: sessions.Null{},
			PreWares: []buffalo.PreWare{
				cors.Default().Handler,
			},
			SessionName: "_node_session",
		})

		// Log request parameters (filters apply).
		app.Use(paramlogger.ParameterLogger)

		// Set the request content type to JSON
		app.Use(contenttype.Set("application/json"))

		// Wraps each request in a transaction.
		//  c.Value("tx").(*pop.Connection)
		// Remove to disable this.
		app.Use(popmw.Transaction(models.DB))

		// Load and initialize the Routes object
		routesContainer := &Routes{}
		if err := routesContainer.Init(); err != nil {
			panic(err)
		}

		// Authorization middleware
		app.Use(AuthMiddleware)
		app.Middleware.Skip(AuthMiddleware, routesContainer.StatusHandler)

		// Routes
		app.GET("/status", routesContainer.StatusHandler)

		app.POST("/adopt", routesContainer.AdoptHandler)

		app.POST("/site", routesContainer.CreateSiteHandler)
		app.GET("/site", routesContainer.ListSiteHandler)
		app.GET("/site/{site}", routesContainer.ShowSiteHandler)
		app.DELETE("/site/{site}", routesContainer.DeleteSiteHandler)

		app.POST("/site/{site}/deploy", routesContainer.DeployHandler)

		// Swagger
		//app.GET("/swagger/{doc:.*}", buffaloSwagger.WrapHandler(swaggerFiles.Handler))
	}

	return app
}
