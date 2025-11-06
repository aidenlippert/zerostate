# zerostate — 12-week MVP sprint plan (FAANG-grade)

Status: draft v0.1 (2025-11-05)
Owner (TPM): TBD • Tech Lead: TBD • SRE Lead: TBD • Security Lead: TBD

Assumptions
- Name is "zerostate" (lowercase brand).
- Implementation languages: Go for networking (libp2p, DHT, relays), Rust optional for high-performance executors; TypeScript for CLI/tools; Python for load generation/sim.
- Infrastructure: GitHub + Actions, Docker/OCI, k8s for relays/indexers in staging, Terraform for cloud infra, Grafana/Prometheus/Tempo/Loki for observability, NATS or Kafka optional for internal events.
- MVP scope targets a functional testnet with regional relays, centralized HNSW service, P2P identity/discovery, guild lifecycle with WASM execution, state-channel mock, basic reputation ledger, and Q-routing prototype.

North-star outcomes (MVP)
- Join: new peer bootstraps over libp2p+Kademlia, publishes a signed Agent Card, discoverable via DHT.
- Find: semantic discovery via a centralized HNSW index (per-region shards starting as a single service), shortlisted agents validated via DHT metadata.
- Form: ephemeral guild with private tenant ring (encrypted), run a task manifest (WASM), record receipts and signed logs.
- Settle: state channel mock for off-chain metering, dispute path to settlement placeholder.
- Route: Q-routing on relays chooses next-hops under latency/failure/cost.
- Observe: metrics, traces, logs, dashboards; well-defined SLOs and error budgets; runbooks and oncall.

KPIs / Success metrics
- P95 task end-to-end latency <= 1.5s for intra-region WASM jobs (<=128MB IO) on staging.
- Discovery latency (DHT+HNSW shortlist) <= 300ms P95 intra-region.
- 99.5% success rate for guild formation and task execution on staging under 500 concurrent tasks.
- Crash-free session rate >= 99.9% for edge node.
- CI: >90% unit test coverage for core libraries; e2e green for main flows; mean PR review turnaround < 24h.

Quality bars & DoD (global)
- Every service: structured logs, metrics, traces with OpenTelemetry; health endpoints; readiness/liveness probes.
- Security: dependency scanning, SAST, secrets scanning; signed container images (Sigstore cosign) and SBOM.
- Tests: unit + integration + e2e; load test for critical paths; golden examples for schemas.
- Docs: ADRs for major decisions; updated READMEs, runbooks, and API references.

Team & roles (MVP-scale)
- Tech Lead (1), Senior BE (2), Networking Eng (1), Systems/Rust Eng (1), SRE (1), Security Eng (0.5), PM/TPM (0.5), Data/Search Eng (0.5), QA (0.5). Total ~8 FTEs.

Tooling & process
- Branching: trunk-based with short-lived feature branches; protected main; mandatory code review (2 LGTMs for critical code paths).
- RFC/ADR: light RFC issue → ADR file for accepted decisions (docs/adr/ADR-XXXX-title.md).
- CI/CD: GitHub Actions with matrix builds (Linux/amd64/arm64), caching, lint/format, unit/integration, container build, SBOM + cosign attestations.
- Releases: semantic versioning; weekly tagged releases to staging; canary deployment for relays.
- Issue tracking: Epics/Sprints in Jira (or GitHub Projects). Labels: component, priority, risk.

Architecture recap (MVP)
- Overlays: DHT (Kademlia) for identity/pointers, centralized HNSW for semantic discovery, private tenant rings for guilds, state channel mock for payments, optional chain anchors.
- Tiers: edge nodes (phones/laptops), regional relays (PoPs), backbone (archival/GPU later), boot/anchor nodes.

Epics (with owners, DoD)
1. Identity & Discovery (Owner: Networking Eng)
   - Go libp2p node; Kademlia DHT (k≈20, α≈3); Agent Card publish/resolve; signed JSON-LD.
   - DoD: peer joins, publishes card, discoverable in <=300ms P95 intra-region; e2e test covers create/update/resolve.
2. Semantic Index (Owner: Data/Search Eng)
   - Centralized HNSW service; API: upsert(query) with metadata; region-aware shard config (start with 1 shard).
   - DoD: 100k vectors, QPS 500, P95 <= 100ms query latency local; e2e shortlist flow integrated.
3. Guild Lifecycle & Execution (Owner: Systems Eng)
   - Private tenant ring (encrypted channels), WASM sandbox, Task Manifest execution, receipts + signed logs.
   - DoD: run hello-world WASM with inputs; receipts visible; teardown cleans keys and channels.
4. Payments & Reputation (Owner: Senior BE)
   - State channel mock API, escrow accounting, dispute stub; reputation ledger append-only; slashing skeleton.
   - DoD: open/update/close channel in tests; metered settlement off-chain; reputation increment path.
5. Routing & Relays (Owner: Networking Eng)
   - Regional relay service, Q-routing policy, policy-based next-hop; fallback to backbone path.
   - DoD: Q-router selects hop; measurable latency improvement under synthetic load; observability for decisions.
6. SRE/Observability/Security (Owner: SRE Lead)
   - OTel traces/metrics/logs, dashboards, SLOs, alert rules, oncall, chaos drills, security scanning and SBOM.
   - DoD: dashboards live; error budgets tracked; runbooks for top 5 incidents; weekly canaries + synthetic.

Milestones
- M1 (end Sprint 2): Identity/Discovery + Semantic shortlist; basic dashboards and CI are live.
- M2 (end Sprint 4): Guilds with WASM, receipts, Q-routing prototype; load tests green at 200 concurrent tasks.
- M3 (end Sprint 6): Payment mock, reputation, region hardening, chaos tests, release MVP v0.1.

Sprints (6×2 weeks)

Sprint 1 — Bootstrap, Identity, CI/CD
Goals
- Repo scaffolding, CI/CD, lint/format, pre-commit, CODEOWNERS, PR templates.
- Go libp2p node skeleton; Kademlia join; Agent Card schema + signing/verification library.
- Basic observability scaffold (OTel exporter; Prometheus metrics).
Deliverables
- edge-node and relay repos or monorepo packages; libp2p join with bootnodes.
- Agent Card publish/resolve API; example CLI.
- CI: unit tests, lint, container build; signed images, SBOM via Syft; cosign attestations.
Acceptance criteria
- New node publishes Agent Card; discoverable by another node within same region; CI fully green.

Sprint 2 — Semantic Index v0, Discovery Flow
Goals
- Centralized HNSW service (Go or Rust) with REST/gRPC; vector upsert/query; metadata filters.
- Integrate with DHT: shortlist from HNSW → validate via Agent Card pointers.
- Dashboards for discovery latency and error rate.
Deliverables
- hnsw-index service container; SDK client; integration tests covering candidate shortlist.
- Load test: 100k vectors, QPS 500; P95 <= 100ms local; discovery P95 <= 300ms.
Acceptance criteria
- End-to-end: query → shortlist → DHT resolve works on staging; dashboards show SLI/SLOs.

Sprint 3 — Guilds & WASM Execution v0
Goals
- Private tenant ring: ephemeral keys, ACL; encrypted channels; membership TTL.
- WASM runner: WASI support, resource limits; Task Manifest execution; inputs via IPFS fetch.
- Receipts + signed logs; audit anchors (hashes) stub.
Deliverables
- guild-service in relay; wasm-runner library and sidecar; Task Manifest validation.
- e2e: hello-world WASM with inputs; receipts visible; teardown cleans resources.
Acceptance criteria
- Guild form/execute/dissolve in < 2s P95; resource limits enforced; signed logs stored.

Sprint 4 — Q-Routing Prototype & Load Tests
Goals
- Q-router in relays (ε-greedy, tabular/linear), hook into libp2p dial policy.
- Workload-aware policy (SLA class, region); synthetic load harness; path telemetry.
- SLOs defined: discovery, guild formation, task execution; error budgets and alerts.
Deliverables
- qrouter module; feature flags; comparative latency report vs baseline.
- Load tests at 200 concurrent tasks; chaos: relay kill and network impairment.
Acceptance criteria
- Measurable improvement in P95 under injected jitter/loss; alerts and runbooks created.

Sprint 5 — Payments Mock, Reputation & Compliance
Goals
- State channel mock API (open/update/close), escrow accounting; dispute path stub; settlement placeholder.
- Reputation ledger append-only; slashing skeleton; minimal ZK accumulator placeholder.
- Compliance: signed logs, optional audit anchor hashes.
Deliverables
- payment-mock service; client SDK; reputation store; policy gates in discovery based on reputation.
Acceptance criteria
- Task execution metered and reflected in channel balance; reputation updated; dispute stub path works.

Sprint 6 — Hardening, Scale-out, Release v0.1
Goals
- Regionalization knobs (HNSW shard config); cache policy (LRU+trust); edge energy budget policy.
- Security review; dependency and container scans; secrets rotation; supply chain hardening.
- Scale tests: 500 concurrent tasks; disaster drills; finalize docs and runbooks.
Deliverables
- regional configs; tuned caches; security findings addressed; release notes.
Acceptance criteria
- All SLOs met; chaos tests pass; MVP v0.1 tagged and deployed to staging testnet.

Detailed work breakdown (per epic)

Identity & Discovery
- libp2p QUIC/TLS, NAT traversal (hole punching/relay as needed), mDNS LAN discovery.
- Kademlia params: k=20, α=3; bootnode list; peerstore; provider records for Agent Card pointers (ipfs:// or content hash).
- Agent Card signing: Ed25519 initially, PQC upgrade path noted in schema; verification library with deterministic tests.
- CLI: publish/update/revoke card; resolve and show endpoints/capabilities.
- E2E tests: multi-peer discovery on docker-compose/k8s-kind.

Semantic Index
- HNSW index build and persistence (mmap); upsert/query APIs; metadata filters (region, capability tags).
- Batch import from a fixture; warmup and background compaction.
- ANN recall/latency benchmarks; tracing spans across query → DHT resolve.

Guild Lifecycle & Execution
- Group key agreement (X25519), ephemeral per guild; control plane messages signed.
- Tenant ring addressing (private DHT buckets or overlay topic); ACL enforcement.
- WASM runner with WASI, memory/cpu limits; sandboxed FS; deterministic mode where possible.
- IPFS client integration with timeouts/retries; artifact cache with RF=3 in-region.
- Task Manifest validation; receipts include metering and content hashes.

Payments & Reputation
- Channel mock: escrow ledger, signed updates (off-chain), cooperative close; dispute stub writes to settlement placeholder.
- Pricing/metering units (req/sec/token/GB); adapters for task metering.
- Reputation: append-only events; score function; gates in discovery/routing.

Routing & Relays
- Relay service: neighbor maintenance, health checks, capacity reporting.
- Q-router features: rtt/loss/jitter/price/reputation/regionMatch/SLA/size/ttl; persisted Q-table with EMA decay.
- Policy guards: allow/deny lists; privacy/guild boundary enforcement.

SRE/Observability/Security
- OpenTelemetry SDKs; context propagation across components; exemplars.
- Metrics: SLI dashboards (discovery latency, guild formation time, task success); logs in Loki; traces in Tempo.
- SLOs and alert rules; synthetic probes; blackbox exporter for discovery and task run.
- Security: SAST (CodeQL), dependency scans, Trivy image scans; SBOM (Syft), cosign signing in CI; secrets via Vault or OIDC + cloud KMS.
- Runbooks: incident response, paging policy, severity matrix.

Environments
- Dev: docker-compose; make targets.
- CI: ephemeral containers; unit/integration; e2e smoke on k8s-kind.
- Staging: k8s cluster with 2 regions simulated; canary relays; HNSW service.
- Testnet (optional by end of Sprint 6): external access with gated allowlist.

Testing strategy
- Unit: >90% coverage for core libs (signing, DHT interface, Task Manifest validation).
- Integration: DHT publish/resolve; HNSW shortlist; guild formation; WASM execution.
- E2E: full flow from query → execution → receipts → payment mock.
- Load: k6 or Locust for concurrency; network impairment via tc/netem; chaos via pod deletes.

Risk register (top 8)
1) NAT traversal flakiness on some ISPs → Mitigation: relay fallbacks, ICE+hole punching, integration tests in CI.
2) HNSW centralization bottleneck → Mitigation: focus on read-heavy performance; phase-in regional shards; caching.
3) WASM runtime limitations → Mitigation: compatible toolchains; container bridge for GPU workloads (backbone-only in post-MVP).
4) Payment complexity creep → Mitigation: mock channels only; strict scope control; clear stubs for future integration.
5) Security regressions → Mitigation: CI security gates, threat model review in Sprint 6, periodic scans.
6) Observability blind spots → Mitigation: define SLIs early (Sprint 2), synthetic probes by Sprint 4.
7) Data consistency between HNSW and DHT → Mitigation: validation step always consults DHT; background reconciliation jobs.
8) Team bandwidth → Mitigation: explicit WIP limits, focus epics per sprint, reduce context switching.

Backlog (Phase 2+)
- Regional HNSW shards + federated ANN queries; gossip for index deltas.
- Real state channels + settlement chain integration (Cosmos SDK zone).
- ZK attestations for reputation thresholds.
- GPU marketplace and container execution on backbone.
- Governance primitives (emergency multisig, proposal queues).

Operational playbooks (MVP)
- Incident response: triage, rollback, user comms.
- Disaster recovery: restore HNSW index from snapshot; DHT reseed; key rotation.
- Change management: canary releases; feature flags; dark launches.

Appendices
- ADR template and sample entries.
- API surface summaries for: Agent Card publish/resolve, HNSW upsert/query, Guild control plane, Payment mock.
- Example dashboards: Discovery latency, Guild lifecycle timings, Q-router decisions.
