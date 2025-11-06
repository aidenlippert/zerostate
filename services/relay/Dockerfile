# Multi-stage build for relay
FROM golang:1.21-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git make

# Copy all source code
COPY libs/ ./libs/
COPY services/relay/ ./services/relay/

# Build the binary
WORKDIR /build/services/relay
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /relay .

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /relay /relay

USER nonroot:nonroot

EXPOSE 4004/udp
EXPOSE 8080/tcp

ENTRYPOINT ["/relay"]
