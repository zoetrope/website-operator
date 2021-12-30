FROM ghcr.io/zoetrope/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

COPY /ui /ui
COPY /dist /dist

USER 10000:10000

ENTRYPOINT ["/ui"]
