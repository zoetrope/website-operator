FROM docker.elastic.co/elasticsearch/elasticsearch:7.9.3

LABEL org.opencontainers.image.source https://github.com/zoetrope/website-operator

RUN elasticsearch-plugin install analysis-kuromoji \
    && elasticsearch-plugin install analysis-icu
