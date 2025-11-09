#!/bin/bash

# Test Agent Auction System
# Registers multiple agents with different prices and tests auction selection

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

echo -e "${BLUE}🎯 Testing Agent Auction System${NC}"
echo "========================================"
echo ""

# Check if echo agent exists
ECHO_AGENT_WASM="examples/agents/echo-agent/dist/echo-agent.wasm"
if [ ! -f "$ECHO_AGENT_WASM" ]; then
    echo -e "${YELLOW}Building echo agent...${NC}"
    cd examples/agents/echo-agent
    ./build.sh
    cd ../../..
fi

# Register 3 agents with different prices
echo -e "${BLUE}Step 1: Registering 3 agents with different prices...${NC}"
echo ""

AGENTS=()

# Agent 1: Cheap ($0.50)
echo "Registering CheapAgent (\$0.50)..."
RESPONSE1=$(curl -s -X POST "$API_URL/api/v1/agents/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "wasm_binary=@$ECHO_AGENT_WASM" \
    -F "name=CheapAgent" \
    -F "description=Low-cost agent for testing" \
    -F "version=1.0.0" \
    -F "capabilities=echo,test" \
    -F "price=0.50")

AGENT1_ID=$(echo "$RESPONSE1" | jq -r '.agent_id')
if [ ! -z "$AGENT1_ID" ] && [ "$AGENT1_ID" != "null" ]; then
    AGENTS+=("$AGENT1_ID")
    echo -e "${GREEN}✓ CheapAgent registered: $AGENT1_ID${NC}"
else
    echo -e "${RED}✗ Failed to register CheapAgent${NC}"
fi

sleep 1

# Agent 2: Mid-price ($1.50)
echo "Registering MidAgent (\$1.50)..."
RESPONSE2=$(curl -s -X POST "$API_URL/api/v1/agents/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "wasm_binary=@$ECHO_AGENT_WASM" \
    -F "name=MidAgent" \
    -F "description=Medium-cost agent for testing" \
    -F "version=1.0.0" \
    -F "capabilities=echo,test" \
    -F "price=1.50")

AGENT2_ID=$(echo "$RESPONSE2" | jq -r '.agent_id')
if [ ! -z "$AGENT2_ID" ] && [ "$AGENT2_ID" != "null" ]; then
    AGENTS+=("$AGENT2_ID")
    echo -e "${GREEN}✓ MidAgent registered: $AGENT2_ID${NC}"
else
    echo -e "${RED}✗ Failed to register MidAgent${NC}"
fi

sleep 1

# Agent 3: Premium ($3.00)
echo "Registering PremiumAgent (\$3.00)..."
RESPONSE3=$(curl -s -X POST "$API_URL/api/v1/agents/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "wasm_binary=@$ECHO_AGENT_WASM" \
    -F "name=PremiumAgent" \
    -F "description=High-cost agent for testing" \
    -F "version=1.0.0" \
    -F "capabilities=echo,test" \
    -F "price=3.00")

AGENT3_ID=$(echo "$RESPONSE3" | jq -r '.agent_id')
if [ ! -z "$AGENT3_ID" ] && [ "$AGENT3_ID" != "null" ]; then
    AGENTS+=("$AGENT3_ID")
    echo -e "${GREEN}✓ PremiumAgent registered: $AGENT3_ID${NC}"
else
    echo -e "${RED}✗ Failed to register PremiumAgent${NC}"
fi

echo ""
echo -e "${GREEN}✓ Registered ${#AGENTS[@]} agents${NC}"
echo ""

# List all agents
echo -e "${BLUE}Step 2: Viewing registered agents...${NC}"
AGENTS_LIST=$(curl -s "$API_URL/api/v1/agents" \
    -H "Authorization: Bearer $TOKEN")

echo "$AGENTS_LIST" | jq -r '.agents[] | select(.capabilities | contains("echo")) | "\(.name) - $\(.price) - \(.id)"'
echo ""

# Test auction by submitting task
echo -e "${BLUE}Step 3: Submitting task to trigger auction...${NC}"
echo "Budget: \$5.00 (all agents can participate)"
echo ""

TASK_RESPONSE=$(curl -s -X POST "$API_URL/api/v1/tasks/submit" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
        "capabilities": ["echo"],
        "input": {"message": "Testing auction system", "timestamp": '$(date +%s)'},
        "budget": 5.0,
        "priority": 1
    }')

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.task_id // .id // empty')

if [ -z "$TASK_ID" ]; then
    echo -e "${RED}❌ Task submission failed${NC}"
    echo "$TASK_RESPONSE" | jq .
    exit 1
fi

echo -e "${GREEN}✓ Task submitted: $TASK_ID${NC}"
echo ""

# Wait and check which agent was selected
echo -e "${BLUE}Step 4: Checking auction winner...${NC}"
sleep 2

TASK_STATUS=$(curl -s "$API_URL/api/v1/tasks/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN")

ASSIGNED_AGENT=$(echo "$TASK_STATUS" | jq -r '.assigned_agent // .agent_id // empty')
TASK_STATE=$(echo "$TASK_STATUS" | jq -r '.status // empty')

if [ ! -z "$ASSIGNED_AGENT" ]; then
    # Get agent details
    AGENT_DETAILS=$(curl -s "$API_URL/api/v1/agents/$ASSIGNED_AGENT" \
        -H "Authorization: Bearer $TOKEN")

    WINNER_NAME=$(echo "$AGENT_DETAILS" | jq -r '.name')
    WINNER_PRICE=$(echo "$AGENT_DETAILS" | jq -r '.price')

    echo -e "${GREEN}✓ Auction completed${NC}"
    echo ""
    echo "================================================"
    echo "🏆 Auction Winner"
    echo "================================================"
    echo "Agent:  $WINNER_NAME"
    echo "ID:     $ASSIGNED_AGENT"
    echo "Price:  \$$WINNER_PRICE"
    echo "Status: $TASK_STATE"
    echo ""

    # Analyze auction result
    echo "================================================"
    echo "📊 Auction Analysis"
    echo "================================================"

    if [ "$WINNER_PRICE" = "0.50" ]; then
        echo -e "${GREEN}✓ Correct! CheapAgent won (lowest price)${NC}"
        echo "Expected: MetaAgent should select lowest-priced agent"
        echo "Result:   CheapAgent (\$0.50) was selected"
    elif [ "$WINNER_PRICE" = "1.50" ]; then
        echo -e "${YELLOW}⚠ MidAgent won${NC}"
        echo "This might indicate quality/reputation weighting"
    elif [ "$WINNER_PRICE" = "3.00" ]; then
        echo -e "${YELLOW}⚠ PremiumAgent won${NC}"
        echo "This might indicate quality/reputation weighting"
    fi

    echo ""
    echo "Scoring weights (from MetaAgent config):"
    echo "  Price:      30%"
    echo "  Quality:    30%"
    echo "  Speed:      20%"
    echo "  Reputation: 20%"

else
    echo -e "${YELLOW}⚠ No agent assigned yet${NC}"
    echo "Task Status:"
    echo "$TASK_STATUS" | jq .
fi

echo ""
echo "================================================"
echo "🎯 Auction Metrics"
echo "================================================"

# Try to get auction metrics from Prometheus endpoint
METRICS=$(curl -s "$API_URL/metrics" 2>/dev/null || echo "")

if echo "$METRICS" | grep -q "zerostate_meta_agent"; then
    echo ""
    echo "Meta-Agent Metrics:"
    echo "$METRICS" | grep "zerostate_meta_agent" | grep -v "^#"
else
    echo "Metrics endpoint not available or no auction metrics yet"
fi

echo ""
echo "================================================"
echo -e "${GREEN}✅ Auction test complete!${NC}"
echo "================================================"
echo ""
echo "🧪 More Tests"
echo "================================================"
echo "Test with low budget:    Modify budget to \$0.30 (only CheapAgent qualifies)"
echo "Test with requirements:  Add more capabilities to filter agents"
echo "View task history:       curl $API_URL/api/v1/tasks -H 'Authorization: Bearer $TOKEN' | jq"
echo ""
