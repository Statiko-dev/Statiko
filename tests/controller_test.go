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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	petname "github.com/dustinkirkland/golang-petname"
	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/controller/api"
	controllerApp "github.com/statiko-dev/statiko/controller/app"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/shared/utils"
	"github.com/stretchr/testify/assert"
)

// TestController is the runner for the tests for the controller app
func TestController(t *testing.T) {
	suite := ControllerTestSuite{}
	suite.Run(t)
}

// Testing suite for the controller app
type ControllerTestSuite struct {
	app        *controllerApp.Controller
	ctx        context.Context
	stop       context.CancelFunc
	client     *http.Client
	apiUrl     string
	authHeader string
	configDir  string
}

// Run the test sequence
func (s *ControllerTestSuite) Run(t *testing.T) {
	// Configure the environment
	s.configureEnvironment(t)

	// Start the app
	ok := t.Run("start-app", s.startApp)
	if !ok {
		return
	}

	// Run the test sequence
	for n, f := range s.testSequence() {
		ok = t.Run(n, f)
		if !ok {
			return
		}
	}

	// Stop the app
	s.stop()

	// Print all logs
	all, _ := ioutil.ReadAll(buildinfo.LogDestination)
	_ = all
	//fmt.Println(string(all))
}

// Configure the environment
func (s *ControllerTestSuite) configureEnvironment(t *testing.T) {
	t.Helper()

	// Create a temporary dir for the state
	stateDir := t.TempDir()

	// Set the configuration file
	var err error
	s.configDir, err = filepath.Abs("etc/statiko/")
	if !assert.NoError(t, err) {
		return
	}
	os.Setenv("STATIKO_CONFIG_PATH", s.configDir)

	// Set the path to the state file
	os.Setenv("STATIKO_STATE_STORE", "file")
	os.Setenv("STATIKO_STATE_FILE_PATH", stateDir+"/state")

	// Get an available port for the API server
	apiPort, err := utils.GetFreePort()
	if !assert.NoError(t, err) {
		return
	}

	// Get another port for the gRPC server
	grpcPort, err := utils.GetFreePort()
	if !assert.NoError(t, err) {
		return
	}

	// Pre-shared key
	psk := petname.Generate(4, "-")
	os.Setenv("STATIKO_AUTH_PSK_KEY", psk)
	s.authHeader = "Bearer " + psk

	// API server
	os.Setenv("STATIKO_CONTROLLER_API_PORT", strconv.Itoa(apiPort))
	os.Setenv("STATIKO_CONTROLLER_TLS_ENABLED", "0")
	s.apiUrl = fmt.Sprintf("%s://localhost:%d", "http", apiPort)

	// gRPC server
	os.Setenv("STATIKO_CONTROLLER_GRPC_PORT", strconv.Itoa(grpcPort))

	// Init the HTTP client
	s.client = &http.Client{
		// Request timeout = 15 seconds
		Timeout: 15 * time.Second,
	}
}

// Start the app
func (s *ControllerTestSuite) startApp(t *testing.T) {
	// Start the controller app
	wait := make(chan int)
	go func() {
		s.ctx, s.stop = context.WithCancel(context.Background())
		s.app = &controllerApp.Controller{
			NoWorker: true,
			StartedCb: func() {
				wait <- 1
			},
		}
		err := s.app.Run(s.ctx)
		if !assert.NoError(t, err) {
			return
		}
	}()

	// Wait for initialization
	<-wait
}

// Returns the test sequence
func (s *ControllerTestSuite) testSequence() (seq map[string]func(t *testing.T)) {
	seq = make(map[string]func(t *testing.T))

	// Request the /info endpoint to ensure the app is running
	seq["test-info-endpoint"] = func(t *testing.T) {
		// Request
		res, err := s.client.Get(s.apiUrl + "/info")
		if !assert.NoError(t, err) {
			return
		}
		defer res.Body.Close()
		read, err := ioutil.ReadAll(res.Body)
		if !assert.NoError(t, err) {
			return
		}
		if !assert.NotEmpty(t, read) {
			return
		}

		// Parse the response
		data := &api.InfoResponse{}
		err = json.Unmarshal(read, data)
		if !assert.NoError(t, err) {
			return
		}

		assert.NotEmpty(t, data.NodeName)
		assert.True(t, strings.HasPrefix(data.Version, "canary"))
		assert.NotEmpty(t, data.AuthMethods)
		assert.Contains(t, data.AuthMethods, "psk")
		assert.Contains(t, data.AuthMethods, "azureAD")
		assert.NotEmpty(t, data.AzureAD)
		assert.NotEmpty(t, data.AzureAD.ClientID)
		assert.True(t, strings.HasPrefix(data.AzureAD.AuthorizeURL, "https://login.microsoftonline.com"))
		assert.True(t, strings.HasPrefix(data.AzureAD.TokenURL, "https://login.microsoftonline.com"))
	}

	// Add a simple site
	seq["add-simple-site"] = func(t *testing.T) {
		// Make the request
		body := &pb.Site{
			Domain: "site1.test",
		}
		req, err := s.newRequest("POST", "/site", body)
		if !assert.NoError(t, err) {
			return
		}
		res, err := s.client.Do(req)
		if !assert.NoError(t, err) {
			return
		}
		defer res.Body.Close()

		// Read the response
		data, err := ioutil.ReadAll(res.Body)
		if !assert.NoError(t, err) {
			return
		}
		fmt.Println(string(data))
	}

	return
}

// Creates a new request with the authorization header set and with an optional body (for POST/PUT requests) that will be encoded as JSON
func (s *ControllerTestSuite) newRequest(method string, url string, body interface{}) (*http.Request, error) {
	// If the url doesn't start with http:// or https:// add the apiUrl prefix
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = s.apiUrl + url
	}

	// If there's data for the body, convert it to JSON
	var bodyContent io.Reader
	if body != nil {
		enc, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyContent = bytes.NewBuffer(enc)
	}

	// Create the request
	req, err := http.NewRequestWithContext(s.ctx, method, url, bodyContent)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", s.authHeader)

	return req, nil
}
