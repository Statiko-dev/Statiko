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

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Notifications is the object that can be used to send notifications
type Notifications struct {
	senders []notificationSender
	logger  *log.Logger
}

// Init the right object that will send notifications
func (n *Notifications) Init(opts []*pb.ClusterOptions_NotificationsOpts) error {
	// Init the logger
	n.logger = log.New(os.Stdout, "notifications: ", log.Ldate|log.Ltime|log.LUTC)

	// Init the notifier objects
	n.senders = make([]notificationSender, len(opts))
	for i, o := range opts {
		switch x := o.Opts.(type) {
		// Webhook
		case *pb.ClusterOptions_NotificationsOpts_Webhook:
			sender := &NotificationWebhook{}
			err := sender.Init(x.Webhook)
			if err != nil {
				return err
			}
			n.senders[i] = sender
		default:
			n.logger.Println("Invalid notification method")
		}
	}

	return nil
}

// SendNotification sends a notification to the admins
// This function is meant to be run asynchronously (`go SendNotification(...)`), so it doesn't return any error
// Instead, errors are printed on the console
func (n *Notifications) SendNotification(message string) {
	var err error
	for i, sender := range n.senders {
		err = sender.SendNotification(message)
		if err != nil {
			n.logger.Printf("[Error] SendNotification returned an error from sender %d: %s", i, err)
		}
	}
}

// Interface for the classes
type notificationSender interface {
	Init(opts interface{}) error
	SendNotification(message string) error
}
