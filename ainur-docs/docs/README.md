# Ainur Protocol

## Vision
Ainur is a planetary nervous system for autonomous agents. Every component is designed to make trust, discovery, coordination, and settlement deterministic: decentralized identity rooted in Substrate, discovery and routing over Aether topics, standardized execution via the Ainur Runtime Interface, and audited settlement through a purpose-built economic layer. The goal is to make heterogeneous agents collaborate at the speed of software while retaining verifiability end to end.

## Architecture
1. Temporal Ledger (chain-v2) – custom pallets for DID, registry, reputation, VCG auctions, and escrow run on a Polkadot SDK solochain.
2. Orchestrator (cmd/api, libs/*) – Go services handle task ingestion, ARI dispatch, market making, payment lifecycle, and monitoring.
3. Agent Runtime (reference-runtime-v1) – a reference ARI implementation that can execute WASM agents securely, publish L3 presence, and participate in auctions.
4. Storage and intelligence – Cloudflare R2 for binaries, Groq for meta-orchestration, HNSW indexes for semantic agent search.

## Canonical Documents
- [00_AINUR_PROTOCOL_OVERVIEW](./00_AINUR_PROTOCOL_OVERVIEW.md) – full-stack narrative of all nine layers.
- [01_TEMPORAL_LEDGER](./01_TEMPORAL_LEDGER.md) – chain design, pallets, and on-chain economics.
- [PLANETARY_AI_PROTOCOL_COMPLETE_ARCHITECTURE](./PLANETARY_AI_PROTOCOL_COMPLETE_ARCHITECTURE.md) – blueprint synthesizing state-of-the-art research.
- [AI_COLLABORATION_BRIEF](./AI_COLLABORATION_BRIEF.md) – context packet for collaborating AI systems.
- [L3-Aether-Topics-v1](./L3-Aether-Topics-v1.md) – transport topics, CQ-routing, and presence semantics.
- [L5-ARI-v1](./L5-ARI-v1.md) – runtime interface specification for WASM, Python, Docker, and hybrid runtimes.
- [COMPLETE_SPRINT_ROADMAP](./COMPLETE_SPRINT_ROADMAP.md) and [COMPREHENSIVE_FEATURE_BRAINSTORM](./COMPREHENSIVE_FEATURE_BRAINSTORM.md) – roadmap lineage and feature backlog.
- [GETTING_STARTED](./GETTING_STARTED.md) – build scripts, environment setup, and operational runbooks.

## Build Instructions
```
# Chain
cd chain-v2
cargo fmt --all
cargo clippy --all -- -D warnings
cargo test --all

# Orchestrator
cd cmd/api
go test ./...
go build ./...

# Reference Runtime
cd reference-runtime-v1
go test ./...
```

## Quality Gates
1. `cargo clippy --all -- -D warnings` and `cargo test --all` for the chain.
2. `go test ./...` for every Go module in `libs/`, `cmd/api/`, and `reference-runtime-v1/`.
3. Integration scripts under `tests/` for P2P discovery, WASM execution, escrow, and reputation.
4. Docusaurus build for this documentation (`npm run build` inside ainur-docs/).

## Deployment Targets
- API and orchestrator on Fly.io with horizontal scaling controlled by queue depth.
- Substrate node cluster with RPC endpoints exposed through Cloudflare Zero Trust.
- Cloudflare R2 buckets for agent binaries and wasm modules, versioned by hash.
- Grafana, Loki, Tempo, and Sentry dashboards linked from `docs/OPERATIONS`. (These documents will be migrated into this site as they are rewritten in MDX-compliant form.)

## Current Priorities
1. Finalize Sprint 9: ARI-first execution path, runtime registry backed by L3 presence, and end-to-end ARI integration tests.
2. Harden the Substrate economic layer with milestone escrow and payment proofs.
3. Publish the remaining whitepapers (L2 Verity, L4 Concordat, L4.5 Nexus, L5.5 Warden, L6 Koinos) in this docs site.
4. Launch `docs.ainur.network` as the canonical reference once all legacy Markdown has been normalized for MDX.

## Runtime Registry Observability
- `/api/v1/runtime-registry` – lists every discovered ARI runtime with DID, capabilities, and last-seen timestamps.
- `/api/v1/runtime-registry/:did` – retrieves the live manifest for a specific runtime.
- `/api/v1/runtime-registry/health` – returns aggregated counts per status/capability plus the active presence topic.
- Prometheus metrics exported as:
  - `ainur_runtime_registry_count`
  - `ainur_runtime_registry_status{status="online|offline|busy"}`
  - `ainur_runtime_registry_capabilities{capability="<name>"}`
  - `ainur_runtime_registry_events_total{event="discovered|updated|removed|timed_out"}`

Use these endpoints to populate control-plane dashboards (“Discovered Runtimes”) and to validate that P2P presence is functioning before dispatching tasks through ARI-v1.

Every commit should reinforce determinism, verifiability, and composability. If a change does not move the protocol closer to that standard, re-evaluate it.
