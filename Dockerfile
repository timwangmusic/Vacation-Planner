FROM golang:alpine as unwindtest

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . /app/
WORKDIR /app

RUN go build -v .
