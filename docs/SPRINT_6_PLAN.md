# Sprint 6: Monitoring & Observability

**Status**: 🚧 In Progress  
**Started**: 2025-11-06  
**Goal**: Add production-grade monitoring, metrics, and observability

## Objectives

1. **Prometheus Metrics** - Export metrics from all components
2. **Grafana Dashboards** - Visual monitoring and alerting
3. **OpenTelemetry Tracing** - Distributed request tracing
4. **Structured Logging** - Enhanced log aggregation
5. **Health Checks** - Readiness and liveness probes
6. **Alert Rules** - Automated incident detection

## Phase 1: Prometheus Metrics (Tasks 1-4)

### Task 1: Core Metrics Infrastructure
- [ ] Create `libs/metrics` package with Prometheus client
- [ ] Define standard metric types (counters, gauges, histograms)
- [ ] Implement metrics registry and HTTP handler
- [ ] Add metrics middleware for HTTP servers

**Files to create**:
- `libs/metrics/registry.go` - Central metrics registry
- `libs/metrics/http.go` - HTTP metrics handler
- `libs/metrics/middleware.go` - HTTP/gRPC middleware
- `libs/metrics/metrics_test.go` - Unit tests

### Task 2: P2P Network Metrics
- [ ] Connection pool metrics (active, idle, total)
- [ ] Bandwidth metrics (bytes sent/received)
- [ ] Message metrics (sent, received, failed)
- [ ] Peer metrics (connected, discovered, failed)
- [ ] Latency histograms for operations

**Metrics to add**:
```
zerostate_p2p_connections{state="active|idle"} gauge
zerostate_p2p_bandwidth_bytes{direction="tx|rx"} counter
zerostate_p2p_messages{type="gossip|request|response",status="success|error"} counter
zerostate_p2p_peers{state="connected|discovered|failed"} gauge
zerostate_p2p_operation_duration_seconds{operation="connect|send|receive"} histogram
```

### Task 3: Execution Metrics
- [ ] Guild metrics (created, active, dissolved)
- [ ] Task metrics (submitted, executing, completed, failed)
- [ ] WASM execution metrics (duration, memory, exit codes)
- [ ] Receipt metrics (generated, signed, verified)
- [ ] Cost metrics (estimated, actual, capped)

**Metrics to add**:
```
zerostate_guild_total{state="active|dissolved"} gauge
zerostate_tasks{status="submitted|executing|completed|failed"} counter
zerostate_wasm_execution_duration_seconds histogram
zerostate_wasm_memory_bytes histogram
zerostate_receipts{status="generated|signed|verified"} counter
zerostate_task_cost_units{type="estimated|actual|capped"} histogram
```

### Task 4: Economic Metrics
- [ ] Payment channel metrics (opened, active, closed, disputed)
- [ ] Payment metrics (amount, sequence, failed)
- [ ] Balance metrics (deposits, current balance)
- [ ] Reputation metrics (scores, blacklisted, tasks)
- [ ] Settlement metrics (duration, amount)

**Metrics to add**:
```
zerostate_channels{state="opened|active|closed|disputed"} gauge
zerostate_payments{status="success|failed"} counter
zerostate_payment_amount_units histogram
zerostate_channel_balance_units{party="creator|executor"} gauge
zerostate_reputation_score gauge
zerostate_reputation_tasks{result="success|failed"} counter
zerostate_blacklisted_executors gauge
```

## Phase 2: Grafana Dashboards (Tasks 5-7)

### Task 5: Dashboard Definitions
- [ ] Create `monitoring/grafana/` directory structure
- [ ] Network overview dashboard
- [ ] Execution performance dashboard
- [ ] Economic activity dashboard
- [ ] System health dashboard

**Files to create**:
- `monitoring/grafana/dashboards/network-overview.json`
- `monitoring/grafana/dashboards/execution-performance.json`
- `monitoring/grafana/dashboards/economic-activity.json`
- `monitoring/grafana/dashboards/system-health.json`
- `monitoring/grafana/datasources/prometheus.yaml`

### Task 6: Alert Rules
- [ ] High error rate alerts
- [ ] Resource exhaustion alerts
- [ ] Reputation anomaly alerts
- [ ] Payment failure alerts
- [ ] Network partition alerts

**Files to create**:
- `monitoring/prometheus/alerts/network.yaml`
- `monitoring/prometheus/alerts/execution.yaml`
- `monitoring/prometheus/alerts/economic.yaml`
- `monitoring/prometheus/alerts/system.yaml`

### Task 7: Dashboard Provisioning
- [ ] Create Grafana provisioning configs
- [ ] Define dashboard layouts and panels
- [ ] Add variable templates for filtering
- [ ] Configure alerting integrations

## Phase 3: Distributed Tracing (Tasks 8-10)

### Task 8: OpenTelemetry Setup
- [ ] Add OpenTelemetry SDK dependencies
- [ ] Create trace context propagation
- [ ] Implement span creation helpers
- [ ] Add trace sampling configuration

**Files to create**:
- `libs/telemetry/tracer.go` - Tracer initialization
- `libs/telemetry/context.go` - Context propagation
- `libs/telemetry/middleware.go` - Tracing middleware
- `libs/telemetry/config.go` - Configuration

### Task 9: Trace Instrumentation
- [ ] Instrument guild operations
- [ ] Instrument WASM execution
- [ ] Instrument payment flows
- [ ] Instrument reputation updates
- [ ] Add custom span attributes

**Spans to create**:
- `guild.create`, `guild.join`, `guild.dissolve`
- `wasm.execute`, `wasm.validate`, `wasm.meter`
- `payment.channel.open`, `payment.send`, `payment.settle`
- `reputation.record`, `reputation.calculate`, `reputation.blacklist`

### Task 10: Jaeger Integration
- [ ] Add Jaeger exporter configuration
- [ ] Create docker-compose with Jaeger
- [ ] Add trace visualization examples
- [ ] Document trace analysis workflow

## Phase 4: Enhanced Logging (Tasks 11-12)

### Task 11: Structured Logging Enhancement
- [ ] Add contextual fields to all logs
- [ ] Implement log sampling for high-volume events
- [ ] Add correlation IDs across components
- [ ] Create log level dynamic configuration

**Enhancements**:
- Request ID tracking
- User/agent identification
- Error stack traces
- Performance markers
- Security event logging

### Task 12: Log Aggregation
- [ ] Add Loki configuration
- [ ] Create log shipping configuration
- [ ] Define log retention policies
- [ ] Add log-based metrics

**Files to create**:
- `monitoring/loki/config.yaml`
- `monitoring/promtail/config.yaml`
- `monitoring/docker-compose-logging.yaml`

## Phase 5: Health Checks (Tasks 13-14)

### Task 13: Health Check Endpoints
- [ ] Create `/health/live` liveness endpoint
- [ ] Create `/health/ready` readiness endpoint
- [ ] Add component-level health checks
- [ ] Implement health check aggregation

**Files to create**:
- `libs/health/checker.go` - Health check framework
- `libs/health/endpoints.go` - HTTP endpoints
- `libs/health/checks.go` - Individual health checks
- `libs/health/health_test.go` - Tests

**Health checks to implement**:
- P2P connectivity
- Storage availability
- Payment channel state
- Guild manager status
- WASM runtime readiness

### Task 14: Kubernetes Probes
- [ ] Configure liveness probes in K8s manifests
- [ ] Configure readiness probes
- [ ] Add startup probes for slow initialization
- [ ] Test probe failure scenarios

## Phase 6: Integration & Testing (Tasks 15-16)

### Task 15: Monitoring Stack Deployment
- [ ] Create `docker-compose-monitoring.yaml`
- [ ] Configure Prometheus scraping
- [ ] Set up Grafana with datasources
- [ ] Deploy Jaeger for tracing
- [ ] Configure alert manager

**Services to deploy**:
- Prometheus (metrics storage)
- Grafana (visualization)
- Jaeger (distributed tracing)
- AlertManager (alert routing)
- Loki (log aggregation)
- Promtail (log shipping)

### Task 16: Monitoring Tests
- [ ] Test metric collection
- [ ] Validate dashboard queries
- [ ] Test alert rule firing
- [ ] Verify trace propagation
- [ ] Test health check failures

**Files to create**:
- `tests/monitoring/metrics_test.go`
- `tests/monitoring/traces_test.go`
- `tests/monitoring/health_test.go`

## Success Criteria

- [ ] All metrics exported via `/metrics` endpoint
- [ ] 4+ Grafana dashboards operational
- [ ] Distributed traces visible in Jaeger
- [ ] Health checks respond correctly
- [ ] Alert rules fire on test conditions
- [ ] Documentation complete
- [ ] Integration tests passing

## Performance Targets

- **Metric collection overhead**: <1% CPU, <10MB memory
- **Trace sampling**: 10% of requests (configurable)
- **Health check response**: <100ms
- **Metrics scrape interval**: 15s
- **Log shipping latency**: <5s

## Deliverables

1. ✅ Metrics library with Prometheus integration
2. ✅ All components instrumented with metrics
3. ✅ Grafana dashboards for key metrics
4. ✅ Alert rules for critical conditions
5. ✅ Distributed tracing infrastructure
6. ✅ Health check endpoints
7. ✅ Monitoring stack docker-compose
8. ✅ Documentation and runbooks

## Timeline

- **Phase 1**: Days 1-2 (Metrics infrastructure)
- **Phase 2**: Days 3-4 (Dashboards & alerts)
- **Phase 3**: Days 5-6 (Distributed tracing)
- **Phase 4**: Day 7 (Enhanced logging)
- **Phase 5**: Day 8 (Health checks)
- **Phase 6**: Days 9-10 (Integration & testing)

**Target Completion**: 10 days from start

## Dependencies

- Prometheus Go client: `github.com/prometheus/client_golang`
- OpenTelemetry: `go.opentelemetry.io/otel`
- Jaeger exporter: `go.opentelemetry.io/otel/exporters/jaeger`

## References

- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [OpenTelemetry Go SDK](https://opentelemetry.io/docs/instrumentation/go/)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)

---

*Sprint 6 Plan Created: 2025-11-06*  
*ZeroState Monitoring & Observability Phase*
