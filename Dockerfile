FROM golang:alpine as unwinddev

# Check the working directory
RUN ls -altr
RUN addgroup -S appuser
RUN adduser -S -D -h /app -s /bin/bash -G appuser -u 1001 appuser
#RUN adduser -S -D -H -h /app -u 1001 appuser
RUN chown -R appuser:appuser /app
RUN chmod -R 755 .
USER appuser

RUN mkdir /app
COPY . /app/
WORKDIR /app

#Install deps
RUN go get -v -t -d ./...
RUN ls -altr && cd main/ && go build -v .
WORKDIR /app/
