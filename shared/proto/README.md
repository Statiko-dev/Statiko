# Protobuf definitions

## Requirements

- github.com/golang/protobuf/protoc-gen-go
- github.com/mitchellh/protoc-gen-go-json

```sh
GO111MODULE=off go get -u github.com/golang/protobuf/protoc-gen-go
GO111MODULE=off go get -u github.com/mitchellh/protoc-gen-go-json
```

## Build

```sh
protoc \
  *.proto \
  --go-json_out=. \
  --go_out=plugins=grpc:. \
  --go_opt=paths=source_relative
```

## Other files

The following Go files were created manually and are not auto-generated:

- `extra.go`
