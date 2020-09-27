FROM golang:alpine as unwinddev
RUN mkdir /app
ADD . /app/
WORKDIR /app
RUN go get -v -t -d ./...
RUN cd main/
RUN go build -v .
RUN adduser -S -D -H -h /app appuser
USER appuser

