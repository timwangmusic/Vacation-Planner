FROM golang:alpine as unwinddev
RUN mkdir /app
COPY . /app/
WORKDIR /app/

#Install deps
RUN go get -v -t -d ./...
RUN ls -altr && cd main/ && go build -v .

# Check the working directory
RUN ls -altr
RUN adduser -S -D -H -h /app -u 1001 appuser
USER appuser
RUN echo $UID
