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

package appmanager

import (
	"crypto/rsa"
	"log"
	"strings"

	"github.com/google/renameio"
	"github.com/spf13/viper"

	"github.com/statiko-dev/statiko/agent/certificates"
	"github.com/statiko-dev/statiko/agent/state"
	agentutils "github.com/statiko-dev/statiko/agent/utils"
	"github.com/statiko-dev/statiko/buildinfo"
	"github.com/statiko-dev/statiko/shared/fs"
	pb "github.com/statiko-dev/statiko/shared/proto"
)

// Manager contains helper functions to manage apps and sites
type Manager struct {
	State        *state.AgentState
	Certificates *certificates.AgentCertificates
	Fs           fs.Fs
	ClusterOpts  *pb.ClusterOptions

	codesignKey *rsa.PublicKey
	appRoot     string
	log         *log.Logger
	manifests   map[string]*agentutils.AppManifest
}

// Init the object
func (m *Manager) Init() error {
	// Logger
	m.log = log.New(buildinfo.LogDestination, "appmanager: ", log.Ldate|log.Ltime|log.LUTC)

	// Init properties from config
	m.appRoot = viper.GetString("appRoot")
	if !strings.HasSuffix(m.appRoot, "/") {
		m.appRoot += "/"
	}

	// Init properties from the ClusterOpts
	m.codesignKey = m.ClusterOpts.CodesignKey()

	return nil
}

// SyncState ensures that the state of the filesystem matches the desired one
func (m *Manager) SyncState(sites []*pb.Site) (updated bool, err error) {
	updated = false

	// To start, ensure the basic folders exist
	err = m.InitAppRoot()
	if err != nil {
		return
	}

	// Default app (writing this to disk always, regardless)
	err = m.WriteDefaultApp()
	if err != nil {
		return
	}

	// Misc files
	var u bool
	u, err = m.SyncMiscFiles()
	if err != nil {
		return
	}
	updated = updated || u

	// Apps: ensure we have the right ones
	err = m.SyncApps(sites)
	if err != nil {
		return
	}

	// Lastly, sync the site folders
	u, err = m.SyncSiteFolders(sites)
	if err != nil {
		return
	}
	updated = updated || u

	return
}

// ActivateApp points a site to an app, by creating the symbolic link
func (m *Manager) ActivateApp(app string, domain string) error {
	// Switch the www folder to an app staged
	if err := renameio.Symlink(m.appRoot+"apps/"+app, m.appRoot+"sites/"+domain+"/www"); err != nil {
		return err
	}
	return nil
}

// ManifestForApp returns the manifest for an app, if any
func (m *Manager) ManifestForApp(name string) *agentutils.AppManifest {
	return m.manifests[name]
}
