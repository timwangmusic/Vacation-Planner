FROM golang:alpine as unwindtest

ENV GO111MODULE=on \
    CGO_ENABLED=0

COPY . /app/
WORKDIR /app

RUN go build -v .


# Copy binary from build to main folder
RUN cp /main .

# Export necessary port
EXPOSE 3000

# Command to run when starting the container
CMD ["/main"]
