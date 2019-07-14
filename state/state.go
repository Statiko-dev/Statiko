/*
Copyright © 2019 Alessandro Segala (@ItalyPaleAle)

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

package state

import (
	"time"
)

// Init loads the state from the store
func (m *Manager) Init() error {
	// Debug: use a pre-defined state
	sites := make([]SiteState, 3)
	tls1 := "site1"
	time1 := time.Unix(1562821787, 0)
	sites[0] = SiteState{
		ClientCaching:  true,
		TLSCertificate: &tls1,
		Domain:         "site1.local",
		Aliases:        []string{"site1-alias.local", "mysite.local"},
		App: &SiteApp{
			Name:    "app1",
			Version: "1",
			Time:    &time1,
		},
	}
	tls2 := "site2"
	time2 := time.Unix(1562820787, 0)
	sites[1] = SiteState{
		ClientCaching:  false,
		TLSCertificate: &tls2,
		Domain:         "site2.local",
		Aliases:        []string{"site2-alias.local"},
		App: &SiteApp{
			Name:    "app2",
			Version: "1.0.1",
			Time:    &time2,
		},
	}
	tls3 := "site3"
	time3 := time.Unix(1562820887, 0)
	sites[2] = SiteState{
		ClientCaching:  false,
		TLSCertificate: &tls3,
		Domain:         "site3.local",
		Aliases:        []string{"site3-alias.local"},
		App: &SiteApp{
			Name:    "app3",
			Version: "200",
			Time:    &time3,
		},
	}

	m.state = &NodeState{
		Sites: sites,
	}

	return nil
}

// GetSites return the list of all sites
func (m *Manager) GetSites() []SiteState {
	return m.state.Sites
}
