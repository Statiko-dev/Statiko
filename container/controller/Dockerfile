# Start from a distroless image
FROM gcr.io/distroless/static-debian10

# Build args
# TARGETARCH is set automatically when using BuildKit
ARG TARGETARCH
ARG BUILD_ID
ARG BUILD_TIME
ARG COMMIT_HASH

# Labels
LABEL maintainer="Alessandro Segala <italypaleale@gmail.com>" \
    build=$BUILD_ID \
    org.opencontainers.image.title="statiko-controller" \
    org.opencontainers.image.description="Controller node for Statiko. Statiko is a platform for hosting, serving, and managing static web apps in production." \
    org.opencontainers.image.authors="Alessandro Segala <italypaleale@gmail.com>" \
    org.opencontainers.image.vendor="@ItalyPaleAle" \
    org.opencontainers.image.version=$BUILD_ID \
    org.opencontainers.image.url="https://hub.docker.com/r/statiko/statiko-controller/" \
    org.opencontainers.image.source="https://github.com/statiko-dev/statiko" \
    org.opencontainers.image.revision=$COMMIT_HASH \
    org.opencontainers.image.created=$BUILD_TIME \
    org.opencontainers.image.licenses="AGPL-3.0"

# Copy app
COPY .bin/linux-${TARGETARCH}/controller /statiko-controller

# Configuration for the statiko app
ENV STATIKO_CONTROLLER_API_PORT=2265 STATIKO_CONTROLLER_GRPC_PORT=2300

# Expose ports
EXPOSE 2265 2300

# Start the controller
CMD ["/statiko-controller"]
