# Protobuf models

## Requirements

```sh
go get -u \
  github.com/golang/protobuf/protoc-gen-go \
  github.com/mitchellh/protoc-gen-go-json
```

## Compiling

```sh
protoc \
  *.proto \
  --go-json_out=. \
  --go_out=plugins=grpc:. \
  --go_opt=paths=source_relative
```
