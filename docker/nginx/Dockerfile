FROM ghcr.io/zoetrope/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

USER root
RUN curl -fsSL https://nginx.org/keys/nginx_signing.key | apt-key add - \
    && echo "deb http://nginx.org/packages/ubuntu focal nginx" | tee /etc/apt/sources.list.d/nginx.list \
    && apt-get update && apt-get install --no-install-recommends -y nginx

COPY --chown=www-data:www-data nginx.conf /etc/nginx/nginx.conf

RUN set -x \
    && mkdir -p /var/log/nginx \
    && mkdir -p /var/cache/nginx \
    && chown -R www-data:www-data /var/cache/nginx /var/log/nginx \
    && chmod 775 /var/cache/nginx /var/log/nginx \
    && touch /var/log/nginx/nginx.pid

EXPOSE 8080
USER www-data:www-data

CMD ["nginx", "-g", "daemon off;"]
