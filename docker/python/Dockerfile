FROM ghcr.io/zoetrope/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

USER root
RUN apt-get update \
    && apt-get install --no-install-recommends -y python3.8 python3-pip

USER 10000:10000
