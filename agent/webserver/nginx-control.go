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
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

// Status returns the status of the Nginx server
func (n *NginxConfig) Status() (bool, error) {
	result, err := exec.Command("sh", "-c", viper.GetString("nginx.commands.status")).Output()
	if err != nil {
		n.logger.Printf("Error while checking Nginx server status: %s\n", err)
		return false, err
	}

	running := false
	resultStr := strings.TrimSpace(string(result))
	if resultStr == "1" {
		running = true
	}

	return running, nil
}

// ConfigTest runs the Nginx's config test command and returns whether the configuration is valid
func (n *NginxConfig) ConfigTest() (bool, error) {
	result, err := exec.Command("sh", "-c", viper.GetString("nginx.commands.test")).Output()
	// Ignore the error "exit status 1", which means the configuration test failed
	if err != nil && err.Error() != "exit status 1" {
		n.logger.Printf("Error while testing Nginx server configuration: %s\n", err)
		return false, err
	}

	ok := false
	resultStr := strings.TrimSpace(string(result))
	if resultStr == "" && err == nil {
		// Even if there was nothing printed out, the exit status can't be 1
		ok = true
	}

	return ok, nil
}

// EnsureServerRunning starts the Nginx server if it's not running already
func (n *NginxConfig) EnsureServerRunning() error {
	// Check if Nginx is running
	running, err := n.Status()
	if err != nil {
		return err
	}
	if !running {
		n.logger.Println("Starting Nginx server")
		_, err := exec.Command("sh", "-c", viper.GetString("nginx.commands.start")).Output()
		if err != nil {
			n.logger.Printf("Error while starting Nginx server: %s\n", err)
			return err
		}
	}

	return nil
}

// RestartServer restarts the Nginx server
func (n *NginxConfig) RestartServer() error {
	// Check if Nginx is running
	running, err := n.Status()
	if err != nil {
		return err
	}
	if running {
		// Reload the configuration
		n.logger.Println("Restarting Nginx server")
		_, err := exec.Command("sh", "-c", viper.GetString("nginx.commands.restart")).Output()
		if err != nil {
			n.logger.Printf("Error while restarting Nginx server: %s\n", err)
			return err
		}
	} else {
		// Start Nginx
		n.logger.Println("Starting Nginx server")
		_, err := exec.Command("sh", "-c", viper.GetString("nginx.commands.start")).Output()
		if err != nil {
			n.logger.Printf("Error while starting Nginx server: %s\n", err)
			return err
		}
	}

	return nil
}
