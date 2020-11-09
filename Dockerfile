FROM golang:latest as unwindtest

RUN ls -al
COPY . /app/
WORKDIR /app

RUN go get -v -t -d ./...
RUN ls -altr &&  go build -v .
WORKDIR /app/
