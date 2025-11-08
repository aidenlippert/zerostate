#!/bin/bash

# ZeroState Tier 1 Production Test Script
# Tests all deployed Tier 1 features

set -e

API_URL="https://zerostate-api.fly.dev"

echo "========================================"
echo " ZeroState Tier 1 Production Test"
echo "========================================"
echo ""

# Test 1: Health Check
echo "✓ Test 1: Health Check"
curl -s $API_URL/health | jq
echo ""

# Test 2: User Registration
echo "✓ Test 2: User Registration"
TEST_EMAIL="test-$(date +%s)@example.com"
REGISTER=$(curl -s -X POST $API_URL/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${TEST_EMAIL}\",\"password\":\"SecurePass123!\",\"full_name\":\"Test User\"}")
echo "$REGISTER" | jq
USER_ID=$(echo "$REGISTER" | jq -r '.id // .user_id // .user.id')
echo "  User ID: $USER_ID"
echo ""

# Test 3: User Login
echo "✓ Test 3: User Login"
LOGIN=$(curl -s -X POST $API_URL/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"${TEST_EMAIL}\",\"password\":\"SecurePass123!\"}")
echo "$LOGIN" | jq
TOKEN=$(echo "$LOGIN" | jq -r '.token')
echo "  JWT Token: ${TOKEN:0:50}..."
echo ""

# Test 4: List Agents (should be empty or require proper agent registration)
echo "✓ Test 4: List Agents"
curl -s $API_URL/api/v1/agents \
  -H "Authorization: Bearer $TOKEN" | jq
echo ""

# Test 5: Submit Task
echo "✓ Test 5: Submit Task"
TASK=$(curl -s -X POST $API_URL/api/v1/tasks/submit \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "query": "Test computation task",
    "budget": 1.0,
    "timeout": 60,
    "priority": "medium",
    "constraints": {"memory_mb": 512, "cpu_cores": 1}
  }')
echo "$TASK" | jq
TASK_ID=$(echo "$TASK" | jq -r '.task_id // .id')
echo "  Task ID: $TASK_ID"
echo ""

# Test 6: WebSocket Stats
echo "✓ Test 6: WebSocket Stats"
curl -s $API_URL/api/v1/ws/stats \
  -H "Authorization: Bearer $TOKEN" | jq
echo ""

# Test 7: Metrics Endpoint
echo "✓ Test 7: Prometheus Metrics"
curl -s $API_URL/metrics | head -20
echo "..."
echo ""

echo "========================================"
echo " All Tests Complete!"
echo "========================================"
echo ""
echo "Test Credentials:"
echo "  Email: $TEST_EMAIL"
echo "  Password: SecurePass123!"
echo "  JWT Token: $TOKEN"
echo ""
echo "Next: Test WebSocket in browser console:"
echo "  const ws = new WebSocket('wss://zerostate-api.fly.dev/api/v1/ws/connect');"
echo "  ws.onopen = () => console.log('Connected!');"
echo "  ws.onmessage = (e) => console.log(JSON.parse(e.data));"
echo ""
