# Ainur Protocol Deployment Guide

This guide covers deploying the complete Ainur Protocol stack to production, including the Substrate blockchain (chain-v2), orchestrator API, and monitoring infrastructure.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Database Setup](#database-setup)
4. [Storage Configuration](#storage-configuration)
5. [Blockchain Deployment](#blockchain-deployment)
6. [Orchestrator API Deployment](#orchestrator-api-deployment)
7. [Monitoring Setup](#monitoring-setup)
8. [Frontend Deployment](#frontend-deployment)
9. [Production Deployment](#production-deployment)
10. [Health Checks](#health-checks)
11. [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software Versions

#### Local Development
- **Rust**: 1.75.0+ with nightly toolchain
- **Go**: 1.21+
- **Node.js**: 18.0+ with npm/yarn
- **PostgreSQL**: 14+
- **Git**: 2.30+

#### System Dependencies
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install -y build-essential curl git clang libssl-dev llvm libudev-dev pkg-config

# macOS
brew install openssl pkg-config
```

#### Rust Setup for Substrate
```bash
# Install rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
source ~/.cargo/env

# Configure toolchain
rustup default stable
rustup update
rustup update nightly
rustup target add wasm32-unknown-unknown --toolchain nightly

# Install substrate dependencies
cargo install substrate-contracts-node
```

### Cloud Service Accounts

1. **Fly.io Account**: For API deployment
   ```bash
   curl -L https://fly.io/install.sh | sh
   fly auth login
   ```

2. **Cloudflare R2**: For WASM binary storage
   - R2 Access Key ID and Secret
   - Custom domain (optional)

3. **PostgreSQL Database**:
   - Fly.io PostgreSQL or
   - AWS RDS PostgreSQL or
   - Google Cloud SQL

4. **Vercel Account**: For frontend deployment
   ```bash
   npm i -g vercel
   vercel login
   ```

## Environment Setup

### 1. Clone Repository
```bash
git clone https://github.com/aidenlippert/zerostate.git
cd zerostate
```

### 2. Environment Configuration

Create environment files from templates:

#### Development Environment (.env.development)
```bash
# Database
DATABASE_URL="postgresql://username:password@localhost:5432/ainur_dev"
DB_HOST="localhost"
DB_PORT="5432"
DB_NAME="ainur_dev"
DB_USER="username"
DB_PASSWORD="password"
DB_SSL_MODE="disable"

# JWT Configuration
JWT_SECRET="your-super-secret-jwt-key-change-this-in-production"
JWT_EXPIRES_IN="24h"

# Server Configuration
HOST="0.0.0.0"
PORT="8080"
LOG_LEVEL="debug"
ENABLE_CORS="true"
ENABLE_METRICS="true"
ENABLE_TRACING="true"

# Orchestrator
ORCHESTRATOR_WORKERS="5"
MAX_TASK_DURATION="30m"

# Storage (Cloudflare R2)
R2_ENDPOINT="https://your-account-id.r2.cloudflarestorage.com"
R2_ACCESS_KEY_ID="your-r2-access-key"
R2_SECRET_ACCESS_KEY="your-r2-secret-key"
R2_BUCKET_NAME="ainur-agents"
R2_REGION="auto"

# LLM APIs
GROQ_API_KEY="your-groq-api-key"
GEMINI_API_KEY="your-gemini-api-key"  # Optional

# P2P Configuration
P2P_PORT="9000"
P2P_BOOTSTRAP_NODES=""

# Blockchain RPC
SUBSTRATE_RPC_URL="ws://localhost:9944"
SUBSTRATE_HTTP_URL="http://localhost:9933"

# Monitoring
METRICS_PORT="9090"
PROMETHEUS_URL="http://localhost:9090"
GRAFANA_URL="http://localhost:3000"
```

#### Production Environment (.env.production)
```bash
# Database (Fly.io PostgreSQL)
DATABASE_URL="postgresql://postgres:password@top2.nearest.of.your-app-db.internal:5432/your_app?sslmode=disable"

# JWT (Generate strong secret)
JWT_SECRET="$(openssl rand -base64 64)"
JWT_EXPIRES_IN="24h"

# Server
HOST="0.0.0.0"
PORT="8080"
LOG_LEVEL="info"
ENABLE_CORS="true"
ENABLE_METRICS="true"
ENABLE_TRACING="true"

# Production R2
R2_ENDPOINT="https://your-account-id.r2.cloudflarestorage.com"
R2_ACCESS_KEY_ID="your-production-access-key"
R2_SECRET_ACCESS_KEY="your-production-secret-key"
R2_BUCKET_NAME="ainur-agents-prod"

# Production LLM APIs
GROQ_API_KEY="your-production-groq-key"

# Substrate Production
SUBSTRATE_RPC_URL="wss://your-substrate-node.com:9944"
SUBSTRATE_HTTP_URL="https://your-substrate-node.com:9933"
```

### 3. Secrets Management

**For Fly.io (Production):**
```bash
# Set all secrets at once
fly secrets set \
  DATABASE_URL="$DATABASE_URL" \
  JWT_SECRET="$(openssl rand -base64 64)" \
  R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID" \
  R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY" \
  R2_ENDPOINT="$R2_ENDPOINT" \
  R2_BUCKET_NAME="$R2_BUCKET_NAME" \
  GROQ_API_KEY="$GROQ_API_KEY" \
  --app zerostate-api
```

**For Local Development:**
```bash
cp .env.example .env
# Edit .env with your local configuration
```

## Database Setup

### 1. Local PostgreSQL Setup

#### Installation
```bash
# Ubuntu/Debian
sudo apt install postgresql postgresql-contrib

# macOS
brew install postgresql
brew services start postgresql

# Create database
sudo -u postgres createdb ainur_dev
sudo -u postgres createuser --interactive username
```

#### Schema Setup
```bash
# Run migrations
cd chain-v2
cargo build --release
./target/release/solochain-template-node --dev --tmp &
sleep 10
kill %1

# The database schema is automatically created by the Substrate runtime
```

### 2. Production Database (Fly.io)

#### Create PostgreSQL Instance
```bash
# Create database app
fly postgres create --name ainur-db --region sjc

# Connect API to database
fly postgres attach --app zerostate-api ainur-db
```

#### Manual Database Setup
```sql
-- Connect to database
psql $DATABASE_URL

-- Create tables for orchestrator
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    did VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    capabilities TEXT[],
    metadata JSONB,
    wasm_hash VARCHAR(255),
    s3_key VARCHAR(255),
    status VARCHAR(50) DEFAULT 'registered',
    owner_id UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    agent_id UUID REFERENCES agents(id),
    type VARCHAR(100) NOT NULL,
    description TEXT,
    input_data JSONB,
    status VARCHAR(50) DEFAULT 'pending',
    result JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB
);

-- Add indexes
CREATE INDEX idx_agents_did ON agents(did);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_user_id ON tasks(user_id);
CREATE INDEX idx_tasks_agent_id ON tasks(agent_id);
```

## Storage Configuration

### Cloudflare R2 Setup

#### 1. Create R2 Bucket
```bash
# Using Cloudflare CLI (wrangler)
npm install -g wrangler
wrangler login

# Create bucket
wrangler r2 bucket create ainur-agents-prod

# Create API token with R2 permissions
# Go to Cloudflare Dashboard > API Tokens > Create Token
# Use Custom Token with:
# - Account:Cloudflare R2:Edit
# - Zone Resources: All zones
```

#### 2. Configure CORS (Optional)
```json
[
  {
    "AllowedOrigins": ["https://your-frontend-domain.com"],
    "AllowedMethods": ["GET", "POST", "PUT", "DELETE"],
    "AllowedHeaders": ["*"],
    "ExposedHeaders": ["ETag"],
    "MaxAge": 3600
  }
]
```

#### 3. Test Storage Connection
```bash
# Test R2 upload
go run scripts/test-r2-upload.go
```

## Blockchain Deployment

### 1. Local Development Node

#### Start Development Chain
```bash
cd chain-v2
cargo build --release

# Start node
./target/release/solochain-template-node --dev --rpc-cors all --rpc-external

# The node will be available at:
# - WebSocket: ws://localhost:9944
# - HTTP: http://localhost:9933
```

#### Genesis Configuration
The development chain includes:
- **Alice**: sudo account
- **Bob**: validator
- Pre-funded accounts for testing

### 2. Production Testnet Deployment

#### Build for Production
```bash
cd chain-v2
cargo build --release --features runtime-benchmarks

# Create chain specification
./target/release/solochain-template-node build-spec --disable-default-bootnode --chain local > ainur-testnet.json

# Convert to raw spec
./target/release/solochain-template-node build-spec --chain ainur-testnet.json --raw --disable-default-bootnode > ainur-testnet-raw.json
```

#### Deploy Bootstrap Node
```bash
# Create systemd service
sudo tee /etc/systemd/system/ainur-node.service << EOF
[Unit]
Description=Ainur Blockchain Node
After=network.target
StartLimitIntervalSec=0

[Service]
Type=simple
Restart=always
RestartSec=1
User=ainur
ExecStart=/home/ainur/chain-v2/target/release/solochain-template-node \\
  --base-path /var/lib/ainur \\
  --chain /home/ainur/chain-v2/ainur-testnet-raw.json \\
  --port 30333 \\
  --rpc-port 9933 \\
  --ws-port 9944 \\
  --rpc-cors all \\
  --rpc-external \\
  --ws-external \\
  --unsafe-rpc-external \\
  --validator \\
  --name "Bootstrap-Node"

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable ainur-node
sudo systemctl start ainur-node

# Check status
sudo systemctl status ainur-node
```

### 3. Validator Node Setup

#### Key Generation
```bash
# Generate session keys
curl -H "Content-Type: application/json" -d '{"id":1, "jsonrpc":"2.0", "method": "author_rotateKeys", "params":[]}' http://localhost:9933

# Insert keys using subkey
subkey generate --scheme sr25519 --output-type json > session-key.json

# Insert into keystore
./target/release/solochain-template-node key insert \
  --base-path /var/lib/ainur \
  --chain ainur-testnet-raw.json \
  --scheme Sr25519 \
  --suri "your-secret-seed-phrase" \
  --key-type aura

./target/release/solochain-template-node key insert \
  --base-path /var/lib/ainur \
  --chain ainur-testnet-raw.json \
  --scheme Ed25519 \
  --suri "your-secret-seed-phrase" \
  --key-type gran
```

## Orchestrator API Deployment

### 1. Local Development

#### Build and Run
```bash
# Build orchestrator
cd cmd/api
go build -o ../../bin/api

# Run with development settings
./bin/api -debug -workers 3
```

### 2. Production Build

#### Optimized Build
```bash
# Build for production
cd cmd/api
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o ../../bin/api-linux

# Verify binary
file bin/api-linux
# Output: bin/api-linux: ELF 64-bit LSB executable, statically linked
```

### 3. Docker Deployment

#### Dockerfile Optimization
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git build-base

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o api ./cmd/api

# Production stage
FROM alpine:3.18

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/api .
COPY --from=builder /app/web ./web

EXPOSE 8080

CMD ["./api"]
```

#### Build and Test
```bash
# Build image
docker build -t ainur-api:latest .

# Test locally
docker run -p 8080:8080 --env-file .env ainur-api:latest

# Test health endpoint
curl http://localhost:8080/health
```

### 4. Fly.io Deployment

#### Initialize Fly.io App
```bash
# Create app
fly apps create zerostate-api --org personal

# Generate fly.toml
fly launch --no-deploy

# Deploy
fly deploy
```

#### Custom fly.toml
```toml
app = 'zerostate-api'
primary_region = 'sjc'

[build]
  dockerfile = 'Dockerfile'

[env]
  PORT = '8080'
  HOST = '0.0.0.0'
  LOG_LEVEL = 'info'
  ENABLE_METRICS = 'true'
  ORCHESTRATOR_WORKERS = '5'

[http_service]
  internal_port = 8080
  force_https = true
  auto_stop_machines = 'stop'
  auto_start_machines = true
  min_machines_running = 1

  [[http_service.checks]]
    interval = '15s'
    timeout = '2s'
    grace_period = '5s'
    method = 'GET'
    path = '/health'

[metrics]
  port = 9091
  path = "/metrics"

[[vm]]
  cpu_kind = 'shared'
  cpus = 2
  memory_mb = 1024

# Autoscaling
[scaling]
  min_machines = 1
  max_machines = 10

[[scaling.http_concurrency]]
  soft_limit = 25
  hard_limit = 1000
```

#### Deploy with Monitoring
```bash
# Deploy with secrets
fly deploy --strategy bluegreen

# Check deployment
fly status
fly logs --app zerostate-api

# Scale if needed
fly scale count 2 --app zerostate-api
fly scale vm shared-cpu-2x --app zerostate-api
```

## Monitoring Setup

### 1. Prometheus Configuration

#### prometheus.yml
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "ainur_rules.yml"

scrape_configs:
  - job_name: 'ainur-api'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 5s

  - job_name: 'ainur-blockchain'
    static_configs:
      - targets: ['localhost:9615']  # Substrate metrics port
    scrape_interval: 10s

  - job_name: 'postgres-exporter'
    static_configs:
      - targets: ['localhost:9187']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

#### Alert Rules (ainur_rules.yml)
```yaml
groups:
  - name: ainur_alerts
    rules:
      - alert: APIHighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.1
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "High API error rate"
          description: "API error rate is {{ $value }} errors per second"

      - alert: DatabaseConnectionFailure
        expr: postgres_up == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Database connection failure"

      - alert: BlockchainSyncLag
        expr: (substrate_block_height_finalized - substrate_block_height_best) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Blockchain sync lag detected"
```

### 2. Grafana Dashboard

#### Docker Compose for Monitoring Stack
```yaml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'
    restart: unless-stopped
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus:/etc/prometheus
      - prometheus_data:/prometheus

  grafana:
    image: grafana/grafana:latest
    container_name: grafana
    restart: unless-stopped
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin123
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/provisioning:/etc/grafana/provisioning
      - ./monitoring/grafana/dashboards:/var/lib/grafana/dashboards

  postgres-exporter:
    image: prometheuscommunity/postgres-exporter:latest
    container_name: postgres-exporter
    restart: unless-stopped
    ports:
      - "9187:9187"
    environment:
      - DATA_SOURCE_NAME=postgresql://username:password@postgres:5432/ainur?sslmode=disable

volumes:
  prometheus_data:
  grafana_data:
```

#### Start Monitoring Stack
```bash
# Create monitoring directory
mkdir -p monitoring/{prometheus,grafana/{provisioning,dashboards}}

# Start services
docker-compose up -d

# Check services
curl http://localhost:9090/targets  # Prometheus
curl http://localhost:3000          # Grafana (admin/admin123)
```

### 3. Production Monitoring

#### Fly.io Monitoring
```bash
# Enable metrics for Fly.io
fly scale count 1 --process-group metrics
fly secrets set ENABLE_METRICS=true

# View metrics
curl https://zerostate-api.fly.dev/metrics
```

#### External Monitoring Services
- **DataDog**: Application and infrastructure monitoring
- **New Relic**: APM and real-user monitoring
- **Sentry**: Error tracking and performance monitoring

## Frontend Deployment

### 1. Build React Frontend

#### Install Dependencies
```bash
cd web
npm install
```

#### Environment Configuration (.env.production)
```bash
REACT_APP_API_BASE_URL=https://zerostate-api.fly.dev
REACT_APP_WS_URL=wss://zerostate-api.fly.dev
REACT_APP_CHAIN_WS_URL=wss://your-substrate-node.com:9944
REACT_APP_ENVIRONMENT=production
```

#### Build for Production
```bash
# Build optimized bundle
npm run build

# Test build locally
npm run serve
```

### 2. Deploy to Vercel

#### Configure vercel.json
```json
{
  "version": 2,
  "builds": [
    {
      "src": "package.json",
      "use": "@vercel/static-build",
      "config": { "distDir": "build" }
    }
  ],
  "routes": [
    {
      "src": "/static/(.*)",
      "headers": { "cache-control": "s-maxage=31536000" },
      "dest": "/static/$1"
    },
    { "src": "/(.*)", "dest": "/index.html" }
  ],
  "env": {
    "REACT_APP_API_BASE_URL": "https://zerostate-api.fly.dev"
  }
}
```

#### Deploy
```bash
# Deploy to Vercel
vercel --prod

# Set environment variables
vercel env add REACT_APP_API_BASE_URL production
vercel env add REACT_APP_WS_URL production

# Redeploy with new environment
vercel --prod
```

## Production Deployment

### 1. Complete Deployment Script

Create a comprehensive deployment script:

```bash
#!/bin/bash
# scripts/deploy-production.sh

set -e

echo "🚀 Starting Ainur Protocol Production Deployment"

# Step 1: Build blockchain
echo "📦 Building Substrate blockchain..."
cd chain-v2
cargo build --release
cd ..

# Step 2: Build API
echo "📦 Building orchestrator API..."
cd cmd/api
go build -o ../../bin/api
cd ../..

# Step 3: Deploy database
echo "🗄️ Setting up production database..."
fly postgres create --name ainur-db --region sjc || echo "Database already exists"

# Step 4: Set secrets
echo "🔐 Configuring secrets..."
source .env.production
fly secrets set \
  DATABASE_URL="$DATABASE_URL" \
  JWT_SECRET="$(openssl rand -base64 64)" \
  R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID" \
  R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY" \
  GROQ_API_KEY="$GROQ_API_KEY" \
  --app zerostate-api

# Step 5: Deploy API
echo "🌐 Deploying API to Fly.io..."
fly deploy --app zerostate-api

# Step 6: Deploy frontend
echo "🎨 Deploying frontend to Vercel..."
cd web
npm run build
vercel --prod
cd ..

# Step 7: Health checks
echo "🏥 Running health checks..."
sleep 30
curl -f https://zerostate-api.fly.dev/health || exit 1

echo "✅ Deployment completed successfully!"
echo "📊 API: https://zerostate-api.fly.dev"
echo "🌐 Frontend: https://your-vercel-app.vercel.app"
```

### 2. Blue-Green Deployment

#### Setup Blue-Green on Fly.io
```bash
# Create staging app
fly apps create zerostate-api-staging

# Deploy to staging
fly deploy --app zerostate-api-staging

# Test staging
curl https://zerostate-api-staging.fly.dev/health

# Promote to production
fly apps rename zerostate-api zerostate-api-blue
fly apps rename zerostate-api-staging zerostate-api
```

### 3. Database Migration Strategy

#### Zero-Downtime Migration
```bash
# Create migration scripts
mkdir -p migrations

# migrations/001_initial_schema.sql
# migrations/002_add_agents_table.sql
# migrations/003_add_reputation_system.sql

# Run migrations
psql $DATABASE_URL -f migrations/001_initial_schema.sql
psql $DATABASE_URL -f migrations/002_add_agents_table.sql
```

## Health Checks

### 1. API Health Checks

#### Basic Health
```bash
# Local
curl http://localhost:8080/health

# Production
curl https://zerostate-api.fly.dev/health
```

#### Detailed Health
```bash
curl https://zerostate-api.fly.dev/health/detailed
```

### 2. Blockchain Health

#### Node Status
```bash
# Check if node is running
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_health","params":[]}' \
     http://localhost:9933
```

#### Synchronization Status
```bash
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_syncState","params":[]}' \
     http://localhost:9933
```

### 3. Database Health

#### Connection Test
```bash
# Test database connection
psql $DATABASE_URL -c "SELECT 1;"

# Check table status
psql $DATABASE_URL -c "SELECT COUNT(*) FROM agents;"
```

### 4. Storage Health

#### R2 Connectivity
```bash
# Test R2 upload
go run scripts/test-r2-upload.go
```

## Troubleshooting

### Common Issues

#### 1. Database Connection Issues

**Symptom**: `connection refused` or `timeout`
```bash
# Check database status
fly status --app ainur-db

# Test connection
psql $DATABASE_URL -c "SELECT version();"

# Check firewall rules
fly ips list --app ainur-db
```

**Solution**:
```bash
# Restart database
fly restart --app ainur-db

# Check logs
fly logs --app ainur-db
```

#### 2. R2 Storage Issues

**Symptom**: `403 Forbidden` or `InvalidAccessKeyId`
```bash
# Verify R2 credentials
aws configure list  # If using AWS CLI compatibility

# Test endpoint
curl -I https://your-account-id.r2.cloudflarestorage.com
```

**Solution**:
```bash
# Regenerate R2 tokens
# Update secrets
fly secrets set \
  R2_ACCESS_KEY_ID="new-access-key" \
  R2_SECRET_ACCESS_KEY="new-secret-key"
```

#### 3. Blockchain Sync Issues

**Symptom**: Node falls behind or stops syncing
```bash
# Check node status
systemctl status ainur-node

# View logs
journalctl -u ainur-node -f

# Check peers
curl -H "Content-Type: application/json" \
     -d '{"id":1, "jsonrpc":"2.0", "method":"system_peers","params":[]}' \
     http://localhost:9933
```

**Solution**:
```bash
# Restart with fresh sync
systemctl stop ainur-node
rm -rf /var/lib/ainur/chains/*/db
systemctl start ainur-node
```

#### 4. High Memory Usage

**Symptom**: Out of memory errors or slow performance
```bash
# Monitor memory usage
fly ssh console --app zerostate-api
htop

# Check Go memory stats
curl https://zerostate-api.fly.dev/metrics | grep go_memstats
```

**Solution**:
```bash
# Scale up memory
fly scale memory 2048 --app zerostate-api

# Restart to clear memory
fly restart --app zerostate-api
```

#### 5. API Response Timeouts

**Symptom**: 504 Gateway Timeout errors
```bash
# Check API logs
fly logs --app zerostate-api | grep timeout

# Monitor response times
curl -w "@curl-format.txt" https://zerostate-api.fly.dev/health
```

**Solution**:
```bash
# Increase timeout in fly.toml
[http_service.checks]
timeout = "10s"

# Scale horizontally
fly scale count 3 --app zerostate-api
```

### Monitoring Commands

#### System Metrics
```bash
# Fly.io metrics
fly status --app zerostate-api
fly logs --app zerostate-api

# Prometheus metrics
curl https://zerostate-api.fly.dev/metrics

# Database metrics
psql $DATABASE_URL -c "
SELECT
  pid,
  usename,
  application_name,
  client_addr,
  backend_start,
  state
FROM pg_stat_activity
WHERE state = 'active';
"
```

#### Performance Monitoring
```bash
# API performance
time curl https://zerostate-api.fly.dev/api/v1/agents

# Database performance
psql $DATABASE_URL -c "
SELECT query, calls, total_time, mean_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;
"
```

### Recovery Procedures

#### 1. Database Recovery
```bash
# Create backup
pg_dump $DATABASE_URL > backup-$(date +%Y%m%d).sql

# Restore from backup
psql $DATABASE_URL < backup-20241114.sql
```

#### 2. Application Recovery
```bash
# Rollback deployment
fly releases --app zerostate-api
fly rollback v123 --app zerostate-api

# Emergency maintenance mode
fly secrets set MAINTENANCE_MODE=true --app zerostate-api
```

#### 3. Full System Recovery
```bash
# Emergency deployment script
./scripts/emergency-deploy.sh

# Verify all services
./scripts/health-check-all.sh
```

---

For production support, monitor the [Operations Guide](OPERATIONS.md) for day-to-day operational procedures.