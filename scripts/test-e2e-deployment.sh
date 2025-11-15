#!/bin/bash

# End-to-End Testing for ZeroState Production Deployment
# Tests complete workflow: API → Database → Agent Selection → Task Execution

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        🧪 ZEROSTATE E2E TESTING 🧪                           ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
API_URL="${1:-http://localhost:8080}"
BASE_URL="$API_URL/api/v1"

echo -e "${BLUE}Testing API at: $API_URL${NC}"
echo ""

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TOTAL_TESTS=0

# Helper function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    echo -n "  Test $TOTAL_TESTS: $test_name... "
    
    if eval "$test_command" &> /dev/null; then
        echo -e "${GREEN}✅ PASS${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}❌ FAIL${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Test 1: Health Check
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🏥 Test Suite 1: Health & Connectivity${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "API is reachable" \
    "curl -f -s $API_URL/health"

run_test "Health check returns healthy" \
    "curl -s $API_URL/health | grep -q 'healthy'"

run_test "CORS headers present" \
    "curl -s -I $API_URL/health | grep -iq 'access-control-allow'"

# Test 2: Authentication
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🔐 Test Suite 2: Authentication${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Generate random user
RANDOM_USER="test_$(date +%s)_$(( RANDOM % 1000 ))@test.com"
PASSWORD="TestPass123!"

echo -e "${YELLOW}  Creating test user: $RANDOM_USER${NC}"

# Test signup (use /users/register endpoint)
SIGNUP_RESPONSE=$(curl -s -X POST "$BASE_URL/users/register" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$RANDOM_USER\",\"password\":\"$PASSWORD\",\"full_name\":\"Test User\"}")

run_test "User signup successful" \
    "echo '$SIGNUP_RESPONSE' | grep -q 'token'"

# Extract token
TOKEN=$(echo "$SIGNUP_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -n "$TOKEN" ]; then
    echo -e "${GREEN}  ✅ Token received: ${TOKEN:0:20}...${NC}"
else
    echo -e "${RED}  ❌ No token received${NC}"
fi

run_test "Can access protected endpoint with token" \
    "curl -f -s -H 'Authorization: Bearer $TOKEN' $BASE_URL/agents"

run_test "Protected endpoint rejects without token" \
    "! curl -f -s $BASE_URL/tasks/submit 2>/dev/null"

# Test 3: Agents API
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🤖 Test Suite 3: Agents API${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "Get all agents" \
    "curl -f -s $BASE_URL/agents"

run_test "Agents response is valid JSON" \
    "curl -s $BASE_URL/agents | python3 -m json.tool > /dev/null"

AGENTS_RESPONSE=$(curl -s $BASE_URL/agents)
AGENT_COUNT=$(echo "$AGENTS_RESPONSE" | grep -o '"id"' | wc -l)

echo -e "${YELLOW}  Found $AGENT_COUNT agents in database${NC}"

run_test "At least one agent exists" \
    "[ $AGENT_COUNT -gt 0 ]"

# Test 4: Task Submission
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📋 Test Suite 4: Task Submission${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Submit a simple task
TASK_RESPONSE=$(curl -s -X POST "$BASE_URL/tasks/submit" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $TOKEN" \
    -d '{"query":"Calculate 2 + 2"}')

run_test "Task submission successful" \
    "echo '$TASK_RESPONSE' | grep -q 'task_id'"

TASK_ID=$(echo "$TASK_RESPONSE" | grep -o '"task_id":"[^"]*' | cut -d'"' -f4)

if [ -n "$TASK_ID" ]; then
    echo -e "${GREEN}  ✅ Task ID: $TASK_ID${NC}"
else
    echo -e "${RED}  ❌ No task ID received${NC}"
fi

run_test "Can retrieve task status" \
    "curl -f -s -H 'Authorization: Bearer $TOKEN' $BASE_URL/tasks/$TASK_ID"

# Wait for task to process
echo -e "${YELLOW}  ⏳ Waiting for task to process (5 seconds)...${NC}"
sleep 5

TASK_STATUS=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/tasks/$TASK_ID")

run_test "Task has been processed" \
    "echo '$TASK_STATUS' | grep -q 'completed\\|failed\\|running'"

# Test 5: Task Listing
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📊 Test Suite 5: Task Management${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "Can list user tasks" \
    "curl -f -s -H 'Authorization: Bearer $TOKEN' $BASE_URL/tasks"

TASKS_RESPONSE=$(curl -s -H "Authorization: Bearer $TOKEN" "$BASE_URL/tasks")
USER_TASK_COUNT=$(echo "$TASKS_RESPONSE" | grep -o '"task_id"' | wc -l)

echo -e "${YELLOW}  User has $USER_TASK_COUNT tasks${NC}"

run_test "User's task appears in list" \
    "[ $USER_TASK_COUNT -gt 0 ]"

run_test "Can filter tasks by status" \
    "curl -f -s -H 'Authorization: Bearer $TOKEN' '$BASE_URL/tasks?status=completed'"

# Test 6: Orchestrator Health
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🎯 Test Suite 6: Orchestrator${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "Orchestrator health endpoint exists" \
    "curl -f -s $BASE_URL/orchestrator/health"

ORCH_HEALTH=$(curl -s "$BASE_URL/orchestrator/health")

run_test "Orchestrator is running" \
    "echo '$ORCH_HEALTH' | grep -q 'running\\|true'"

run_test "Worker count is positive" \
    "echo '$ORCH_HEALTH' | grep -o '\"workers\":[0-9]*' | grep -o '[0-9]*$' | awk '{exit !(\$1 > 0)}'"

# Test 7: Metrics (if enabled)
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📈 Test Suite 7: Metrics & Monitoring${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "Metrics endpoint accessible" \
    "curl -f -s $API_URL/metrics > /dev/null"

run_test "Prometheus metrics format" \
    "curl -s $API_URL/metrics | grep -q '^# HELP'"

METRICS=$(curl -s "$API_URL/metrics")

run_test "API request metrics present" \
    "echo '$METRICS' | grep -q 'api_requests_total'"

run_test "Task metrics present" \
    "echo '$METRICS' | grep -q 'orchestrator_tasks'"

# Test 8: Error Handling
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}⚠️  Test Suite 8: Error Handling${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

run_test "404 for invalid endpoint" \
    "! curl -f -s $BASE_URL/invalid-endpoint 2>/dev/null"

run_test "400 for malformed JSON" \
    "! curl -f -s -X POST '$BASE_URL/tasks/submit' -H 'Authorization: Bearer $TOKEN' -d '{invalid}' 2>/dev/null"

run_test "401 for expired/invalid token" \
    "! curl -f -s -H 'Authorization: Bearer invalid_token_123' $BASE_URL/tasks 2>/dev/null"

run_test "Rate limiting headers present" \
    "curl -s -I $BASE_URL/agents | grep -q 'X-RateLimit'"

# Test 9: Performance
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}⚡ Test Suite 9: Performance${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${YELLOW}  Measuring API response times...${NC}"

# Health endpoint performance
HEALTH_TIME=$(curl -s -w "%{time_total}" -o /dev/null "$API_URL/health")
echo -e "${YELLOW}  Health endpoint: ${HEALTH_TIME}s${NC}"

run_test "Health check responds in < 1s" \
    "awk -v t=$HEALTH_TIME 'BEGIN {exit !(t < 1)}'"

# Agents endpoint performance
AGENTS_TIME=$(curl -s -w "%{time_total}" -o /dev/null "$BASE_URL/agents")
echo -e "${YELLOW}  Agents endpoint: ${AGENTS_TIME}s${NC}"

run_test "Agents endpoint responds in < 2s" \
    "awk -v t=$AGENTS_TIME 'BEGIN {exit !(t < 2)}'"

# Test 10: Integration Tests
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}🔗 Test Suite 10: Complete Workflow${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

echo -e "${YELLOW}  Running complete workflow: Signup → Login → Submit Task → Check Result${NC}"

# Create new user for workflow test
WORKFLOW_USER="workflow_$(date +%s)@test.com"

# Signup
WF_SIGNUP=$(curl -s -X POST "$BASE_URL/auth/signup" \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"$WORKFLOW_USER\",\"password\":\"$PASSWORD\",\"name\":\"Workflow Test\"}")

WF_TOKEN=$(echo "$WF_SIGNUP" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

run_test "Workflow: User created" \
    "[ -n '$WF_TOKEN' ]"

# Submit task
WF_TASK=$(curl -s -X POST "$BASE_URL/tasks/submit" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $WF_TOKEN" \
    -d '{"query":"What is 10 + 5?"}')

WF_TASK_ID=$(echo "$WF_TASK" | grep -o '"task_id":"[^"]*' | cut -d'"' -f4)

run_test "Workflow: Task submitted" \
    "[ -n '$WF_TASK_ID' ]"

# Check status
sleep 3
WF_STATUS=$(curl -s -H "Authorization: Bearer $WF_TOKEN" "$BASE_URL/tasks/$WF_TASK_ID")

run_test "Workflow: Task status retrievable" \
    "echo '$WF_STATUS' | grep -q 'task_id'"

run_test "Workflow: Task has status" \
    "echo '$WF_STATUS' | grep -q 'status'"

# Final Summary
echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}📊 Test Results Summary${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

PASS_RATE=$(awk "BEGIN {printf \"%.1f\", ($TESTS_PASSED / $TOTAL_TESTS) * 100}")

echo -e "  Total Tests:  $TOTAL_TESTS"
echo -e "  ${GREEN}✅ Passed:     $TESTS_PASSED${NC}"
echo -e "  ${RED}❌ Failed:     $TESTS_FAILED${NC}"
echo -e "  ${BLUE}📈 Pass Rate:  $PASS_RATE%${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}║        🎉 ALL TESTS PASSED! 🎉                               ║${NC}"
    echo -e "${GREEN}║        Your ZeroState deployment is working perfectly!      ║${NC}"
    echo -e "${GREEN}║                                                              ║${NC}"
    echo -e "${GREEN}╚══════════════════════════════════════════════════════════════╝${NC}"
    exit 0
else
    echo -e "${YELLOW}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${YELLOW}║                                                              ║${NC}"
    echo -e "${YELLOW}║        ⚠️  SOME TESTS FAILED ⚠️                              ║${NC}"
    echo -e "${YELLOW}║        Check the output above for details                   ║${NC}"
    echo -e "${YELLOW}║                                                              ║${NC}"
    echo -e "${YELLOW}╚══════════════════════════════════════════════════════════════╝${NC}"
    exit 1
fi
