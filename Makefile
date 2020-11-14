# Defaults
BUILD_ID ?= dev
BUILD_TIME ?= $(shell date -u +'%Y-%m-%dT%H:%M:%S')
COMMIT_HASH ?= $(shell git rev-parse --short HEAD)

# Version of pkger and go-acc
PKGER_VERSION ?= 0.17.1
GO_ACC_VERSION ?= 0.2.6

# Define LD flags for Go
LDFLAGS := "-X github.com/statiko-dev/statiko/buildinfo.ENV=production -X github.com/statiko-dev/statiko/buildinfo.BuildID=$(BUILD_ID) -X github.com/statiko-dev/statiko/buildinfo.BuildTime=$(BUILD_TIME) -X github.com/statiko-dev/statiko/buildinfo.CommitHash=$(COMMIT_HASH)"

# Performs a builld for all archs
all: get-tools build build-arm64 build-arm

# Build the app
build: fetch-deps build-default-app pkger build-amd64

# Runs the build step for all archs, but no preparation steps
build-all-archs: build-amd64 build-arm64 build-arm

# Fetches the tools that are required to build the app
get-tools:
	mkdir -p .bin
	# go-acc
	test -f .bin/go-acc || \
	  curl -sf https://gobinaries.com/github.com/ory/go-acc@v$(GO_ACC_VERSION) | PREFIX=.bin/ sh
	# pkger
	test -f .bin/pkger || \
	  curl -sf https://gobinaries.com/github.com/markbates/pkger/cmd/pkger@v$(PKGER_VERSION) | PREFIX=.bin/ sh

# Clean all compiled files
clean:
	rm -rfv default-app/dist/* || true
	rm -rfv .bin/agent* bin || true
	rm -rfv .bin/controller* bin || true
	rm -v agent/pkged.go || true
	rm -v controller/pkged.go || true

# Fetch all dependencies
fetch-deps:
	GO111MODULE=on \
      go get

# Build for amd64
build-amd64:
	$(call compile,linux-amd64,GOOS=linux GOARCH=amd64)

# Build for arm64v8
build-arm64:
	$(call compile,linux-arm64,GOOS=linux GOARCH=arm64)

# Build for armv7
build-arm:
	$(call compile,linux-armhf,GOOS=linux GOARCH=arm GOARM=7)

# Run pkger
pkger:
	#(cd controller; ../.bin/pkger -o controller)
	(cd agent; ../.bin/pkger -o agent)

# Build the default app
build-default-app:
	(cd default-app; npm ci; sh build.sh)

# Run tests
test:
	mkdir -p tests/results
	GO_ENV=test \
	  .bin/go-acc -o tests/results/coverage.txt $(shell go list ./...) -- -v

# Function that runs the compilation steps
define compile
    # Build the controller
	# Disable CGO so the binary is fully static
	CGO_ENABLED=0 \
	GO111MODULE=on \
	$(2) \
	  go build \
	    -ldflags $(LDFLAGS) \
	    -o .bin/controller_$(1) \
	    controller/main.go

	# Build the agent
	# Disable CGO so the binary is fully static
	CGO_ENABLED=0 \
	GO111MODULE=on \
	$(2) \
	  go build \
	    -ldflags $(LDFLAGS) \
	    -o .bin/agent_$(1) \
	    agent/main.go
endef
