.PHONY: all build proto clean test run

# Variables
PROTO_DIR := pkg/ari/v1
PROTO_FILES := $(wildcard $(PROTO_DIR)/*.proto)
GO_OUT_DIR := $(PROTO_DIR)
BIN_DIR := bin
BINARY_NAME := runtime

# Default target
all: proto build

# Generate Go code from Protocol Buffers
proto:
	@echo "Generating Go code from Protocol Buffers..."
	@mkdir -p $(GO_OUT_DIR)
	protoc --go_out=$(GO_OUT_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GO_OUT_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
	@echo "✅ Protocol Buffers compiled successfully!"

# Build the binary
build:
	@echo "Building reference-runtime-v1..."
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/runtime
	@echo "✅ Build complete: $(BIN_DIR)/$(BINARY_NAME)"

# Run the runtime
run: build
	@echo "Starting reference-runtime-v1..."
	./$(BIN_DIR)/$(BINARY_NAME) --agent-config testdata/math-agent.yaml

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	rm -f $(GO_OUT_DIR)/*.pb.go
	@echo "✅ Clean complete!"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed!"

# Install protoc plugins
install-proto:
	@echo "Installing Protocol Buffer compiler plugins..."
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✅ Plugins installed!"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted!"

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run ./...

# Development mode (rebuild on changes)
dev:
	@echo "Starting development mode..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Show help
help:
	@echo "Available targets:"
	@echo "  make proto        - Generate Go code from .proto files"
	@echo "  make build        - Build the runtime binary"
	@echo "  make run          - Build and run the runtime"
	@echo "  make test         - Run tests"
	@echo "  make clean        - Clean build artifacts"
	@echo "  make deps         - Install dependencies"
	@echo "  make install-proto- Install protoc plugins"
	@echo "  make fmt          - Format code"
	@echo "  make lint         - Lint code"
	@echo "  make dev          - Run in development mode"
