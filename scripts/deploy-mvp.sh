#!/bin/bash

# =============================================================================
# Ainur Protocol MVP Deployment Automation Script
#
# This script handles complete deployment of the Ainur Protocol MVP including:
# - Substrate blockchain (chain-v2)
# - Orchestrator API
# - Frontend
# - Infrastructure setup
# - Health verification
# =============================================================================

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENT_MODE=${1:-"production"}  # production, staging, development
SKIP_TESTS=${SKIP_TESTS:-false}
FORCE_REBUILD=${FORCE_REBUILD:-false}
BACKUP_ENABLED=${BACKUP_ENABLED:-true}

# Deployment configuration
FLY_APP_NAME="zerostate-api"
FLY_DB_APP_NAME="ainur-db"
VERCEL_PROJECT="ainur-protocol"
HEALTH_CHECK_TIMEOUT=300  # 5 minutes
ROLLBACK_ON_FAILURE=${ROLLBACK_ON_FAILURE:-true}

# =============================================================================
# Utility Functions
# =============================================================================

print_banner() {
    echo -e "${CYAN}"
    echo "╔══════════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                              ║"
    echo "║                    🚀 AINUR PROTOCOL MVP DEPLOYMENT 🚀                     ║"
    echo "║                                                                              ║"
    echo "║                        Decentralized Agent Marketplace                      ║"
    echo "║                                                                              ║"
    echo "╚══════════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')] $1${NC}"
}

log_info() {
    echo -e "${BLUE}[INFO] $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}[WARN] $1${NC}"
}

log_error() {
    echo -e "${RED}[ERROR] $1${NC}"
}

log_step() {
    echo -e "${PURPLE}[STEP] $1${NC}"
}

check_command() {
    if ! command -v "$1" &> /dev/null; then
        log_error "Command '$1' not found. Please install it first."
        exit 1
    fi
}

check_prerequisites() {
    log_step "Checking prerequisites..."

    # Required commands
    local commands=("git" "cargo" "go" "node" "npm" "fly" "vercel" "curl" "jq" "psql")
    for cmd in "${commands[@]}"; do
        check_command "$cmd"
    done

    # Check environment file
    if [[ ! -f "$PROJECT_ROOT/.env.$DEPLOYMENT_MODE" ]]; then
        log_error "Environment file .env.$DEPLOYMENT_MODE not found!"
        log_error "Please create it with required configuration."
        exit 1
    fi

    # Load environment
    source "$PROJECT_ROOT/.env.$DEPLOYMENT_MODE"

    # Check required environment variables
    local required_vars=("DATABASE_URL" "JWT_SECRET" "R2_ACCESS_KEY_ID" "R2_SECRET_ACCESS_KEY" "GROQ_API_KEY")
    for var in "${required_vars[@]}"; do
        if [[ -z "${!var:-}" ]]; then
            log_error "Required environment variable $var is not set!"
            exit 1
        fi
    done

    log "✅ Prerequisites check completed"
}

create_backup() {
    if [[ "$BACKUP_ENABLED" != "true" ]]; then
        log_info "Backup disabled, skipping..."
        return 0
    fi

    log_step "Creating pre-deployment backup..."

    local timestamp=$(date +%Y%m%d_%H%M%S)
    local backup_dir="$PROJECT_ROOT/backups/deployment_$timestamp"

    mkdir -p "$backup_dir"

    # Database backup
    if pg_dump "$DATABASE_URL" > "$backup_dir/database_backup.sql" 2>/dev/null; then
        log "✅ Database backup created"
    else
        log_warn "Failed to create database backup (database might not exist yet)"
    fi

    # Configuration backup
    cp "$PROJECT_ROOT/.env.$DEPLOYMENT_MODE" "$backup_dir/" || true
    fly config save --app "$FLY_APP_NAME" > "$backup_dir/fly_config.toml" 2>/dev/null || true
    fly secrets list --app "$FLY_APP_NAME" > "$backup_dir/secrets_list.txt" 2>/dev/null || true

    # Store backup location for potential rollback
    echo "$backup_dir" > "$PROJECT_ROOT/.last_backup"

    log "✅ Backup created at: $backup_dir"
}

# =============================================================================
# Build Functions
# =============================================================================

build_blockchain() {
    log_step "Building Substrate blockchain..."

    cd "$PROJECT_ROOT/chain-v2"

    # Clean build if requested
    if [[ "$FORCE_REBUILD" == "true" ]]; then
        log_info "Force rebuild requested, cleaning..."
        cargo clean
    fi

    # Build runtime and node
    log_info "Building Substrate node (this may take 10-15 minutes)..."
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        # Production build with optimizations
        cargo build --release --features runtime-benchmarks
    else
        # Development build (faster)
        cargo build --release
    fi

    # Verify build
    if [[ ! -f "target/release/solochain-template-node" ]]; then
        log_error "Substrate node build failed!"
        exit 1
    fi

    # Generate chain specification for production
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        log_info "Generating chain specification..."
        ./target/release/solochain-template-node build-spec \
            --disable-default-bootnode \
            --chain local > ainur-testnet.json

        ./target/release/solochain-template-node build-spec \
            --chain ainur-testnet.json \
            --raw \
            --disable-default-bootnode > ainur-testnet-raw.json
    fi

    cd "$PROJECT_ROOT"
    log "✅ Blockchain build completed"
}

build_orchestrator() {
    log_step "Building orchestrator API..."

    cd "$PROJECT_ROOT/cmd/api"

    # Test before building
    if [[ "$SKIP_TESTS" != "true" ]]; then
        log_info "Running Go tests..."
        go test ../../libs/... -v -timeout 30s
    fi

    # Build API server
    log_info "Building orchestrator API..."
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        # Production build with optimizations
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo \
            -ldflags '-extldflags "-static" -s -w' \
            -o ../../bin/api-linux
    else
        # Development build
        go build -o ../../bin/api
    fi

    # Verify build
    local binary_path="$PROJECT_ROOT/bin/api"
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        binary_path="$PROJECT_ROOT/bin/api-linux"
    fi

    if [[ ! -f "$binary_path" ]]; then
        log_error "API server build failed!"
        exit 1
    fi

    cd "$PROJECT_ROOT"
    log "✅ Orchestrator API build completed"
}

build_frontend() {
    log_step "Building React frontend..."

    cd "$PROJECT_ROOT/web"

    # Install dependencies
    log_info "Installing frontend dependencies..."
    npm ci --silent

    # Run tests
    if [[ "$SKIP_TESTS" != "true" ]]; then
        log_info "Running frontend tests..."
        npm test -- --coverage --watchAll=false --silent
    fi

    # Build for production
    log_info "Building production frontend..."
    npm run build

    # Verify build
    if [[ ! -d "build" ]]; then
        log_error "Frontend build failed!"
        exit 1
    fi

    cd "$PROJECT_ROOT"
    log "✅ Frontend build completed"
}

run_comprehensive_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        log_info "Tests skipped per configuration"
        return 0
    fi

    log_step "Running comprehensive test suite..."

    # Unit tests
    log_info "Running unit tests..."

    # Rust tests
    cd "$PROJECT_ROOT/chain-v2"
    cargo test --release --all-features

    # Go tests
    cd "$PROJECT_ROOT"
    go test ./libs/... -race -timeout 60s

    # Integration tests
    log_info "Running integration tests..."
    cd "$PROJECT_ROOT/tests"
    go test -v ./... -timeout 120s

    cd "$PROJECT_ROOT"
    log "✅ All tests passed"
}

# =============================================================================
# Infrastructure Setup Functions
# =============================================================================

setup_database() {
    log_step "Setting up production database..."

    # Check if database app exists
    if fly apps list | grep -q "$FLY_DB_APP_NAME"; then
        log_info "Database app '$FLY_DB_APP_NAME' already exists"
    else
        log_info "Creating new PostgreSQL database..."
        fly postgres create \
            --name "$FLY_DB_APP_NAME" \
            --region sjc \
            --vm-size shared-cpu-1x \
            --initial-cluster-size 1 \
            --volume-size 10
    fi

    # Attach database to API app
    if fly postgres attach --app "$FLY_APP_NAME" "$FLY_DB_APP_NAME" 2>/dev/null; then
        log "✅ Database attached to API app"
    else
        log_info "Database already attached to API app"
    fi

    # Wait for database to be ready
    log_info "Waiting for database to be ready..."
    local max_attempts=30
    local attempt=0

    while [[ $attempt -lt $max_attempts ]]; do
        if psql "$DATABASE_URL" -c "SELECT 1;" &>/dev/null; then
            break
        fi
        ((attempt++))
        sleep 10
        log_info "Database not ready yet, attempt $attempt/$max_attempts"
    done

    if [[ $attempt -eq $max_attempts ]]; then
        log_error "Database failed to become ready within timeout"
        exit 1
    fi

    # Run database migrations
    setup_database_schema

    log "✅ Database setup completed"
}

setup_database_schema() {
    log_info "Setting up database schema..."

    # Create database schema
    psql "$DATABASE_URL" << 'EOF'
-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create agents table
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

-- Create tasks table
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

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_agents_did ON agents(did);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX IF NOT EXISTS idx_agents_owner ON agents(owner_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_user_id ON tasks(user_id);
CREATE INDEX IF NOT EXISTS idx_tasks_agent_id ON tasks(agent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Update updated_at trigger function
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply triggers
DROP TRIGGER IF EXISTS set_timestamp ON users;
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_set_timestamp();

DROP TRIGGER IF EXISTS set_timestamp ON agents;
CREATE TRIGGER set_timestamp
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE PROCEDURE trigger_set_timestamp();
EOF

    log "✅ Database schema setup completed"
}

setup_storage() {
    log_step "Setting up R2 storage..."

    # Test R2 connectivity
    if go run "$PROJECT_ROOT/scripts/test-r2-upload.go" &>/dev/null; then
        log "✅ R2 storage connectivity verified"
    else
        log_error "R2 storage connectivity test failed"
        log_error "Please verify R2 credentials and endpoint configuration"
        exit 1
    fi
}

configure_secrets() {
    log_step "Configuring application secrets..."

    # Generate a strong JWT secret if not provided
    if [[ -z "${JWT_SECRET:-}" ]] || [[ "$JWT_SECRET" == "development-secret-key-change-in-production" ]]; then
        JWT_SECRET=$(openssl rand -base64 64 | tr -d '\n')
        log_info "Generated new JWT secret"
    fi

    # Set all secrets at once
    log_info "Setting Fly.io secrets..."
    fly secrets set \
        DATABASE_URL="$DATABASE_URL" \
        JWT_SECRET="$JWT_SECRET" \
        R2_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID" \
        R2_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY" \
        R2_ENDPOINT="$R2_ENDPOINT" \
        R2_BUCKET_NAME="$R2_BUCKET_NAME" \
        GROQ_API_KEY="$GROQ_API_KEY" \
        ${GEMINI_API_KEY:+GEMINI_API_KEY="$GEMINI_API_KEY"} \
        --app "$FLY_APP_NAME"

    log "✅ Secrets configured"
}

# =============================================================================
# Deployment Functions
# =============================================================================

deploy_api() {
    log_step "Deploying orchestrator API to Fly.io..."

    # Check if app exists
    if fly apps list | grep -q "$FLY_APP_NAME"; then
        log_info "App '$FLY_APP_NAME' exists, updating..."
    else
        log_info "Creating new Fly.io app..."
        fly apps create "$FLY_APP_NAME" --org personal
    fi

    # Deploy with blue-green strategy for production
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        log_info "Deploying with blue-green strategy..."
        fly deploy --app "$FLY_APP_NAME" --strategy bluegreen
    else
        log_info "Deploying with rolling strategy..."
        fly deploy --app "$FLY_APP_NAME" --strategy rolling
    fi

    # Wait for deployment to complete
    log_info "Waiting for deployment to complete..."
    sleep 30

    # Verify deployment
    if ! fly status --app "$FLY_APP_NAME" | grep -q "running"; then
        log_error "API deployment verification failed"
        if [[ "$ROLLBACK_ON_FAILURE" == "true" ]]; then
            rollback_deployment
        fi
        exit 1
    fi

    log "✅ API deployment completed"
}

deploy_frontend() {
    log_step "Deploying frontend to Vercel..."

    cd "$PROJECT_ROOT/web"

    # Set environment variables for production
    local api_url
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        api_url="https://${FLY_APP_NAME}.fly.dev"
    else
        api_url="https://${FLY_APP_NAME}-staging.fly.dev"
    fi

    # Deploy to Vercel
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        vercel --prod --confirm \
            -e REACT_APP_API_BASE_URL="$api_url" \
            -e REACT_APP_WS_URL="${api_url/https:/wss:}" \
            -e REACT_APP_ENVIRONMENT="$DEPLOYMENT_MODE"
    else
        vercel --confirm \
            -e REACT_APP_API_BASE_URL="$api_url" \
            -e REACT_APP_WS_URL="${api_url/https:/wss:}" \
            -e REACT_APP_ENVIRONMENT="$DEPLOYMENT_MODE"
    fi

    cd "$PROJECT_ROOT"
    log "✅ Frontend deployment completed"
}

# =============================================================================
# Health Check Functions
# =============================================================================

wait_for_service() {
    local service_url="$1"
    local service_name="$2"
    local max_attempts=$((HEALTH_CHECK_TIMEOUT / 10))
    local attempt=0

    log_info "Waiting for $service_name to be healthy..."

    while [[ $attempt -lt $max_attempts ]]; do
        if curl -f -s "$service_url/health" > /dev/null 2>&1; then
            log "✅ $service_name is healthy"
            return 0
        fi

        ((attempt++))
        log_info "$service_name not ready yet, attempt $attempt/$max_attempts"
        sleep 10
    done

    log_error "$service_name failed to become healthy within $HEALTH_CHECK_TIMEOUT seconds"
    return 1
}

run_health_checks() {
    log_step "Running comprehensive health checks..."

    local api_url
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        api_url="https://${FLY_APP_NAME}.fly.dev"
    else
        api_url="https://${FLY_APP_NAME}-staging.fly.dev"
    fi

    # Wait for API to be healthy
    if ! wait_for_service "$api_url" "API Server"; then
        log_error "API health check failed"
        return 1
    fi

    # Test database connectivity
    log_info "Testing database connectivity..."
    local db_health=$(curl -s "$api_url/health/detailed" | jq -r '.services.database.status')
    if [[ "$db_health" != "healthy" ]]; then
        log_error "Database health check failed"
        return 1
    fi

    # Test storage connectivity
    log_info "Testing storage connectivity..."
    local storage_health=$(curl -s "$api_url/health/detailed" | jq -r '.services.storage.status')
    if [[ "$storage_health" != "healthy" ]]; then
        log_error "Storage health check failed"
        return 1
    fi

    # Test core API endpoints
    log_info "Testing core API endpoints..."

    # Test agents endpoint
    if ! curl -f -s "$api_url/api/v1/agents" > /dev/null; then
        log_error "Agents endpoint health check failed"
        return 1
    fi

    # Test metrics endpoint
    if ! curl -f -s "$api_url/metrics" > /dev/null; then
        log_error "Metrics endpoint health check failed"
        return 1
    fi

    log "✅ All health checks passed"
    return 0
}

run_smoke_tests() {
    log_step "Running smoke tests..."

    local api_url
    if [[ "$DEPLOYMENT_MODE" == "production" ]]; then
        api_url="https://${FLY_APP_NAME}.fly.dev"
    else
        api_url="https://${FLY_APP_NAME}-staging.fly.dev"
    fi

    # Test agent registration (mock)
    log_info "Testing agent registration flow..."
    local test_response=$(curl -s -w "%{http_code}" "$api_url/api/v1/agents?limit=1")
    local http_code="${test_response: -3}"

    if [[ "$http_code" -ne 200 ]]; then
        log_error "Agent registration smoke test failed (HTTP $http_code)"
        return 1
    fi

    # Test WebSocket connection
    log_info "Testing WebSocket connectivity..."
    # Note: This is a basic connectivity test, not a full WebSocket test
    if ! curl -f -s -H "Connection: Upgrade" -H "Upgrade: websocket" \
        "$api_url/api/v1/ws/connect" > /dev/null; then
        log_warn "WebSocket connectivity test failed (may be expected)"
    fi

    log "✅ Smoke tests completed"
    return 0
}

# =============================================================================
# Rollback and Recovery Functions
# =============================================================================

rollback_deployment() {
    log_error "Initiating rollback procedure..."

    # Get previous release
    local releases
    releases=$(fly releases --app "$FLY_APP_NAME" --json | jq -r '.[1].version')

    if [[ -n "$releases" ]] && [[ "$releases" != "null" ]]; then
        log_info "Rolling back to release $releases"
        fly rollback "v$releases" --app "$FLY_APP_NAME"

        # Wait for rollback to complete
        sleep 30

        # Verify rollback
        if curl -f -s "https://${FLY_APP_NAME}.fly.dev/health" > /dev/null; then
            log "✅ Rollback completed successfully"
        else
            log_error "Rollback verification failed"
        fi
    else
        log_error "No previous release found for rollback"
    fi

    # Restore database if backup exists
    if [[ -f "$PROJECT_ROOT/.last_backup" ]]; then
        local backup_dir
        backup_dir=$(cat "$PROJECT_ROOT/.last_backup")
        if [[ -f "$backup_dir/database_backup.sql" ]]; then
            log_info "Restoring database from backup..."
            psql "$DATABASE_URL" < "$backup_dir/database_backup.sql"
            log "✅ Database restored from backup"
        fi
    fi
}

cleanup_failed_deployment() {
    log_info "Cleaning up failed deployment artifacts..."

    # Remove temporary files
    rm -f "$PROJECT_ROOT/.deployment_lock"
    rm -f "$PROJECT_ROOT/.deployment_status"

    # Clean up any hanging processes
    pkill -f "solochain-template-node" || true
}

# =============================================================================
# Monitoring and Reporting Functions
# =============================================================================

generate_deployment_report() {
    log_step "Generating deployment report..."

    local report_file="$PROJECT_ROOT/deployment_report_$(date +%Y%m%d_%H%M%S).md"
    local api_url="https://${FLY_APP_NAME}.fly.dev"

    cat > "$report_file" << EOF
# Ainur Protocol Deployment Report

**Deployment Date**: $(date)
**Deployment Mode**: $DEPLOYMENT_MODE
**Deployed By**: $(whoami)
**Git Commit**: $(git rev-parse HEAD)
**Git Branch**: $(git branch --show-current)

## Deployment Summary

- ✅ Substrate blockchain built successfully
- ✅ Orchestrator API deployed to Fly.io
- ✅ Frontend deployed to Vercel
- ✅ Database schema updated
- ✅ Health checks passed

## Service URLs

- **API Server**: $api_url
- **API Health**: $api_url/health
- **API Metrics**: $api_url/metrics
- **Frontend**: $(vercel ls 2>/dev/null | grep "ainur" | head -1 | awk '{print $2}' || echo "Not available")

## Performance Metrics

EOF

    # Add performance metrics
    if curl -s "$api_url/metrics/summary" > /dev/null 2>&1; then
        echo "### Current System Metrics" >> "$report_file"
        curl -s "$api_url/metrics/summary" | jq -r '
            "- **Uptime**: " + .system.uptime_seconds + "s",
            "- **Memory Usage**: " + (.system.memory_bytes / 1024 / 1024 | tostring) + "MB",
            "- **Active Connections**: " + (.database.connections_active | tostring),
            "- **Response Time**: " + .http.average_response_time
        ' >> "$report_file" 2>/dev/null || echo "- Metrics not available" >> "$report_file"
    fi

    cat >> "$report_file" << EOF

## Database Status

$(psql "$DATABASE_URL" -c "
SELECT
    'Total Users: ' || COUNT(*)
FROM users
UNION ALL
SELECT
    'Total Agents: ' || COUNT(*)
FROM agents
UNION ALL
SELECT
    'Total Tasks: ' || COUNT(*)
FROM tasks;" 2>/dev/null || echo "Database metrics not available")

## Post-Deployment Actions

- [ ] Monitor system metrics for 30 minutes
- [ ] Update DNS records if needed
- [ ] Notify team of successful deployment
- [ ] Update status page
- [ ] Schedule post-deployment review

## Rollback Information

**Backup Location**: $(cat "$PROJECT_ROOT/.last_backup" 2>/dev/null || echo "No backup created")
**Previous Release**: $(fly releases --app "$FLY_APP_NAME" --json 2>/dev/null | jq -r '.[1].version // "None"')

---
Generated by Ainur Protocol MVP Deployment Script
EOF

    log "✅ Deployment report generated: $report_file"

    # Display summary
    echo
    log_info "📊 DEPLOYMENT SUMMARY"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🌐 API URL:      $api_url"
    echo "🏥 Health Check: $api_url/health"
    echo "📊 Metrics:      $api_url/metrics"
    echo "📋 Report:       $report_file"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

post_deployment_monitoring() {
    log_step "Starting post-deployment monitoring..."

    local api_url="https://${FLY_APP_NAME}.fly.dev"
    local monitor_duration=300  # 5 minutes
    local check_interval=30     # 30 seconds
    local checks=$((monitor_duration / check_interval))

    for ((i=1; i<=checks; i++)); do
        log_info "Monitoring check $i/$checks"

        # Health check
        if ! curl -f -s "$api_url/health" > /dev/null; then
            log_error "Health check failed during monitoring"
            return 1
        fi

        # Check error rate
        local metrics
        metrics=$(curl -s "$api_url/metrics/summary" 2>/dev/null)
        if [[ -n "$metrics" ]]; then
            local error_rate
            error_rate=$(echo "$metrics" | jq -r '.http.error_rate // 0' 2>/dev/null || echo "0")
            if (( $(echo "$error_rate > 0.05" | bc -l) )); then
                log_warn "High error rate detected: $error_rate"
            fi
        fi

        sleep $check_interval
    done

    log "✅ Post-deployment monitoring completed successfully"
}

# =============================================================================
# Main Deployment Function
# =============================================================================

main() {
    local start_time=$(date +%s)

    print_banner

    # Create deployment lock
    if [[ -f "$PROJECT_ROOT/.deployment_lock" ]]; then
        log_error "Deployment already in progress (lock file exists)"
        exit 1
    fi

    echo $$ > "$PROJECT_ROOT/.deployment_lock"
    trap 'cleanup_failed_deployment' ERR EXIT

    log "🚀 Starting Ainur Protocol MVP deployment"
    log "Mode: $DEPLOYMENT_MODE"
    log "Skip Tests: $SKIP_TESTS"
    log "Force Rebuild: $FORCE_REBUILD"
    log "Backup Enabled: $BACKUP_ENABLED"
    echo

    # Pre-deployment steps
    check_prerequisites
    create_backup

    # Build phase
    log "📦 BUILD PHASE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    build_blockchain
    build_orchestrator
    build_frontend
    run_comprehensive_tests
    echo

    # Infrastructure phase
    log "🏗️ INFRASTRUCTURE PHASE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    setup_database
    setup_storage
    configure_secrets
    echo

    # Deployment phase
    log "🚀 DEPLOYMENT PHASE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    deploy_api
    deploy_frontend
    echo

    # Verification phase
    log "🏥 VERIFICATION PHASE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    if ! run_health_checks; then
        log_error "Health checks failed"
        if [[ "$ROLLBACK_ON_FAILURE" == "true" ]]; then
            rollback_deployment
        fi
        exit 1
    fi

    if ! run_smoke_tests; then
        log_error "Smoke tests failed"
        if [[ "$ROLLBACK_ON_FAILURE" == "true" ]]; then
            rollback_deployment
        fi
        exit 1
    fi
    echo

    # Monitoring and reporting
    log "📊 MONITORING & REPORTING PHASE"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    generate_deployment_report
    post_deployment_monitoring

    # Success
    local end_time=$(date +%s)
    local duration=$((end_time - start_time))

    echo
    echo -e "${GREEN}"
    echo "╔══════════════════════════════════════════════════════════════════════════════╗"
    echo "║                                                                              ║"
    echo "║                          ✅ DEPLOYMENT SUCCESSFUL! ✅                       ║"
    echo "║                                                                              ║"
    echo "║                     Ainur Protocol MVP is now live!                         ║"
    echo "║                                                                              ║"
    echo "║  Total deployment time: $(printf "%02d:%02d:%02d" $((duration/3600)) $((duration%3600/60)) $((duration%60)))                                        ║"
    echo "║                                                                              ║"
    echo "╚══════════════════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"

    # Cleanup
    rm -f "$PROJECT_ROOT/.deployment_lock"

    exit 0
}

# =============================================================================
# Script Entry Point
# =============================================================================

# Handle command line arguments
case "${1:-help}" in
    "production"|"staging"|"development")
        main "$@"
        ;;
    "help"|"-h"|"--help")
        echo "Ainur Protocol MVP Deployment Script"
        echo
        echo "Usage: $0 [MODE] [OPTIONS]"
        echo
        echo "Modes:"
        echo "  production    Deploy to production environment"
        echo "  staging       Deploy to staging environment"
        echo "  development   Deploy to development environment"
        echo
        echo "Environment Variables:"
        echo "  SKIP_TESTS=true           Skip test execution"
        echo "  FORCE_REBUILD=true        Force clean rebuild"
        echo "  BACKUP_ENABLED=false      Disable backup creation"
        echo "  ROLLBACK_ON_FAILURE=false Disable automatic rollback"
        echo
        echo "Examples:"
        echo "  $0 production                    # Standard production deployment"
        echo "  SKIP_TESTS=true $0 staging      # Quick staging deployment"
        echo "  FORCE_REBUILD=true $0 production # Force rebuild deployment"
        ;;
    *)
        log_error "Invalid deployment mode: $1"
        log_info "Use '$0 help' for usage information"
        exit 1
        ;;
esac