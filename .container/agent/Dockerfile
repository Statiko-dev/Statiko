ARG ARCH=amd64
FROM yobasystems/alpine-nginx:stable-${ARCH}

# Configuration for the statiko app
ENV GO_ENV=production PORT=2265

# Install dependencies and create folders
RUN set -x \
    # Add supervisor
    && apk add --no-cache supervisor \
    # Forward nginx's request and error logs to docker log collector
    && ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log \
    # Folder required by supervisor to store its logs
    && mkdir /var/log/supervisor/ \
    # Create folders for statiko
    && mkdir -p /etc/statiko /data

# Supervisord configuration
COPY .container/agent/supervisord.conf /etc/supervisord.conf

# Copy app
COPY .bin/statiko_linux_${ARCH} /usr/local/bin/statiko
RUN chmod +x /usr/local/bin/statiko

# Expose ports
EXPOSE 80 443 2265

# Start supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
