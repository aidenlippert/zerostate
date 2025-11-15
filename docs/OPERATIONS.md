# Ainur Protocol Operations Guide

This guide covers day-to-day operational procedures for the Ainur Protocol including monitoring, maintenance, troubleshooting, and scaling.

## Table of Contents

1. [Daily Operations](#daily-operations)
2. [Service Management](#service-management)
3. [Monitoring Dashboard Usage](#monitoring-dashboard-usage)
4. [Alert Response Procedures](#alert-response-procedures)
5. [Backup and Recovery](#backup-and-recovery)
6. [Scaling Guidelines](#scaling-guidelines)
7. [Performance Tuning](#performance-tuning)
8. [Security Best Practices](#security-best-practices)
9. [Incident Response](#incident-response)
10. [Maintenance Procedures](#maintenance-procedures)

## Daily Operations

### Morning Health Check Routine

#### 1. System Status Verification
```bash
# Check all service statuses
curl https://zerostate-api.fly.dev/health/detailed | jq '.'

# Verify Fly.io app status
fly status --app zerostate-api

# Check database health
fly postgres db status --app ainur-db

# Verify frontend availability
curl -I https://your-frontend-domain.vercel.app
```

#### 2. Metrics Review
```bash
# Quick metrics summary
curl https://zerostate-api.fly.dev/metrics/summary | jq '.'

# Key metrics to check:
# - API response time < 100ms average
# - Database connection count < 80% of max
# - Memory usage < 80%
# - Error rate < 1%
# - Active agent count
```

#### 3. Log Review
```bash
# Check for errors in last 24 hours
fly logs --app zerostate-api | grep -E "(ERROR|FATAL|PANIC)" | tail -20

# Database error logs
fly postgres logs --app ainur-db | grep ERROR | tail -10

# Monitor for common issues:
# - Database connection timeouts
# - R2 storage failures
# - WASM execution timeouts
# - Authentication failures
```

### Evening Summary Report

#### Automated Daily Report Script
```bash
#!/bin/bash
# scripts/daily-report.sh

echo "Ainur Protocol Daily Report - $(date)"
echo "============================================"

# Service uptime
echo "🟢 Service Uptime:"
curl -s https://zerostate-api.fly.dev/health | jq -r '.uptime'

# Daily statistics
echo "📊 Daily Statistics:"
METRICS=$(curl -s https://zerostate-api.fly.dev/metrics/summary)
echo "  • Total Requests: $(echo $METRICS | jq -r '.http.requests_total')"
echo "  • Average Response Time: $(echo $METRICS | jq -r '.http.average_response_time')"
echo "  • Active Agents: $(echo $METRICS | jq -r '.agents.online')"
echo "  • Tasks Completed: $(echo $METRICS | jq -r '.agents.tasks_completed_today')"

# Error summary
echo "❌ Errors (Last 24h):"
ERROR_COUNT=$(fly logs --app zerostate-api --since 24h | grep -c ERROR || echo "0")
echo "  • Total Errors: $ERROR_COUNT"

# Database health
echo "🗄️ Database Health:"
DB_CONNECTIONS=$(fly postgres db list --app ainur-db --json | jq -r '.[0].current_connections')
echo "  • Active Connections: $DB_CONNECTIONS"

echo "============================================"
```

Run daily report:
```bash
chmod +x scripts/daily-report.sh
./scripts/daily-report.sh
```

## Service Management

### Starting and Stopping Services

#### API Server Management
```bash
# Start/stop Fly.io services
fly start --app zerostate-api
fly stop --app zerostate-api

# Scale to zero (emergency stop)
fly scale count 0 --app zerostate-api

# Scale back up
fly scale count 2 --app zerostate-api

# Rolling restart (zero downtime)
fly restart --app zerostate-api
```

#### Database Management
```bash
# Database status
fly postgres db status --app ainur-db

# Restart database (downtime expected)
fly restart --app ainur-db

# Scale database
fly scale vm dedicated-cpu-2x --app ainur-db
fly scale memory 4096 --app ainur-db
```

#### Blockchain Node Management
```bash
# Check node status
systemctl status ainur-node

# Start/stop/restart
sudo systemctl start ainur-node
sudo systemctl stop ainur-node
sudo systemctl restart ainur-node

# View real-time logs
journalctl -u ainur-node -f

# Check sync status
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_health","params":[]}' \
     http://localhost:9933
```

### Service Dependencies

#### Dependency Chain
1. **Database** (PostgreSQL) - Required for API
2. **Storage** (R2) - Required for agent binaries
3. **Blockchain** - Required for on-chain operations
4. **API Server** - Required for frontend
5. **Frontend** - User interface

#### Graceful Shutdown Order
```bash
# 1. Frontend (redirect to maintenance page)
# 2. API Server (finish processing requests)
fly restart --app zerostate-api --wait-timeout 60

# 3. Background workers (complete current tasks)
# 4. Database (last to ensure data integrity)
```

### Configuration Management

#### Environment Variables
```bash
# List current secrets
fly secrets list --app zerostate-api

# Update configuration
fly secrets set LOG_LEVEL=info --app zerostate-api

# Bulk update from file
fly secrets import --app zerostate-api < production-secrets.env
```

#### Feature Flags
```bash
# Enable/disable features via environment
fly secrets set ENABLE_AUCTION_SYSTEM=true --app zerostate-api
fly secrets set MAINTENANCE_MODE=false --app zerostate-api
```

## Monitoring Dashboard Usage

### Grafana Dashboard

#### Key Dashboards

1. **System Overview Dashboard**
   - URL: `http://grafana.your-domain.com/d/system-overview`
   - Metrics: CPU, memory, network, disk usage
   - Alerts: System-level performance issues

2. **API Performance Dashboard**
   - URL: `http://grafana.your-domain.com/d/api-performance`
   - Metrics: Request rate, response time, error rate
   - SLA tracking: 99.9% uptime, <100ms response time

3. **Database Dashboard**
   - URL: `http://grafana.your-domain.com/d/database`
   - Metrics: Connection count, query performance, lock wait time
   - Capacity planning: Connection pool utilization

4. **Blockchain Dashboard**
   - URL: `http://grafana.your-domain.com/d/blockchain`
   - Metrics: Block height, peer count, finalization lag
   - Network health indicators

#### Alert Configuration
```yaml
# grafana/alerts/api-alerts.yml
groups:
  - name: api_alerts
    rules:
      - alert: HighResponseTime
        expr: http_request_duration_seconds{quantile="0.95"} > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "API response time is high"
          description: "95th percentile response time is {{ $value }}s"

      - alert: ErrorRateHigh
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.01
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
```

### Prometheus Queries

#### Essential Queries
```promql
# API request rate
rate(http_requests_total[5m])

# Error rate percentage
rate(http_requests_total{status=~"5.."}[5m]) / rate(http_requests_total[5m]) * 100

# Database connection usage
postgres_connections_active / postgres_connections_max * 100

# Memory usage
process_resident_memory_bytes / 1024 / 1024

# Substrate block height
substrate_block_height_finalized
```

## Alert Response Procedures

### Severity Levels

#### Critical (P1) - Immediate Response Required
- **Service Down**: API returns 5xx errors
- **Database Unavailable**: Connection failures
- **Security Breach**: Unauthorized access detected
- **Data Loss**: Database corruption or missing data

**Response Time**: 15 minutes
**Escalation**: On-call engineer immediately

#### High (P2) - Response within 1 hour
- **Performance Degradation**: Response time > 500ms
- **High Error Rate**: >5% error rate sustained
- **Blockchain Sync Issues**: Node behind >100 blocks
- **Storage Issues**: R2 upload failures

#### Medium (P3) - Response within 4 hours
- **Minor Performance Issues**: Response time > 100ms
- **Low Error Rate**: 1-5% error rate
- **Capacity Warnings**: >80% resource utilization

### Alert Response Playbooks

#### API Server Down (Critical)
```bash
# Step 1: Verify the issue
curl -f https://zerostate-api.fly.dev/health || echo "CONFIRMED: API DOWN"

# Step 2: Check Fly.io status
fly status --app zerostate-api

# Step 3: Check logs for errors
fly logs --app zerostate-api | tail -50

# Step 4: Quick fixes
# Option A: Restart the service
fly restart --app zerostate-api

# Option B: Scale horizontally if one instance is problematic
fly scale count 3 --app zerostate-api

# Step 5: Verify recovery
sleep 30
curl -f https://zerostate-api.fly.dev/health

# Step 6: If not recovered, rollback to previous version
fly releases --app zerostate-api
fly rollback v$(fly releases --app zerostate-api | head -2 | tail -1 | awk '{print $1}') --app zerostate-api
```

#### Database Connection Issues (Critical)
```bash
# Step 1: Test database connectivity
psql $DATABASE_URL -c "SELECT 1;" || echo "DB CONNECTION FAILED"

# Step 2: Check database status
fly postgres db status --app ainur-db

# Step 3: Check connection pool
psql $DATABASE_URL -c "SELECT count(*) FROM pg_stat_activity;"

# Step 4: Kill hanging connections if needed
psql $DATABASE_URL -c "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle in transaction'
AND query_start < NOW() - INTERVAL '1 hour';"

# Step 5: Restart database if necessary (DOWNTIME)
fly restart --app ainur-db
```

#### High Response Time (High Priority)
```bash
# Step 1: Check current metrics
curl https://zerostate-api.fly.dev/metrics/summary | jq '.http.average_response_time'

# Step 2: Identify bottlenecks
# Check database slow queries
psql $DATABASE_URL -c "
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;"

# Step 3: Scale horizontally
fly scale count +1 --app zerostate-api

# Step 4: Monitor improvement
watch -n 5 'curl -s https://zerostate-api.fly.dev/metrics/summary | jq ".http.average_response_time"'
```

### Communication Templates

#### Incident Notification
```
INCIDENT ALERT - Ainur Protocol

Severity: [CRITICAL/HIGH/MEDIUM]
Service: [API/Database/Blockchain/Frontend]
Issue: [Brief description]
Started: [Timestamp]
Impact: [User impact description]
Status: [INVESTIGATING/IDENTIFIED/FIXING/RESOLVED]

Actions taken:
- [Action 1]
- [Action 2]

Next update: [Time]
```

#### Resolution Notification
```
RESOLVED - Ainur Protocol

The incident affecting [service] has been resolved.

Duration: [X minutes]
Root cause: [Explanation]
Resolution: [What was done]
Prevention: [Steps to prevent recurrence]

Post-mortem will be published within 24 hours.
```

## Backup and Recovery

### Database Backup Strategy

#### Automated Daily Backups
```bash
#!/bin/bash
# scripts/backup-database.sh

BACKUP_DIR="/backups/ainur"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="ainur_backup_${DATE}.sql"

# Create backup directory
mkdir -p $BACKUP_DIR

# Create backup
echo "Creating backup: $BACKUP_FILE"
pg_dump $DATABASE_URL > "$BACKUP_DIR/$BACKUP_FILE"

# Compress backup
gzip "$BACKUP_DIR/$BACKUP_FILE"

# Upload to R2 for long-term storage
aws s3 cp "$BACKUP_DIR/${BACKUP_FILE}.gz" \
    s3://ainur-backups/database/ \
    --endpoint-url $R2_ENDPOINT

# Clean up old local backups (keep last 7 days)
find $BACKUP_DIR -name "*.gz" -mtime +7 -delete

echo "Backup completed: ${BACKUP_FILE}.gz"
```

#### Setup Automated Backups
```bash
# Add to crontab (run at 2 AM daily)
crontab -e
```
```cron
0 2 * * * /home/ainur/scripts/backup-database.sh >> /var/log/ainur-backup.log 2>&1
```

#### Point-in-Time Recovery
```bash
# Restore from specific backup
RESTORE_DATE="20241114_020000"
gunzip /backups/ainur/ainur_backup_${RESTORE_DATE}.sql.gz
psql $DATABASE_URL < /backups/ainur/ainur_backup_${RESTORE_DATE}.sql
```

### Application State Backup

#### Configuration Backup
```bash
# Backup Fly.io configuration
fly config save --app zerostate-api > fly-config-backup.toml

# Backup secrets (names only - values are encrypted)
fly secrets list --app zerostate-api > secrets-list-backup.txt

# Backup environment variables
fly secrets export --app zerostate-api > secrets-backup.env
```

#### WASM Binary Backup
```bash
# R2 bucket backup (redundant storage)
aws s3 sync s3://ainur-agents-prod s3://ainur-agents-backup \
    --endpoint-url $R2_ENDPOINT
```

### Disaster Recovery Plan

#### Recovery Time Objectives (RTO)
- **Database**: 15 minutes (from backup)
- **API Service**: 5 minutes (redeploy)
- **Frontend**: 2 minutes (Vercel auto-recovery)
- **Blockchain**: 30 minutes (resync from network)

#### Recovery Procedures

1. **Complete Infrastructure Loss**
```bash
# Step 1: Recreate Fly.io apps
fly apps create zerostate-api-recovery

# Step 2: Restore database
fly postgres create --name ainur-db-recovery
pg_restore --data-only $LATEST_BACKUP

# Step 3: Restore secrets
fly secrets import < secrets-backup.env --app zerostate-api-recovery

# Step 4: Deploy application
fly deploy --app zerostate-api-recovery

# Step 5: Update DNS
# Point domain to recovery app
```

2. **Data Corruption Recovery**
```bash
# Restore from last known good backup
GOOD_BACKUP="/backups/ainur/ainur_backup_20241113_020000.sql.gz"

# Create new database
createdb ainur_recovery

# Restore data
gunzip -c $GOOD_BACKUP | psql postgresql://user:pass@host/ainur_recovery

# Validate data integrity
psql postgresql://user:pass@host/ainur_recovery -c "
SELECT COUNT(*) FROM users;
SELECT COUNT(*) FROM agents;
SELECT COUNT(*) FROM tasks;
"

# Switch to recovered database
fly secrets set DATABASE_URL="new_recovery_url" --app zerostate-api
```

## Scaling Guidelines

### Horizontal Scaling

#### API Server Scaling
```bash
# Monitor request load
curl -s https://zerostate-api.fly.dev/metrics | grep http_requests_total

# Auto-scaling thresholds:
# Scale up when: CPU > 70% OR Response time > 100ms
# Scale down when: CPU < 30% AND Response time < 50ms

# Manual scaling
fly scale count 5 --app zerostate-api  # Scale to 5 instances
fly scale count +2 --app zerostate-api # Add 2 instances
fly scale count -1 --app zerostate-api # Remove 1 instance
```

#### Database Scaling
```bash
# Vertical scaling (more resources per instance)
fly scale vm dedicated-cpu-4x --app ainur-db
fly scale memory 8192 --app ainur-db

# Read replicas for read-heavy workloads
fly postgres create --name ainur-db-read-replica --replica-of ainur-db
```

### Vertical Scaling

#### Performance Monitoring for Scaling Decisions
```bash
# CPU utilization
fly metrics --app zerostate-api | grep cpu

# Memory usage
fly metrics --app zerostate-api | grep memory

# Database performance
psql $DATABASE_URL -c "
SELECT
  schemaname,
  tablename,
  seq_scan,
  seq_tup_read,
  idx_scan,
  idx_tup_fetch
FROM pg_stat_user_tables
ORDER BY seq_tup_read DESC;"
```

#### Scaling Triggers
```yaml
# Auto-scaling configuration
scaling_rules:
  scale_up:
    - metric: cpu_usage
      threshold: 70
      duration: 5m
    - metric: response_time_95th
      threshold: 100ms
      duration: 3m

  scale_down:
    - metric: cpu_usage
      threshold: 30
      duration: 15m
    - metric: response_time_95th
      threshold: 50ms
      duration: 10m
```

### Capacity Planning

#### Growth Projections
```bash
# Weekly growth analysis
echo "Weekly Metrics Analysis:"
echo "========================"

# User growth
USER_GROWTH=$(psql $DATABASE_URL -t -c "
SELECT
  COUNT(*) as total_users,
  COUNT(CASE WHEN created_at > NOW() - INTERVAL '7 days' THEN 1 END) as new_users_7d,
  COUNT(CASE WHEN created_at > NOW() - INTERVAL '30 days' THEN 1 END) as new_users_30d
FROM users;")

echo "User Growth: $USER_GROWTH"

# Agent registration growth
AGENT_GROWTH=$(psql $DATABASE_URL -t -c "
SELECT
  COUNT(*) as total_agents,
  COUNT(CASE WHEN created_at > NOW() - INTERVAL '7 days' THEN 1 END) as new_agents_7d
FROM agents;")

echo "Agent Growth: $AGENT_GROWTH"

# Task volume growth
TASK_GROWTH=$(psql $DATABASE_URL -t -c "
SELECT
  COUNT(*) as total_tasks,
  COUNT(CASE WHEN created_at > NOW() - INTERVAL '7 days' THEN 1 END) as tasks_7d,
  COUNT(CASE WHEN created_at > NOW() - INTERVAL '1 day' THEN 1 END) as tasks_1d
FROM tasks;")

echo "Task Growth: $TASK_GROWTH"
```

## Performance Tuning

### Database Optimization

#### Query Performance Analysis
```sql
-- Find slow queries
SELECT
  query,
  calls,
  total_time,
  mean_time,
  stddev_time,
  (total_time / sum(total_time) OVER ()) * 100 AS percentage
FROM pg_stat_statements
WHERE calls > 10
ORDER BY mean_time DESC
LIMIT 20;

-- Index usage analysis
SELECT
  schemaname,
  tablename,
  indexname,
  idx_scan,
  idx_tup_read,
  idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_scan DESC;

-- Table size analysis
SELECT
  schemaname,
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size,
  pg_total_relation_size(schemaname||'.'||tablename) as size_bytes
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY size_bytes DESC;
```

#### Database Configuration Tuning
```sql
-- Connection pooling settings
ALTER SYSTEM SET max_connections = '200';
ALTER SYSTEM SET shared_buffers = '256MB';
ALTER SYSTEM SET effective_cache_size = '1GB';
ALTER SYSTEM SET work_mem = '4MB';

-- Logging for performance analysis
ALTER SYSTEM SET log_min_duration_statement = '1000';  -- Log queries > 1s
ALTER SYSTEM SET log_checkpoints = 'on';
ALTER SYSTEM SET log_connections = 'on';

SELECT pg_reload_conf();
```

### API Server Optimization

#### Go Application Tuning
```bash
# Environment variables for performance
export GOGC=100                    # Garbage collection target
export GOMAXPROCS=2               # Number of CPU cores to use
export GODEBUG=gctrace=1          # Enable GC tracing

# Profile the application
go tool pprof http://localhost:8080/debug/pprof/profile?seconds=30
```

#### Connection Pool Optimization
```go
// Database connection pool settings
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(1 * time.Minute)
```

### Blockchain Node Optimization

#### Substrate Node Tuning
```bash
# Optimized startup parameters
./target/release/solochain-template-node \
  --base-path /var/lib/ainur \
  --chain ainur-testnet-raw.json \
  --validator \
  --port 30333 \
  --rpc-port 9933 \
  --ws-port 9944 \
  --rpc-cors all \
  --rpc-methods Safe \
  --ws-max-connections 1000 \
  --in-peers 50 \
  --out-peers 50 \
  --db-cache 1024 \  # Database cache size in MB
  --wasm-execution Compiled \
  --execution wasm
```

#### Storage Optimization
```bash
# Use faster storage for blockchain data
# SSD preferred over HDD
# NVMe preferred over SATA SSD

# Database pruning (if running archive node)
./target/release/solochain-template-node \
  --pruning 1000 \  # Keep last 1000 blocks
  --database RocksDb
```

## Security Best Practices

### Access Control

#### API Security
```bash
# Rate limiting per endpoint
curl -H "X-RateLimit-Limit: 100" https://zerostate-api.fly.dev/api/v1/agents

# JWT token validation
# Tokens expire after 24 hours
# Use strong JWT secrets (min 64 characters)

# Input validation
# All inputs validated against strict schemas
# SQL injection prevention via parameterized queries
```

#### Database Security
```sql
-- Create read-only user for monitoring
CREATE USER monitoring WITH PASSWORD 'strong_password';
GRANT CONNECT ON DATABASE ainur TO monitoring;
GRANT USAGE ON SCHEMA public TO monitoring;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO monitoring;

-- Revoke unnecessary permissions
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
```

### Network Security

#### Firewall Configuration
```bash
# Fly.io network isolation
# Only required ports exposed: 8080 (HTTP), 443 (HTTPS)

# Database access restricted to Fly.io internal network
# No external database connections allowed

# Blockchain node ports:
# 30333 (P2P) - Required for network participation
# 9933 (HTTP RPC) - Restricted to internal network
# 9944 (WebSocket RPC) - Restricted to internal network
```

### Security Monitoring

#### Automated Security Checks
```bash
#!/bin/bash
# scripts/security-check.sh

echo "Security Check Report - $(date)"
echo "================================"

# Check for unauthorized API access
FAILED_AUTH=$(fly logs --app zerostate-api --since 1h | grep -c "401 Unauthorized" || echo "0")
echo "Failed auth attempts (1h): $FAILED_AUTH"

# Check for SQL injection attempts
SQL_INJECTION=$(fly logs --app zerostate-api --since 1h | grep -c -i "union\|select\|drop\|insert" || echo "0")
echo "Potential SQL injection attempts: $SQL_INJECTION"

# Check unusual traffic patterns
HIGH_RATE_IPS=$(fly logs --app zerostate-api --since 1h | \
  grep -oE '[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+' | \
  sort | uniq -c | sort -nr | head -5)
echo "High-traffic IPs:"
echo "$HIGH_RATE_IPS"

# Check for admin endpoint access
ADMIN_ACCESS=$(fly logs --app zerostate-api --since 1h | grep -c "/admin\|/debug" || echo "0")
echo "Admin endpoint access attempts: $ADMIN_ACCESS"

echo "================================"
```

### Compliance and Auditing

#### Audit Log Configuration
```go
// Enable audit logging for sensitive operations
type AuditEvent struct {
    UserID    string    `json:"user_id"`
    Action    string    `json:"action"`
    Resource  string    `json:"resource"`
    Timestamp time.Time `json:"timestamp"`
    IP        string    `json:"ip_address"`
    UserAgent string    `json:"user_agent"`
}

// Log security-relevant events:
// - User registration/login
// - Agent registration/updates
// - Task submissions
// - Payment transactions
// - Admin actions
```

## Incident Response

### Incident Classification

#### Severity Matrix
```
Critical (P1): Service unavailable, data loss, security breach
High (P2):     Performance degradation, partial service loss
Medium (P3):   Minor issues, limited user impact
Low (P4):      Cosmetic issues, no user impact
```

### Incident Response Team

#### Roles and Responsibilities
- **Incident Commander**: Coordinates response, makes decisions
- **Technical Lead**: Investigates technical issues, implements fixes
- **Communications Lead**: Updates stakeholders, manages communications
- **Subject Matter Expert**: Domain expertise (database, blockchain, etc.)

### Post-Incident Review

#### Post-Mortem Template
```markdown
# Incident Post-Mortem: [Incident Title]

## Summary
- **Date**: [YYYY-MM-DD]
- **Duration**: [X hours Y minutes]
- **Severity**: [P1/P2/P3/P4]
- **Impact**: [Description of user impact]

## Timeline
- [Time]: Issue first detected
- [Time]: Incident response began
- [Time]: Root cause identified
- [Time]: Fix implemented
- [Time]: Service fully restored

## Root Cause
[Detailed explanation of what caused the incident]

## What Went Well
- [Things that worked well during the response]

## What Went Wrong
- [Things that could have been handled better]

## Action Items
- [ ] [Specific action item] - Owner: [Name] - Due: [Date]
- [ ] [Another action item] - Owner: [Name] - Due: [Date]

## Lessons Learned
[Key takeaways and learnings from this incident]
```

## Maintenance Procedures

### Scheduled Maintenance

#### Maintenance Windows
- **Preferred Time**: Sunday 02:00-06:00 UTC (lowest usage)
- **Duration**: Maximum 4 hours for major updates
- **Notification**: 72 hours advance notice for planned maintenance

#### Pre-Maintenance Checklist
```bash
# 1. Schedule maintenance window
# 2. Notify users via dashboard/email
# 3. Create fresh backups
./scripts/backup-database.sh

# 4. Test maintenance procedures in staging
# 5. Prepare rollback plan
# 6. Verify monitoring systems are operational
```

### Regular Maintenance Tasks

#### Weekly Tasks
```bash
# 1. Database maintenance
psql $DATABASE_URL -c "VACUUM ANALYZE;"
psql $DATABASE_URL -c "REINDEX DATABASE ainur;"

# 2. Log rotation
find /var/log -name "*.log" -type f -mtime +30 -delete

# 3. Certificate renewal (if applicable)
certbot renew --quiet

# 4. Security updates
sudo apt update && sudo apt upgrade -y

# 5. Backup verification
./scripts/test-backup-restore.sh
```

#### Monthly Tasks
```bash
# 1. Capacity review
./scripts/capacity-analysis.sh

# 2. Performance review
./scripts/performance-report.sh

# 3. Security audit
./scripts/security-check.sh

# 4. Dependency updates
go mod update
cargo update
npm audit fix

# 5. Cost optimization review
fly billing show
```

### Emergency Maintenance

#### Unplanned Maintenance Process
```bash
# 1. Assess urgency (security vs. performance)
# 2. Enable maintenance mode if needed
fly secrets set MAINTENANCE_MODE=true --app zerostate-api

# 3. Perform emergency fix
# 4. Test fix in staging first if possible
# 5. Apply fix to production
# 6. Verify fix and disable maintenance mode
fly secrets set MAINTENANCE_MODE=false --app zerostate-api

# 7. Post-incident communication
```

---

For deployment procedures, refer to the [Deployment Guide](DEPLOYMENT.md). For development workflows, see the [Development Guide](DEVELOPMENT.md).