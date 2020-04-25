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

var notifier notificationSender
var logger *log.Logger

// InitNotifications creates the right object that will send notifications
func InitNotifications() error {
	// Init the logger
	logger = log.New(os.Stdout, "notifications: ", log.Ldate|log.Ltime|log.LUTC)

	// Init the notifier object
	method := notificationsMethod()
	method = strings.ToLower(method)
	if method == "" || method == "off" {
		logger.Println("Notifications are off")
		return nil
	}

	var obj notificationSender = nil
	switch method {
	case "webhook":
		obj = &NotificationWebhook{}
		if err := obj.Init(); err != nil {
			return err
		}
	default:
		logger.Println("Invalid notification method")
	}

	notifier = obj
	return nil
}

// SendNotification sends a notification to the admin
// This function is meant to be run asynchronously (`go SendNotification(...)`), so it doesn't return any error
// Instead, errors are printed on the console
func SendNotification(message string) {
	if notifier == nil {
		return
	}

	if err := notifier.SendNotification(message); err != nil {
		logger.Println("[Error] SendNotification returned an error:", err)
	}
}

// Returns the method used for notifications
func notificationsMethod() string {
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
