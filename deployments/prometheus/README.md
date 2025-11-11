# Prometheus Alert Rules for ZeroState API

Production-ready Prometheus alert rules for monitoring the ZeroState API platform.

## Overview

This directory contains comprehensive Prometheus alert rules covering:
- **HTTP/API**: Error rates, latency, availability
- **Database**: Connection health, query performance
- **Task Execution**: Task failures, queue depth, timeouts
- **Orchestrator**: Worker health and capacity
- **WASM**: Execution failures and resource usage
- **P2P Network**: Peer connectivity, message delivery
- **System**: CPU, memory, goroutines
- **Economic Layer**: Auction and bidding metrics
- **Storage**: S3 operations and capacity

## Quick Start

### 1. Prometheus Configuration

Add the alert rules to your Prometheus configuration:

```yaml
# prometheus.yml
rule_files:
  - /etc/prometheus/alert_rules.yml

# Alertmanager configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - alertmanager:9093
```

### 2. Verify Rules

Validate the rules syntax:

```bash
promtool check rules deployments/prometheus/alert_rules.yml
```

Test a specific alert expression:

```bash
promtool query instant \
  'http://prometheus:9090' \
  'rate(zerostate_tasks_total{status="failed"}[10m]) / rate(zerostate_tasks_total[10m]) > 0.20'
```

### 3. Reload Configuration

Reload Prometheus without downtime:

```bash
curl -X POST http://prometheus:9090/-/reload
```

Or send SIGHUP:

```bash
kill -HUP <prometheus-pid>
```

## Alert Severity Levels

### Critical (P0)
- **Response Time**: Immediate (< 5 minutes)
- **Escalation**: PagerDuty + Slack
- **Examples**: Service down, high error rate, no orchestrator workers
- **Impact**: User-facing failures, service outages

### Warning (P1-P2)
- **Response Time**: Within business hours
- **Escalation**: Slack only
- **Examples**: High latency, database slowness, queue backlog
- **Impact**: Degraded performance, potential issues

### Info (P3)
- **Response Time**: Review during normal operations
- **Escalation**: Slack info channel
- **Examples**: High bid rejection rate
- **Impact**: Operational visibility, optimization opportunities

## Key Alerts

### Service Availability

**ServiceDown**
- **Trigger**: Health endpoint not responding for 1 minute
- **Impact**: Service completely unavailable
- **Action**: Check pod status, logs, recent deployments

**ServiceNotReady**
- **Trigger**: Readiness probe failing for 3 minutes
- **Impact**: Service can't handle traffic
- **Action**: Check database connectivity, orchestrator workers

### HTTP/API

**HighHTTPErrorRate**
- **Trigger**: >5% of requests returning 5xx errors for 2 minutes
- **Impact**: User-facing service failures
- **Action**: Check logs for error patterns, database health, resource usage

**CriticalAPILatency**
- **Trigger**: p99 latency > 5s for 2 minutes
- **Impact**: Severe UX degradation
- **Action**: Check database query performance, CPU usage, blocking operations

### Database

**DatabaseConnectionFailures**
- **Trigger**: Connection failures >1/sec for 2 minutes
- **Impact**: Service unable to access data
- **Action**: Check database health, connection pool settings, network connectivity

**DatabaseConnectionPoolExhaustion**
- **Trigger**: >90% of connection pool in use for 3 minutes
- **Impact**: Risk of connection exhaustion
- **Action**: Review connection pool size, check for connection leaks

### Task Execution

**HighTaskFailureRate**
- **Trigger**: >20% of tasks failing for 5 minutes
- **Impact**: Task execution system degraded
- **Action**: Check WASM execution logs, agent availability, resource limits

**CriticalTaskQueueBacklog**
- **Trigger**: >500 tasks in queue for 3 minutes
- **Impact**: System overloaded, severe delays
- **Action**: Scale orchestrator workers, check for stuck tasks, review resource limits

### Orchestrator

**OrchestratorNoWorkers**
- **Trigger**: 0 active workers for 2 minutes
- **Impact**: No task processing capability
- **Action**: Check orchestrator service, restart if needed, review startup logs

### System Resources

**CriticalMemoryUsage**
- **Trigger**: >95% memory usage for 2 minutes
- **Impact**: Imminent OOM risk, service crash
- **Action**: Restart service, investigate memory leaks, scale up resources

**CriticalGoroutineCount**
- **Trigger**: >10,000 goroutines for 3 minutes
- **Impact**: Goroutine leak, service instability
- **Action**: Check for goroutine leaks, review long-running operations

## Alertmanager Configuration

### Example Configuration

Create `alertmanager.yml`:

```yaml
global:
  resolve_timeout: 5m
  slack_api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'

route:
  group_by: ['alertname', 'component', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'default'

  routes:
    # Critical alerts to PagerDuty + Slack
    - match:
        severity: critical
      receiver: 'pagerduty-critical'
      continue: true
      repeat_interval: 5m

    # Warning alerts to Slack only
    - match:
        severity: warning
      receiver: 'slack-warnings'
      repeat_interval: 1h

    # Info alerts to separate channel
    - match:
        severity: info
      receiver: 'slack-info'
      repeat_interval: 24h

receivers:
  - name: 'default'
    slack_configs:
      - channel: '#alerts'
        title: 'ZeroState Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'

  - name: 'pagerduty-critical'
    pagerduty_configs:
      - service_key: '<YOUR_PAGERDUTY_SERVICE_KEY>'
        description: '{{ .GroupLabels.alertname }}'
    slack_configs:
      - channel: '#alerts-critical'
        title: ':rotating_light: CRITICAL Alert'
        text: |
          *Alert:* {{ .GroupLabels.alertname }}
          *Severity:* {{ .CommonLabels.severity }}
          *Component:* {{ .CommonLabels.component }}
          {{ range .Alerts }}
          *Summary:* {{ .Annotations.summary }}
          *Description:* {{ .Annotations.description }}
          *Impact:* {{ .Annotations.impact }}
          {{ if .Annotations.runbook_url }}*Runbook:* {{ .Annotations.runbook_url }}{{ end }}
          {{ end }}
        color: 'danger'

  - name: 'slack-warnings'
    slack_configs:
      - channel: '#alerts-warnings'
        title: ':warning: Warning Alert'
        text: |
          *Alert:* {{ .GroupLabels.alertname }}
          *Component:* {{ .CommonLabels.component }}
          {{ range .Alerts }}
          {{ .Annotations.summary }}
          {{ end }}
        color: 'warning'

  - name: 'slack-info'
    slack_configs:
      - channel: '#alerts-info'
        title: ':information_source: Info Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
        color: 'good'

inhibit_rules:
  # Inhibit warning if critical alert is firing
  - source_match:
      severity: 'critical'
    target_match:
      severity: 'warning'
    equal: ['alertname', 'component']

  # Inhibit ServiceNotReady if ServiceDown is firing
  - source_match:
      alertname: 'ServiceDown'
    target_match:
      alertname: 'ServiceNotReady'
    equal: ['component']
```

### Deployment

```bash
# Start Alertmanager
docker run -d \
  --name alertmanager \
  -p 9093:9093 \
  -v /path/to/alertmanager.yml:/etc/alertmanager/alertmanager.yml \
  prom/alertmanager:latest \
  --config.file=/etc/alertmanager/alertmanager.yml
```

## Testing Alerts

### Simulate High Error Rate

```bash
# Generate 5xx errors
for i in {1..100}; do
  curl -X POST http://localhost:8080/api/v1/invalid-endpoint
done
```

### Simulate High Latency

Add artificial delay in code temporarily:

```go
time.Sleep(6 * time.Second) // Will trigger CriticalAPILatency
```

### Simulate Queue Backlog

```bash
# Submit many tasks quickly
for i in {1..1000}; do
  curl -X POST http://localhost:8080/api/v1/tasks/submit \
    -H "Content-Type: application/json" \
    -d '{"query":"test","budget":0.10,"timeout":300,"priority":"normal"}' &
done
```

### Test Alert Routing

Send test alert to Alertmanager:

```bash
curl -X POST http://localhost:9093/api/v1/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestAlert",
      "severity": "warning",
      "component": "test"
    },
    "annotations": {
      "summary": "Test alert for routing verification"
    }
  }]'
```

## Kubernetes Deployment

### ServiceMonitor

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: zerostate-api
  namespace: default
spec:
  selector:
    matchLabels:
      app: zerostate-api
  endpoints:
    - port: metrics
      path: /metrics
      interval: 30s
```

### PrometheusRule

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: zerostate-alerts
  namespace: default
spec:
  groups:
    - name: zerostate
      interval: 30s
      rules:
        # Import rules from alert_rules.yml
```

## Monitoring Dashboard

Key metrics to visualize in Grafana:

1. **HTTP Overview**
   - Request rate by status code
   - Error rate (%)
   - p50/p95/p99 latency

2. **Task Execution**
   - Task submission rate
   - Task success/failure rate
   - Queue depth over time

3. **System Resources**
   - CPU usage
   - Memory usage
   - Goroutine count

4. **Database**
   - Query latency
   - Connection pool usage
   - Error rate

## Troubleshooting

### Alert Not Firing

1. Check metric exists:
   ```bash
   curl http://prometheus:9090/api/v1/query?query=zerostate_tasks_total
   ```

2. Verify rule expression:
   ```bash
   promtool query instant 'http://prometheus:9090' '<expression>'
   ```

3. Check Prometheus logs for evaluation errors:
   ```bash
   docker logs prometheus 2>&1 | grep -i error
   ```

### Alert Firing Constantly

1. Adjust threshold or duration in rule
2. Check if metric labels are causing duplicate alerts
3. Review `group_by` and `group_interval` in Alertmanager

### Alerts Not Reaching Slack/PagerDuty

1. Verify Alertmanager is receiving alerts:
   ```bash
   curl http://alertmanager:9093/api/v1/alerts
   ```

2. Check Alertmanager logs:
   ```bash
   docker logs alertmanager
   ```

3. Verify webhook URLs and credentials

## Best Practices

1. **Tune Alert Thresholds**
   - Monitor for false positives
   - Adjust based on baseline metrics
   - Consider business hours vs off-hours

2. **Meaningful Annotations**
   - Include impact and suggested actions
   - Link to runbooks
   - Provide context for on-call engineers

3. **Alert Fatigue Prevention**
   - Use appropriate severity levels
   - Set reasonable repeat intervals
   - Use inhibit rules to reduce noise

4. **Regular Review**
   - Review fired alerts weekly
   - Remove or tune noisy alerts
   - Add new alerts based on incidents

5. **Testing**
   - Test critical alerts monthly
   - Verify escalation paths
   - Validate runbook accuracy

## Metrics Reference

All ZeroState metrics use the `zerostate_` prefix:

- **HTTP**: `http_requests_total`, `http_request_duration_seconds`
- **Database**: `zerostate_database_*`
- **Tasks**: `zerostate_tasks_*`, `zerostate_task_*`
- **WASM**: `zerostate_wasm_*`
- **P2P**: `zerostate_p2p_*`
- **Orchestrator**: `zerostate_orchestrator_*`

See `/metrics` endpoint for complete list:
```bash
curl http://localhost:8080/metrics
```

## Additional Resources

- [Prometheus Documentation](https://prometheus.io/docs/)
- [Alertmanager Configuration](https://prometheus.io/docs/alerting/latest/configuration/)
- [Best Practices for Alerting](https://prometheus.io/docs/practices/alerting/)
- [PromQL Basics](https://prometheus.io/docs/prometheus/latest/querying/basics/)

## Support

For issues or questions:
- Internal documentation: https://docs.zerostate.ai/observability
- Team channel: #platform-observability
- On-call: Check PagerDuty schedule
