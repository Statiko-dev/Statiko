# Dockerfile inspired by 
# https://github.com/nginxinc/docker-nginx/blob/f7738edec51adb47470a96ad120cd63975a9d3d0/stable/alpine/Dockerfile
FROM alpine:3.10

# Software versions
ENV NGINX_VERSION=1.16.1 NGINX_PKG_RELEASE=1

# Configuration for the statiko app
ARG ARCH=amd64
ENV ARCH=$ARCH GO_ENV=production PORT=2265

# Install dependencies and create folders
RUN set -x \
    # Create nginx user/group first, to be consistent throughout docker variants
    && addgroup -g 101 -S nginx \
    && adduser -S -D -H -u 101 -h /var/cache/nginx -s /sbin/nologin -G nginx -g nginx nginx \
    && nginxPackages=" \
        nginx=${NGINX_VERSION}-r${NGINX_PKG_RELEASE} \
    " \
    && KEY_SHA512="e7fa8303923d9b95db37a77ad46c68fd4755ff935d0a534d26eba83de193c76166c68bfe7f65471bf8881004ef4aa6df3e34689c305662750c0172fca5d8552a *stdin" \
    && apk add --no-cache --virtual .cert-deps \
        openssl \
    && wget -O /tmp/nginx_signing.rsa.pub https://nginx.org/keys/nginx_signing.rsa.pub \
    && if [ "$(openssl rsa -pubin -in /tmp/nginx_signing.rsa.pub -text -noout | openssl sha512 -r)" = "$KEY_SHA512" ]; then \
        echo "key verification succeeded!"; \
        mv /tmp/nginx_signing.rsa.pub /etc/apk/keys/; \
    else \
        echo "key verification failed!"; \
        exit 1; \
    fi \
    && apk del .cert-deps \
    && apk add -X "https://nginx.org/packages/alpine/v$(egrep -o '^[0-9]+\.[0-9]+' /etc/alpine-release)/main" --no-cache $nginxPackages \
    # Bring in gettext so we can get `envsubst`, then throw the rest away. To do this, we need to install `gettext` then move `envsubst` out of the way so `gettext` can be deleted completely, then move `envsubst` back
    && apk add --no-cache --virtual .gettext gettext \
    && mv /usr/bin/envsubst /tmp/ \
    \
    && runDeps="$( \
        scanelf --needed --nobanner /tmp/envsubst \
            | awk '{ gsub(/,/, "\nso:", $2); print "so:" $2 }' \
            | sort -u \
            | xargs -r apk info --installed \
            | sort -u \
    )" \
    && apk add --no-cache $runDeps \
    && apk del .gettext \
    && mv /tmp/envsubst /usr/local/bin/ \
    # Bring in tzdata so users could set the timezones through the environment variables
    # Also add supervisor and ca-certificates
    && apk add --no-cache supervisor tzdata ca-certificates \
    # Forward nginx's request and error logs to docker log collector
    && ln -sf /dev/stdout /var/log/nginx/access.log \
    && ln -sf /dev/stderr /var/log/nginx/error.log \
    # Folder required by supervisor to store its logs
    && mkdir /var/log/supervisor/ \
    # Create folders for statiko
    && mkdir -p /etc/statiko /data

# Supervisord configuration
COPY container/supervisord.conf /etc/supervisord.conf

# Copy app
COPY bin/statiko_linux_${ARCH} /usr/local/bin/statiko
RUN chmod +x /usr/local/bin/statiko

# Expose ports
EXPOSE 80 443 2265

# Start supervisor
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
