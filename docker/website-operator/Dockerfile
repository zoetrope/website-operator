FROM quay.io/cybozu/ubuntu:20.04

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

COPY /website-operator /website-operator

USER 10000:10000

ENTRYPOINT ["/website-operator"]
