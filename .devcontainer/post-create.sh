#!/bin/sh

# Add the CA certificate
sudo cp tests/assets/ca.crt /usr/local/share/ca-certificates/italypaleale-ci.crt
sudo update-ca-certificates

# Create a symbolic link to the etc folder
sudo ln -vs $(pwd)/tests/etc/statiko /etc

# Create the data directory
sudo mkdir /data

# Grant user dev permission to /data and /etc/nginx
#sudo setfacl -Rm u:dev:rwx /data /etc/nginx
sudo chown -Rv dev:dev /data /etc/nginx
