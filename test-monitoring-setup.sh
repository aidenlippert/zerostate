#!/bin/bash

# Test script for Ainur Protocol monitoring setup
# Validates that all metrics and health endpoints are working correctly

set -e

API_BASE_URL="${API_BASE_URL:-http://localhost:8080}"
PROMETHEUS_URL="${PROMETHEUS_URL:-http://localhost:9090}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"

echo "🔍 Testing Ainur Protocol Monitoring Setup"
echo "=========================================="
echo "API Base URL: $API_BASE_URL"
echo "Prometheus URL: $PROMETHEUS_URL"
echo "Grafana URL: $GRAFANA_URL"
echo ""

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to test HTTP endpoint
test_endpoint() {
    local endpoint="$1"
    local expected_status="$2"
    local description="$3"

    echo -n "Testing $description... "

    if response=$(curl -s -o /dev/null -w "%{http_code}" "$endpoint" 2>/dev/null); then
        if [ "$response" = "$expected_status" ]; then
            echo -e "${GREEN}✅ PASS${NC} ($response)"
            ((TESTS_PASSED++))
        else
            echo -e "${RED}❌ FAIL${NC} (got $response, expected $expected_status)"
            ((TESTS_FAILED++))
        fi
    else
        echo -e "${RED}❌ FAIL${NC} (connection error)"
        ((TESTS_FAILED++))
    fi
}

# Function to test JSON response structure
test_json_endpoint() {
    local endpoint="$1"
    local expected_key="$2"
    local description="$3"

    echo -n "Testing $description... "

    if response=$(curl -s "$endpoint" 2>/dev/null); then
        if echo "$response" | jq -e ".$expected_key" >/dev/null 2>&1; then
            echo -e "${GREEN}✅ PASS${NC} (JSON valid, key '$expected_key' found)"
            ((TESTS_PASSED++))
        else
            echo -e "${RED}❌ FAIL${NC} (JSON invalid or key '$expected_key' missing)"
            echo "Response: $response"
            ((TESTS_FAILED++))
        fi
    else
        echo -e "${RED}❌ FAIL${NC} (connection error)"
        ((TESTS_FAILED++))
    fi
}

# Function to test Prometheus metrics
test_prometheus_metrics() {
    local endpoint="$1"
    local metric_name="$2"
    local description="$3"

    echo -n "Testing $description... "

    if response=$(curl -s "$endpoint" 2>/dev/null); then
        if echo "$response" | grep -q "^$metric_name"; then
            echo -e "${GREEN}✅ PASS${NC} (metric '$metric_name' found)"
            ((TESTS_PASSED++))
        else
            echo -e "${RED}❌ FAIL${NC} (metric '$metric_name' not found)"
            ((TESTS_FAILED++))
        fi
    else
        echo -e "${RED}❌ FAIL${NC} (connection error)"
        ((TESTS_FAILED++))
    fi
}

echo -e "${BLUE}📊 Testing Health Endpoints${NC}"
echo "================================"

# Test basic health endpoints
test_endpoint "$API_BASE_URL/health" "200" "Basic health check"
test_endpoint "$API_BASE_URL/ready" "200" "Readiness check"
test_endpoint "$API_BASE_URL/health/detailed" "200" "Detailed health check"

# Test health JSON structure
test_json_endpoint "$API_BASE_URL/health" "status" "Basic health JSON structure"
test_json_endpoint "$API_BASE_URL/ready" "ready" "Readiness JSON structure"
test_json_endpoint "$API_BASE_URL/health/detailed" "services" "Detailed health JSON structure"

echo ""
echo -e "${BLUE}📈 Testing Metrics Endpoints${NC}"
echo "===================================="

# Test metrics endpoints
test_endpoint "$API_BASE_URL/metrics" "200" "Primary Prometheus metrics"
test_endpoint "$API_BASE_URL/metrics/summary" "200" "Metrics summary"
test_endpoint "$API_BASE_URL/metrics/health" "200" "Health metrics"

# Test metrics JSON structure
test_json_endpoint "$API_BASE_URL/metrics/summary" "timestamp" "Metrics summary JSON"
test_json_endpoint "$API_BASE_URL/health/metrics" "timestamp" "Health metrics JSON"

echo ""
echo -e "${BLUE}🎯 Testing Prometheus Metrics Content${NC}"
echo "=========================================="

# Test specific metrics are exported
test_prometheus_metrics "$API_BASE_URL/metrics" "ainur_tasks_total" "Task metrics"
test_prometheus_metrics "$API_BASE_URL/metrics" "ainur_api_requests_total" "API request metrics"
test_prometheus_metrics "$API_BASE_URL/metrics" "ainur_agents_registered_total" "Agent metrics"
test_prometheus_metrics "$API_BASE_URL/metrics" "go_goroutines" "Go runtime metrics"
test_prometheus_metrics "$API_BASE_URL/metrics" "go_memstats_alloc_bytes" "Go memory metrics"

echo ""
echo -e "${BLUE}🔧 Testing Additional Endpoints${NC}"
echo "==================================="

# Test blockchain health endpoints (may fail if blockchain not connected)
test_endpoint "$API_BASE_URL/health/blockchain" "200" "Blockchain health (optional)"
test_endpoint "$API_BASE_URL/health/blockchain/metrics" "200" "Blockchain metrics (optional)"

# Test legacy endpoints for backward compatibility
test_endpoint "$API_BASE_URL/health/legacy" "200" "Legacy health endpoint"
test_endpoint "$API_BASE_URL/ready/legacy" "200" "Legacy readiness endpoint"

echo ""
echo -e "${BLUE}🌐 Testing External Services${NC}"
echo "================================"

# Test Prometheus (if available)
if command -v curl >/dev/null 2>&1; then
    test_endpoint "$PROMETHEUS_URL/api/v1/label/__name__/values" "200" "Prometheus API (optional)"
    test_endpoint "$PROMETHEUS_URL/-/healthy" "200" "Prometheus health (optional)"
fi

# Test Grafana (if available)
if command -v curl >/dev/null 2>&1; then
    test_endpoint "$GRAFANA_URL/api/health" "200" "Grafana health (optional)"
fi

echo ""
echo -e "${BLUE}📊 Performance Test${NC}"
echo "====================="

# Simple performance test
echo -n "Testing metrics endpoint performance... "
start_time=$(date +%s%N)
curl -s "$API_BASE_URL/metrics" >/dev/null 2>&1
end_time=$(date +%s%N)
duration=$(( (end_time - start_time) / 1000000 ))  # Convert to milliseconds

if [ $duration -lt 100 ]; then
    echo -e "${GREEN}✅ PASS${NC} (${duration}ms < 100ms threshold)"
    ((TESTS_PASSED++))
else
    echo -e "${YELLOW}⚠️  WARN${NC} (${duration}ms > 100ms threshold)"
    ((TESTS_FAILED++))
fi

echo ""
echo -e "${BLUE}📋 Test Summary${NC}"
echo "=================="
echo "Tests Passed: $TESTS_PASSED"
echo "Tests Failed: $TESTS_FAILED"
echo "Total Tests: $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}🎉 All tests passed! Monitoring setup is working correctly.${NC}"
    exit 0
else
    echo -e "${RED}⚠️  Some tests failed. Please check the configuration.${NC}"

    echo ""
    echo -e "${BLUE}🔧 Troubleshooting Tips${NC}"
    echo "========================"
    echo "1. Ensure the API server is running: ./api --host=0.0.0.0 --port=8080"
    echo "2. Check that metrics are enabled in the configuration"
    echo "3. Verify network connectivity to the API endpoints"
    echo "4. Check the API logs for any error messages"
    echo "5. Ensure all required dependencies are installed"
    echo ""
    echo "For more help, see MONITORING_SETUP.md"

    exit 1
fi