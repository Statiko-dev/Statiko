FROM nginx:1-alpine

# Build args
# TARGETARCH is set automatically when using BuildKit
ARG TARGETARCH
ARG BUILD_ID
ARG BUILD_TIME
ARG COMMIT_HASH

# Labels
LABEL maintainer="Alessandro Segala <italypaleale@gmail.com>" \
    build=$BUILD_ID \
    org.opencontainers.image.title="statiko-agent" \
    org.opencontainers.image.description="Agent node for Statiko. Statiko is a platform for hosting, serving, and managing static web apps in production." \
    org.opencontainers.image.authors="Alessandro Segala <italypaleale@gmail.com>" \
    org.opencontainers.image.vendor="@ItalyPaleAle" \
    org.opencontainers.image.version=$BUILD_ID \
    org.opencontainers.image.url="https://hub.docker.com/r/statiko/statiko-agent/" \
    org.opencontainers.image.source="https://github.com/statiko-dev/statiko" \
    org.opencontainers.image.revision=$COMMIT_HASH \
    org.opencontainers.image.created=$BUILD_TIME \
    org.opencontainers.image.licenses="AGPL-3.0"

# Install dependencies and create folders
RUN set -x \
    # Add runtime dependencies
    && apk add --no-cache supervisor ca-certificates openssl \
    # Forward nginx's request and error logs to docker log collector
    && ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log \
    # Folder required by supervisor to store its logs
    && mkdir /var/log/supervisor/ \
    # Create folders for statiko
    && mkdir -p /etc/statiko /data \
    # Remove the "daemon" directive from nginx's configuration
    && sed -i 's/^daemon off\;//' /etc/nginx/nginx.conf

# Supervisord configuration
COPY container/agent/supervisord.conf /etc/supervisord.conf

# Copy app
COPY .bin/linux-${TARGETARCH}/agent /usr/local/bin/statiko-agent

# Expose ports
EXPOSE 80 443

# Start supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
