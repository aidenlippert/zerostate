# Multi-stage build for bootnode
FROM golang:1.21-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git make

# Copy all source code
COPY libs/ ./libs/
COPY services/bootnode/ ./services/bootnode/

# Build the binary
WORKDIR /build/services/bootnode
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o /bootnode .

FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /bootnode /bootnode

USER nonroot:nonroot

EXPOSE 4001/udp
EXPOSE 8080/tcp

ENTRYPOINT ["/bootnode"]
