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
	"flag"
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/statiko-dev/statiko/buildinfo"
	controllerApp "github.com/statiko-dev/statiko/controller/app"
)

func TestMain(m *testing.M) {
	// Ensure that the GO_ENV variable is set to "test"
	if os.Getenv("GO_ENV") != "test" {
		panic("For tests, environmental variable `GO_ENV=test` must be set")
	}
	fmt.Println("env:", buildinfo.ENV)

	// Parse the flags
	flag.Parse()

	// Init the PRNG
	rand.Seed(time.Now().UnixNano())

	// Start the tests
	os.Exit(m.Run())

	// Create the controller
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	controller := controllerApp.Controller{}
	controller.Run(ctx)
}
