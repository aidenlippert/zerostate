# ZeroState Monitoring Stack Guide

**Last Updated:** November 7, 2025
**Sprint 6 - Phase 2 Complete** ✅

## Overview

Comprehensive monitoring stack with 90+ metrics, 4 Grafana dashboards, and 25+ alert rules.

### Stack Components

- **Prometheus** - Metrics collection and alerting
- **Grafana** - Visualization and dashboards
- **OpenTelemetry Collector** - Distributed tracing (future)
- **Jaeger** - Trace visualization (future)

---

## Quick Start

### 1. Start the Monitoring Stack

```bash
cd deployments/

# Start all services (including monitoring)
docker-compose up -d

# Or start only monitoring services
docker-compose up -d prometheus grafana
```

### 2. Access Dashboards

- **Grafana**: http://localhost:3000
  - Username: `admin`
  - Password: `admin` (change on first login)
- **Prometheus**: http://localhost:9090
- **Jaeger**: http://localhost:16686 (future tracing)

### 3. Import Dashboards

Dashboards are **auto-provisioned** on startup. No manual import needed!

Available dashboards:
1. **Network Overview** - P2P health, connections, bandwidth
2. **Execution Performance** - WASM tasks, guilds, receipts
3. **Economic Activity** - Payments, channels, reputation, TVL
4. **System Health** - Overall status, alerts, resource usage

---

## Dashboard Breakdown

### 1. Network Overview Dashboard

**Focus:** P2P networking layer health

**Key Panels:**
- **Network Health Overview** - Active connections, overall status
- **Total Peers Discovered** - DHT discovery effectiveness
- **Bandwidth Usage (TX/RX)** - Network throughput
- **Message Success Rate** - Message delivery reliability
- **Active Connections by State** - Connection pool health
- **Health Check Status** - Peer latency and availability
- **Gossip Propagation** - Message distribution rate
- **Circuit Relay Usage** - NAT traversal statistics
- **DHT Operations** - PUT/GET/FIND operations

**When to use:**
- Debugging connectivity issues
- Analyzing network performance
- Monitoring relay effectiveness
- Investigating message delivery failures

**Critical thresholds:**
- Active connections < 3 → 🟡 Warning
- Message success rate < 95% → 🟡 Warning
- Health check latency > 5s → 🔴 Critical

---

### 2. Execution Performance Dashboard

**Focus:** Task execution and WASM runtime

**Key Panels:**
- **Task Execution Success Rate** - Overall execution quality
- **Active Guilds** - Current guild count
- **Tasks/sec** - Execution throughput
- **Avg Task Duration (p50/p95/p99)** - Latency distribution
- **WASM Memory Usage** - Memory consumption patterns
- **Task Results (Success/Failure)** - Stacked area chart
- **Guild Lifecycle** - Created/dissolved/active guilds
- **Task Cost Distribution** - Economic metrics per task
- **WASM Exit Codes** - Pie chart of execution outcomes
- **Receipt Generation Rate** - Cryptographic proof creation

**When to use:**
- Optimizing task execution performance
- Debugging WASM failures
- Monitoring resource usage
- Analyzing guild formation patterns

**Critical thresholds:**
- Task success rate < 90% → 🟡 Warning
- p95 task duration > 25s → 🟡 Warning (limit: 30s)
- p95 memory usage > 100MB → 🟡 Warning (limit: 128MB)
- Task timeout rate > 5% → 🟡 Warning

---

### 3. Economic Activity Dashboard

**Focus:** Payments, reputation, and network economics

**Key Panels:**
- **Total Value Locked (TVL)** - Economic health indicator
- **Active Payment Channels** - Current channel count
- **Active Participants** - Network engagement
- **Payment Success Rate** - Transaction reliability
- **Blacklisted Peers** - Trust violations
- **TVL Trend (24h)** - Capital flow visualization
- **Channel Lifecycle** - Opening/active/closed/disputed
- **Payment Volume & Throughput** - Transaction activity
- **Payment Amount Distribution** - P50/P90/P99 payment sizes
- **Reputation Score Distribution** - Top 10 peers by reputation
- **Task Success Rate by Peer** - Individual executor quality
- **Settlement Activity** - Channel finalization rate
- **Dispute Rate** - Conflict resolution metrics
- **Blacklist Events** - Added/removed/expired events
- **Average Task Cost by Peer** - Pricing analytics
- **Transaction Throughput (TPS)** - Network-wide TPS

**When to use:**
- Monitoring network economic health
- Analyzing reputation system effectiveness
- Investigating payment failures
- Detecting fraud or attacks
- TVL tracking for business metrics

**Critical thresholds:**
- TVL drop > 50% in 1h → 🔴 Critical
- Blacklisted peers > 10% → 🟡 Warning
- Blacklisted peers > 25% → 🔴 Critical
- Payment failure rate > 5% → 🟡 Warning
- Dispute rate > 0.5/s → 🟡 Warning

---

### 4. System Health Dashboard

**Focus:** Overall system status and alerting

**Key Panels:**
- **Overall System Status** - 🟢 HEALTHY / 🟡 DEGRADED / 🔴 CRITICAL
- **Service Uptime** - Bootnode/edge-nodes/relay availability
- **Active Alerts** - Current firing alerts count
- **Error Rate by Component** - P2P/execution/economic errors
- **Component Health Scores** - Success rate per layer
- **Key Metrics Summary** - Table of critical metrics
- **Alert History (Last Hour)** - Recent alert log
- **Network Latency (p95)** - Cross-component latency
- **Resource Warnings** - Log viewer for ERROR/WARN
- **Critical Metrics Heatmap** - Service availability timeline

**When to use:**
- High-level system health monitoring
- Alert triage and investigation
- Incident response dashboard
- Executive/stakeholder demos

**Status Definitions:**
- 🟢 **HEALTHY**: Availability > 95%, all systems operational
- 🟡 **DEGRADED**: Availability 70-95%, some issues detected
- 🔴 **CRITICAL**: Availability < 70%, major outage

---

## Alert Rules

### Network Alerts

| Alert | Condition | Severity | For | Description |
|-------|-----------|----------|-----|-------------|
| **LowPeerConnections** | Active connections < 3 | ⚠️ Warning | 5m | Network connectivity degraded |
| **CriticalPeerConnections** | Active connections = 0 | 🚨 Critical | 2m | Node completely isolated |
| **HighMessageFailureRate** | Failure rate > 20% | ⚠️ Warning | 5m | Message delivery issues |
| **HighBandwidthUsage** | Bandwidth > 10MB/s | ⚠️ Warning | 5m | Potential bandwidth saturation |
| **HealthCheckFailures** | Failure rate > 30% | ⚠️ Warning | 5m | Peer health degraded |

### Execution Alerts

| Alert | Condition | Severity | For | Description |
|-------|-----------|----------|-----|-------------|
| **HighTaskFailureRate** | Failure rate > 10% | ⚠️ Warning | 5m | Execution quality degraded |
| **CriticalTaskFailureRate** | Failure rate > 50% | 🚨 Critical | 2m | Execution layer severely degraded |
| **HighTaskDuration** | p95 duration > 25s | ⚠️ Warning | 5m | Tasks approaching timeout |
| **HighMemoryUsage** | p95 memory > 100MB | ⚠️ Warning | 5m | Memory approaching limit |
| **TaskTimeoutRate** | Timeout rate > 5% | ⚠️ Warning | 5m | Tasks timing out frequently |
| **NoActiveGuilds** | Active guilds = 0 | ℹ️ Info | 10m | No task execution activity |

### Economic Alerts

| Alert | Condition | Severity | For | Description |
|-------|-----------|----------|-----|-------------|
| **TVLDrop** | TVL drop > 50% in 1h | 🚨 Critical | 5m | Mass exit or dispute |
| **HighBlacklistRate** | Blacklisted > 10% | ⚠️ Warning | 5m | Trust violations increasing |
| **CriticalBlacklistRate** | Blacklisted > 25% | 🚨 Critical | 2m | Network trust compromised |
| **PaymentFailureRate** | Failure rate > 5% | ⚠️ Warning | 5m | Payment processing issues |
| **LowActiveChannels** | Active channels < 5 | ℹ️ Info | 10m | Low economic activity |
| **HighDisputeRate** | Disputes > 0.5/s | ⚠️ Warning | 5m | Potential fraud or quality issues |
| **SettlementFailures** | Failure rate > 10% | ⚠️ Warning | 5m | Settlement processing issues |

### System Alerts

| Alert | Condition | Severity | For | Description |
|-------|-----------|----------|-----|-------------|
| **ServiceDown** | Service uptime = 0 | 🚨 Critical | 1m | Service outage detected |
| **HighSystemLatency** | p95 latency > 10s | ⚠️ Warning | 5m | System-wide performance degradation |
| **ComponentHealthDegraded** | Success rate < 80% | ⚠️ Warning | 5m | Component health below threshold |
| **HighErrorRate** | Error rate > 10/s | ⚠️ Warning | 5m | System-wide error spike |

### SLA Alerts

| Alert | Condition | Severity | For | Description |
|-------|-----------|----------|-----|-------------|
| **SLAViolation_TaskExecution** | p95 duration > 30s | 🚨 Critical | 10m | Task execution SLA violated |
| **SLAViolation_Availability** | Availability < 99.9% | 🚨 Critical | 5m | System availability SLA violated |
| **SLAViolation_PaymentSuccess** | Success rate < 99.9% | 🚨 Critical | 10m | Payment success SLA violated |

---

## Useful Queries

### Network Queries

```promql
# Active connections
sum(p2p_connections_total{state="active"})

# Message success rate
sum(rate(p2p_messages_total{status="success"}[5m])) / sum(rate(p2p_messages_total[5m]))

# Bandwidth usage (MB/s)
rate(p2p_bandwidth_bytes_total[1m]) / 1024 / 1024

# DHT operations rate
rate(p2p_dht_operations_total[1m])
```

### Execution Queries

```promql
# Task success rate
sum(rate(execution_tasks_total{result="success"}[5m])) / sum(rate(execution_tasks_total[5m]))

# Task duration p95
histogram_quantile(0.95, rate(execution_task_duration_seconds_bucket[5m]))

# Memory usage p95
histogram_quantile(0.95, rate(execution_memory_bytes_bucket[5m]))

# Active guilds
execution_guilds_total{state="active"}
```

### Economic Queries

```promql
# Total Value Locked
economic_total_value_locked

# Active payment channels
sum(economic_channels_active)

# Payment success rate
sum(rate(economic_payments_total{status="success"}[5m])) / sum(rate(economic_payments_total[5m]))

# Top 10 reputation scores
topk(10, economic_reputation_score)

# Blacklist percentage
(economic_blacklisted_peers / economic_active_participants) * 100
```

---

## Troubleshooting

### Dashboard Not Loading

**Problem:** Dashboard shows "No Data" or panels are empty

**Solutions:**
1. Check Prometheus is scraping metrics:
   ```bash
   curl http://localhost:9090/api/v1/targets
   ```
2. Verify services expose `/metrics` endpoint:
   ```bash
   curl http://localhost:8080/metrics
   ```
3. Check Grafana datasource connection:
   - Grafana → Configuration → Data Sources → Prometheus
   - Click "Test" button, should show "Data source is working"

### Alerts Not Firing

**Problem:** Prometheus alerts not triggering

**Solutions:**
1. Check alert rules are loaded:
   ```bash
   curl http://localhost:9090/api/v1/rules
   ```
2. Verify Prometheus can read alert file:
   ```bash
   docker exec -it zs-prometheus cat /etc/prometheus/prometheus-alerts.yml
   ```
3. Reload Prometheus config:
   ```bash
   curl -X POST http://localhost:9090/-/reload
   ```

### High Memory Usage

**Problem:** Prometheus/Grafana consuming too much memory

**Solutions:**
1. Reduce retention period (default: 15 days):
   ```yaml
   # In docker-compose.yml, add to prometheus command:
   - '--storage.tsdb.retention.time=7d'
   ```
2. Reduce scrape interval (default: 15s):
   ```yaml
   # In prometheus.yml:
   global:
     scrape_interval: 30s
   ```
3. Limit dashboard time range (use "Last 1h" instead of "Last 24h")

---

## Production Deployment

### Security Hardening

1. **Change Default Credentials**:
   ```yaml
   # In docker-compose.yml, Grafana section:
   environment:
     - GF_SECURITY_ADMIN_PASSWORD=<strong-password>
   ```

2. **Enable Authentication**:
   ```yaml
   # Add to prometheus command:
   - '--web.enable-admin-api=false'
   ```

3. **Use TLS**:
   - Configure reverse proxy (Nginx/Traefik) with Let's Encrypt
   - Enable HTTPS for Grafana

### Alerting Integration

**Alertmanager Configuration** (future):
```yaml
# alertmanager.yml
route:
  receiver: 'default'

receivers:
  - name: 'default'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
        channel: '#zerostate-alerts'
```

### Backup & Restore

**Backup Grafana Dashboards**:
```bash
docker exec zs-grafana grafana-cli admin export-dashboard > backup.json
```

**Backup Prometheus Data**:
```bash
docker run --rm -v prometheus-data:/data -v $(pwd):/backup alpine \
  tar czf /backup/prometheus-backup.tar.gz -C /data .
```

---

## Metrics Reference

### P2P Metrics (30+)

- `p2p_connections_total{state}` - Connection count by state
- `p2p_bandwidth_bytes_total{direction}` - Bandwidth TX/RX
- `p2p_messages_total{status}` - Message delivery stats
- `p2p_health_check_latency_seconds` - Peer latency
- `p2p_dht_operations_total{operation}` - DHT activity
- `p2p_gossip_messages_total{status}` - Gossip propagation
- `p2p_relay_connections_total{status}` - Circuit relay usage

### Execution Metrics (25+)

- `execution_tasks_total{result}` - Task execution outcomes
- `execution_task_duration_seconds` - Task duration histogram
- `execution_memory_bytes` - WASM memory usage histogram
- `execution_guilds_total{state}` - Guild lifecycle
- `execution_receipts_total{status}` - Receipt generation
- `execution_wasm_exit_codes_total{exit_code}` - Exit code distribution
- `execution_task_cost` - Task cost histogram

### Economic Metrics (35+)

- `economic_total_value_locked` - TVL gauge
- `economic_channels_active{party}` - Active channel count
- `economic_payments_total{status}` - Payment outcomes
- `economic_payment_amount` - Payment size histogram
- `economic_reputation_score{peer_id}` - Reputation by peer
- `economic_success_rate{peer_id}` - Success rate by peer
- `economic_blacklisted_peers` - Blacklist count
- `economic_settlements_total{result}` - Settlement outcomes
- `economic_disputes_total{resolution}` - Dispute metrics
- `economic_transaction_throughput` - TPS histogram

---

## Next Steps

### Sprint 6 Remaining Tasks

- [ ] **Task 8-10**: OpenTelemetry distributed tracing
- [ ] **Task 11-12**: Structured logging with log aggregation
- [ ] **Task 13-14**: Health check endpoints for Kubernetes
- [ ] **Task 15-16**: Integration tests for monitoring stack

### Future Enhancements

1. **Alertmanager Integration** - Slack/PagerDuty/Email notifications
2. **Long-term Storage** - Thanos/Cortex for multi-year retention
3. **Service Mesh** - Istio/Linkerd for advanced observability
4. **APM Integration** - Distributed tracing with Jaeger/Zipkin
5. **Log Aggregation** - Loki/Elasticsearch for centralized logging
6. **Cost Analytics** - Economic forecasting dashboards
7. **Anomaly Detection** - ML-based alerting (Prometheus + Grafana ML)

---

## Support

**Documentation**:
- [Prometheus Docs](https://prometheus.io/docs/)
- [Grafana Docs](https://grafana.com/docs/)
- [ZeroState Architecture](../docs/ARCHITECTURE.md)

**Issues**:
- Report monitoring bugs in GitHub Issues
- Tag with `monitoring`, `grafana`, or `prometheus`

**Metrics Not Showing Up?**
1. Check service logs: `docker logs zs-edge-1`
2. Verify metrics endpoint: `curl http://localhost:8080/metrics`
3. Check Prometheus targets: http://localhost:9090/targets

---

**Generated:** November 7, 2025
**Sprint 6 - Phase 2 Complete** ✅
**90+ Metrics | 4 Dashboards | 25+ Alerts**
