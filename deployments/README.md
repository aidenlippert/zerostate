# ZeroState Deployment & Monitoring

Quick start guide for deploying ZeroState with full observability stack.

---

## 🚀 Quick Start (3 Steps)

### 1. Start Services

```bash
cd deployments/
docker-compose up -d
```

### 2. Access Dashboards

- **Grafana**: http://localhost:3000 (admin/admin)
- **Prometheus**: http://localhost:9090
- **Jaeger**: http://localhost:16686 (future)

### 3. Verify Metrics

```bash
# Check Prometheus targets
curl http://localhost:9090/api/v1/targets | jq

# Check metrics endpoint
curl http://localhost:8080/metrics | grep -E "p2p_|execution_|economic_"

# Check alerts
curl http://localhost:9090/api/v1/rules | jq
```

---

## 📊 Available Dashboards

1. **Network Overview** - P2P health, connections, bandwidth, DHT
2. **Execution Performance** - Tasks, guilds, WASM runtime, receipts
3. **Economic Activity** - TVL, payments, channels, reputation
4. **System Health** - Overall status, alerts, component health

All dashboards are **auto-provisioned** on startup - no manual import needed!

---

## 🚨 Alert Rules (25+)

- **Network**: Connectivity, bandwidth, message delivery
- **Execution**: Task failures, timeouts, memory usage
- **Economic**: TVL drops, blacklists, payment failures
- **System**: Service downtime, latency, error rates
- **SLA**: Availability, task duration, payment success

See [prometheus-alerts.yml](prometheus-alerts.yml) for details.

---

## 📚 Documentation

- **[Monitoring Guide](MONITORING_GUIDE.md)** - Comprehensive guide (7,500+ words)
- **[Sprint 6 Phase 2 Summary](../docs/SPRINT_6_PHASE2_COMPLETE.md)** - Implementation details
- **[Architecture](../docs/ARCHITECTURE.md)** - System design

---

## 🔧 Common Commands

```bash
# View logs
docker-compose logs -f grafana
docker-compose logs -f prometheus

# Restart services
docker-compose restart prometheus grafana

# Reload Prometheus config (zero downtime)
curl -X POST http://localhost:9090/-/reload

# Stop all services
docker-compose down

# Remove volumes (clean slate)
docker-compose down -v
```

---

## 🎯 Metrics Summary

- **90+ metrics** across P2P, Execution, and Economic layers
- **53 dashboard panels** in 4 dashboards
- **25+ alert rules** with smart thresholds
- **Production-ready** monitoring stack

---

## 📁 Directory Structure

```
deployments/
├── docker-compose.yml          # Main orchestration
├── prometheus.yml              # Prometheus config
├── prometheus-alerts.yml       # Alert rules
├── otel-collector-config.yaml  # OpenTelemetry config
├── MONITORING_GUIDE.md         # Detailed documentation
├── README.md                   # This file
├── grafana/
│   ├── provisioning/
│   │   ├── datasources/        # Auto-configured datasources
│   │   └── dashboards/         # Auto-provisioning config
│   └── dashboards/
│       ├── network-overview.json
│       ├── execution-performance.json
│       ├── economic-activity.json
│       └── system-health.json
└── k8s/                        # Kubernetes manifests (future)
```

---

## 🐛 Troubleshooting

### Dashboard shows "No Data"

1. Check Prometheus targets: http://localhost:9090/targets
2. Verify metrics endpoint: `curl http://localhost:8080/metrics`
3. Check Grafana datasource: Grafana → Configuration → Data Sources → Test

### Alerts not firing

1. Validate alert rules: `docker exec zs-prometheus promtool check rules /etc/prometheus/prometheus-alerts.yml`
2. Check Prometheus rules: http://localhost:9090/rules
3. Reload config: `curl -X POST http://localhost:9090/-/reload`

### High memory usage

1. Reduce retention: Add `--storage.tsdb.retention.time=7d` to Prometheus
2. Increase scrape interval: Change `scrape_interval: 30s` in prometheus.yml
3. Limit dashboard time ranges: Use "Last 1h" instead of "Last 24h"

See [MONITORING_GUIDE.md](MONITORING_GUIDE.md) for detailed troubleshooting.

---

## 🔐 Production Deployment

### Security Checklist

- [ ] Change Grafana admin password
- [ ] Enable HTTPS (reverse proxy + Let's Encrypt)
- [ ] Disable Prometheus admin API (`--web.enable-admin-api=false`)
- [ ] Set up Alertmanager (Slack/PagerDuty notifications)
- [ ] Configure backup strategy for dashboards
- [ ] Enable authentication for Prometheus
- [ ] Use secrets management for credentials

See [MONITORING_GUIDE.md - Production Deployment](MONITORING_GUIDE.md#production-deployment) for details.

---

## 🎉 Sprint 6 - Phase 2 Complete

✅ 4 production-ready Grafana dashboards
✅ 25+ Prometheus alert rules
✅ 90+ instrumented metrics
✅ Comprehensive documentation
✅ Auto-provisioned deployment

**Next**: Phase 3 - Distributed Tracing (OpenTelemetry + Jaeger)

---

**Last Updated:** November 7, 2025
**Version:** 1.0.0 (Sprint 6 Phase 2)
