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

	"github.com/statiko-dev/statiko/appconfig"
)

// NotificationWebhook is the class that sends notifications to a webhook
type NotificationWebhook struct {
	url        string
	httpClient *http.Client
}

// Init method
func (n *NotificationWebhook) Init() error {
	// URL
	url := appconfig.Config.GetString("notifications.webhook.url")
	if url == "" {
		return errors.New("Empty webhook URL in configuration")
	}
	n.url = url

	// HTTP Client
	n.httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}

	return nil
}

// SendNotification method
func (n *NotificationWebhook) SendNotification(message string) error {
	// Add the node name at the beginning of the message
	message = fmt.Sprintf("[statiko] (%s) %s", appconfig.Config.GetString("nodeName"), message)

	// Request body is a JSON message in the format: `{<key>: string}`
	key := appconfig.Config.GetString("notifications.webhook.payloadKey")
	payload := make(map[string]string, 1)
	payload[key] = message
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
