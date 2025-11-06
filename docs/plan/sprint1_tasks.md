# Sprint 1 — Bootstrap, Identity, CI/CD (Task Breakdown)

**Sprint Goal:** Establish repo structure, CI/CD pipelines, libp2p+Kademlia DHT, Agent Card publish/resolve, and foundational observability. All code is tested, linted, containerized, signed, and deployed to dev/staging.

**Duration:** 2 weeks (10 working days)
**Team:** Tech Lead, 2 Senior BE, Networking Eng, Systems Eng, SRE, Security Eng (0.5)

---

## Epic 1: Repository & Tooling Setup

### INFRA-1: Initialize monorepo structure
**Owner:** Tech Lead  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Create monorepo layout (Go modules or Turborepo/Nx if multi-lang).
- Directories: `services/`, `libs/`, `specs/`, `docs/`, `tools/`, `tests/`, `deployments/`, `examples/`.
- Root-level: `go.work` (or equivalent), `Makefile`, `scripts/`, `.github/`, `.vscode/`.
**Acceptance Criteria:**
- Directory structure matches docs/plan/sprint_plan.md conventions.
- `make help` lists all targets.
- CODEOWNERS file defines ownership for each top-level directory.

---

### INFRA-2: Configure linting and formatting
**Owner:** Senior BE  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Go: `golangci-lint` with config (`.golangci.yml`) enabling: `gofmt`, `goimports`, `govet`, `staticcheck`, `errcheck`, `gosec`, `ineffassign`.
- Markdown: `markdownlint`.
- YAML/JSON: `yamllint`, `jsonlint`, or `prettier`.
- Pre-commit hooks via `pre-commit` framework or `lefthook`.
**Acceptance Criteria:**
- `make lint` runs all checks; exit 0 on clean code.
- Pre-commit hook blocks commits with lint errors.
- CI enforces lint on every PR.

---

### INFRA-3: Set up GitHub Actions CI/CD
**Owner:** SRE  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Workflows: `ci.yml` (lint, test, build), `release.yml` (tag, sign, publish containers).
- Matrix builds: Linux amd64/arm64.
- Caching: Go modules, Docker layers.
- Secrets: GitHub OIDC for cosign, registry creds.
**Acceptance Criteria:**
- PR triggers lint + unit tests + integration tests.
- Main branch triggers container build + SBOM + cosign signing.
- Workflow completes in <5 min for typical PR.

---

### INFRA-4: Container build and OCI registry
**Owner:** SRE  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Multi-stage Dockerfiles for services: `edge-node`, `relay`, `hnsw-index`.
- Base images: `gcr.io/distroless/static` or `alpine:3.18`.
- Push to GitHub Container Registry (ghcr.io) or internal registry.
- Tag strategy: `git-sha`, `branch-name`, `vX.Y.Z` for releases.
**Acceptance Criteria:**
- `make docker-build` builds all service images.
- Images tagged and pushed on main merge.
- Images scannable by Trivy.

---

### INFRA-5: SBOM generation and signing
**Owner:** Security Eng  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Generate SBOM with Syft for each container image.
- Sign images and SBOM with cosign using keyless (OIDC) signing.
- Store signatures in registry alongside images.
**Acceptance Criteria:**
- `cosign verify` succeeds for all published images.
- SBOM attached and queryable via `cosign download sbom`.
- CI fails if signing step fails.

---

### INFRA-6: Dependency scanning and vulnerability checks
**Owner:** Security Eng  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- Dependabot or Renovate for automated dependency updates.
- Trivy scans for container vulnerabilities in CI.
- SAST with CodeQL (Go) on every PR.
**Acceptance Criteria:**
- CVE threshold: fail CI if HIGH or CRITICAL vulnerabilities.
- Dependabot PRs auto-created weekly.
- CodeQL scans complete without blocking issues.

---

### INFRA-7: Local dev environment (docker-compose)
**Owner:** Senior BE  
**Priority:** P1  
**Effort:** 3 points  
**Description:**
- `docker-compose.yml` for local dev: bootnode, 2 edge nodes, 1 relay, Prometheus, Grafana, Jaeger.
- Makefile targets: `make dev-up`, `make dev-down`, `make dev-logs`.
- Environment variables via `.env.example`.
**Acceptance Criteria:**
- `make dev-up` starts all services; edge node joins DHT.
- Grafana accessible at localhost:3000 with dashboards.
- Jaeger UI shows traces.

---

### INFRA-8: ADR template and process
**Owner:** Tech Lead  
**Priority:** P2  
**Effort:** 1 point  
**Description:**
- Create `docs/adr/template.md` (Nygard format).
- Document process in `docs/adr/README.md`.
- First ADR: ADR-0001-use-libp2p-and-kademlia.md.
**Acceptance Criteria:**
- Template exists with numbered sections: Context, Decision, Consequences, Alternatives.
- ADR-0001 written and merged.

---

## Epic 2: Networking & Identity (libp2p + DHT)

### NET-1: Initialize libp2p host with QUIC transport
**Owner:** Networking Eng  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Create `libs/p2p` module.
- Initialize libp2p host with QUIC/TLS transport, noise security, yamux muxer.
- NAT traversal: AutoNAT, hole punching (relay v2 optional).
- Peer ID from Ed25519 keypair.
**Acceptance Criteria:**
- Host starts and listens on configurable ports.
- Unit test: two hosts dial each other over loopback.
- Logs peer ID and multiaddrs on startup.

---

### NET-2: Kademlia DHT integration
**Owner:** Networking Eng  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Integrate go-libp2p-kad-dht (or equivalent).
- Config: k=20, alpha=3, protocol prefix `/zerostate/kad/1.0.0`.
- Bootstrap mode vs. server mode switch.
- Provider records for Agent Card content hashes.
**Acceptance Criteria:**
- Node bootstraps from configured bootnode list.
- DHT routing table populated (visible via debug endpoint).
- Provider records published and retrievable.

---

### NET-3: Agent Card signing and verification library
**Owner:** Senior BE  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Create `libs/identity` module.
- Sign Agent Card JSON with Ed25519 (W3C Data Integrity style proof).
- Verify signature; reject tampered cards.
- DID generation helpers (did:key or did:peer).
**Acceptance Criteria:**
- Sign and verify golden test cases pass.
- Malformed or tampered cards rejected with clear errors.
- DID generation deterministic from keypair.

---

### NET-4: Agent Card publish to DHT
**Owner:** Networking Eng  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Serialize Agent Card to JSON-LD.
- Compute content hash (multihash, e.g., sha256).
- Publish provider record to DHT; optionally store card in IPFS-lite or local store.
- API: `PublishCard(ctx, card) -> (cid, error)`.
**Acceptance Criteria:**
- Publish succeeds; card retrievable via DHT within 300ms P95 local.
- Integration test: publish from node A, resolve from node B.

---

### NET-5: Agent Card resolve from DHT
**Owner:** Networking Eng  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- API: `ResolveCard(ctx, did) -> (card, error)`.
- Lookup provider record by DID or content hash.
- Fetch card content, verify signature, parse JSON-LD.
**Acceptance Criteria:**
- Resolve returns valid card; signature verified.
- Latency logged; P95 <= 300ms in local docker-compose setup.

---

### NET-6: CLI for Agent Card publish/update/resolve
**Owner:** Senior BE  
**Priority:** P1  
**Effort:** 3 points  
**Description:**
- CLI tool: `zerostate-cli` (Cobra framework).
- Commands: `card publish`, `card update`, `card resolve`, `card show`.
- Flags: `--did`, `--key-file`, `--bootnode`, `--output json|yaml`.
**Acceptance Criteria:**
- `zerostate-cli card publish` creates and publishes card.
- `zerostate-cli card resolve <did>` fetches and displays card.
- Help text clear; examples in `--help`.

---

### NET-7: mDNS local discovery (LAN peers)
**Owner:** Networking Eng  
**Priority:** P2  
**Effort:** 2 points  
**Description:**
- Enable libp2p mDNS for zero-config LAN discovery.
- Auto-connect to discovered peers if same protocol prefix.
**Acceptance Criteria:**
- Two nodes on same LAN discover each other without bootnode.
- Integration test in docker-compose with custom network.

---

### NET-8: Bootnode service
**Owner:** Networking Eng  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- Standalone bootnode service (minimal libp2p host in DHT server mode).
- Config: persistent peer ID, well-known multiaddrs.
- Health endpoint: `/healthz`.
**Acceptance Criteria:**
- Bootnode container runs; edge nodes bootstrap successfully.
- Logs show incoming connections.
- Health endpoint returns 200.

---

## Epic 3: Observability Foundation

### OBS-1: OpenTelemetry SDK integration
**Owner:** SRE  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Integrate OTel SDK (Go): traces, metrics, logs.
- Exporter: OTLP to collector (Jaeger for traces, Prometheus for metrics).
- Context propagation across service boundaries.
**Acceptance Criteria:**
- Traces visible in Jaeger for DHT operations.
- Metrics scraped by Prometheus.
- `otel-collector` config in `deployments/`.

---

### OBS-2: Structured logging
**Owner:** SRE  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Use `slog` (Go 1.21+) or `zap` for structured logs.
- Standard fields: `timestamp`, `level`, `msg`, `trace_id`, `peer_id`.
- Log levels: debug, info, warn, error; configurable via env var.
**Acceptance Criteria:**
- All services log in JSON format.
- Trace IDs propagate from OTel context.
- Logs parseable by Loki or ELK.

---

### OBS-3: Prometheus metrics and exporters
**Owner:** SRE  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Metrics library wrapper around OTel or native Prometheus client.
- Core metrics:
  - `zerostate_dht_lookups_total` (counter)
  - `zerostate_dht_lookup_duration_seconds` (histogram)
  - `zerostate_agent_card_publish_total` (counter)
  - `zerostate_peer_connections` (gauge)
- `/metrics` endpoint on each service.
**Acceptance Criteria:**
- Prometheus scrapes all services.
- Metrics visible in Prometheus UI.
- Example PromQL queries documented.

---

### OBS-4: Grafana dashboards (initial)
**Owner:** SRE  
**Priority:** P1  
**Effort:** 3 points  
**Description:**
- Dashboard: "Identity & Discovery".
  - Panels: DHT lookup rate, P95 latency, error rate, peer count.
- Dashboard: "System Health".
  - Panels: CPU, memory, goroutines, request rate.
- Export JSON to `deployments/grafana/dashboards/`.
**Acceptance Criteria:**
- Dashboards load in Grafana on `make dev-up`.
- Data sources auto-configured (Prometheus, Jaeger).

---

### OBS-5: Health and readiness endpoints
**Owner:** Senior BE  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- HTTP endpoints: `/healthz` (liveness), `/readyz` (readiness).
- Readiness checks: DHT bootstrap complete, keystore loaded.
- Return 200 OK or 503 Service Unavailable with JSON body.
**Acceptance Criteria:**
- K8s probes use these endpoints.
- Readiness fails until DHT routing table populated.

---

### OBS-6: Tracing instrumentation for DHT ops
**Owner:** Networking Eng  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- Add OTel spans for: DHT publish, DHT resolve, peer dial.
- Annotate spans with metadata: peer ID, content hash, latency.
**Acceptance Criteria:**
- End-to-end trace visible in Jaeger for publish → resolve flow.
- Span attributes include all critical metadata.

---

## Epic 4: Testing Infrastructure

### TEST-1: Unit test framework and coverage
**Owner:** Senior BE  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Go testing with `testify` assertions.
- Coverage target: >90% for `libs/` packages.
- `make test` runs all unit tests with coverage report.
**Acceptance Criteria:**
- CI fails if coverage drops below 85%.
- Coverage report uploaded to Codecov or similar.

---

### TEST-2: Integration test harness
**Owner:** Senior BE  
**Priority:** P0  
**Effort:** 3 points  
**Description:**
- Integration tests in `tests/integration/`.
- Use `testcontainers-go` or docker-compose for multi-node setups.
- Tests: publish card from node A, resolve from node B.
**Acceptance Criteria:**
- `make test-integration` runs tests in isolation.
- Tests clean up containers on exit.
- CI runs integration tests on PR.

---

### TEST-3: End-to-end smoke test
**Owner:** Networking Eng  
**Priority:** P1  
**Effort:** 3 points  
**Description:**
- E2E test: bootnode + 2 edge nodes + 1 relay.
- Flow: node publishes card → other node resolves → validates signature.
- Use CLI or SDK client in test script.
**Acceptance Criteria:**
- `make test-e2e` passes in CI.
- Test runs in <60s.
- Logs and artifacts saved on failure.

---

### TEST-4: Golden test fixtures
**Owner:** Senior BE  
**Priority:** P2  
**Effort:** 1 point  
**Description:**
- Golden files for Agent Card JSON (valid, invalid, malformed).
- Golden test: deserialize, sign, verify, compare output.
**Acceptance Criteria:**
- Fixtures in `tests/fixtures/agent_cards/`.
- Update script: `make update-golden`.

---

## Epic 5: Security & Compliance

### SEC-1: Secrets management setup
**Owner:** Security Eng  
**Priority:** P0  
**Effort:** 2 points  
**Description:**
- Use GitHub OIDC for cosign keyless signing.
- Store sensitive config in GitHub Secrets or cloud KMS.
- Document secrets rotation process in `docs/security/secrets.md`.
**Acceptance Criteria:**
- No hardcoded secrets in repo.
- CI accesses secrets securely.
- Rotation process tested.

---

### SEC-2: SAST with CodeQL
**Owner:** Security Eng  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- Enable GitHub CodeQL for Go.
- Weekly scheduled scans + scans on PR.
- Alert threshold: medium or higher.
**Acceptance Criteria:**
- CodeQL workflow running.
- No high/critical alerts on main branch.

---

### SEC-3: Dependency update policy
**Owner:** Security Eng  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Enable Dependabot for Go modules.
- Auto-merge patch updates if CI green.
- Weekly digest of minor/major updates.
**Acceptance Criteria:**
- Dependabot config in `.github/dependabot.yml`.
- At least one auto-merge in first week.

---

### SEC-4: Threat model (initial draft)
**Owner:** Security Eng  
**Priority:** P2  
**Effort:** 2 points  
**Description:**
- Document threat model in `docs/security/threat_model.md`.
- Focus: Sybil attacks, signature forgery, DHT poisoning, DoS.
- Mitigations mapped to design.
**Acceptance Criteria:**
- Reviewed by team; accepted mitigations documented.

---

## Epic 6: Documentation & Developer Experience

### DOC-1: Developer setup guide
**Owner:** Tech Lead  
**Priority:** P1  
**Effort:** 2 points  
**Description:**
- `docs/dev/setup.md`: prerequisites (Go, Docker, Make), clone, build, run.
- Troubleshooting section.
**Acceptance Criteria:**
- New dev follows guide and runs local stack in <15 min.

---

### DOC-2: API reference (initial)
**Owner:** Senior BE  
**Priority:** P2  
**Effort:** 2 points  
**Description:**
- Godoc comments for public APIs in `libs/p2p`, `libs/identity`.
- Generate docs with `go doc` or `pkgsite`.
**Acceptance Criteria:**
- `make docs` generates browsable API docs.
- Hosted on GitHub Pages or internal docs site.

---

### DOC-3: Architecture decision record (ADR-0001)
**Owner:** Tech Lead  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Write ADR-0001: "Use libp2p and Kademlia DHT for peer discovery and identity".
- Rationale, alternatives considered, trade-offs.
**Acceptance Criteria:**
- ADR reviewed and merged.

---

### DOC-4: PR and issue templates
**Owner:** Tech Lead  
**Priority:** P2  
**Effort:** 1 point  
**Description:**
- `.github/PULL_REQUEST_TEMPLATE.md`: checklist for tests, docs, changelog.
- `.github/ISSUE_TEMPLATE/`: bug, feature, epic.
**Acceptance Criteria:**
- Templates used on first PR and issue.

---

### DOC-5: CODEOWNERS file
**Owner:** Tech Lead  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Define owners for: `libs/`, `services/`, `docs/`, `deployments/`.
- Require 2 approvals for critical paths.
**Acceptance Criteria:**
- CODEOWNERS enforced on PRs.

---

## Epic 7: Deployment & Infrastructure

### DEPLOY-1: Kubernetes manifests (dev/staging)
**Owner:** SRE  
**Priority:** P1  
**Effort:** 3 points  
**Description:**
- Helm chart or kustomize for: bootnode, edge-node, relay.
- ConfigMaps for env vars; Secrets for keys.
- Namespace: `zerostate-dev`, `zerostate-staging`.
**Acceptance Criteria:**
- `make deploy-dev` deploys to local k8s (kind or minikube).
- Services reachable; health checks pass.

---

### DEPLOY-2: Terraform for cloud infra (optional/staging)
**Owner:** SRE  
**Priority:** P2  
**Effort:** 3 points  
**Description:**
- Terraform modules for: VPC, EKS/GKE, managed Prometheus, load balancer.
- State backend: S3/GCS with locking.
**Acceptance Criteria:**
- `terraform apply` provisions staging cluster.
- Outputs include cluster endpoint, Prometheus URL.

---

### DEPLOY-3: Continuous deployment (CD) to staging
**Owner:** SRE  
**Priority:** P2  
**Effort:** 2 points  
**Description:**
- GitHub Actions workflow: on main merge, deploy to staging.
- Blue-green or rolling update strategy.
- Smoke test post-deploy.
**Acceptance Criteria:**
- Successful main merge triggers deploy.
- Staging URL updated within 5 min.
- Rollback tested manually.

---

## Epic 8: Sprint Wrap-up & Review

### REVIEW-1: Sprint demo prep
**Owner:** Tech Lead  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Prepare demo script: publish card → resolve → show in Grafana.
- Record video or live demo.
**Acceptance Criteria:**
- Demo runs smoothly; stakeholders can reproduce.

---

### REVIEW-2: Retrospective and action items
**Owner:** PM/TPM  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Hold retro meeting; capture wins, blockers, action items.
- Document in `docs/retros/sprint1.md`.
**Acceptance Criteria:**
- Action items assigned and tracked in next sprint.

---

### REVIEW-3: Sprint report and metrics
**Owner:** PM/TPM  
**Priority:** P1  
**Effort:** 1 point  
**Description:**
- Report: velocity, completed points, test coverage, CI stability.
- Share with stakeholders.
**Acceptance Criteria:**
- Report published; SLOs tracked vs. targets.

---

## Summary

**Total tasks:** 49  
**Total estimated effort:** ~100 story points (assumes team velocity ~50 points/sprint; adjust sizing based on team capacity)

**Critical path (must-complete for Sprint 1 exit):**
- INFRA-1 to INFRA-5 (repo, CI/CD, containers, signing)
- NET-1 to NET-5 (libp2p, DHT, Agent Card publish/resolve)
- OBS-1 to OBS-3 (OTel, logs, metrics)
- TEST-1 to TEST-3 (unit, integration, e2e)
- SEC-1 (secrets)
- DOC-1 (setup guide)

**Nice-to-have (can slip to Sprint 2 if needed):**
- DEPLOY-2 (Terraform)
- NET-7 (mDNS)
- DOC-2 (API reference)
- TEST-4 (golden fixtures)

**Daily standups:** track blockers; adjust WIP limits (max 2 tasks per engineer in progress).

**Definition of Done (per task):**
- Code merged to main.
- Tests passing (unit + integration where applicable).
- Docs updated (inline comments + relevant markdown).
- No lint/security warnings.
- Reviewed and approved by >=1 peer (>=2 for critical path).
