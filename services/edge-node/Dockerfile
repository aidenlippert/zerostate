# Multi-stage build for edge-node
FROM golang:1.21-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git make

# Copy all source code
COPY libs/ ./libs/
COPY services/edge-node/ ./services/edge-node/

# Build the binary
WORKDIR /build/services/edge-node
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /edge-node .

# Final stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /

# Copy binary
COPY --from=builder /edge-node /edge-node

# Use nonroot user
USER nonroot:nonroot

# Expose ports
EXPOSE 4001/udp
EXPOSE 8080/tcp

ENTRYPOINT ["/edge-node"]
