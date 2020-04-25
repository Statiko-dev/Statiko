module github.com/ItalyPaleAle/statiko

go 1.14

require (
	github.com/Azure/azure-pipeline-go v0.2.2
	github.com/Azure/azure-sdk-for-go v41.3.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.8.0
	github.com/Azure/go-autorest/autorest v0.10.0
	github.com/Azure/go-autorest/autorest/adal v0.8.3
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/Luzifer/go-dhparam v1.0.0
	github.com/coreos/etcd v3.3.20+incompatible // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/etcd-io/etcd v3.3.20+incompatible
	github.com/gin-contrib/cors v1.3.1
	github.com/gin-gonic/gin v1.6.2
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr/v2 v2.8.0
	github.com/google/renameio v0.1.0
	github.com/google/uuid v1.1.1 // indirect
	github.com/spf13/viper v1.6.3
	golang.org/x/crypto v0.0.0-20200423211502-4bdfaf469ed5
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.8
)

// See https://github.com/etcd-io/etcd/issues/11563
replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
