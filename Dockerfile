FROM golang:latest as unwindtest
RUN ls -altr

RUN ls -al
COPY . /app/
WORKDIR /app

RUN go get -v -t -d ./...
RUN ls -altr && cd main/ && go build -v .
WORKDIR /app/
