# 5-Line Dockerfile for Genesis Bootstrap Node
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download && CGO_ENABLED=0 go build -o bootstrap-node .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/bootstrap-node /bootstrap-node
EXPOSE 4001
CMD ["/bootstrap-node"]
