# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.work and root go.mod/go.sum
COPY go.work go.mod go.sum ./

# Copy all go.mod files first
COPY libs/analytics/go.mod ./libs/analytics/
COPY libs/api/go.mod ./libs/api/
COPY libs/auth/go.mod ./libs/auth/
COPY libs/database/go.mod ./libs/database/
COPY libs/execution/go.mod ./libs/execution/
COPY libs/guild/go.mod ./libs/guild/
COPY libs/identity/go.mod ./libs/identity/
COPY libs/metrics/go.mod ./libs/metrics/
COPY libs/orchestration/go.mod ./libs/orchestration/
COPY libs/p2p/go.mod ./libs/p2p/
COPY libs/payment/go.mod ./libs/payment/
COPY libs/reputation/go.mod ./libs/reputation/
COPY libs/routing/go.mod ./libs/routing/
COPY libs/search/go.mod ./libs/search/
COPY libs/telemetry/go.mod ./libs/telemetry/
COPY libs/economic/go.mod ./libs/economic/
COPY libs/storage/go.mod ./libs/storage/
COPY libs/websocket/go.mod ./libs/websocket/
COPY cmd/api/go.mod ./cmd/api/

# Copy all go.sum files (only where they exist)
COPY libs/api/go.sum ./libs/api/
COPY libs/database/go.sum ./libs/database/
COPY libs/guild/go.sum ./libs/guild/
COPY libs/identity/go.sum ./libs/identity/
COPY libs/metrics/go.sum ./libs/metrics/
COPY libs/orchestration/go.sum ./libs/orchestration/
COPY libs/p2p/go.sum ./libs/p2p/
COPY libs/payment/go.sum ./libs/payment/
COPY libs/reputation/go.sum ./libs/reputation/
COPY libs/routing/go.sum ./libs/routing/
COPY libs/search/go.sum ./libs/search/
COPY libs/telemetry/go.sum ./libs/telemetry/
COPY libs/storage/go.sum ./libs/storage/
COPY libs/websocket/go.sum ./libs/websocket/
COPY cmd/api/go.sum ./cmd/api/

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o /bin/zerostate-api ./cmd/api

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /bin/zerostate-api .

# Copy static files
COPY --from=builder /app/web /root/web

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./zerostate-api"]
