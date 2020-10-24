FROM ghcr.io/zoetrope/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

COPY ./repo-checker /repo-checker

USER 10000:10000

ENTRYPOINT ["/repo-checker"]
