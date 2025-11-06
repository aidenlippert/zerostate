# Developer Setup Guide

This guide helps you set up a local zerostate development environment.

## Prerequisites

- **Go 1.21 or later** — [Download](https://go.dev/dl/)
- **Docker 24.0+** and Docker Compose — [Install Docker](https://docs.docker.com/get-docker/)
- **Make 4.0+** — Usually pre-installed on Linux/macOS; Windows users can use WSL2
- **Git** — [Install Git](https://git-scm.com/downloads)

Optional:
- **golangci-lint** — Install via `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **VS Code** with Go extension

## Quick Start

### 1. Clone the repository

```bash
git clone https://github.com/YOUR_ORG/zerostate.git
cd zerostate
```

### 2. Install dependencies

```bash
make deps
```

This installs Go dependencies and required tools (golangci-lint, gosec).

### 3. Build the project

```bash
make build
```

Binaries will be created in `bin/`:
- `bin/edge-node`
- `bin/relay`
- `bin/bootnode`
- `bin/zerostate-cli`

### 4. Run tests

```bash
# Unit tests
make test-unit

# Integration tests
make test-integration

# All tests
make test
```

### 5. Start local dev environment

```bash
make dev-up
```

This starts:
- **bootnode** — DHT bootstrap node
- **edge-node-1, edge-node-2** — Edge peers
- **relay** — Regional relay (placeholder)
- **Prometheus** — Metrics at http://localhost:9090
- **Grafana** — Dashboards at http://localhost:3000 (admin/admin)
- **Jaeger** — Traces at http://localhost:16686
- **OTel Collector** — Telemetry aggregation

View logs:
```bash
make dev-logs
```

Stop environment:
```bash
make dev-down
```

### 6. Run linters

```bash
make lint
```

### 7. Format code

```bash
make fmt
```

## Project Structure

```
zerostate/
├── services/           # Deployable services
│   ├── edge-node/      # Edge peer node
│   ├── relay/          # Regional relay
│   ├── bootnode/       # DHT bootstrap node
│   └── hnsw-index/     # Semantic search index
├── libs/               # Shared libraries
│   ├── p2p/            # libp2p networking
│   ├── identity/       # DID and Agent Card signing
│   ├── protocol/       # Protocol definitions
│   └── routing/        # Q-routing algorithms
├── tools/              # CLI tools
│   └── cli/            # zerostate-cli
├── tests/              # Integration and e2e tests
├── docs/               # Documentation
├── specs/              # JSON schemas
├── deployments/        # Docker Compose, Kubernetes manifests
└── examples/           # Example configs and data
```

## Development Workflow

### Creating a new feature

1. Create a feature branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. Make changes and write tests

3. Run tests and linters:
   ```bash
   make test lint
   ```

4. Commit with clear messages:
   ```bash
   git commit -m "feat: add capability discovery API"
   ```

5. Push and create a PR:
   ```bash
   git push origin feature/your-feature-name
   ```

### Running a single service

```bash
# Edge node with custom config
./bin/edge-node --listen /ip4/0.0.0.0/udp/4001/quic-v1 --log-level debug

# With bootstrap peer
./bin/edge-node --bootstrap /ip4/127.0.0.1/udp/5001/quic-v1/p2p/12D3KooW...
```

### Debugging

Use VS Code launch configurations or run with delve:
```bash
dlv debug ./services/edge-node -- --log-level debug
```

### Working with Go workspace

The monorepo uses Go workspaces. To sync modules:
```bash
go work sync
```

To add a new module to the workspace:
```bash
go work use ./path/to/module
```

## Common Tasks

### Add a new library

```bash
mkdir -p libs/my-library
cd libs/my-library
go mod init github.com/zerostate/libs/my-library
go work use .
```

### Update dependencies

```bash
go get -u ./...
go work sync
```

### Generate coverage report

```bash
make coverage
open coverage.html
```

### Build Docker images locally

```bash
make docker-build
```

## Troubleshooting

### Tests fail with "connection refused"

Ensure no services are already running on the required ports (4001-4004, 8080, 3000, 9090, 16686).

### golangci-lint errors

Run `make fmt` to auto-fix formatting issues, then address remaining issues manually.

### Docker build fails

Clear Docker cache:
```bash
docker system prune -a
make docker-build
```

### Go workspace issues

Reset workspace:
```bash
rm go.work go.work.sum
go work init
go work use ./libs/* ./services/* ./tools/*
```

## Next Steps

- Read the [Architecture Documentation](../architecture.md)
- Review the [Sprint Plan](../plan/sprint_plan.md)
- Check [Sprint 1 Tasks](../plan/sprint1_tasks.md)
- Explore example [Agent Cards](../../examples/agent_card.example.json)

## Getting Help

- GitHub Issues: https://github.com/YOUR_ORG/zerostate/issues
- Slack: #zerostate-dev (internal)
- Documentation: `docs/`
