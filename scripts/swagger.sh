#!/bin/sh

# Requires swagger:
# GO111MODULE=off go get -u github.com/swaggo/swag/cmd/swag

if [ ! -f actions/app.go ]; then
    echo "Please run this script from the root directory of the project (ie. run \`cd .. && ./scripts/swagger.sh\`)"
    return 1
fi

(cd $(pwd) && swag init -g actions/app.go)
