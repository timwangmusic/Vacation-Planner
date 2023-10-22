FROM golang:1.21-alpine

ENV GO111MODULE=on

COPY . /app/
WORKDIR /app/

RUN go build -v .

EXPOSE 10000

CMD ["./Vacation-planner"]
