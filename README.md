## Starting the Application

````sh
GO111MODULE=on PORT=2265 go run main.go
````

## Building for production

### Install dependencies

Packages:

````sh
apt-get update
apt-get install -y git ca-certificates
````

### Packr

````sh
PACKR_VERSION=2.7.1
curl -LO "https://github.com/gobuffalo/packr/releases/download/v${PACKR_VERSION}/packr_${PACKR_VERSION}_linux_amd64.tar.gz"
tar xzf "packr_${PACKR_VERSION}_linux_amd64.tar.gz"
mv packr2 /usr/local/bin
````

### Compile the application

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
GO111MODULE=on \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_amd64
````

### Compile the application for tests

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
GO111MODULE=on \
  go test \
    -coverpkg=smplatform/... \
    -c \
    -tags e2etests \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_amd64.test
````

### Cross-compile for arm64/armhf

Build the app for arm64:

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
GO111MODULE=on \
GOOS=linux \
GOARCH=arm64 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_arm64
````

Build the app for armhf:

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
GO111MODULE=on \
GOOS=linux \
GOARCH=arm \
GOARM=7 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_armhf
````
