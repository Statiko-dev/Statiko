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

package notifications

import (
	"log"
	"os"
	"strings"

	"github.com/statiko-dev/statiko/appconfig"
)

// Notifications is the object that can be used to send notifications
type Notifications struct {
	sender notificationSender
	logger *log.Logger
}

// Init the right object that will send notifications
func (n *Notifications) Init() error {
	// Init the logger
	n.logger = log.New(os.Stdout, "notifications: ", log.Ldate|log.Ltime|log.LUTC)

	// Init the notifier object
	method := strings.ToLower(n.notificationsMethod())
	if method == "" {
		n.logger.Println("Notifications are off")
		return nil
	}

	switch method {
	case "webhook":
		n.sender = &NotificationWebhook{}
		if err := n.sender.Init(); err != nil {
			return err
		}
	default:
		n.logger.Println("Invalid notification method")
	}

	return nil
}

// SendNotification sends a notification to the admin
// This function is meant to be run asynchronously (`go SendNotification(...)`), so it doesn't return any error
// Instead, errors are printed on the console
func (n *Notifications) SendNotification(message string) {
	if err := n.sender.SendNotification(message); err != nil {
		n.logger.Println("[Error] SendNotification returned an error:", err)
	}
}

// Returns the method used for notifications
func (n *Notifications) notificationsMethod() string {
	method := strings.ToLower(appconfig.Config.GetString("notifications.method"))

	// Check if notifications are enabled
	if method == "" || method == "off" || method == "no" || method == "0" {
		return ""
	}

	return method
}

// Interface for the classes
type notificationSender interface {
	Init() error
	SendNotification(message string) error
}
