FROM golang:1.25.5 AS builder

WORKDIR /workspace/app
COPY cmd/adapter  ./cmd/adapter
COPY core/ ./core
COPY pkg/ ./pkg
COPY go.mod .
COPY go.sum .
RUN go mod download

RUN go build -o server cmd/adapter/main.go