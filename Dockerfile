# Build web frontend
FROM node:22-alpine AS web-builder
WORKDIR /app
COPY web/package.json web/pnpm-lock.yaml ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web .
RUN pnpm build

# Build Go binary
FROM golang:1.26-alpine AS builder
ARG VERSION=dev
WORKDIR /app
COPY jkserver/go.mod jkserver/go.sum ./jkserver/
RUN cd jkserver && go mod download
COPY jkserver ./jkserver
COPY --from=web-builder /app/.output ./jkserver/web/.output
RUN cd jkserver && go build -ldflags="-s -w -X main.Version=$VERSION" -o /jkrouter ./cmd/

# Runtime
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /jkrouter /usr/local/bin/jkrouter
EXPOSE 20128
ENV DATA_DIR=/data PORT=20128
VOLUME ["/data"]
ENTRYPOINT ["jkrouter", "serve", "--port", "20128"]
