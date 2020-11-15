#FROM golang:latest as unwindtest
FROM golang:alpine as unwindtest

ENV GO111MODULE=on \
    CGO_ENABLED=0

RUN ls -al
COPY . /app/
WORKDIR /app

RUN go get -v -t -d ./...
RUN ls -altr &&  go build -v .
WORKDIR /app/
