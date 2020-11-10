#!/bin/sh

# Add the CA certificate
sudo cp tests/etc/statiko/ca/ca.pem /usr/local/share/ca-certificates/statiko-dev.crt
sudo update-ca-certificates

# Create a symbolic link to the etc folder
sudo ln -vs $(pwd)/tests/etc/statiko /etc

# Create the data directory
sudo mkdir /data

# Grant user dev permission to /data, /etc/nginx, and /repo
#sudo setfacl -Rm u:dev:rwx /data /etc/nginx
sudo chown -Rv dev:dev /data /etc/nginx /repo
