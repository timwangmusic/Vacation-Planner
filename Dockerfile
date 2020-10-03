FROM golang:alpine as unwinddev
RUN mkdir /app
COPY . /app/
WORKDIR /app/

#Install deps
#RUN go get -v -t -d ./...

# Check the working directory
RUN ls -altr
RUN adduser -S -D -H -h /app appuser
USER appuser
