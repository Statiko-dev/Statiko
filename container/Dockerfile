FROM ubuntu:18.04

# Configuration for the smplatform app
ARG ARCH
ENV ARCH=$ARCH GO_ENV=production PORT=3000

# Install dependencies and create folders
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        supervisor \
        nginx \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /etc/smplatform /data

# Supervisord configuration
COPY container/supervisord.conf /etc/supervisor/conf.d/smplatform.conf

# Copy app
COPY bin/smplatform_linux_$ARCH /usr/local/bin/smplatform
RUN chmod +x /usr/local/bin/smplatform

# Expose ports
EXPOSE 80 443 4000

# Start supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
