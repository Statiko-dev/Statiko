###
### First stage: test image
###
FROM statiko/statiko:latest AS testimage

# Install Node.js which is necessary for running tests
# Source: https://raw.githubusercontent.com/nodejs/docker-node/master/12/alpine3.11/Dockerfile
ENV NODE_VERSION 12.16.0
RUN addgroup -g 1000 node \
  && adduser -u 1000 -G node -s /bin/sh -D node \
  && apk add --no-cache \
    libstdc++ openssl \
    # Required for installing Node.js and useful for debugging
    curl jq \
  && export CHECKSUM="c7c38c170c38a491ecffbd5324415818f88abc2a6a79076493c1028a19bf64df" \
  && curl -fsSLO --compressed "https://unofficial-builds.nodejs.org/download/release/v$NODE_VERSION/node-v$NODE_VERSION-linux-x64-musl.tar.xz" \
  && echo "$CHECKSUM  node-v$NODE_VERSION-linux-x64-musl.tar.xz" | sha256sum -c - \
  && tar -xJf "node-v$NODE_VERSION-linux-x64-musl.tar.xz" -C /usr/local --strip-components=1 --no-same-owner \
  && ln -s /usr/local/bin/node /usr/local/bin/nodejs \
  && rm -f "node-v$NODE_VERSION-linux-x64-musl.tar.xz"

# Add the CA
COPY misc/ca.crt /usr/local/share/ca-certificates/italypaleale-ci.crt
RUN update-ca-certificates
ENV NODE_EXTRA_CA_CERTS /usr/local/share/ca-certificates/italypaleale-ci.crt

# Set workdir
WORKDIR /test

# Copy app
COPY . .

# Install dependencies (from package-lock.json)
RUN npm ci

# Configure supervisord to run as daemon
RUN sed -i 's/nodaemon=true/nodaemon=false/' /etc/supervisord.conf \
    && sed -i 's/\/dev\/fd\/1/\/results\/stdout/' /etc/supervisord.conf \
    && sed -i 's/\/dev\/fd\/2/\/results\/stderr/' /etc/supervisord.conf

# When the container starts, start supervisord, then run tests
ENV MOCHA_FILE=/results/statiko.xml
CMD rm -f /etc/statiko/state.json \
    && supervisord -c /etc/supervisord.conf \
    && sleep 8 \
    && supervisorctl status \
    && npm run test-ci \
    && supervisorctl stop all \
    && sleep 2

###
### Second stage: test image with coverage
###
FROM testimage 

# Copy the binary with the coverage test, then change the command in the supervisord.conf
RUN mv .bin/statiko_linux_amd64.test /usr/local/bin/statiko.test \
    && chmod +x /usr/local/bin/statiko.test \
    && sed -i 's/\/usr\/local\/bin\/statiko/\/usr\/local\/bin\/statiko.test -test.run "^TestRunMain$" -test.coverprofile=\/results\/e2e.out/' /etc/supervisord.conf 
