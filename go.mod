module github.com/ItalyPaleAle/statiko

go 1.13

require (
	github.com/Azure/azure-pipeline-go v0.2.2
	github.com/Azure/azure-sdk-for-go v39.2.0+incompatible
	github.com/Azure/azure-storage-blob-go v0.8.0
	github.com/Azure/go-autorest/autorest v0.9.6
	github.com/Azure/go-autorest/autorest/adal v0.8.2
	github.com/Azure/go-autorest/autorest/to v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.2.0 // indirect
	github.com/coreos/etcd v3.3.18+incompatible // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/etcd-io/etcd v3.3.18+incompatible
	github.com/gin-gonic/gin v1.5.0
	github.com/gobuffalo/packd v1.0.0
	github.com/gobuffalo/packr/v2 v2.7.1
	github.com/google/renameio v0.1.0
	github.com/google/uuid v1.1.1 // indirect
	github.com/spf13/viper v1.6.2
	golang.org/x/crypto v0.0.0-20200219234226-1ad67e1f0ef4
	google.golang.org/grpc v1.26.0
	gopkg.in/yaml.v2 v2.2.8
)

// See https://github.com/etcd-io/etcd/issues/11563
replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.0.0
