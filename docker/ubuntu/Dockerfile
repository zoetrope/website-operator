FROM quay.io/cybozu/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

RUN apt-get update \
    && apt-get install --no-install-recommends -y gnupg git openssh-client connect-proxy

RUN useradd -m -u 10000 ubuntu
USER 10000:10000
