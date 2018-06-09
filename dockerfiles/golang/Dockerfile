FROM golang:latest

RUN cd /usr/local/src/ \
    && curl -sLJO https://github.com/uryoya/record/releases/download/v0.0.1/record.tar.gz \
    && tar -xf record.tar.gz \
    && cp record /usr/local/bin/
