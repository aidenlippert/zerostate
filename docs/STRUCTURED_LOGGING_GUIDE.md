# ZeroState Structured Logging Guide

**Last Updated:** November 7, 2025
**Sprint 6 - Phase 4 Complete** ✅

---

## Overview

Comprehensive structured logging implementation for ZeroState using Zap, Loki, and Grafana. Correlate logs with distributed traces for complete observability.

### Stack Components

- **Zap** - High-performance structured logging
- **Loki** - Log aggregation and storage
- **Promtail** - Log collection and forwarding
- **Grafana** - Log visualization and exploration

---

## Quick Start

### 1. Start the Logging Stack

```bash
cd deployments/

# Start all services (includes Loki + Promtail)
docker-compose up -d

# Or start only logging services
docker-compose up -d loki promtail grafana
```

### 2. Access Grafana Logs Explorer

- **Grafana**: http://localhost:3000
- **Loki API**: http://localhost:3100
- Navigate to **Explore** → Select **Loki** datasource

### 3. Initialize Structured Logger

```go
package main

import (
    "context"
    "github.com/zerostate/libs/telemetry"
    "go.uber.org/zap"
)

func main() {
    // Create logger
    logger, err := telemetry.NewLogger(&telemetry.LogConfig{
        ServiceName:  "edge-node-1",
        ServiceVersion: "0.1.0",
        Level:        "info",
        Format:       "json",  // For Loki parsing
        Environment:  "development",
    })
    if err != nil {
        panic(err)
    }
    defer logger.Sync()

    // Use logger
    logger.Info("service started",
        zap.String("listen_addr", "0.0.0.0:8080"),
        zap.Int("max_peers", 50),
    )
}
```

---

## Architecture

### Log Flow

```
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Service   │─────▶│  JSON Log   │─────▶│  Promtail   │
│  (Zap)      │      │  (stdout)   │      │  (Collect)  │
└─────────────┘      └─────────────┘      └─────┬───────┘
                                                 │
                                                 ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│   Grafana   │◀─────│    Loki     │◀─────│  Promtail   │
│  (Explore)  │      │  (Storage)  │      │  (Forward)  │
└─────────────┘      └─────────────┘      └─────────────┘
```

### Trace-Log Correlation

Logs automatically include trace IDs from OpenTelemetry context:

```json
{
  "level": "info",
  "timestamp": "2025-11-07T10:30:15.123Z",
  "service": "edge-node-1",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "message": "task execution started",
  "task_id": "task-abc123",
  "guild_id": "guild-xyz789"
}
```

---

## Structured Logging Patterns

### Pattern 1: Basic Structured Logging

```go
import (
    "github.com/zerostate/libs/telemetry"
    "go.uber.org/zap"
)

func (s *Service) ProcessTask(taskID string, data []byte) error {
    s.logger.Info("processing task",
        telemetry.TaskID(taskID),
        telemetry.SizeBytes(int64(len(data))),
    )

    // Process task...

    if err != nil {
        s.logger.Error("task processing failed",
            telemetry.TaskID(taskID),
            zap.Error(err),
        )
        return err
    }

    s.logger.Info("task processed successfully",
        telemetry.TaskID(taskID),
        telemetry.Status("success"),
    )
    return nil
}
```

### Pattern 2: Context-Aware Logging (with Trace IDs)

```go
import "github.com/zerostate/libs/telemetry"

func (s *Service) HandleRequest(ctx context.Context, req *Request) error {
    // Automatically includes trace_id and span_id
    telemetry.InfoCtx(ctx, s.logger, "handling request",
        zap.String("request_id", req.ID),
        zap.Int("payload_size", len(req.Payload)),
    )

    // Process request...

    if err != nil {
        telemetry.ErrorCtx(ctx, s.logger, "request failed",
            zap.Error(err),
            zap.String("error_type", classifyError(err)),
        )
        return err
    }

    telemetry.InfoCtx(ctx, s.logger, "request completed",
        telemetry.DurationMS(elapsedMS),
    )
    return nil
}
```

### Pattern 3: Structured Logger with Persistent Fields

```go
import "github.com/zerostate/libs/telemetry"

type TaskExecutor struct {
    logger *telemetry.StructuredLogger
}

func NewTaskExecutor(base *zap.Logger, executorID string) *TaskExecutor {
    return &TaskExecutor{
        logger: telemetry.NewStructuredLogger(base).
            WithFields(
                zap.String("executor_id", executorID),
                zap.String("component", "executor"),
            ),
    }
}

func (e *TaskExecutor) Execute(ctx context.Context, task *Task) error {
    // All logs include executor_id and component
    logger := e.logger.
        WithTaskID(task.ID).
        WithGuildID(task.GuildID).
        WithContext(ctx)  // Adds trace_id, span_id

    logger.Info("executing task",
        zap.String("wasm_hash", task.WASMHash),
    )

    // Execute...

    logger.Info("task completed",
        telemetry.DurationMS(duration),
        telemetry.Status("success"),
    )
    return nil
}
```

---

## Layer-Specific Logging

### P2P Network Layer

**Common Fields:**
- `peer_id` - Remote peer identifier
- `message_type` - P2P message type
- `protocol` - Protocol version
- `operation` - Connection, DHT, gossip, etc.

**Example:**

```go
import "github.com/zerostate/libs/telemetry"

func (n *Node) HandleMessage(ctx context.Context, peerID peer.ID, msg *Message) error {
    telemetry.InfoCtx(ctx, n.logger, "received p2p message",
        telemetry.PeerID(peerID.String()),
        telemetry.MessageType(msg.Type),
        telemetry.SizeBytes(int64(len(msg.Data))),
    )

    // Process message...

    if err != nil {
        telemetry.ErrorCtx(ctx, n.logger, "message processing failed",
            telemetry.PeerID(peerID.String()),
            zap.Error(err),
        )
        return err
    }

    return nil
}
```

**LogQL Queries:**

```logql
# All P2P logs
{job="docker"} |~ "p2p|peer|connection"

# Connection errors
{job="docker", level="error"} |= "connection"

# Messages by peer
{job="docker"} | json | peer_id="12D3KooW..."
```

---

### Guild Formation Layer

**Common Fields:**
- `guild_id` - Guild identifier
- `member_count` - Current members
- `role` - Member role (creator, executor, observer)

**Example:**

```go
func (gm *GuildManager) CreateGuild(ctx context.Context, capabilities []string) (*Guild, error) {
    telemetry.InfoCtx(ctx, gm.logger, "creating guild",
        zap.Strings("capabilities", capabilities),
        zap.Int("max_members", gm.config.MaxMembers),
    )

    guild, err := gm.createGuild(ctx, capabilities)
    if err != nil {
        telemetry.ErrorCtx(ctx, gm.logger, "guild creation failed",
            zap.Error(err),
        )
        return nil, err
    }

    telemetry.InfoCtx(ctx, gm.logger, "guild created",
        telemetry.GuildID(string(guild.ID)),
        telemetry.DurationMS(elapsedMS),
    )

    return guild, nil
}
```

**LogQL Queries:**

```logql
# All guild logs
{job="docker"} | json | guild_id!=""

# Guild lifecycle
{job="docker"} |~ "guild created|guild joined|guild dissolved"

# Guild errors
{job="docker", level="error"} | json | guild_id!=""
```

---

### Execution Layer

**Common Fields:**
- `task_id` - Task identifier
- `wasm_hash` - WASM module hash
- `memory_bytes` - Memory usage
- `duration_ms` - Execution duration
- `exit_code` - WASM exit code

**Example:**

```go
func (r *WASMRunner) Execute(ctx context.Context, taskID string, wasmBytes []byte) (*Result, error) {
    telemetry.InfoCtx(ctx, r.logger, "starting wasm execution",
        telemetry.TaskID(taskID),
        telemetry.SizeBytes(int64(len(wasmBytes))),
    )

    result, err := r.executeWASM(ctx, wasmBytes)
    if err != nil {
        telemetry.ErrorCtx(ctx, r.logger, "wasm execution failed",
            telemetry.TaskID(taskID),
            zap.Error(err),
            zap.String("error_type", classifyError(err)),
        )
        return nil, err
    }

    telemetry.InfoCtx(ctx, r.logger, "wasm execution completed",
        telemetry.TaskID(taskID),
        telemetry.DurationMS(result.Duration.Milliseconds()),
        zap.Int64("memory_bytes", int64(result.MemoryUsed)),
        zap.Int("exit_code", int(result.ExitCode)),
    )

    return result, nil
}
```

**LogQL Queries:**

```logql
# All task executions
{job="docker"} | json | task_id!=""

# Failed executions
{job="docker", level="error"} |= "execution failed"

# High memory usage
{job="docker"} | json | memory_bytes > 100000000  # >100MB

# Slow tasks
{job="docker"} | json | duration_ms > 20000  # >20s
```

---

### Payment Layer

**Common Fields:**
- `channel_id` - Payment channel identifier
- `payment_id` - Payment transaction identifier
- `amount` - Payment amount
- `party_a` - Payer peer ID
- `party_b` - Payee peer ID

**Example:**

```go
func (cm *ChannelManager) SendPayment(ctx context.Context, channelID string, amount float64) error {
    telemetry.InfoCtx(ctx, cm.logger, "sending payment",
        telemetry.ChannelID(channelID),
        zap.Float64("amount", amount),
    )

    payment, err := cm.createPayment(ctx, channelID, amount)
    if err != nil {
        telemetry.ErrorCtx(ctx, cm.logger, "payment failed",
            telemetry.ChannelID(channelID),
            zap.Error(err),
        )
        return err
    }

    telemetry.InfoCtx(ctx, cm.logger, "payment sent",
        telemetry.ChannelID(channelID),
        zap.String("payment_id", payment.ID),
        zap.Float64("amount", amount),
    )

    return nil
}
```

**LogQL Queries:**

```logql
# All payment logs
{job="docker"} | json | channel_id!=""

# Failed payments
{job="docker", level="error"} |= "payment"

# Large payments
{job="docker"} | json | amount > 100

# Channel disputes
{job="docker"} |= "dispute"
```

---

## Grafana Logs Explorer

### Dashboard Features

**Log Volume by Service:**
- Bar chart showing logs/min per service
- Identify chatty services

**Error Rate:**
- Time series of errors/min
- Detect error spikes

**Recent Logs:**
- Real-time log stream
- Filter by service, level, trace ID

**Logs by Trace ID:**
- Click trace ID → opens Jaeger UI
- Complete request flow visualization

**Error Hotspots:**
- Table showing top error sources
- Group by service + operation

**Layer-Specific Views:**
- P2P Network logs
- Execution logs
- Payment logs

### LogQL Query Examples

**Basic Filtering:**

```logql
# All logs from bootnode
{service="bootnode"}

# Error logs only
{level="error"}

# Last hour, info level or higher
{job="docker", level=~"info|warn|error"} [1h]
```

**JSON Field Filtering:**

```logql
# Logs with specific task
{job="docker"} | json | task_id="task-abc123"

# High-value payments
{job="docker"} | json | amount > 1000

# Specific peer activity
{job="docker"} | json | peer_id=~"12D3KooW.*"
```

**Aggregations:**

```logql
# Error rate
sum by (service) (rate({level="error"}[5m]))

# Top error sources
topk(10, sum by (service, operation) (count_over_time({level="error"}[1h])))

# Log volume
sum(count_over_time({job="docker"}[1m]))
```

**Trace Correlation:**

```logql
# All logs for a specific trace
{job="docker"} | json | trace_id="4bf92f3577b34da6a3ce929d0e0e4736"

# Failed requests with traces
{level="error"} | json | trace_id!="" | status="failed"
```

---

## Configuration

### Log Levels

**Development:**
```go
telemetry.LogConfig{
    Level: "debug",  // Verbose logging
    Format: "console",  // Human-readable
}
```

**Production:**
```go
telemetry.LogConfig{
    Level: "info",  // Normal logging
    Format: "json",  // Machine-readable for Loki
}
```

**Debugging:**
```go
telemetry.LogConfig{
    Level: "debug",
    EnableCaller: true,  // Add file:line
    EnableStacktrace: true,  // Stack traces on errors
}
```

### Loki Retention

**Default: 7 days** (168 hours)

```yaml
# deployments/loki-config.yaml
limits_config:
  retention_period: 168h  # 7 days
```

**Custom Retention:**

```yaml
# 30 days
retention_period: 720h

# 90 days (requires more storage)
retention_period: 2160h
```

### Performance Tuning

**Promtail Batch Size:**

```yaml
# promtail-config.yaml
clients:
  - url: http://loki:3100/loki/api/v1/push
    batchwait: 1s
    batchsize: 102400  # 100KB
```

**Loki Ingestion Rate:**

```yaml
# loki-config.yaml
limits_config:
  ingestion_rate_mb: 10  # 10 MB/s per tenant
  ingestion_burst_size_mb: 20  # 20 MB burst
```

---

## Best Practices

### DO ✅

- **Use JSON format** in production for Loki parsing
- **Include trace IDs** with `telemetry.InfoCtx()` helpers
- **Use structured fields** instead of string interpolation
- **Add context fields** at logger creation (service, version, environment)
- **Log at appropriate levels** (debug, info, warn, error)
- **Include error types** for better categorization
- **Use consistent field names** across services

### DON'T ❌

- **Don't log sensitive data** (passwords, tokens, PII)
- **Don't use string formatting** in log messages
- **Don't log high-frequency events** at info level (use debug)
- **Don't forget to call logger.Sync()** before exit
- **Don't create new loggers** in hot paths (reuse)
- **Don't log full payloads** (use hashes or sizes)

---

## Troubleshooting

### Issue: Logs Not Appearing in Loki

**Solutions:**

1. **Check Promtail is running:**
```bash
docker logs zs-promtail
```

2. **Check Loki is receiving logs:**
```bash
curl http://localhost:3100/ready
curl "http://localhost:3100/loki/api/v1/label/__name__/values"
```

3. **Verify log format is JSON:**
```go
telemetry.LogConfig{
    Format: "json",  // Not "console"
}
```

4. **Check Promtail scrape config:**
```yaml
# Ensure ZeroState containers are matched
regex: 'zs-.*'
```

### Issue: Trace IDs Not Linking

**Cause:** Context not passed or trace not active

**Solutions:**

```go
// GOOD: Pass context
telemetry.InfoCtx(ctx, logger, "message")

// BAD: No context
logger.Info("message")  // trace_id will be empty

// Verify span is recording
span := trace.SpanFromContext(ctx)
if span.IsRecording() {
    // Trace is active
}
```

### Issue: High Storage Usage

**Solutions:**

1. **Reduce retention:**
```yaml
retention_period: 72h  # 3 days instead of 7
```

2. **Lower log level:**
```go
Level: "info"  // Instead of "debug"
```

3. **Enable compaction:**
```yaml
compactor:
  retention_enabled: true
  compaction_interval: 10m
```

---

## Production Deployment

### Security Considerations

1. **Sanitize Sensitive Fields:**

```go
func sanitizePeerID(id string) string {
    if len(id) > 12 {
        return id[:12] + "..."
    }
    return id
}

logger.Info("peer connected",
    zap.String("peer_id", sanitizePeerID(peerID)),
)
```

2. **Restrict Loki Access:**

```yaml
# docker-compose.yml
loki:
  ports:
    - "127.0.0.1:3100:3100"  # Localhost only
```

3. **Enable Authentication (Production):**

```yaml
# loki-config.yaml
auth_enabled: true
```

### Multi-Tenancy

For multi-environment logging:

```yaml
# Promtail adds tenant header
clients:
  - url: http://loki:3100/loki/api/v1/push
    tenant_id: production  # or staging, development
```

---

## Next Steps

### Sprint 6 Remaining Tasks

- [x] **Task 11-12**: Structured logging with Loki ✅
- [ ] **Task 13-14**: Health check endpoints
- [ ] **Task 15-16**: Integration tests

### Future Enhancements

1. **Log-based Alerting**: Alert on log patterns (e.g., >10 errors/min)
2. **Log Sampling**: Sample debug logs in production (reduce volume)
3. **Log Enrichment**: Add metadata (region, cluster, pod name)
4. **Long-term Storage**: Archive to S3/GCS for compliance
5. **Log Analytics**: Aggregate logs for business intelligence

---

## Support

**Documentation:**
- [Zap Logger](https://github.com/uber-go/zap)
- [Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Promtail](https://grafana.com/docs/loki/latest/clients/promtail/)
- [LogQL](https://grafana.com/docs/loki/latest/logql/)

**Files Created:**
- [libs/telemetry/logger.go](../libs/telemetry/logger.go) - Structured logger with trace correlation
- [deployments/loki-config.yaml](../deployments/loki-config.yaml) - Loki configuration
- [deployments/promtail-config.yaml](../deployments/promtail-config.yaml) - Log collection config
- [deployments/grafana/provisioning/datasources/loki.yaml](../deployments/grafana/provisioning/datasources/loki.yaml) - Loki datasource
- [deployments/grafana/dashboards/logs-explorer.json](../deployments/grafana/dashboards/logs-explorer.json) - Logs dashboard

---

**Generated:** November 7, 2025
**Sprint 6 - Phase 4 Complete** ✅
**Structured Logging: Production-Ready**
