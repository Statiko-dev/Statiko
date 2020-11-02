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
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/Azure/azure-storage-blob-go/azblob"
	pb "github.com/statiko-dev/statiko/shared/proto"
	"github.com/statiko-dev/statiko/utils"
	"github.com/stretchr/testify/assert"
)

var azContainerUrl azblob.ContainerURL

func TestAzureStorageInit(t *testing.T) {
	opts := &pb.ClusterOptions_StorageAzure{
		AccessKey:      os.Getenv("STATIKO_REPO_AZURE_ACCESS_KEY"),
		EndpointSuffix: os.Getenv("STATIKO_REPO_AZURE_ENDPOINT_SUFFIX"),
		CustomEndpoint: os.Getenv("STATIKO_REPO_AZURE_CUSTOM_ENDPOINT"),
		NoTls:          utils.IsTruthy(os.Getenv("STATIKO_REPO_AZURE_NO_TLS")),
	}

	// Generate a container name
	container := "statikotest" + RandString(6)

	t.Run("empty account", func(t *testing.T) {
		o := &AzureStorage{}

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing account, but got none")
		}
		opts.Account = os.Getenv("STATIKO_REPO_AZURE_ACCOUNT")

		if o.Init(opts) == nil {
			t.Fatal("Expected error for missing container, but got none")
		}
		opts.Container = container
	})
	t.Run("init correctly", func(t *testing.T) {
		obj = &AzureStorage{}
		if err := obj.Init(opts); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("create test container", func(t *testing.T) {
		// Create the container
		objAzure := obj.(*AzureStorage)
		u, err := url.Parse(objAzure.storageURL)
		if !assert.NoError(t, err) {
			return
		}
		azContainerUrl = azblob.NewContainerURL(*u, objAzure.storagePipeline)
		_, err = azContainerUrl.Create(context.Background(), azblob.Metadata{}, azblob.PublicAccessNone)
		if !assert.NoError(t, err) {
			return
		}
		t.Log("Created container", container)
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

func TestAzureStorageGetMetadata(t *testing.T) {
	sharedGetMetadataTest(t, obj)()
}

func TestAzureStorageSetMetadata(t *testing.T) {
	sharedSetMetadataTest(t, obj)()
}

func TestAzureStorageDelete(t *testing.T) {
	sharedDeleteTest(t, obj)()
}

func TestAzureStorageCleanup(t *testing.T) {
	_, err := azContainerUrl.Delete(context.Background(), azblob.ContainerAccessConditions{})
	if !assert.NoError(t, err) {
		return
	}
	t.Log("Deleted container", azContainerUrl.String())
}
