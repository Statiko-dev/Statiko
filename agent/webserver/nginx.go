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

package webserver

import (
	"log"
	"os"
	"text/template"

	"github.com/statiko-dev/statiko/agent/appmanager"
	"github.com/statiko-dev/statiko/agent/state"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// List of template files
var templateFiles = [3]string{"nginx.conf", "mime.types", "site.conf"}

// ConfigData is a map of each configuration file and its content
type ConfigData map[string][]byte

// NginxConfig creates the configuration for nginx
type NginxConfig struct {
	State       *state.AgentState
	AppManager  *appmanager.Manager
	ClusterOpts *pb.ClusterOptions

	logger    *log.Logger
	templates map[string]*template.Template
}

// Init initializes the object and loads the templates from file
func (n *NginxConfig) Init() error {
	// Logger
	n.logger = log.New(os.Stdout, "nginx: ", log.Ldate|log.Ltime|log.LUTC)

	// Load the templates
	n.templates = make(map[string]*template.Template, len(templateFiles))
	if err := n.loadTemplates(); err != nil {
		return err
	}

	return nil
}
