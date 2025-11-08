# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum go.work ./
COPY libs/api/go.mod libs/api/go.sum ./libs/api/
COPY libs/auth/go.mod ./libs/auth/
COPY libs/database/go.mod ./libs/database/
COPY libs/execution/go.mod libs/execution/go.sum ./libs/execution/
COPY libs/identity/go.mod libs/identity/go.sum ./libs/identity/
COPY libs/orchestration/go.mod libs/orchestration/go.sum ./libs/orchestration/
COPY libs/p2p/go.mod libs/p2p/go.sum ./libs/p2p/
COPY libs/search/go.mod libs/search/go.sum ./libs/search/
COPY libs/telemetry/go.mod libs/telemetry/go.sum ./libs/telemetry/
COPY cmd/api/go.mod cmd/api/go.sum ./cmd/api/

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
