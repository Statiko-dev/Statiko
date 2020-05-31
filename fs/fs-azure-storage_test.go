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

package fs

import (
	"os"
	"testing"

	"github.com/statiko-dev/statiko/appconfig"
)

func TestAzureStorageInit(t *testing.T) {
	t.Run("empty credentials", func(t *testing.T) {
		o := &AzureStorage{}

		appconfig.Config.Set("repo.azure.account", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.azure.account, but got none")
		}
		appconfig.Config.Set("repo.azure.account", os.Getenv("REPO_AZURE_ACCOUNT"))

		appconfig.Config.Set("repo.azure.container", "")
		if o.Init() == nil {
			t.Fatal("Expected error for missing repo.azure.container, but got none")
		}
		appconfig.Config.Set("repo.azure.container", "fs-test")

		// Uses the service principal for authenticating
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &AzureStorage{}
		if err := obj.Init(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestAzureStorageSet(t *testing.T) {
	sharedSetTest(t, obj)()
}

func TestAzureStorageGet(t *testing.T) {
	sharedGetTest(t, obj)()
}
func TestAzureStorageList(t *testing.T) {
	sharedListTest(t, obj)()
}

func TestAzureStorageSetMetadata(t *testing.T) {
	sharedSetMetadataTest(t, obj)()
}

func TestAzureStorageDelete(t *testing.T) {
	sharedDeleteTest(t, obj)()
}
