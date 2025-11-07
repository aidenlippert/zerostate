# ZeroState Observability Stack - Test & Validation Guide

**Last Updated:** November 7, 2025
**Sprint 6 - Phase 6 Complete** ✅

---

## Overview

Comprehensive testing and validation guide for the ZeroState observability stack, including integration tests, chaos engineering validation, and production readiness verification.

### Testing Scope

- **Integration Tests**: End-to-end observability pipeline validation
- **Chaos Tests**: Resilience and recovery under failure conditions
- **Performance Tests**: Metrics, traces, and logs under load
- **Health Check Tests**: Kubernetes probe validation

---

## Quick Start

### Prerequisites

```bash
# 1. Start observability stack
cd deployments
docker-compose up -d

# 2. Wait for services to be ready (30-60 seconds)
docker-compose ps

# 3. Verify all services are healthy
curl http://localhost:9090/-/healthy  # Prometheus
curl http://localhost:3100/ready      # Loki
curl http://localhost:16686           # Jaeger
curl http://localhost:3000/api/health # Grafana
```

### Run All Tests

```bash
# Integration tests
go test -v ./tests/integration -run TestObservabilityStack

# Chaos tests (requires Docker permissions)
go test -v ./tests/chaos -run TestChaos

# Quick validation (skip chaos tests)
go test -v -short ./tests/integration ./tests/chaos
```

---

## Integration Tests

**File:** `tests/integration/observability_stack_test.go`

### Test Coverage

| Test | Purpose | Duration | Prerequisites |
|------|---------|----------|---------------|
| **MetricsFlow** | Prometheus metrics pipeline | 5s | None |
| **TracingFlow** | Jaeger trace export | 10s | Jaeger running |
| **LoggingFlow** | Structured logging format | 5s | None |
| **HealthChecks** | Health endpoints | 5s | None |
| **Integration** | All components together | 15s | Jaeger optional |
| **TraceLogCorrelation** | Trace-log linking | 10s | Jaeger running |
| **MetricsHealthCorrelation** | Metrics-health sync | 5s | None |
| **EndToEnd** | Realistic application flow | 15s | None |

### Running Integration Tests

#### Run All Integration Tests

```bash
go test -v ./tests/integration -run TestObservabilityStack
```

**Expected Output:**
```
=== RUN   TestObservabilityStack_MetricsFlow
    observability_stack_test.go:30: Testing Metrics → Prometheus → Grafana flow
    observability_stack_test.go:55: ✅ Metric found with correct value: 3.000000
    observability_stack_test.go:67: ✅ Metrics endpoint returns correct Prometheus format
--- PASS: TestObservabilityStack_MetricsFlow (0.15s)

=== RUN   TestObservabilityStack_TracingFlow
    observability_stack_test.go:75: Testing Traces → Jaeger flow
    observability_stack_test.go:112: ✅ Traces exported to Jaeger
    observability_stack_test.go:113:    View in Jaeger UI: http://localhost:16686
--- PASS: TestObservabilityStack_TracingFlow (2.20s)

...
```

#### Run Specific Test

```bash
# Test metrics only
go test -v ./tests/integration -run TestObservabilityStack_MetricsFlow

# Test tracing only
go test -v ./tests/integration -run TestObservabilityStack_TracingFlow

# Test health checks only
go test -v ./tests/integration -run TestObservabilityStack_HealthChecks
```

#### Skip Tests Requiring External Services

```bash
# Skip tests that need Jaeger
go test -v -short ./tests/integration
```

---

### Test Details

#### 1. MetricsFlow Test

**Validates:**
- Prometheus metric registration
- Metric exposition format
- Counter increments
- Metrics endpoint accessibility

**What It Does:**
1. Creates test counter metric
2. Increments counter 3 times
3. Collects metrics from registry
4. Verifies Prometheus text format
5. Checks metric values

**Success Criteria:**
- ✅ Metric registered successfully
- ✅ Counter value equals 3
- ✅ Prometheus format is correct

---

#### 2. TracingFlow Test

**Validates:**
- Jaeger exporter configuration
- Trace creation and export
- Parent-child span relationships
- Trace attribute setting

**What It Does:**
1. Creates Jaeger exporter
2. Initializes tracer provider
3. Creates parent span with attributes
4. Creates child span
5. Exports to Jaeger

**Success Criteria:**
- ✅ Traces exported without errors
- ✅ Spans visible in Jaeger UI
- ✅ Parent-child relationship preserved

**Manual Verification:**
```bash
# View traces in Jaeger
open http://localhost:16686

# Search for:
# - Service: integration-test
# - Operation: test-operation
```

---

#### 3. LoggingFlow Test

**Validates:**
- Structured JSON logging
- Trace context correlation
- Log levels (info/warn/error)
- Persistent field support

**What It Does:**
1. Creates Zap logger
2. Logs at different levels
3. Adds trace context
4. Uses structured logger with persistent fields

**Success Criteria:**
- ✅ Logs in JSON format
- ✅ Trace IDs included
- ✅ Structured fields preserved

**Manual Verification:**
```bash
# View logs in Grafana
open http://localhost:3000/explore

# Select Loki datasource
# Query: {job="docker"} |= "integration test"
```

---

#### 4. HealthChecks Test

**Validates:**
- `/healthz` (liveness) endpoint
- `/readyz` (readiness) endpoint
- `/health` (detailed) endpoint
- Healthy/degraded/unhealthy states

**What It Does:**
1. Creates health checker with components
2. Registers mock checkers
3. Tests liveness endpoint (200 if alive)
4. Tests readiness endpoint (503 if degraded)
5. Tests detailed endpoint (full status)

**Success Criteria:**
- ✅ Liveness returns 200 for healthy/degraded
- ✅ Liveness returns 503 for unhealthy
- ✅ Readiness returns 200 only if ready
- ✅ Detailed returns all components

---

#### 5. Integration Test

**Validates:**
- All observability components working together
- No resource conflicts
- Concurrent operation support

**What It Does:**
1. Sets up metrics, tracing, logging, health
2. Performs integrated operation
3. Logs with trace context
4. Increments metrics
5. Checks health

**Success Criteria:**
- ✅ All components initialized
- ✅ No conflicts or errors
- ✅ Data flows to all systems

---

#### 6. TraceLogCorrelation Test

**Validates:**
- Trace IDs in log entries
- Span IDs in log entries
- Grafana Loki → Jaeger linking

**What It Does:**
1. Creates span with trace context
2. Logs with trace context
3. Verifies trace/span IDs present

**Success Criteria:**
- ✅ Logs contain trace_id
- ✅ Logs contain span_id
- ✅ Can search logs by trace ID

**Manual Verification:**
```bash
# In Grafana Explore (Loki):
{job="docker"} | json | trace_id="<TRACE_ID_FROM_TEST>"

# Click "View Trace" link → opens in Jaeger
```

---

#### 7. EndToEnd Test

**Validates:**
- Realistic application workflow
- Multiple operations
- Metrics + logs + health combined

**What It Does:**
1. Simulates 3 operations (create_guild, execute_task, process_payment)
2. Records metrics (counter + histogram)
3. Logs each operation
4. Checks health after operations

**Success Criteria:**
- ✅ All operations complete
- ✅ Metrics recorded
- ✅ Logs written
- ✅ Health checks pass

---

## Chaos Engineering Tests

**File:** `tests/chaos/chaos_test.go`

### Test Coverage

| Test | Failure Mode | Recovery Validation |
|------|--------------|---------------------|
| **ServiceKill** | Container kill | Auto-restart, health |
| **PrometheusRecovery** | Prometheus kill | Metric continuity |
| **JaegerRecovery** | Jaeger kill | Trace ingestion |
| **LokiRecovery** | Loki kill | Log ingestion |
| **GrafanaRecovery** | Grafana kill | Dashboard availability |
| **CascadingFailure** | All services kill | System-wide recovery |
| **DataPersistence** | Restart with data | Volume persistence |

### Running Chaos Tests

⚠️ **Warning:** Chaos tests will kill and restart Docker containers. Ensure you have:
- Docker permissions
- No production workloads on the same Docker host
- Observability stack running via docker-compose

```bash
# Run all chaos tests
go test -v ./tests/chaos -run TestChaos

# Run specific test
go test -v ./tests/chaos -run TestChaos_ServiceKill

# Skip chaos tests
go test -v -short ./tests/chaos
```

**Expected Output:**
```
=== RUN   TestChaos_ServiceKill
    chaos_test.go:32: 🔥 Chaos Test: Service Kill & Recovery
=== RUN   TestChaos_ServiceKill/zs-prometheus
    chaos_test.go:41: Testing kill & recovery: zs-prometheus
    chaos_test.go:47:   💀 Killing service: zs-prometheus
    chaos_test.go:54:   🔄 Restarting service: zs-prometheus
    chaos_test.go:62:   ✅ Service recovered: zs-prometheus
--- PASS: TestChaos_ServiceKill/zs-prometheus (15.23s)
...
```

---

### Test Details

#### 1. ServiceKill Test

**Validates:**
- Container restart capability
- Service recovery time
- Health restoration

**For Each Service:**
1. Verify service running
2. Kill container (`docker kill`)
3. Verify service down
4. Restart container (`docker start`)
5. Wait for recovery (30s timeout)
6. Verify service healthy

**Success Criteria:**
- ✅ Container restarts successfully
- ✅ Recovery within 30 seconds
- ✅ Service returns to healthy state

---

#### 2. PrometheusRecovery Test

**Validates:**
- Prometheus restart
- Metric continuity
- TSDB integrity

**Steps:**
1. Query metrics before kill
2. Kill Prometheus
3. Verify unreachable
4. Restart Prometheus
5. Wait for recovery
6. Query metrics after recovery
7. Verify data continuity

**Success Criteria:**
- ✅ Prometheus restarts
- ✅ Metrics still available
- ✅ Historical data preserved

---

#### 3. JaegerRecovery Test

**Validates:**
- Jaeger restart
- Trace ingestion resumption
- UI availability

**Steps:**
1. Check Jaeger health
2. Kill Jaeger
3. Verify unavailable
4. Restart Jaeger
5. Wait for recovery
6. Verify health and UI

**Success Criteria:**
- ✅ Jaeger restarts
- ✅ UI accessible
- ✅ Accepting new traces

---

#### 4. LokiRecovery Test

**Validates:**
- Loki restart
- Log ingestion resumption
- Query API availability

**Steps:**
1. Check Loki health
2. Kill Loki
3. Verify unavailable
4. Restart Loki
5. Wait for recovery
6. Verify health and ready state

**Success Criteria:**
- ✅ Loki restarts
- ✅ API accessible
- ✅ Accepting new logs

---

#### 5. GrafanaRecovery Test

**Validates:**
- Grafana restart
- Dashboard persistence
- Datasource connectivity

**Steps:**
1. Check Grafana health
2. Kill Grafana
3. Verify unavailable
4. Restart Grafana
5. Wait for recovery
6. Verify dashboards available

**Success Criteria:**
- ✅ Grafana restarts
- ✅ Dashboards accessible
- ✅ Datasources connected

---

#### 6. CascadingFailure Test

**Validates:**
- System-wide recovery
- Service interdependencies
- Bulk restart handling

**Steps:**
1. Verify all services running
2. Kill all services simultaneously
3. Verify all down
4. Restart all services
5. Wait for recovery (45s timeout)
6. Verify all healthy

**Success Criteria:**
- ✅ All services restart
- ✅ Recovery within 45 seconds
- ✅ All health checks pass

---

#### 7. DataPersistence Test

**Validates:**
- Volume-mounted data persistence
- TSDB continuity (Prometheus)
- Index continuity (Loki)

**Steps:**
1. Query data before restart
2. Kill and restart service
3. Query data after restart
4. Verify data preserved

**Success Criteria:**
- ✅ Historical data preserved
- ✅ No data loss on restart
- ✅ Queries return same results

---

## Manual Validation

### Metrics Validation

**1. Verify Prometheus Scraping**

```bash
# Check Prometheus targets
open http://localhost:9090/targets

# Should show:
# - prometheus (self-scraping)
# - Any ZeroState services with /metrics
```

**2. Query Test Metrics**

```bash
# In Prometheus UI (http://localhost:9090)
# Query: up
# Should show target health status

# Query: prometheus_http_requests_total
# Should show Prometheus request metrics
```

**3. View Grafana Dashboards**

```bash
open http://localhost:3000

# Navigate to:
# - P2P Network Metrics
# - Task Execution Metrics
# - Economic Layer Metrics
# - System Overview

# Verify panels show data
```

---

### Tracing Validation

**1. Verify Jaeger UI**

```bash
open http://localhost:16686

# Search for services:
# - integration-test (if tests ran)
# - Any ZeroState services
```

**2. Create Test Trace**

Run integration test:
```bash
go test -v ./tests/integration -run TestObservabilityStack_TracingFlow
```

Then search in Jaeger:
- Service: `integration-test`
- Operation: `test-operation`
- Verify parent-child span relationship

**3. Verify Trace Context Propagation**

Check trace headers in HTTP requests:
```bash
# Example trace header
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
```

---

### Logging Validation

**1. Verify Loki Ingestion**

```bash
# Check Loki ready state
curl http://localhost:3100/ready

# Should return: ready
```

**2. Query Logs in Grafana**

```bash
open http://localhost:3000/explore

# Select Loki datasource
# Example queries:

# All logs
{job="docker"}

# Error logs
{job="docker", level="error"}

# Logs by service
{job="docker", service="p2p"}

# Logs by trace ID
{job="docker"} | json | trace_id="<TRACE_ID>"
```

**3. Verify Log Format**

Logs should be JSON with structure:
```json
{
  "level": "info",
  "timestamp": "2025-11-07T10:30:15.123Z",
  "message": "operation completed",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "service": "p2p",
  "peer_id": "QmXxxx",
  "operation": "connect"
}
```

---

### Health Check Validation

**1. Test Health Endpoints**

```bash
# Liveness (should always return 200 unless unhealthy)
curl http://localhost:8080/healthz

# Readiness (returns 503 if not ready)
curl http://localhost:8080/readyz

# Detailed health
curl http://localhost:8080/health | jq
```

**2. Verify Kubernetes Probes**

If deployed to Kubernetes:
```bash
# Check probe status
kubectl describe pod <pod-name>

# Look for:
# - Liveness: healthy
# - Readiness: ready
# - Startup: completed

# View probe failures
kubectl get events --field-selector involvedObject.name=<pod-name>
```

**3. Simulate Health Degradation**

```go
// In your service code, temporarily set component to degraded:
checker := health.NewP2PChecker(
    func() int { return 1 },  // Below minimum peers
    3,
    func() float64 { return 0.95 },
)

// Liveness should still be OK
// Readiness should be 503 (not ready)
```

---

## Performance Testing

### Load Testing Metrics

**1. Generate Metrics Load**

```go
// Create high-frequency metric updates
for i := 0; i < 10000; i++ {
    requestCounter.Inc()
    requestDuration.Observe(rand.Float64())
}
```

**Verify:**
- Prometheus scraping keeps up
- No metric drops
- Grafana dashboards update smoothly

---

### Load Testing Traces

**1. Generate Trace Load**

```go
// Create many spans
for i := 0; i < 1000; i++ {
    ctx, span := tracer.Start(context.Background(), "load-test")
    time.Sleep(10 * time.Millisecond)
    span.End()
}
```

**Verify:**
- Jaeger ingests all traces
- No dropped spans
- Query performance acceptable

---

### Load Testing Logs

**1. Generate Log Load**

```go
// Create high-volume logs
for i := 0; i < 10000; i++ {
    logger.Info("load test",
        zap.Int("iteration", i),
        zap.String("data", generateRandomString(100)),
    )
}
```

**Verify:**
- Promtail processes all logs
- Loki ingests without errors
- Grafana queries remain fast

---

## Troubleshooting

### Integration Tests Failing

**Issue:** Tests fail with "service not available"

**Solutions:**
```bash
# 1. Verify Docker Compose services
docker-compose ps

# 2. Check service health
curl http://localhost:9090/-/healthy  # Prometheus
curl http://localhost:16686           # Jaeger
curl http://localhost:3100/ready      # Loki

# 3. Restart services
docker-compose restart

# 4. Check logs
docker-compose logs prometheus
docker-compose logs jaeger
docker-compose logs loki
```

---

**Issue:** Trace tests skip with "Jaeger not available"

**Solutions:**
```bash
# 1. Verify Jaeger is running
docker ps | grep jaeger

# 2. Check Jaeger collector port
curl http://localhost:14268

# 3. Check Jaeger logs
docker logs zs-jaeger

# 4. Restart Jaeger
docker-compose restart jaeger
```

---

### Chaos Tests Failing

**Issue:** Permission denied when killing containers

**Solutions:**
```bash
# 1. Add user to docker group
sudo usermod -aG docker $USER
newgrp docker

# 2. Or run with sudo
sudo go test -v ./tests/chaos

# 3. Verify Docker socket permissions
ls -l /var/run/docker.sock
```

---

**Issue:** Services don't recover

**Solutions:**
```bash
# 1. Check if containers exist
docker ps -a | grep zs-

# 2. Manually start service
docker start zs-prometheus

# 3. Check for errors
docker logs zs-prometheus

# 4. Recreate service
docker-compose up -d prometheus
```

---

### Manual Validation Issues

**Issue:** Grafana dashboards show no data

**Solutions:**
```bash
# 1. Verify Prometheus datasource
curl http://localhost:3000/api/datasources

# 2. Check Prometheus has data
curl 'http://localhost:9090/api/v1/query?query=up'

# 3. Verify time range in Grafana (default: last 6h)

# 4. Check Grafana logs
docker logs zs-grafana
```

---

**Issue:** Logs not appearing in Loki

**Solutions:**
```bash
# 1. Verify Promtail is scraping
docker logs zs-promtail

# 2. Check Promtail targets
curl http://localhost:9080/targets

# 3. Verify Loki is receiving
curl 'http://localhost:3100/loki/api/v1/query?query={job="docker"}'

# 4. Check container labels match Promtail config
docker inspect zs-prometheus | grep -A5 Labels
```

---

## Production Readiness Checklist

### Pre-Production Validation

- [ ] All integration tests pass
- [ ] All chaos tests pass (if applicable)
- [ ] Manual validation completed
- [ ] Load testing performed
- [ ] Data persistence verified
- [ ] Health checks functional
- [ ] Alerting rules tested
- [ ] Dashboard access verified
- [ ] Trace sampling configured
- [ ] Log retention set
- [ ] Metrics retention configured
- [ ] Backup procedures documented

### Production Deployment

- [ ] Kubernetes health probes configured
- [ ] Resource limits set
- [ ] High availability enabled (replicas ≥3)
- [ ] Pod disruption budgets defined
- [ ] Persistent volumes configured
- [ ] Monitoring alerts active
- [ ] Runbooks created
- [ ] On-call rotation established
- [ ] Incident response plan documented

---

## Continuous Validation

### Automated Testing

**CI/CD Integration:**
```yaml
# .github/workflows/observability-tests.yml
name: Observability Tests

on: [push, pull_request]

jobs:
  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - name: Start observability stack
        run: docker-compose -f deployments/docker-compose.yml up -d
      - name: Wait for services
        run: sleep 30
      - name: Run integration tests
        run: go test -v ./tests/integration
```

### Scheduled Chaos Testing

**Weekly Chaos:**
```bash
# Cron job for weekly chaos testing
0 2 * * 0 /path/to/run-chaos-tests.sh
```

**run-chaos-tests.sh:**
```bash
#!/bin/bash
cd /path/to/zerostate
docker-compose up -d
sleep 60
go test -v ./tests/chaos > chaos-results-$(date +%Y%m%d).log
```

---

## Support & Resources

**Documentation:**
- [Prometheus Testing](https://prometheus.io/docs/prometheus/latest/getting_started/)
- [Jaeger Deployment](https://www.jaegertracing.io/docs/latest/deployment/)
- [Loki Query Language](https://grafana.com/docs/loki/latest/logql/)

**Files Created:**
- `tests/integration/observability_stack_test.go` - Integration tests
- `tests/chaos/chaos_test.go` - Chaos engineering tests
- `docs/OBSERVABILITY_TEST_GUIDE.md` - This guide

---

**Generated:** November 7, 2025
**Sprint 6 - Phase 6 Complete** ✅
**Observability Stack: Production-Ready & Validated**
