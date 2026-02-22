# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o go-web-search-mcp ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy CA certificates and binary
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/go-web-search-mcp .

# Copy default config file (can be overridden by volume mount)
COPY --from=builder /app/config.yaml .

# Expose port
EXPOSE 3000

# Run the binary
CMD ["./go-web-search-mcp"]
