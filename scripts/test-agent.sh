#!/bin/bash

# Test Agent Communication
# Usage: ./test-agent.sh [agent_id]

set -e

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Configuration
API_PORT="${API_PORT:-8080}"
API_URL="http://localhost:$API_PORT"

# Get token
if [ -f "/tmp/zerostate-token.txt" ]; then
    TOKEN=$(cat /tmp/zerostate-token.txt)
else
    echo -e "${RED}❌ No authentication token found${NC}"
    echo "Run ./scripts/setup-local-network.sh first"
    exit 1
fi

# Get agent ID
AGENT_ID="$1"
if [ -z "$AGENT_ID" ]; then
    if [ -f "/tmp/zerostate-last-agent-id.txt" ]; then
        AGENT_ID=$(cat /tmp/zerostate-last-agent-id.txt)
        echo -e "${BLUE}Using last registered agent: $AGENT_ID${NC}"
    else
        echo "Usage: $0 [agent_id]"
        echo ""
        echo "No agent ID provided and no last agent found."
        echo "Register an agent first: ./scripts/register-agent.sh"
        exit 1
    fi
fi

echo -e "${BLUE}🧪 Testing Agent Communication${NC}"
echo "========================================"
echo "Agent ID: $AGENT_ID"
echo "API:      $API_URL"
echo ""

# Test 1: Get agent info
echo -e "${BLUE}Test 1: Fetching agent info...${NC}"
AGENT_INFO=$(curl -s "$API_URL/api/v1/agents/$AGENT_ID" \
    -H "Authorization: Bearer $TOKEN")

if echo "$AGENT_INFO" | jq -e '.error' >/dev/null 2>&1; then
    ERROR=$(echo "$AGENT_INFO" | jq -r '.error')
    echo -e "${RED}❌ Failed: $ERROR${NC}"
    exit 1
fi

AGENT_NAME=$(echo "$AGENT_INFO" | jq -r '.name')
AGENT_STATUS=$(echo "$AGENT_INFO" | jq -r '.status')
CAPABILITIES=$(echo "$AGENT_INFO" | jq -r '.capabilities')
PRICE=$(echo "$AGENT_INFO" | jq -r '.price')

echo -e "${GREEN}✓ Agent info retrieved${NC}"
echo "  Name:         $AGENT_NAME"
echo "  Status:       $AGENT_STATUS"
echo "  Capabilities: $CAPABILITIES"
echo "  Price:        \$$PRICE"
echo ""

# Test 2: Submit task to agent
echo -e "${BLUE}Test 2: Submitting test task...${NC}"

TASK_INPUT='{"message": "Hello from test script!", "timestamp": '$(date +%s)', "test_data": {"key": "value", "number": 42}}'

TASK_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/tasks/submit" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
        \"capabilities\": [\"echo\"],
        \"input\": $TASK_INPUT,
        \"budget\": 1.0,
        \"priority\": 1
    }")

if echo "$TASK_RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    ERROR=$(echo "$TASK_RESPONSE" | jq -r '.error')
    echo -e "${RED}❌ Task submission failed: $ERROR${NC}"
    echo ""
    echo "Full response:"
    echo "$TASK_RESPONSE" | jq .
    exit 1
fi

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.task_id // .id // empty')

if [ -z "$TASK_ID" ]; then
    echo -e "${YELLOW}⚠ Task submitted but no task ID returned${NC}"
    echo ""
    echo "Full response:"
    echo "$TASK_RESPONSE" | jq .
else
    echo -e "${GREEN}✓ Task submitted${NC}"
    echo "  Task ID: $TASK_ID"
    echo ""

    # Test 3: Check task status
    echo -e "${BLUE}Test 3: Checking task status...${NC}"

    MAX_ATTEMPTS=10
    ATTEMPT=0

    while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
        sleep 1

        TASK_STATUS_RESPONSE=$(curl -s "$API_URL/api/v1/tasks/$TASK_ID" \
            -H "Authorization: Bearer $TOKEN")

        TASK_STATUS=$(echo "$TASK_STATUS_RESPONSE" | jq -r '.status // empty')

        echo -n "  Attempt $((ATTEMPT + 1))/$MAX_ATTEMPTS: Status = $TASK_STATUS"

        if [ "$TASK_STATUS" = "COMPLETED" ]; then
            echo -e " ${GREEN}✓${NC}"
            echo ""
            echo -e "${GREEN}✅ Task completed successfully!${NC}"
            echo ""
            echo "Task Result:"
            echo "$TASK_STATUS_RESPONSE" | jq '.result // .'
            break
        elif [ "$TASK_STATUS" = "FAILED" ]; then
            echo -e " ${RED}✗${NC}"
            echo ""
            echo -e "${RED}❌ Task failed${NC}"
            echo ""
            echo "Error:"
            echo "$TASK_STATUS_RESPONSE" | jq '.error // .'
            exit 1
        else
            echo ""
        fi

        ATTEMPT=$((ATTEMPT + 1))
    done

    if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
        echo -e "${YELLOW}⚠ Task did not complete within timeout${NC}"
        echo "Last status: $TASK_STATUS"
    fi
fi

echo ""
echo "================================================"
echo -e "${GREEN}✅ Agent communication test complete!${NC}"
echo "================================================"
echo ""
echo "🎯 More Tests"
echo "================================================"
echo "View all tasks:   curl $API_URL/api/v1/tasks -H 'Authorization: Bearer $TOKEN' | jq"
echo "View all agents:  curl $API_URL/api/v1/agents -H 'Authorization: Bearer $TOKEN' | jq"
echo "View metrics:     curl $API_URL/metrics"
echo ""
