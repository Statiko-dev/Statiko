## Starting the Application

````sh
GO111MODULE=on PORT=3000 go run main.go
````

## Building for production

### Install dependencies

````sh
apt-get update
apt-get install -y build-essential autoconf libtool cmake pkg-config git automake autogen ca-certificates clang llvm-dev libtool libxml2-dev uuid-dev libssl-dev swig patch make xz-utils cpio
````

### Compile the application

````sh
# Set Build ID number
BUILD_ID="123"

GO111MODULE=on \
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

CC=aarch64-linux-gnu-gcc-5 \
CXX=aarch64-linux-gnu-g++-5 \
GO111MODULE=on \
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

CC=arm-linux-gnueabihf-gcc-5 \
CXX=arm-linux-gnueabihf-g++-5 \
GO111MODULE=on \
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

CC=aarch64-linux-gnu-gcc-6 \
CXX=aarch64-linux-gnu-g++-6 \
GO111MODULE=on \
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

CC=arm-linux-gnueabihf-gcc-6 \
CXX=arm-linux-gnueabihf-g++-6 \
GO111MODULE=on \
GOOS=linux \
GOARCH=arm \
GOARM=7 \
CGO_ENABLED=1 \
  go build \
    -ldflags "-X smplatform/buildinfo.ENV=production -X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    -o bin/smplatform_linux_armhf
````
