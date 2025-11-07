# Contributing to ZeroState

Thank you for your interest in contributing to ZeroState! This document provides guidelines and workflows for contributing to the project.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Coding Standards](#coding-standards)
- [Testing Requirements](#testing-requirements)
- [Pull Request Process](#pull-request-process)
- [Issue Reporting](#issue-reporting)
- [Communication](#communication)

---

## Code of Conduct

### Our Standards

- **Be respectful** and inclusive
- **Be collaborative** and constructive
- **Focus on what is best** for the community
- **Show empathy** towards other community members

### Unacceptable Behavior

- Harassment, discrimination, or offensive comments
- Trolling, insulting/derogatory comments, personal attacks
- Public or private harassment
- Publishing others' private information
- Other unethical or unprofessional conduct

---

## Getting Started

### Prerequisites

- **Go 1.21+** installed
- **Docker** and **Docker Compose**
- **Git** for version control
- **Make** for build automation

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/YOUR_ORG/zerostate.git
cd zerostate

# Install dependencies
go work sync
go mod download

# Run tests to verify setup
make test

# Start development environment
docker-compose up -d
```

### Development Environment

```bash
# Start observability stack
cd deployments
docker-compose up -d

# Verify services
curl http://localhost:9090/-/healthy  # Prometheus
curl http://localhost:16686           # Jaeger
curl http://localhost:3100/ready      # Loki
curl http://localhost:3000/api/health # Grafana
```

---

## Development Workflow

### Branching Strategy

We use **Trunk-Based Development** with feature branches:

- **`main`** - Production-ready code, always deployable
- **`develop`** - Integration branch for next release
- **`feature/*`** - Feature branches (short-lived, < 2 days)
- **`fix/*`** - Bug fix branches
- **`docs/*`** - Documentation updates

### Branch Naming Convention

```bash
feature/agent-registration-api
feature/task-submission-endpoint
fix/payment-channel-race-condition
docs/api-documentation-update
```

### Workflow Steps

1. **Create an issue** (or find an existing one)
2. **Create a branch** from `develop`
   ```bash
   git checkout develop
   git pull origin develop
   git checkout -b feature/your-feature-name
   ```

3. **Make your changes**
   - Write code
   - Write tests
   - Update documentation

4. **Test locally**
   ```bash
   make test          # Unit tests
   make test-integration  # Integration tests
   make lint          # Linting
   ```

5. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add agent registration API

   - Implement POST /api/agents/register endpoint
   - Add WASM binary validation
   - Generate Agent Cards automatically
   - Publish to DHT and HNSW index

   Closes #123"
   ```

6. **Push and create PR**
   ```bash
   git push origin feature/your-feature-name
   ```

7. **Address review feedback**
8. **Merge** (after approval and CI passes)

---

## Coding Standards

### Go Code Style

Follow **official Go conventions**:

```go
// ✅ GOOD: Exported function with GoDoc
// RegisterAgent creates a new agent registration in the system.
// It validates the WASM binary, generates an Agent Card, and publishes
// to the DHT and HNSW index.
func RegisterAgent(ctx context.Context, req *RegisterRequest) (*Agent, error) {
    // Validate input
    if req.WASMBinary == nil {
        return nil, ErrMissingWASMBinary
    }

    // Business logic
    agent := &Agent{
        ID:          generateID(),
        Name:        req.Name,
        Capabilities: req.Capabilities,
    }

    return agent, nil
}

// ❌ BAD: No doc comment, poor naming
func regAgent(r *RegisterRequest) *Agent {
    // ...
}
```

### Naming Conventions

- **Packages**: lowercase, single word (avoid `_` or mixedCaps)
- **Files**: lowercase with `_` separators (`agent_card.go`)
- **Types**: PascalCase (`AgentCard`, `TaskManifest`)
- **Functions**: PascalCase for exported, camelCase for unexported
- **Variables**: camelCase
- **Constants**: PascalCase or ALL_CAPS for special cases

### Error Handling

```go
// ✅ GOOD: Wrap errors with context
func ProcessTask(ctx context.Context, taskID string) error {
    task, err := fetchTask(taskID)
    if err != nil {
        return fmt.Errorf("failed to fetch task %s: %w", taskID, err)
    }

    if err := validateTask(task); err != nil {
        return fmt.Errorf("task validation failed: %w", err)
    }

    return nil
}

// ❌ BAD: Lose error context
func ProcessTask(taskID string) error {
    task, _ := fetchTask(taskID)  // Ignoring error!
    validateTask(task)             // Not checking error!
    return nil
}
```

### Logging

Use **structured logging** with Zap:

```go
import "github.com/zerostate/libs/telemetry"

// ✅ GOOD: Structured logging
telemetry.InfoCtx(ctx, logger, "agent registered",
    zap.String("agent_id", agent.ID),
    zap.String("name", agent.Name),
    zap.Int("capabilities", len(agent.Capabilities)),
)

// ❌ BAD: Unstructured logging
fmt.Printf("Agent %s registered\n", agent.ID)
```

### Observability

**Every feature must include:**

1. **Metrics**
   ```go
   var agentRegistrations = prometheus.NewCounter(
       prometheus.CounterOpts{
           Namespace: "zerostate",
           Subsystem: "api",
           Name:      "agent_registrations_total",
           Help:      "Total number of agent registrations",
       },
   )
   ```

2. **Tracing**
   ```go
   ctx, span := tracer.Start(ctx, "RegisterAgent")
   defer span.End()

   span.SetAttributes(
       attribute.String("agent.name", req.Name),
   )
   ```

3. **Logging**
   ```go
   telemetry.InfoCtx(ctx, logger, "operation completed",
       zap.Duration("duration", time.Since(start)),
   )
   ```

---

## Testing Requirements

### Test Coverage

- **Minimum**: 80% overall coverage
- **Critical paths**: 90%+ coverage
- **New code**: Must not decrease coverage

### Test Types

#### 1. Unit Tests

```go
func TestRegisterAgent(t *testing.T) {
    tests := []struct {
        name    string
        req     *RegisterRequest
        wantErr bool
    }{
        {
            name: "valid registration",
            req: &RegisterRequest{
                Name: "test-agent",
                WASMBinary: []byte("valid wasm"),
            },
            wantErr: false,
        },
        {
            name: "missing WASM binary",
            req: &RegisterRequest{
                Name: "test-agent",
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            agent, err := RegisterAgent(context.Background(), tt.req)
            if (err != nil) != tt.wantErr {
                t.Errorf("RegisterAgent() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && agent == nil {
                t.Error("expected agent, got nil")
            }
        })
    }
}
```

#### 2. Integration Tests

Place in `tests/integration/`:

```go
func TestAgentRegistrationE2E(t *testing.T) {
    // Setup
    server := setupTestServer(t)
    defer server.Close()

    // Execute
    resp, err := http.Post(
        server.URL+"/api/agents/register",
        "application/json",
        body,
    )

    // Assert
    require.NoError(t, err)
    assert.Equal(t, http.StatusCreated, resp.StatusCode)
}
```

#### 3. Benchmark Tests

```go
func BenchmarkRegisterAgent(b *testing.B) {
    req := &RegisterRequest{ /* ... */ }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = RegisterAgent(context.Background(), req)
    }
}
```

### Running Tests

```bash
# Unit tests
make test-unit

# Integration tests
make test-integration

# All tests with coverage
make test-coverage

# Benchmarks
make bench

# Race detection
make test-race
```

---

## Pull Request Process

### Before Creating PR

- [ ] All tests pass locally
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated
- [ ] CHANGELOG.md updated (if user-facing change)

### PR Title Format

Use **Conventional Commits**:

```
feat: add agent registration API
fix: resolve payment channel race condition
docs: update API documentation
refactor: simplify guild formation logic
test: add integration tests for task execution
chore: update dependencies
```

### PR Description Template

See `.github/pull_request_template.md` - it's auto-populated!

### Code Review Process

1. **Self-review** your own PR first
2. Request review from **2 reviewers** minimum
3. Address feedback within **24 hours**
4. **All CI checks** must pass
5. **At least 2 approvals** required to merge

### Review Checklist

**Reviewers should verify:**

- [ ] Code follows style guidelines
- [ ] Tests are comprehensive
- [ ] Documentation is updated
- [ ] No security vulnerabilities
- [ ] Performance is acceptable
- [ ] Error handling is robust
- [ ] Observability is included

---

## Issue Reporting

### Bug Reports

Use the bug report template (`.github/ISSUE_TEMPLATE/bug_report.md`):

**Good bug report:**
```markdown
### Description
Payment channel fails to close when...

### Reproduction
1. Create payment channel
2. Send 10 payments
3. Attempt to close
4. Observe error: "channel state mismatch"

### Environment
- Version: v0.1.0 (commit abc123)
- OS: Ubuntu 22.04
- Go: 1.21.5

### Logs
```
ERROR: channel state mismatch: expected=5, actual=10
```

### Feature Requests

Use the feature request template (`.github/ISSUE_TEMPLATE/feature_request.md`):

**Good feature request:**
```markdown
### Problem
Agent providers can't update their agents after registration

### Proposed Solution
Add PATCH /api/agents/:id endpoint for updates

### Acceptance Criteria
- [ ] Can update agent metadata
- [ ] Can upload new WASM binary (versioned)
- [ ] Old tasks use old version
- [ ] New tasks use new version
```

---

## Communication

### Channels

- **GitHub Issues**: Bug reports, feature requests
- **GitHub Discussions**: Questions, brainstorming
- **Pull Requests**: Code reviews
- **Discord/Slack**: Real-time chat (if available)

### Response Times

- **Critical bugs (P0)**: < 4 hours
- **High priority (P1)**: < 24 hours
- **Medium priority (P2)**: < 3 days
- **Low priority (P3)**: < 1 week

---

## Git Commit Guidelines

### Commit Message Format

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation only
- **style**: Code style (formatting, missing semicolons, etc.)
- **refactor**: Code change that neither fixes a bug nor adds a feature
- **perf**: Performance improvement
- **test**: Adding or updating tests
- **chore**: Maintenance (dependencies, build, etc.)

### Examples

```bash
feat(api): add agent registration endpoint

Implements POST /api/agents/register with:
- WASM binary validation
- Agent Card generation
- DHT publication
- HNSW indexing

Closes #123

---

fix(payment): resolve channel race condition

Fixed race condition in payment channel state updates
by adding mutex protection around balance modifications.

Fixes #456

---

docs(readme): update installation instructions

Added Docker Compose setup instructions for local development.
```

---

## Development Tips

### Local Testing

```bash
# Run specific test
go test -v ./libs/api -run TestRegisterAgent

# Run with coverage
go test -v -cover ./...

# Run with race detection
go test -v -race ./...

# Watch mode (requires entr)
find . -name '*.go' | entr -c go test ./...
```

### Debugging

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Enable tracing
export OTEL_EXPORTER_JAEGER_ENDPOINT=http://localhost:14268/api/traces

# View traces
open http://localhost:16686
```

### Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof
```

---

## Questions?

- **General questions**: Open a GitHub Discussion
- **Bug reports**: Open a GitHub Issue
- **Security issues**: Email security@zerostate.io (do NOT open public issue)

---

**Thank you for contributing to ZeroState!** 🚀
