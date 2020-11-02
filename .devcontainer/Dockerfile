# Based on: https://github.com/microsoft/vscode-dev-containers/tree/v0.140.1/containers/go/.devcontainer/base.Dockerfile

# Go version
ARG VARIANT="1"
FROM golang:${VARIANT}

# Install needed packages and setup non-root user
ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=$USER_UID
COPY library-scripts/common-debian.sh /tmp/library-scripts/
RUN bash /tmp/library-scripts/common-debian.sh "${INSTALL_ZSH}" "${USERNAME}" "${USER_UID}" "${USER_GID}" "${UPGRADE_PACKAGES}" \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts

# Install Go tools
ENV GO111MODULE=auto
COPY library-scripts/go-debian.sh /tmp/library-scripts/
RUN bash /tmp/library-scripts/go-debian.sh "none" "/usr/local/go" "${GOPATH}" "${USERNAME}" "false" \
    && apt-get clean -y && rm -rf /tmp/library-scripts

# Install Node.js
ARG NODE_VERSION="lts/*"
ENV NVM_DIR=/usr/local/share/nvm
ENV NVM_SYMLINK_CURRENT=true \
    PATH=${NVM_DIR}/current/bin:${PATH}
COPY library-scripts/node-debian.sh /tmp/library-scripts/
RUN bash /tmp/library-scripts/node-debian.sh "${NVM_DIR}" "${NODE_VERSION}" "${USERNAME}" \
    && su dev -c "source /usr/local/share/nvm/nvm.sh && nvm install ${NODE_VERSION} 2>&1" \
    && apt-get clean -y && rm -rf /var/lib/apt/lists/* /tmp/library-scripts

# Install additional OS packages
RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
    && apt-get -y install --no-install-recommends nginx

# Install a set of Go development tools
RUN curl -sf https://gobinaries.com/github.com/ory/go-acc@v0.2.6 | PREFIX=/usr/local/bin sh \
  && curl -sf https://gobinaries.com/github.com/gobuffalo/packr/packr2@v2.8.0 | PREFIX=/usr/local/bin sh \
  && curl -sf https://gobinaries.com/github.com/markbates/pkger/cmd/pkger@v0.17.1 | PREFIX=/usr/local/bin sh \
  && curl -sf https://gobinaries.com/github.com/fullstorydev/grpcurl/cmd/grpcurl | PREFIX=/usr/local/bin sh \
  && curl -sf https://gobinaries.com/github.com/axw/gocov/gocov | PREFIX=/usr/local/bin sh \
  # Fork of github.com/matm/gocov-html but with tags so it works with gobinaries.com
  && curl -sf https://gobinaries.com/github.com/ItalyPaleAle/gocov-html | PREFIX=/usr/local/bin sh \
  # Fork of github.com/mitchellh/protoc-gen-go-json but with tags so it works with gobinaries.com
  && curl -sf https://gobinaries.com/github.com/ItalyPaleAle/protoc-gen-go-json | PREFIX=/usr/local/bin sh \
  && curl -sf https://gobinaries.com/github.com/golang/protobuf/protoc-gen-go@v1.4.3 | PREFIX=/usr/local/bin sh \
  # Protobuf compiler
  && export PROTOC_VERSION="3.13.0" \
  && PB_REL="https://github.com/protocolbuffers/protobuf/releases" \
  && curl -LO $PB_REL/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip \
  && unzip protoc-${PROTOC_VERSION}-linux-x86_64.zip -d protoc/ \
  && cp -rvp protoc/bin/protoc /usr/local/bin/ \
  && cp -rvp protoc/include/* /usr/local/include/ \
  && chmod 0755 /usr/local/bin/protoc \
  && chmod -R 0755 /usr/local/include/google \
  && rm -rvf protoc*

# Install global packages from NPM
RUN su dev -c "source /usr/local/share/nvm/nvm.sh && npm install -g eslint" 2>&1

# Enable ZSH plugins
RUN su dev -c "sed -i 's/plugins=(git)/plugins=(git dotenv)/g' /home/dev/.zshrc" \
  && echo "ZSH_DOTENV_PROMPT=false" >> /home/dev/.zshrc

# Expose ports
EXPOSE 80 443 2265
