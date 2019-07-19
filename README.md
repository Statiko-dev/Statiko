## Starting the Application

````sh
GO111MODULE=on PORT=2265 GOFLAGS=-mod=vendor go run main.go
````

## Building for production

### Install dependencies

Packages:

````sh
apt-get update
apt-get install -y build-essential autoconf libtool cmake pkg-config git automake autogen ca-certificates clang llvm-dev libtool libxml2-dev uuid-dev libssl-dev swig patch make xz-utils cpio
````

### Packr

````sh
PACKR_VERSION=2.5.1
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
GOFLAGS=-mod=vendor \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_amd64
````

### Cross-compile for arm64/armhf on Ubuntu

First, ensure you have the toolchain installed:

````sh
apt-get update
apt-get install -y crossbuild-essential-armhf crossbuild-essential-arm64
````

Build the app for arm64:

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
CC=aarch64-linux-gnu-gcc-5 \
CXX=aarch64-linux-gnu-g++-5 \
GO111MODULE=on \
GOFLAGS=-mod=vendor \
GOOS=linux \
GOARCH=arm64 \
CGO_ENABLED=1 \
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
CC=arm-linux-gnueabihf-gcc-5 \
CXX=arm-linux-gnueabihf-g++-5 \
GO111MODULE=on \
GOFLAGS=-mod=vendor \
GOOS=linux \
GOARCH=arm \
GOARM=7 \
CGO_ENABLED=1 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_armhf
````

### Cross-compile for arm64/armhf on Debian

First, ensure you have the toolchain installed:

````sh
apt-get update
apt-get install -y \
  build-essential \
  crossbuild-essential-armhf \
  crossbuild-essential-arm64 \
  autoconf libtool cmake pkg-config git automake autogen ca-certificates clang llvm-dev libtool libxml2-dev uuid-dev libssl-dev swig patch make xz-utils cpio
````

Build for arm64:

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
CC=aarch64-linux-gnu-gcc-6 \
CXX=aarch64-linux-gnu-g++-6 \
GO111MODULE=on \
GOFLAGS=-mod=vendor \
GOOS=linux \
GOARCH=arm64 \
CGO_ENABLED=1 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_arm64
````

Build for armhf:

````sh
# Set Build ID number
BUILD_ID="123"

# Run packr
packr2

# Build
CC=arm-linux-gnueabihf-gcc-6 \
CXX=arm-linux-gnueabihf-g++-6 \
GO111MODULE=on \
GOFLAGS=-mod=vendor \
GOOS=linux \
GOARCH=arm \
GOARM=7 \
CGO_ENABLED=1 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_armhf
````
