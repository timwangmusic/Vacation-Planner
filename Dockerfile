FROM golang:alpine as unwindtest

ENV GO111MODULE=on

COPY . /app/
WORKDIR /app/

RUN go build -v .

EXPOSE 10000

CMD ["./Vacation-planner"]
