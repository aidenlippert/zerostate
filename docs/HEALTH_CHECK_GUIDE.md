# ZeroState Health Check Guide

**Last Updated:** November 7, 2025
**Sprint 6 - Phase 5 Complete** ✅

---

## Overview

Production-grade health checking for ZeroState services with Kubernetes liveness and readiness probes.

### Health Check Endpoints

- **`/healthz`** - Liveness check (is the service alive?)
- **`/readyz`** - Readiness check (is the service ready for traffic?)
- **`/health`** - Detailed health check (all component status)

---

## Quick Start

### 1. Initialize Health Checker

```go
package main

import (
    "net/http"
    "github.com/zerostate/libs/health"
)

func main() {
    // Create health checker
    h := health.New()

    // Register component checkers
    h.Register("p2p", health.NewP2PChecker(
        func() int { return node.PeerCount() },
        3,  // Minimum 3 peers
        func() float64 { return node.HealthCheckRate() },
    ))

    h.Register("execution", health.NewExecutionChecker(
        func() int { return runner.ActiveExecutions() },
        func() float64 { return runner.SuccessRate() },
        func() time.Duration { return runner.AvgDuration() },
        25*time.Second,  // Max avg duration
    ))

    // Create HTTP handler
    handler := health.NewHandler(h,
        health.WithCriticalComponents("p2p", "execution"),
        health.WithMetadata("service", "edge-node-1"),
        health.WithMetadata("version", "0.1.0"),
    )

    // Register endpoints
    http.HandleFunc("/healthz", handler.LivenessHandler())
    http.HandleFunc("/readyz", handler.ReadinessHandler())
    http.HandleFunc("/health", handler.DetailedHandler())

    http.ListenAndServe(":8080", nil)
}
```

### 2. Test Health Endpoints

```bash
# Liveness check
curl http://localhost:8080/healthz

# Readiness check
curl http://localhost:8080/readyz

# Detailed health
curl http://localhost:8080/health | jq
```

---

## Health Check Architecture

### Liveness vs Readiness

**Liveness (`/healthz`):**
- **Purpose**: Is the service alive and functioning?
- **Action**: Restart container if unhealthy
- **Returns 200**: Service is healthy or degraded
- **Returns 503**: Service is unhealthy (needs restart)

**Readiness (`/readyz`):**
- **Purpose**: Is the service ready to accept traffic?
- **Action**: Remove from load balancer if not ready
- **Returns 200**: All critical components healthy
- **Returns 503**: Not ready (bootstrapping, degraded dependencies)

**Startup (`/healthz` with startup probe):**
- **Purpose**: Allow extra time for initial startup
- **Action**: Don't run liveness/readiness until startup succeeds
- **Use case**: Services with slow initialization (DHT bootstrap, etc.)

---

## Response Format

### Successful Response (200 OK)

```json
{
  "status": "healthy",
  "timestamp": "2025-11-07T10:30:15.123Z",
  "components": {
    "p2p": {
      "status": "healthy",
      "message": "5 peers connected, health rate 95.00%",
      "timestamp": "2025-11-07T10:30:15.120Z",
      "duration_ms": 2,
      "metadata": {
        "peer_count": 5,
        "health_rate": 0.95
      }
    },
    "execution": {
      "status": "healthy",
      "message": "2 active, 98.50% success, avg 1.5s",
      "timestamp": "2025-11-07T10:30:15.121Z",
      "duration_ms": 1,
      "metadata": {
        "active_executions": 2,
        "success_rate": 0.985,
        "avg_duration_ms": 1500
      }
    }
  },
  "metadata": {
    "service": "edge-node-1",
    "version": "0.1.0"
  }
}
```

### Degraded Response (200 OK for liveness, 503 for readiness)

```json
{
  "status": "degraded",
  "timestamp": "2025-11-07T10:30:15.123Z",
  "components": {
    "p2p": {
      "status": "degraded",
      "message": "peer count 2 below minimum 3",
      "metadata": {
        "peer_count": 2,
        "min_peers": 3
      }
    }
  }
}
```

### Unhealthy Response (503 Service Unavailable)

```json
{
  "status": "unhealthy",
  "timestamp": "2025-11-07T10:30:15.123Z",
  "components": {
    "p2p": {
      "status": "unhealthy",
      "message": "no peer connections",
      "metadata": {
        "peer_count": 0
      }
    }
  }
}
```

---

## Component Health Checkers

### P2P Network Health

```go
h.Register("p2p", health.NewP2PChecker(
    func() int { return node.PeerCount() },
    3,  // Minimum peers
    func() float64 { return node.HealthCheckRate() },
))
```

**Health Criteria:**
- ✅ **Healthy**: ≥ min peers, health check rate ≥ 70%
- ⚠️ **Degraded**: < min peers OR health check rate < 70%
- ❌ **Unhealthy**: 0 peers

---

### Task Execution Health

```go
h.Register("execution", health.NewExecutionChecker(
    func() int { return runner.ActiveExecutions() },
    func() float64 { return runner.SuccessRate() },
    func() time.Duration { return runner.AvgDuration() },
    25*time.Second,
))
```

**Health Criteria:**
- ✅ **Healthy**: Success rate ≥ 90%, avg duration < max
- ⚠️ **Degraded**: Success rate 50-90% OR avg duration > max
- ❌ **Unhealthy**: Success rate < 50%

---

### Payment Channel Health

```go
h.Register("payment", health.NewPaymentChecker(
    func() int { return manager.ActiveChannels() },
    func() float64 { return manager.SuccessRate() },
    func() float64 { return manager.TotalLocked() },
))
```

**Health Criteria:**
- ✅ **Healthy**: Success rate ≥ 95%
- ⚠️ **Degraded**: Success rate 80-95%
- ❌ **Unhealthy**: Success rate < 80%

---

### Guild Formation Health

```go
h.Register("guild", health.NewGuildChecker(
    func() int { return guildManager.ActiveGuilds() },
    func() float64 { return guildManager.AvgMembers() },
))
```

**Health Criteria:**
- ✅ **Healthy**: Avg members ≥ 2.0 OR no active guilds (idle)
- ⚠️ **Degraded**: Avg members < 2.0

---

### DHT (Kademlia) Health

```go
h.Register("dht", health.NewDHTChecker(
    func() int { return dht.RoutingTableSize() },
    func() float64 { return dht.SuccessRate() },
))
```

**Health Criteria:**
- ✅ **Healthy**: Routing table ≥ 10 peers, success rate ≥ 80%
- ⚠️ **Degraded**: Routing table < 10 OR success rate < 80%
- ❌ **Unhealthy**: Empty routing table

---

### Storage Health

```go
h.Register("storage", health.NewStorageChecker(
    func() float64 { return storage.DiskUsagePercent() },
    80.0,  // Warn threshold
    95.0,  // Critical threshold
))
```

**Health Criteria:**
- ✅ **Healthy**: Disk usage < 80%
- ⚠️ **Degraded**: Disk usage 80-95%
- ❌ **Unhealthy**: Disk usage ≥ 95%

---

## Kubernetes Integration

### Deployment with Health Probes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: zerostate-edge-node
spec:
  template:
    spec:
      containers:
      - name: edge-node
        image: zerostate-edge-node:latest
        ports:
        - containerPort: 8080
          name: metrics

        # Liveness probe - restart if unhealthy
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3

        # Readiness probe - remove from service if not ready
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 5
          timeoutSeconds: 3
          failureThreshold: 2

        # Startup probe - allow time for bootstrap
        startupProbe:
          httpGet:
            path: /healthz
            port: 8080
          periodSeconds: 10
          failureThreshold: 12  # 2 minutes max
```

### Probe Timing Guidelines

**Liveness Probe:**
- `initialDelaySeconds`: 20-30s (enough for basic initialization)
- `periodSeconds`: 10s (check every 10 seconds)
- `timeoutSeconds`: 5s (allow 5s for response)
- `failureThreshold`: 3 (restart after 3 consecutive failures = 30s)

**Readiness Probe:**
- `initialDelaySeconds`: 5-10s (quick check)
- `periodSeconds`: 5s (check frequently)
- `timeoutSeconds`: 3s (fast response expected)
- `failureThreshold`: 2 (remove from LB after 2 failures = 10s)

**Startup Probe:**
- `initialDelaySeconds`: 0s (start checking immediately)
- `periodSeconds`: 10s (check every 10 seconds)
- `failureThreshold`: 12-30 (allow 2-5 minutes for bootstrap)

---

## Best Practices

### DO ✅

- **Use different thresholds** for liveness (more forgiving) vs readiness (stricter)
- **Allow startup time** with startup probes for slow-initializing services
- **Check critical dependencies** in readiness (P2P, DHT bootstrap)
- **Include metadata** (service name, version, pod name)
- **Return quickly** (health checks should complete in < 3s)
- **Test failure scenarios** (simulate peer loss, resource exhaustion)

### DON'T ❌

- **Don't check external services** in liveness (use readiness instead)
- **Don't use aggressive timeouts** (< 3s) for complex checks
- **Don't restart too quickly** (allow time for transient issues)
- **Don't forget startup probes** for services with slow bootstrap
- **Don't ignore degraded state** (log warnings, alert if persistent)

---

## Custom Health Checkers

### Example: Custom Threshold Checker

```go
type CustomChecker struct {
    name      string
    getValue  func() float64
    threshold float64
}

func (c *CustomChecker) Name() string {
    return c.name
}

func (c *CustomChecker) Check(ctx context.Context) health.CheckResult {
    value := c.getValue()

    if value > c.threshold {
        return health.CheckResult{
            Status:  health.StatusUnhealthy,
            Message: fmt.Sprintf("value %.2f exceeds threshold %.2f", value, c.threshold),
        }
    }

    return health.CheckResult{
        Status:  health.StatusHealthy,
        Message: "within threshold",
    }
}

// Register
h.Register("custom", &CustomChecker{
    name:      "cpu_usage",
    getValue:  func() float64 { return getCPUUsage() },
    threshold: 90.0,
})
```

---

## Monitoring & Alerting

### Prometheus Metrics

Health check results can be exported as Prometheus metrics:

```promql
# Service health status (0=unhealthy, 1=degraded, 2=healthy)
zerostate_health_status{component="p2p"} 2

# Health check duration
zerostate_health_check_duration_seconds{component="p2p"} 0.002

# Component-specific metrics
zerostate_p2p_peer_count 5
zerostate_execution_success_rate 0.98
```

### Grafana Alerts

```yaml
- alert: ServiceUnhealthy
  expr: zerostate_health_status < 1
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Service {{ $labels.service }} is unhealthy"

- alert: ServiceDegraded
  expr: zerostate_health_status == 1
  for: 10m
  labels:
    severity: warning
  annotations:
    summary: "Service {{ $labels.service }} is degraded"
```

---

## Troubleshooting

### Issue: Liveness Probe Failing

**Symptoms:** Container keeps restarting

**Solutions:**

1. **Check initialDelaySeconds:**
```yaml
livenessProbe:
  initialDelaySeconds: 30  # Increase if needed
```

2. **Increase timeout:**
```yaml
livenessProbe:
  timeoutSeconds: 10  # From 5s
```

3. **Check logs:**
```bash
kubectl logs <pod> --previous
```

---

### Issue: Readiness Probe Failing

**Symptoms:** Pod not receiving traffic

**Solutions:**

1. **Check component health:**
```bash
kubectl exec <pod> -- curl localhost:8080/health | jq
```

2. **Verify critical components:**
```go
// Ensure critical components are ready
criticalComponents: []string{"p2p", "dht"}
```

3. **Check bootstrap time:**
```yaml
readinessProbe:
  initialDelaySeconds: 20  # Allow DHT bootstrap
```

---

### Issue: Startup Probe Timeout

**Symptoms:** Pod never becomes ready

**Solutions:**

1. **Increase failure threshold:**
```yaml
startupProbe:
  failureThreshold: 30  # 5 minutes instead of 2
```

2. **Check bootstrap dependencies:**
```bash
# Check if bootnode is reachable
kubectl exec <pod> -- ping zerostate-bootnode
```

---

## Production Deployment

### High Availability

**Multiple Replicas:**
```yaml
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 1
```

**Pod Disruption Budget:**
```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: zerostate-edge-node-pdb
spec:
  minAvailable: 2
  selector:
    matchLabels:
      app: zerostate
      component: edge-node
```

### Resource Management

```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

---

## Next Steps

### Sprint 6 Remaining Tasks

- [x] **Task 13-14**: Health check endpoints ✅
- [ ] **Task 15-16**: Integration tests & chaos validation

### Future Enhancements

1. **Custom Health Aggregators** - Weighted health scores
2. **Health History API** - Track health over time
3. **Predictive Health** - ML-based failure prediction
4. **Auto-Remediation** - Automatic recovery actions
5. **Multi-Cluster Health** - Cross-cluster health dashboard

---

## Support

**Documentation:**
- [Kubernetes Liveness/Readiness](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [Health Check Best Practices](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)

**Files Created:**
- [libs/health/checker.go](../libs/health/checker.go) - Core health checker
- [libs/health/http.go](../libs/health/http.go) - HTTP handlers
- [libs/health/components.go](../libs/health/components.go) - ZeroState-specific checkers
- [deployments/k8s/edge-node-deployment.yaml](../deployments/k8s/edge-node-deployment.yaml) - K8s manifest
- [deployments/k8s/bootnode-deployment.yaml](../deployments/k8s/bootnode-deployment.yaml) - Bootnode manifest

---

**Generated:** November 7, 2025
**Sprint 6 - Phase 5 Complete** ✅
**Health Checks: Production-Ready**
