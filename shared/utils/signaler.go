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

package utils

import (
	"sync"
)

// Signaler is used to send a notification via a signal to multiple subscribers
type Signaler struct {
	sync.Mutex
	chans []chan int
}

// Subscribe adds a channel as a receiver
func (s *Signaler) Subscribe(ch chan int) {
	s.Lock()
	defer s.Unlock()

	s.chans = append(s.chans, ch)
}

// Unsubscribe removes a channel from the list of receivers
func (s *Signaler) Unsubscribe(ch chan int) {
	s.Lock()
	defer s.Unlock()

	i := 0
	for _, el := range s.chans {
		if ch == el {
			continue
		}
		s.chans[i] = el
		i++
	}
	s.chans = s.chans[0:i]
}

// Broadcast a message to all channels and returns the number of messages sent
func (s *Signaler) Broadcast() int {
	s.Lock()
	defer s.Unlock()

	i := 0
	for _, ch := range s.chans {
		ch <- 1
		i++
	}
	return i
}
