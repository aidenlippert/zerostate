#!/bin/bash

# Register Agent on ZeroState Network
# Usage: ./register-agent.sh <path-to-wasm> [name] [capabilities] [price]

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

# Parse arguments
WASM_PATH="$1"
AGENT_NAME="${2:-EchoAgent}"
CAPABILITIES="${3:-echo,test}"
PRICE="${4:-0.10}"

if [ -z "$WASM_PATH" ]; then
    echo "Usage: $0 <path-to-wasm> [name] [capabilities] [price]"
    echo ""
    echo "Examples:"
    echo "  $0 examples/agents/echo-agent/dist/echo-agent.wasm"
    echo "  $0 agent.wasm MyAgent \"image-processing,ml-inference\" 2.50"
    exit 1
fi

if [ ! -f "$WASM_PATH" ]; then
    echo -e "${RED}❌ WASM file not found: $WASM_PATH${NC}"
    exit 1
fi

echo -e "${BLUE}🤖 Registering Agent on ZeroState Network${NC}"
echo "=========================================="
echo "WASM File:     $WASM_PATH"
echo "Name:          $AGENT_NAME"
echo "Capabilities:  $CAPABILITIES"
echo "Price:         \$$PRICE"
echo "API:           $API_URL"
echo ""

# Get file size
FILE_SIZE=$(du -h "$WASM_PATH" | cut -f1)
echo "File Size:     $FILE_SIZE"
echo ""

# Register agent
echo -e "${BLUE}Uploading agent...${NC}"

RESPONSE=$(curl -s -X POST "$API_URL/api/v1/agents/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "wasm_binary=@$WASM_PATH" \
    -F "name=$AGENT_NAME" \
    -F "description=Agent registered via script" \
    -F "version=1.0.0" \
    -F "capabilities=$CAPABILITIES" \
    -F "price=$PRICE")

# Check for errors
if echo "$RESPONSE" | jq -e '.error' >/dev/null 2>&1; then
    ERROR=$(echo "$RESPONSE" | jq -r '.error')
    echo -e "${RED}❌ Registration failed: $ERROR${NC}"
    echo ""
    echo "Full response:"
    echo "$RESPONSE" | jq .
    exit 1
fi

# Extract agent ID
AGENT_ID=$(echo "$RESPONSE" | jq -r '.agent_id')
BINARY_URL=$(echo "$RESPONSE" | jq -r '.binary_url')
STATUS=$(echo "$RESPONSE" | jq -r '.status')

if [ -z "$AGENT_ID" ] || [ "$AGENT_ID" = "null" ]; then
    echo -e "${RED}❌ Registration failed - no agent ID returned${NC}"
    echo ""
    echo "Full response:"
    echo "$RESPONSE" | jq .
    exit 1
fi

echo -e "${GREEN}✅ Agent registered successfully!${NC}"
echo ""
echo "================================================"
echo "📋 Agent Details"
echo "================================================"
echo "Agent ID:      $AGENT_ID"
echo "Status:        $STATUS"
echo "Binary URL:    $BINARY_URL"
echo ""

# Save agent ID for testing
echo "$AGENT_ID" > /tmp/zerostate-last-agent-id.txt

# Verify agent is in database
echo -e "${BLUE}Verifying registration...${NC}"

VERIFY_RESPONSE=$(curl -s "$API_URL/api/v1/agents" \
    -H "Authorization: Bearer $TOKEN")

if echo "$VERIFY_RESPONSE" | jq -e ".agents[] | select(.id == \"$AGENT_ID\")" >/dev/null 2>&1; then
    echo -e "${GREEN}✓ Agent found in database${NC}"

    # Show agent info
    echo ""
    echo "Agent Info:"
    echo "$VERIFY_RESPONSE" | jq ".agents[] | select(.id == \"$AGENT_ID\")"
else
    echo -e "${YELLOW}⚠ Agent not found in database query${NC}"
fi

echo ""
echo "🎯 Next Steps"
echo "================================================"
echo "Test agent:       ./scripts/test-agent.sh $AGENT_ID"
echo "View agents:      curl $API_URL/api/v1/agents -H 'Authorization: Bearer $TOKEN' | jq"
echo "Submit task:      ./scripts/submit-task.sh $AGENT_ID"
echo ""
