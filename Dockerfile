FROM golang:alpine as unwindtest

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . /app/
WORKDIR /app

RUN go build -v .

#Check the contents of the directory
ls -al /

# Move to /dist directory as the place for resulting binary folder
WORKDIR /dist

# Copy binary from build to main folder
RUN cp /build/main .

# Export necessary port
EXPOSE 3000

# Command to run when starting the container
CMD ["/dist/main"]
