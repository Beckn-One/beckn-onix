FROM golang:1.25.5 AS builder

WORKDIR /workspace/app

# Copy go module files first for better caching
COPY go.mod .
COPY go.sum .

# Copy source code
COPY cmd/adapter  ./cmd/adapter
COPY core/ ./core
COPY pkg/ ./pkg

# Download dependencies and tidy
RUN go mod download && go mod tidy

# Build the application
RUN go build -o server cmd/adapter/main.go