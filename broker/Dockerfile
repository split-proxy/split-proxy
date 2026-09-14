FROM golang:1.26-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY broker.go ./

RUN go build -o broker .

CMD ["./broker"]
