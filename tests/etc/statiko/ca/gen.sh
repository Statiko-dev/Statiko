#!/bin/sh

# Install cfssl with
# GO111MODULE=off go get -u github.com/cloudflare/cfssl/cmd/...

# Gen the CA certificate
cfssl gencert -initca ca-sr.json | cfssljson -bare ca

# Generate a certificate for the controller
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=config.json -profile=server controller-sr.json | cfssljson -bare controller

# Use this to generate a client certificate if needed (for agents)
# cfssl gencert -ca=intermediate/cluster/cluster.pem -ca-key=intermediate/cluster/cluster-key.pem -config=config.json -profile=client client.json | cfssljson -bare client
cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=config.json -profile=client agent-sr.json | cfssljson -bare agent

# Remove all csr files which we don't need
rm -v *.csr
