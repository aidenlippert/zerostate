# ZeroState - Team Collaboration Quick Start

**Last Updated:** November 7, 2025
**For:** Team members joining the project

---

## 🚀 Quick Start (5 Minutes)

```bash
# 1. Clone and setup
git clone https://github.com/YOUR_ORG/zerostate.git
cd zerostate
make deps

# 2. Run tests to verify
make test

# 3. Start observability stack
cd deployments && docker-compose up -d && cd ..

# 4. Verify everything works
make health-check

# 5. Pick an issue and start coding!
```

---

## 📋 Project Status

**Current Sprint:** Sprint 7 - Application Layer
**Progress:** ~25% of production system complete
**Focus:** Building user-facing features (APIs, UI, orchestration)

### What We Have ✅
- ✅ P2P networking (libp2p, DHT, gossip)
- ✅ WASM execution engine
- ✅ Payment channels (state machine)
- ✅ Reputation system
- ✅ Observability stack (Prometheus, Grafana, Jaeger, Loki)
- ✅ 254 passing tests

### What We're Building 🔨
- 🔨 Agent registration API
- 🔨 Task submission API
- 🔨 Meta-agent orchestrator
- 🔨 Web UI
- 🔨 User authentication
- 🔨 Database integration
- 🔨 Payment integration (Stripe)

### Critical Gaps ❌
See [docs/GAP_ANALYSIS.md](docs/GAP_ANALYSIS.md) for complete list

---

## 🎯 How to Contribute

### 1. Find Work

**Option A: GitHub Issues**
```bash
# Browse issues
gh issue list --label "good-first-issue"

# Filter by component
gh issue list --label "api"
gh issue list --label "ui"
gh issue list --label "infrastructure"
```

**Option B: GitHub Project Board**
- Visit: https://github.com/YOUR_ORG/zerostate/projects/1
- Pick from "Sprint Backlog" column
- Move to "In Progress"

### 2. Create Branch

```bash
git checkout -b feature/your-feature-name
# Example: feature/agent-registration-api
```

### 3. Make Changes

Follow [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Code style
- Testing requirements
- Commit message format

### 4. Test Locally

```bash
make lint          # Run linters
make test-unit     # Run unit tests
make test-integration  # Run integration tests
```

### 5. Submit PR

```bash
git push origin feature/your-feature-name
gh pr create --fill
```

**PR will auto-populate with template!**

### 6. Get Reviewed

- Auto-assigned to reviewers via CODEOWNERS
- Address feedback
- Get 2 approvals
- Merge!

---

## 🏗️ Architecture Overview

```
Application Layer (🔨 BUILDING THIS)
├── API Server          - REST endpoints
├── Web UI              - React dashboard
├── Orchestrator        - Task routing
└── Authentication      - User management

Economic Layer (✅ DONE)
├── Payment Channels    - Off-chain payments
├── Reputation System   - Quality scoring
└── Settlement          - Dispute resolution

Execution Layer (✅ DONE)
├── WASM Runtime        - Sandboxed execution
├── Guild Manager       - Task coordination
└── Resource Metering   - Cost tracking

Discovery Layer (✅ DONE)
├── HNSW Index          - Vector search
├── Q-Learning Router   - Adaptive routing
└── Agent Cards         - Identity/capabilities

P2P Layer (✅ DONE)
├── libp2p Network      - P2P communication
├── DHT (Kademlia)      - Decentralized discovery
└── Gossip Protocol     - Message propagation

Observability (✅ DONE)
├── Metrics (Prometheus)
├── Tracing (Jaeger)
├── Logging (Loki)
└── Health Checks
```

---

## 📦 Repository Structure

```
zerostate/
├── libs/                    # Core libraries
│   ├── api/                # 🔨 API handlers (building)
│   ├── auth/               # 🔨 Authentication (building)
│   ├── orchestration/      # 🔨 Meta-agent (building)
│   ├── p2p/                # ✅ P2P networking
│   ├── execution/          # ✅ WASM runtime
│   ├── economic/           # ✅ Payments
│   ├── reputation/         # ✅ Reputation
│   ├── telemetry/          # ✅ Observability
│   ├── health/             # ✅ Health checks
│   └── metrics/            # ✅ Metrics
├── web/                    # 🔨 Web UI (building)
├── tests/
│   ├── integration/        # ✅ Integration tests
│   └── chaos/              # ✅ Chaos tests
├── deployments/
│   ├── docker-compose.yml  # Local development
│   ├── k8s/                # Kubernetes manifests
│   └── grafana/            # Dashboards
├── docs/
│   ├── GAP_ANALYSIS.md     # What's missing
│   ├── GITHUB_ISSUES.md    # Pre-formatted issues
│   ├── TEAM_SETUP.md       # Team collaboration guide
│   └── CONTRIBUTING.md     # Contribution guidelines
└── .github/
    ├── workflows/ci.yml    # CI/CD pipeline
    ├── ISSUE_TEMPLATE/     # Issue templates
    └── pull_request_template.md
```

---

## 🛠️ Development Commands

### Essential Commands

```bash
# Install dependencies
make deps

# Run tests
make test              # All tests
make test-unit         # Unit tests only
make test-integration  # Integration tests only

# Code quality
make lint              # Run linters
make fmt               # Format code

# Build
make build             # Build binaries
make docker-build      # Build Docker images

# Development environment
make dev-up            # Start observability stack
make dev-down          # Stop observability stack
make dev-logs          # View logs

# Health checks
make health-check      # Verify all services
```

### Useful Commands

```bash
# Watch for changes and run tests
make watch

# Generate coverage report
make coverage

# Run benchmarks
make bench

# Open dashboards
make dashboard         # Grafana
make traces            # Jaeger

# Run security scans
make security-scan
```

---

## 🔍 Finding Your Way Around

### I want to work on...

**APIs**
- Location: `libs/api/`
- Issues: Label `api`
- Examples: Agent registration, task submission

**Web UI**
- Location: `web/`
- Issues: Label `ui`, `frontend`
- Stack: React + TypeScript + Tailwind

**Orchestration**
- Location: `libs/orchestration/`
- Issues: Label `orchestration`
- Examples: Meta-agent, auction mechanism

**Authentication**
- Location: `libs/auth/`
- Issues: Label `auth`, `security`
- Examples: JWT, API keys, user management

**Database**
- Location: `libs/db/`
- Issues: Label `database`, `infrastructure`
- Examples: PostgreSQL integration, migrations

**Infrastructure**
- Location: `deployments/`
- Issues: Label `infrastructure`, `devops`
- Examples: Docker, Kubernetes, CI/CD

**Documentation**
- Location: `docs/`
- Issues: Label `documentation`
- Examples: API docs, runbooks, guides

---

## 🎓 Learning Resources

### Understanding the Codebase

1. **Start Here:** [docs/PROJECT_STATUS.md](docs/PROJECT_STATUS.md)
2. **Architecture:** [docs/plan/sprint_plan.md](docs/plan/sprint_plan.md)
3. **What's Missing:** [docs/GAP_ANALYSIS.md](docs/GAP_ANALYSIS.md)
4. **Sprint Progress:** [docs/SPRINT_6_COMPLETE.md](docs/SPRINT_6_COMPLETE.md)

### Technical Guides

- **Observability:** [docs/DISTRIBUTED_TRACING_GUIDE.md](docs/DISTRIBUTED_TRACING_GUIDE.md)
- **Logging:** [docs/STRUCTURED_LOGGING_GUIDE.md](docs/STRUCTURED_LOGGING_GUIDE.md)
- **Health Checks:** [docs/HEALTH_CHECK_GUIDE.md](docs/HEALTH_CHECK_GUIDE.md)
- **Testing:** [docs/OBSERVABILITY_TEST_GUIDE.md](docs/OBSERVABILITY_TEST_GUIDE.md)

### Team Guides

- **Contributing:** [CONTRIBUTING.md](CONTRIBUTING.md)
- **Team Setup:** [docs/TEAM_SETUP.md](docs/TEAM_SETUP.md)
- **Code Review:** [CONTRIBUTING.md#code-review-process](CONTRIBUTING.md#code-review-process)

---

## 👥 Team Communication

### Channels

- **GitHub Issues:** Bug reports, feature requests, tasks
- **GitHub Discussions:** Architecture, Q&A, brainstorming
- **Pull Requests:** Code reviews, technical discussion
- **Slack/Discord:** Real-time communication

### Response Times

| Issue Type | Response Time |
|------------|---------------|
| P0 (Critical) | < 4 hours |
| P1 (High) | < 24 hours |
| P2 (Medium) | < 3 days |
| P3 (Low) | < 1 week |

---

## 🚦 CI/CD Pipeline

### Automated Checks

Every PR runs:
- ✅ Linting (golangci-lint)
- ✅ Unit tests (all packages)
- ✅ Integration tests (observability stack)
- ✅ Security scans (gosec, trivy)
- ✅ Build verification
- ✅ Code coverage (uploaded to Codecov)

### Merge Requirements

- [ ] All CI checks pass
- [ ] 2 approvals from reviewers
- [ ] No unresolved comments
- [ ] Up to date with `main` branch

---

## 📊 Dashboards & Monitoring

### Local Development

- **Grafana:** http://localhost:3000 (admin/admin)
  - System Overview dashboard
  - P2P Metrics dashboard
  - Execution Metrics dashboard
  - Economic Layer dashboard

- **Prometheus:** http://localhost:9090
  - Metrics browser
  - Query interface

- **Jaeger:** http://localhost:16686
  - Distributed tracing UI
  - Trace search and analysis

- **Loki:** http://localhost:3100
  - Log aggregation
  - Query via Grafana

### Observability

Every feature should include:
1. **Metrics** (Prometheus counters/gauges/histograms)
2. **Tracing** (OpenTelemetry spans)
3. **Logging** (Structured Zap logs with trace correlation)
4. **Health checks** (if applicable)

---

## 🐛 Troubleshooting

### Common Issues

**Tests failing locally**
```bash
# Clean cache and retry
make clean
go clean -testcache
make test
```

**Docker out of space**
```bash
docker system prune -a --volumes
```

**Port conflicts**
```bash
# Check what's using port
lsof -i :9090

# Kill process
kill -9 <PID>

# Or use different ports in docker-compose.yml
```

**Module issues**
```bash
go work sync
go mod download
go mod tidy
```

**CI passing but failing locally**
```bash
# Run in CI environment
docker run -v $(pwd):/app -w /app golang:1.21 make test
```

---

## 📝 Sprint Planning

### Current Sprint: Sprint 7

**Goal:** Build Application Layer
**Duration:** 2 weeks
**Status:** In Progress

**Key Deliverables:**
- [ ] Agent Registration API (Issue #1)
- [ ] Task Submission API (Issue #2)
- [ ] Meta-Agent Orchestrator (Issue #3)
- [ ] Basic Web UI (Issue #5)
- [ ] User Authentication (Issue #4)

**Next Sprint:** Sprint 8 - Payments & Database

---

## 🎯 Good First Issues

Perfect for new contributors:

1. **Add API documentation** - Document existing APIs
2. **Write unit tests** - Increase test coverage
3. **Fix linting warnings** - Clean up code quality
4. **Add metrics** - Instrument existing code
5. **Improve error messages** - Make errors more helpful

Filter on GitHub: `label:good-first-issue`

---

## 💡 Tips for Success

### Do ✅
- **Small PRs** (<500 lines)
- **Test everything**
- **Document decisions**
- **Ask questions** early
- **Review others' PRs**
- **Update docs**

### Don't ❌
- **Large PRs** (>500 lines)
- **Skip tests**
- **Break CI**
- **Ignore review feedback**
- **Commit secrets**
- **Force push** to `main`

---

## 📞 Getting Help

### Quick Questions
- Comment on the issue
- Ask in Slack/Discord #engineering

### Technical Discussion
- Open a GitHub Discussion
- Schedule a pairing session

### Blocked?
- Comment on your PR/issue
- Tag relevant team members
- Post in #engineering channel

---

## 🎉 Welcome!

We're excited to have you on the team! Don't hesitate to ask questions. Everyone was new once.

**Next Steps:**
1. Set up your development environment
2. Read [CONTRIBUTING.md](CONTRIBUTING.md)
3. Pick a "good first issue"
4. Join the daily standup
5. Make your first PR!

---

**Questions?** Open a GitHub Discussion or ask in #engineering!

**Ready to contribute?** Check out [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.
