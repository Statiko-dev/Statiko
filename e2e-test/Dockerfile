FROM italypaleale/smplatform:latest

# Install Node.js which is necessary for running tests, with dependencies
ARG ARCH=amd64
ARG NODE_ARCH=x64
ENV ARCH=$ARCH NODE_ARCH=$NODE_ARCH NODE_VERSION=12.14.1
RUN apt-get update \
    && apt-get install -y curl \
    && rm -rf /var/lib/apt/lists/* \
    && echo "Downloading https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODE_ARCH.tar.gz" \
    && curl -fsSLO --compressed "https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODE_ARCH.tar.gz" \
    && tar -xzf "node-v$NODE_VERSION-linux-$NODE_ARCH.tar.gz" -C /usr/local --strip-components=1 --no-same-owner \
    && rm -v "node-v$NODE_VERSION-linux-$NODE_ARCH.tar.gz"

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
RUN sed -i 's/nodaemon=true/nodaemon=false/' /etc/supervisor/conf.d/smplatform.conf

# When the container starts, start supervisord, then run tests
ENV MOCHA_FILE=/results/${ARCH}.xml
CMD supervisord -c /etc/supervisor/supervisord.conf \
    && sleep 8 \
    && supervisorctl status \
    && npm run test-ci \
    && supervisorctl stop all \
    && sleep 2
