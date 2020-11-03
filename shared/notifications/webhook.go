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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/spf13/viper"

	pb "github.com/statiko-dev/statiko/shared/proto"
)

// NotificationWebhook is the class that sends notifications to a webhook
type NotificationWebhook struct {
	url        string
	payloadKey string
	httpClient *http.Client
}

// Init method
func (n *NotificationWebhook) Init(optsI interface{}) error {
	opts, ok := optsI.(*pb.ClusterOptions_NotificationsWebhook)
	if !ok {
		return errors.New("invalid options object")
	}

	// URL
	if opts.Url == "" {
		return errors.New("empty webhook URL in configuration")
	}
	n.url = opts.Url

	// Payload key
	n.payloadKey = opts.PayloadKey
	if n.payloadKey == "" {
		// Default value is "message"
		n.payloadKey = "message"
	}

	// HTTP Client
	n.httpClient = &http.Client{
		Timeout: 15 * time.Second,
	}

	return nil
}

// SendNotification method
func (n *NotificationWebhook) SendNotification(message string) error {
	// Add the node name at the beginning of the message
	message = fmt.Sprintf("[statiko] (%s) %s", viper.GetString("nodeName"), message)

	// Request body is a JSON message in the format: `{<key>: string}`
	payload := make(map[string]string, 1)
	payload[n.payloadKey] = message
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(payload); err != nil {
		return err
	}

	// Send the request
	resp, err := n.httpClient.Post(n.url, "application/json", buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		b, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("Response contains an error (%d): %s\n", resp.StatusCode, string(b))
	}
	return nil
}
