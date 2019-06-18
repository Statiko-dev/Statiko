# Welcome to Buffalo!

Thank you for choosing Buffalo for your web development needs.

## Database Setup

It looks like you chose to set up your application using a database! Fantastic!

The first thing you need to do is open up the "database.yml" file and edit it to use the correct usernames, passwords, hosts, etc... that are appropriate for your environment.

You will also need to make sure that **you** start/install the database of your choice. Buffalo **won't** install and start it for you.

### Create Your Databases

Ok, so you've edited the "database.yml" file and started your database, now Buffalo can create the databases in that file for you:

````sh
buffalo pop create -a
````

## Starting the Application

Buffalo ships with a command that will watch your application and automatically rebuild the Go binary and any assets for you. To do that run the "buffalo dev" command:

````sh
GO111MODULE=on ADDR=0.0.0.0 PORT=3000 buffalo dev
````

If you point your browser to [http://127.0.0.1:3000](http://127.0.0.1:3000) you should see a "Welcome to Buffalo!" page.

**Congratulations!** You now have your Buffalo application up and running.

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
  buffalo build \
    --environment production \
    --ldflags "-X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
    --output bin/smplatform_linux_amd64
````

### Cross-compile for arm64

First, ensure you have the toolchain installed:

````sh
apt-get update
apt-get install -y gcc-5-aarch64-linux-gnu g++-5-aarch64-linux-gnu libc6-dev-arm64-cross
````

Build the app:

````sh
# Set Build ID number
BUILD_ID="123"

CC=aarch64-linux-gnu-gcc-5 \
CXX=aarch64-linux-gnu-g++-5 \
GO111MODULE=on \
GOOS=linux \
GOARCH=arm64 \
CGO_ENABLED=1 \
  buffalo build \
  --environment production \
  --ldflags "-X smplatform/buildinfo.BuildID=$BUILD_ID -X smplatform/buildinfo.BuildTime=$(date -u +'%Y-%m-%dT%H:%M:%S') -X smplatform/buildinfo.CommitHash=$(git log --pretty=format:'%h' -n 1)" \
  --output bin/smplatform_linux_arm64
````
