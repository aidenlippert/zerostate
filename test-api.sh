#!/bin/bash

# ZeroState API End-to-End Test Script
# Tests all Tier 1 features in production

set -e

API_URL="https://zerostate-api.fly.dev"
TEST_EMAIL="test-$(date +%s)@example.com"
TEST_PASSWORD="SecurePassword123!"

echo "======================================"
echo "ZeroState API E2E Test Suite"
echo "======================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Test 1: Health Check
echo "Test 1: Health Check"
HEALTH=$(curl -s $API_URL/health)
if echo "$HEALTH" | jq -e '.status == "healthy"' > /dev/null; then
    success "Health check passed"
    echo "$HEALTH" | jq
else
    error "Health check failed"
    exit 1
fi
echo ""

# Test 2: User Registration
echo "Test 2: User Registration"
info "Email: $TEST_EMAIL"
REGISTER_RESPONSE=$(curl -s -X POST $API_URL/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}")

if echo "$REGISTER_RESPONSE" | jq -e '.user_id' > /dev/null; then
    success "User registration successful"
    USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.user_id')
    echo "$REGISTER_RESPONSE" | jq
else
    error "User registration failed"
    echo "$REGISTER_RESPONSE" | jq
    exit 1
fi
echo ""

# Test 3: User Login
echo "Test 3: User Login"
LOGIN_RESPONSE=$(curl -s -X POST $API_URL/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\"}")

if echo "$LOGIN_RESPONSE" | jq -e '.token' > /dev/null; then
    success "User login successful"
    TOKEN=$(echo "$LOGIN_RESPONSE" | jq -r '.token')
    info "JWT Token received (length: ${#TOKEN})"
else
    error "User login failed"
    echo "$LOGIN_RESPONSE" | jq
    exit 1
fi
echo ""

# Test 4: Create Test WASM File
echo "Test 4: Creating Test WASM File"
# WASM magic number + version
echo -ne '\x00\x61\x73\x6d\x01\x00\x00\x00' > /tmp/test-agent.wasm
if [ -f /tmp/test-agent.wasm ]; then
    success "Test WASM file created ($(stat -f%z /tmp/test-agent.wasm 2>/dev/null || stat -c%s /tmp/test-agent.wasm) bytes)"
else
    error "Failed to create test WASM file"
    exit 1
fi
echo ""

# Test 5: Agent Upload
echo "Test 5: Agent Upload"
UPLOAD_RESPONSE=$(curl -s -X POST $API_URL/api/v1/agents//binary \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@/tmp/test-agent.wasm" \
  -F "name=TestAgent-$(date +%s)" \
  -F "description=End-to-end test agent" \
  -F "capabilities=compute,storage")

if echo "$UPLOAD_RESPONSE" | jq -e '.agent_id' > /dev/null; then
    success "Agent upload successful"
    AGENT_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.agent_id')
    BINARY_URL=$(echo "$UPLOAD_RESPONSE" | jq -r '.binary_url')
    info "Agent ID: $AGENT_ID"
    info "Binary URL: $BINARY_URL"
    echo "$UPLOAD_RESPONSE" | jq
else
    error "Agent upload failed"
    echo "$UPLOAD_RESPONSE" | jq
fi
echo ""

# Test 6: List Agents
echo "Test 6: List Agents"
AGENTS_RESPONSE=$(curl -s $API_URL/api/v1/agents \
  -H "Authorization: Bearer $TOKEN")

if echo "$AGENTS_RESPONSE" | jq -e '.agents' > /dev/null; then
    success "Agent listing successful"
    AGENT_COUNT=$(echo "$AGENTS_RESPONSE" | jq '.agents | length')
    info "Total agents: $AGENT_COUNT"
    echo "$AGENTS_RESPONSE" | jq
else
    error "Agent listing failed"
    echo "$AGENTS_RESPONSE" | jq
fi
echo ""

# Test 7: Task Submission
echo "Test 7: Task Submission"
TASK_RESPONSE=$(curl -s -X POST $API_URL/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "description": "E2E test computation task",
    "requirements": {
      "memory_mb": 512,
      "cpu_cores": 1,
      "timeout_seconds": 60
    },
    "priority": 5
  }')

if echo "$TASK_RESPONSE" | jq -e '.task_id' > /dev/null; then
    success "Task submission successful"
    TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.task_id')
    info "Task ID: $TASK_ID"
    echo "$TASK_RESPONSE" | jq
else
    error "Task submission failed"
    echo "$TASK_RESPONSE" | jq
fi
echo ""

# Test 8: Task Queue Stats
echo "Test 8: Task Queue Stats"
STATS_RESPONSE=$(curl -s $API_URL/api/v1/tasks/stats \
  -H "Authorization: Bearer $TOKEN")

if echo "$STATS_RESPONSE" | jq -e '.total_queued' > /dev/null; then
    success "Task stats retrieval successful"
    echo "$STATS_RESPONSE" | jq
else
    error "Task stats retrieval failed"
    echo "$STATS_RESPONSE" | jq
fi
echo ""

# Test 9: WebSocket Stats
echo "Test 9: WebSocket Stats"
WS_STATS=$(curl -s $API_URL/api/v1/ws/stats \
  -H "Authorization: Bearer $TOKEN")

if echo "$WS_STATS" | jq -e '.stats' > /dev/null; then
    success "WebSocket stats retrieval successful"
    echo "$WS_STATS" | jq
else
    error "WebSocket stats retrieval failed"
    echo "$WS_STATS" | jq
fi
echo ""

# Summary
echo "======================================"
echo "Test Summary"
echo "======================================"
success "All tests completed!"
echo ""
echo "Test User Credentials:"
echo "  Email: $TEST_EMAIL"
echo "  Password: $TEST_PASSWORD"
echo "  User ID: $USER_ID"
echo "  JWT Token: ${TOKEN:0:50}..."
echo ""
echo "Created Resources:"
echo "  Agent ID: $AGENT_ID"
echo "  Task ID: $TASK_ID"
echo ""
info "Next steps: Test WebSocket connection in browser console"
echo "  const ws = new WebSocket('wss://zerostate-api.fly.dev/api/v1/ws/connect');"
echo "  ws.onopen = () => console.log('Connected!');"
echo "  ws.onmessage = (e) => console.log('Message:', JSON.parse(e.data));"
echo ""
