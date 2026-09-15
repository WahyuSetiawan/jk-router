# Build stage
FROM golang:1.23-bookworm AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /jkrouter ./jkserver/cmd/

# Runtime stage
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /jkrouter /usr/local/bin/jkrouter
EXPOSE 20128
ENV DATA_DIR=/data
VOLUME ["/data"]
ENTRYPOINT ["jkrouter", "--port", "20128"]
