#!/bin/bash

set -euo pipefail

# verify-deployment-readiness.sh
# Comprehensive deployment verification script for Sprint 5 Phase 4

echo "🚀 Ainur Protocol - Sprint 5 Deployment Verification"
echo "=================================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
CHECKS_PASSED=0
CHECKS_FAILED=0
TOTAL_CHECKS=0

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((CHECKS_PASSED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((CHECKS_FAILED++))
}

increment_check() {
    ((TOTAL_CHECKS++))
}

check_command() {
    local cmd=$1
    local description=$2

    increment_check
    if command -v "$cmd" >/dev/null 2>&1; then
        log_success "$description: $cmd found"
        return 0
    else
        log_error "$description: $cmd not found"
        return 1
    fi
}

check_file() {
    local file=$1
    local description=$2

    increment_check
    if [[ -f "$file" ]]; then
        log_success "$description: $file exists"
        return 0
    else
        log_error "$description: $file not found"
        return 1
    fi
}

check_directory() {
    local dir=$1
    local description=$2

    increment_check
    if [[ -d "$dir" ]]; then
        log_success "$description: $dir exists"
        return 0
    else
        log_error "$description: $dir not found"
        return 1
    fi
}

run_test() {
    local test_name=$1
    local test_cmd=$2

    increment_check
    log_info "Running $test_name..."

    if eval "$test_cmd" >/dev/null 2>&1; then
        log_success "$test_name: PASSED"
        return 0
    else
        log_error "$test_name: FAILED"
        return 1
    fi
}

# Step 1: Environment Prerequisites
echo -e "\n${BLUE}=== Step 1: Environment Prerequisites ===${NC}"

check_command "go" "Go compiler"
check_command "docker" "Docker"
check_command "docker-compose" "Docker Compose"
check_command "cargo" "Rust Cargo"
check_command "git" "Git"

# Check Go version
increment_check
GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\d+\.\d+' || echo "unknown")
if [[ "$GO_VERSION" != "unknown" ]]; then
    log_success "Go version: $GO_VERSION"
else
    log_error "Could not determine Go version"
fi

# Check Docker version
increment_check
if docker --version >/dev/null 2>&1; then
    DOCKER_VERSION=$(docker --version | cut -d' ' -f3 | cut -d',' -f1)
    log_success "Docker version: $DOCKER_VERSION"
else
    log_error "Docker not accessible"
fi

# Step 2: Project Structure
echo -e "\n${BLUE}=== Step 2: Project Structure Verification ===${NC}"

# Check core directories
check_directory "libs/orchestration" "Orchestration library"
check_directory "libs/substrate" "Substrate client library"
check_directory "chain-v2" "Blockchain (chain-v2)"
check_directory "tests/e2e" "E2E tests directory"
check_directory "tests/load" "Load tests directory"
check_directory "tests/benchmarks" "Benchmarks directory"

# Check critical files
check_file "libs/orchestration/orchestrator.go" "Core orchestrator"
check_file "libs/orchestration/reputation_integration.go" "Reputation integration"
check_file "libs/orchestration/vcg_auction.go" "VCG auction implementation"
check_file "libs/substrate/reputation_client.go" "Reputation client"
check_file "chain-v2/pallets/reputation/src/lib.rs" "Reputation pallet"

# Check test files
check_file "tests/e2e/sprint5_reputation_test.go" "Reputation E2E tests"
check_file "tests/e2e/sprint5_vcg_test.go" "VCG auction E2E tests"
check_file "tests/load/sprint5_load_test.go" "Load tests"
check_file "tests/benchmarks/sprint5_bench_test.go" "Performance benchmarks"

# Step 3: Build Verification
echo -e "\n${BLUE}=== Step 3: Build Verification ===${NC}"

# Build chain-v2 blockchain
increment_check
log_info "Building chain-v2 blockchain..."
if (cd chain-v2 && cargo build --release) >/dev/null 2>&1; then
    log_success "Chain-v2 blockchain build: PASSED"
else
    log_error "Chain-v2 blockchain build: FAILED"
fi

# Build Go modules
increment_check
log_info "Building Go modules..."
if go mod tidy && go build ./... >/dev/null 2>&1; then
    log_success "Go modules build: PASSED"
else
    log_error "Go modules build: FAILED"
fi

# Check specific module builds
increment_check
log_info "Building orchestration module..."
if (cd libs/orchestration && go build .) >/dev/null 2>&1; then
    log_success "Orchestration module build: PASSED"
else
    log_error "Orchestration module build: FAILED"
fi

increment_check
log_info "Building substrate client..."
if (cd libs/substrate && go build .) >/dev/null 2>&1; then
    log_success "Substrate client build: PASSED"
else
    log_error "Substrate client build: FAILED"
fi

# Step 4: Test Compilation
echo -e "\n${BLUE}=== Step 4: Test Compilation ===${NC}"

run_test "E2E tests compilation" "cd tests/e2e && go test -c"
run_test "Load tests compilation" "cd tests/load && go test -c"
run_test "Benchmark tests compilation" "cd tests/benchmarks && go test -c"

# Step 5: Dependencies Check
echo -e "\n${BLUE}=== Step 5: Dependencies Verification ===${NC}"

increment_check
log_info "Checking Go module dependencies..."
if go mod verify >/dev/null 2>&1; then
    log_success "Go module dependencies: VERIFIED"
else
    log_error "Go module dependencies: VERIFICATION FAILED"
fi

# Check for missing dependencies
increment_check
log_info "Checking for missing Go dependencies..."
if go list -m all >/dev/null 2>&1; then
    log_success "All Go dependencies: AVAILABLE"
else
    log_error "Missing Go dependencies detected"
fi

# Step 6: Configuration Files
echo -e "\n${BLUE}=== Step 6: Configuration Verification ===${NC}"

check_file "go.mod" "Root go.mod"
check_file "go.work" "Go workspace"
check_file "chain-v2/Cargo.toml" "Blockchain Cargo.toml"

# Check for environment templates
check_file ".env.example" "Environment template"

# Step 7: Health Endpoints Check
echo -e "\n${BLUE}=== Step 7: Health Endpoints Preparation ===${NC}"

# Check if health endpoint code exists
increment_check
if grep -r "health" libs/api/ >/dev/null 2>&1; then
    log_success "Health endpoints: IMPLEMENTED"
else
    log_warning "Health endpoints: NOT FOUND (may need implementation)"
fi

# Check metrics endpoints
increment_check
if grep -r "metrics" libs/ >/dev/null 2>&1; then
    log_success "Metrics endpoints: IMPLEMENTED"
else
    log_warning "Metrics endpoints: LIMITED IMPLEMENTATION"
fi

# Step 8: Database Schema
echo -e "\n${BLUE}=== Step 8: Database Schema Verification ===${NC}"

check_file "libs/database/models.go" "Database models"
check_file "libs/database/repository.go" "Database repository"

# Step 9: Docker Configuration
echo -e "\n${BLUE}=== Step 9: Docker Configuration ===${NC}"

check_file "Dockerfile" "Main Dockerfile"
check_file "chain-v2/docker-compose.yml" "Blockchain Docker Compose"

# Test Docker build (quick check)
increment_check
log_info "Testing Docker build compatibility..."
if docker build -t ainur-test . >/dev/null 2>&1; then
    log_success "Docker build: PASSED"
    # Cleanup
    docker rmi ainur-test >/dev/null 2>&1 || true
else
    log_error "Docker build: FAILED"
fi

# Step 10: Security Checks
echo -e "\n${BLUE}=== Step 10: Basic Security Verification ===${NC}"

# Check for hardcoded secrets
increment_check
log_info "Scanning for potential hardcoded secrets..."
SECRET_PATTERNS=("password" "secret" "token" "key")
SECRETS_FOUND=false

for pattern in "${SECRET_PATTERNS[@]}"; do
    if grep -ri "$pattern.*=" . --include="*.go" --include="*.rs" | grep -v -E "(test|example|demo)" >/dev/null 2>&1; then
        SECRETS_FOUND=true
        break
    fi
done

if $SECRETS_FOUND; then
    log_warning "Potential secrets found in code (manual review recommended)"
else
    log_success "No obvious hardcoded secrets found"
fi

# Check file permissions
increment_check
log_info "Checking executable permissions..."
if [[ -x "scripts/verify-deployment-readiness.sh" ]]; then
    log_success "Script permissions: CORRECT"
else
    log_warning "Script permissions: May need adjustment"
fi

# Step 11: Performance Baseline
echo -e "\n${BLUE}=== Step 11: Performance Baseline ===${NC}"

# Quick performance test compilation
increment_check
log_info "Testing benchmark compilation..."
if (cd tests/benchmarks && go test -c -o /tmp/ainur-bench) >/dev/null 2>&1; then
    log_success "Benchmark compilation: PASSED"
    rm -f /tmp/ainur-bench
else
    log_error "Benchmark compilation: FAILED"
fi

# Step 12: Documentation Check
echo -e "\n${BLUE}=== Step 12: Documentation Verification ===${NC}"

check_file "README.md" "Main README"
check_file "SPRINT_5_PHASE_4_COMPLETE.md" "Sprint 5 documentation"

# Check for API documentation
increment_check
if find . -name "*.md" | grep -i api >/dev/null 2>&1; then
    log_success "API documentation: FOUND"
else
    log_warning "API documentation: LIMITED"
fi

# Step 13: Final Readiness Assessment
echo -e "\n${BLUE}=== Step 13: Deployment Readiness Assessment ===${NC}"

# Calculate readiness score
READINESS_SCORE=$(echo "scale=1; $CHECKS_PASSED * 100 / $TOTAL_CHECKS" | bc -l 2>/dev/null || echo "0")

echo -e "\n${BLUE}=== DEPLOYMENT READINESS SUMMARY ===${NC}"
echo "Total Checks: $TOTAL_CHECKS"
echo "Checks Passed: $CHECKS_PASSED"
echo "Checks Failed: $CHECKS_FAILED"
echo "Readiness Score: ${READINESS_SCORE}%"

if (( CHECKS_FAILED == 0 )); then
    echo -e "\n${GREEN}🎉 DEPLOYMENT READY: All checks passed!${NC}"
    exit 0
elif [[ "${READINESS_SCORE%.*}" -ge 90 ]]; then
    echo -e "\n${YELLOW}⚠️  MOSTLY READY: Minor issues detected (${CHECKS_FAILED} failures)${NC}"
    echo -e "${YELLOW}   Review failed checks before production deployment${NC}"
    exit 1
elif [[ "${READINESS_SCORE%.*}" -ge 75 ]]; then
    echo -e "\n${YELLOW}⚠️  PARTIAL READINESS: Moderate issues detected (${CHECKS_FAILED} failures)${NC}"
    echo -e "${YELLOW}   Address critical failures before deployment${NC}"
    exit 2
else
    echo -e "\n${RED}❌ NOT READY: Significant issues detected (${CHECKS_FAILED} failures)${NC}"
    echo -e "${RED}   Major fixes required before deployment${NC}"
    exit 3
fi