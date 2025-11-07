# Description

## What does this PR do?

<!-- Clear description of changes -->

## Related Issues

Closes #
Relates to #

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Refactoring (code change that neither fixes a bug nor adds a feature)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Test coverage improvement
- [ ] Infrastructure/DevOps change

---

# Changes Made

## Components Modified

- [ ] P2P Network (`libs/p2p/`)
- [ ] Task Execution (`libs/execution/`)
- [ ] Payment System (`libs/economic/`)
- [ ] Reputation System (`libs/reputation/`)
- [ ] Observability (`libs/telemetry/`, `libs/health/`, `libs/metrics/`)
- [ ] API Server (`api/`)
- [ ] Web UI (`web/`)
- [ ] CLI (`cmd/`)
- [ ] Infrastructure (`deployments/`, `.github/`)
- [ ] Documentation (`docs/`)
- [ ] Tests (`tests/`)

## Key Files Changed

<!-- List main files with brief description -->
- `path/to/file.go` - Description
- `path/to/file.go` - Description

---

# Testing

## Test Coverage

- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] E2E tests added/updated
- [ ] Manual testing completed

## Test Results

```bash
# Paste test output
go test -v ./...
```

**Coverage:**
<!-- Before: X% → After: Y% -->

## Performance Impact

- [ ] No performance impact
- [ ] Performance improved (provide benchmarks)
- [ ] Performance degraded (justify and document)

**Benchmarks (if applicable):**
```bash
# Paste benchmark results
```

---

# Checklist

## Code Quality

- [ ] Code follows project style guidelines (ran `make lint`)
- [ ] Self-review completed
- [ ] Comments added for complex logic
- [ ] No unnecessary debug/print statements
- [ ] Error handling is comprehensive
- [ ] Logging uses structured format (Zap)

## Documentation

- [ ] Code comments updated (GoDoc)
- [ ] API documentation updated (if API changed)
- [ ] User-facing documentation updated (`docs/`)
- [ ] CHANGELOG.md updated
- [ ] Migration guide added (if breaking change)

## Security

- [ ] No secrets/credentials in code
- [ ] Input validation added
- [ ] No SQL injection vulnerabilities
- [ ] No XSS vulnerabilities
- [ ] Dependencies scanned (no critical CVEs)
- [ ] Authentication/authorization preserved

## Observability

- [ ] Metrics added for new features
- [ ] Tracing added for critical paths
- [ ] Structured logging added
- [ ] Health checks updated (if applicable)
- [ ] Alerts/runbooks updated (if needed)

## Backward Compatibility

- [ ] API backward compatible (or version bumped)
- [ ] Database migrations included (if schema changed)
- [ ] Configuration backward compatible
- [ ] Deployment notes added (if needed)

---

# Deployment Notes

## Database Migrations

<!-- Describe any schema changes -->

## Configuration Changes

<!-- New env vars, config files, etc. -->

## Rollback Plan

<!-- How to revert if this breaks production -->

## Monitoring

<!-- What metrics/logs to watch after deploy -->

---

# Screenshots

<!-- UI changes, dashboards, etc. -->

---

# Reviewer Notes

## Areas Needing Extra Attention

<!-- Call out tricky parts -->

## Questions for Reviewers

<!-- Anything you're unsure about -->

---

# Post-Merge Tasks

- [ ] Update tracking issue
- [ ] Deploy to staging
- [ ] Smoke test on staging
- [ ] Update documentation site
- [ ] Announce in team channel
