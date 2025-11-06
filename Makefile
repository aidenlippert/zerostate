.PHONY: help deps lint fmt test test-unit test-integration test-e2e build docker-build dev-up dev-down dev-logs clean

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Install dependencies
	@echo "Installing Go dependencies..."
	go work sync
	go mod download
	@echo "Installing tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

lint: ## Run linters
	@echo "Running golangci-lint..."
	golangci-lint run ./...
	@echo "Running gosec..."
	gosec -quiet ./...

fmt: ## Format code
	@echo "Formatting Go code..."
	gofmt -s -w .
	goimports -w .

test: test-unit ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	cd libs/p2p && go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	cd libs/identity && go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	cd tests/integration && go test -v -timeout 120s

test-e2e: build docker-build ## Run e2e tests
	@echo "Running e2e tests..."
	cd tests/e2e && go test -v -timeout 120s ./...

coverage: test-unit ## Generate coverage report
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

build: ## Build all services and tools
	@echo "Building services..."
	@mkdir -p bin
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/edge-node ./services/edge-node
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/relay ./services/relay
	CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/bootnode ./services/bootnode

docker-build: build ## Build Docker images
	@echo "Building Docker images..."
	docker build -t zerostate-bootnode:latest -f services/bootnode/Dockerfile.simple .
	docker build -t zerostate-edge-node:latest -f services/edge-node/Dockerfile.simple .
	docker build -t zerostate-relay:latest -f services/relay/Dockerfile.simple .

dev-up: docker-build ## Start local dev environment
	@echo "Starting dev environment with dynamic bootstrap..."
	@./scripts/start-with-dynamic-bootstrap.sh deployments/docker-compose.simple.yml

dev-down: ## Stop local dev environment
	@echo "Stopping dev environment..."
	docker compose -f deployments/docker-compose.simple.yml down -v

dev-logs: ## Show logs from dev environment
	docker compose -f deployments/docker-compose.simple.yml logs -f

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out coverage.html
	go clean -cache -testcache -modcache
