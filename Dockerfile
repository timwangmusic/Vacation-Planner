FROM golang:alpine as unwinddev
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go get -v -t -d ./...
SHELL ["/bin/bash", "-c", "-l"]
RUN ls -altr && cd main/ && ls -al && go build -v .
RUN adduser -S -D -H -h /app appuser
USER appuser
