ARG target=amd64
ARG BINARY_FILE=.bin/statiko_linux_amd64
ARG QEMU_FILE=qemu-x86_64-static
FROM yobasystems/alpine-nginx:stable-${target}

# Add qemu
COPY $QEMU_FILE /usr/bin/

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
    && mkdir -p /etc/statiko /data

# Supervisord configuration
COPY .container/agent/supervisord.conf /etc/supervisord.conf

# Copy app, then remove qemu
COPY $BINARY_FILE /usr/local/bin/statiko
RUN chmod +x /usr/local/bin/statiko \
    && rm -rf /usr/bin/${QEMU_FILE}

# Configuration for the statiko app
ENV GO_ENV=production PORT=2265

# Expose ports
EXPOSE 80 443 2265

# Start supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
