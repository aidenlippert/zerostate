# zerostate

**Hybrid P2P network for AI agents with DHT-based discovery, regional relays, and Q-routing.**

[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)]() 
[![Coverage](https://img.shields.io/badge/coverage-75%25-yellow)]()
[![Go](https://img.shields.io/badge/go-1.21%2B-blue)]()

## 🚀 Quick Start

```bash
# Build and test
make build test-unit test-integration test-e2e

# Start 3-node local network
make dev-up

# Check status
curl http://localhost:8081/healthz
curl http://localhost:8082/metrics | grep zerostate_

# Stop
make dev-down
```

## ✅ Current Status

**Sprint 1:** 14/49 tasks complete (28%)

| Component | Status | Coverage |
|-----------|--------|----------|
| Unit Tests | ✅ 11/11 passing | 73-78% |
| Integration Tests | ✅ 2/2 passing | Multi-node DHT |
| E2E Tests | ✅ Passing | 3-node network |
| Docker Images | ✅ Built | bootnode, edge-node, relay |
| CI/CD | ✅ Configured | GitHub Actions |

### ✅ Implemented
- libp2p + Kademlia DHT (k=20, α=3)
- Agent Card publish/resolve via DHT
- Ed25519 signing + did:key DIDs  
- W3C Data Integrity proofs
- Prometheus metrics (4 metrics)
- Integration test harness
- E2E smoke test
- Docker containers + compose

### 🚧 Next Steps
- mDNS local discovery
- Q-routing for relays
- OpenTelemetry exporters
- Grafana dashboards
- Kubernetes manifests

## 📚 Documentation

- **[Architecture](docs/architecture.md)** - Network design + Mermaid diagrams
- **[Q-Routing](docs/routing_q_agent.md)** - Relay routing policy
- **[Sprint Plan](docs/plan/sprint_plan.md)** - 12-week MVP roadmap
- **[Sprint 1 Tasks](docs/plan/sprint1_tasks.md)** - 49 granular tasks
- **[Dev Setup](docs/dev/setup.md)** - Developer guide

## 🏗️ Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Edge Nodes  │────▶│   Relays    │────▶│  Backbone   │
│  (libp2p)   │     │ (Q-routing) │     │  (Storage)  │
└─────────────┘     └─────────────┘     └─────────────┘
      │                    │                    │
      └────── Kademlia DHT ──────────────────────┘
```

**Core Features:**
- **Discovery:** Content-addressed Agent Cards via Kademlia DHT
- **Identity:** Self-sovereign DIDs (did:key) with Ed25519
- **Transport:** QUIC over UDP, noise encryption
- **Observability:** Prometheus metrics, health endpoints

See [docs/architecture.md](docs/architecture.md) for details.

## 🛠️ Development

```bash
# Build static binaries
make build

# Run tests
make test-unit           # Unit tests
make test-integration    # Multi-node DHT
make test-e2e           # Docker-compose E2E

# Dev environment
make dev-up             # Start 3-node network
make dev-logs           # View logs
make dev-down           # Stop and clean

# Linting
make lint fmt
```

## 📊 Metrics

Exposed on `:8080/metrics`:

```
zerostate_dht_lookups_total{operation="publish",status="success"} 5
zerostate_dht_lookup_duration_seconds_bucket{operation="resolve"} 0.15
zerostate_agent_card_publish_total 5
zerostate_peer_connections 2
```

## 📦 Schemas

- **[Agent Card](specs/agent_card.schema.json)** - Identity, capabilities, reputation ([example](examples/agent_card.example.json))
- **[Task Manifest](specs/task_manifest.schema.json)** - Job requests, SLAs, payments ([example](examples/task_manifest.example.json))

## 🗺️ Roadmap

- **Sprint 1 (Weeks 1-2):** ✅ Core P2P + DHT + Identity
- **Sprint 2 (Weeks 3-4):** 🔜 Q-routing + mDNS + OpenTelemetry
- **Sprint 3 (Weeks 5-6):** HNSW vector search + federated queries
- **Sprint 4-6:** Guild formation, payments, Sybil resistance

See [docs/plan/sprint_plan.md](docs/plan/sprint_plan.md)

## 📝 License

TBD (MIT or Apache-2.0)
