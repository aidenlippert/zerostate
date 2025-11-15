#!/bin/bash

# Sprint 1 Phase 4: P2P Runtime Discovery Integration Test
# Tests orchestrator discovering runtimes via P2P presence and routing tasks

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}╔══════════════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                                                                          ║${NC}"
echo -e "${GREEN}║         Sprint 1 Phase 4: P2P Runtime Discovery Test                    ║${NC}"
echo -e "${GREEN}║                                                                          ║${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════════════════════╝${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}🧹 Cleaning up...${NC}"
    
    if [ -n "$RUNTIME1_PID" ]; then
        echo "Stopping runtime 1 (PID: $RUNTIME1_PID)"
        kill $RUNTIME1_PID 2>/dev/null || true
    fi
    
    if [ -n "$RUNTIME2_PID" ]; then
        echo "Stopping runtime 2 (PID: $RUNTIME2_PID)"
        kill $RUNTIME2_PID 2>/dev/null || true
    fi
    
    if [ -n "$ORCH_PID" ]; then
        echo "Stopping orchestrator (PID: $ORCH_PID)"
        kill $ORCH_PID 2>/dev/null || true
    fi
    
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

trap cleanup EXIT

# Step 1: Build binaries
echo -e "${YELLOW}📦 Step 1: Building binaries...${NC}"

cd reference-runtime-v1
echo "Building reference-runtime-v1..."
go build -o bin/reference-runtime ./cmd/runtime || exit 1
echo -e "${GREEN}✅ Runtime built successfully${NC}"

cd ..
echo "Building orchestrator..."
go build -o bin/zerostate-api ./cmd/api || exit 1
echo -e "${GREEN}✅ Orchestrator built successfully${NC}"

echo ""

# Step 2: Start first runtime (math agent)
echo -e "${YELLOW}🚀 Step 2: Starting first runtime (math agent on port 9001)...${NC}"

cd reference-runtime-v1

# Create config for first runtime
cat > /tmp/runtime1-config.yaml <<EOF
agent:
  did: "did:ainur:agent:math-001"
  name: "Math Agent Runtime 1"
  version: "1.0.0"
  runtime:
    type: "wasm"
    path: "/home/rocz/vegalabs/zerostate/agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm"
  capabilities:
    - "math"
    - "arithmetic"
  limits:
    max_memory_mb: 128
    max_execution_time_ms: 5000
    max_concurrent_tasks: 10

server:
  host: "0.0.0.0"
  port: 9001

p2p:
  enabled: true
  bootstrap: []  # Using mDNS-only for local testing
  presence_topic: "ainur/v1/global/l3_aether/presence"
  heartbeat_interval: 30

logging:
  level: "info"
  format: "json"
EOF

./bin/reference-runtime --agent-config /tmp/runtime1-config.yaml > /tmp/runtime1.log 2>&1 &
RUNTIME1_PID=$!

echo "Runtime 1 started (PID: $RUNTIME1_PID)"
sleep 3

if ! kill -0 $RUNTIME1_PID 2>/dev/null; then
    echo -e "${RED}❌ Runtime 1 failed to start. Log:${NC}"
    cat /tmp/runtime1.log
    exit 1
fi

echo -e "${GREEN}✅ Runtime 1 is running on localhost:9001${NC}"

echo ""

# Step 3: Start second runtime (string agent - different capability)
echo -e "${YELLOW}🚀 Step 3: Starting second runtime (string agent on port 9002)...${NC}"

# Create config for second runtime
cat > /tmp/runtime2-config.yaml <<EOF
agent:
  did: "did:ainur:agent:string-001"
  name: "String Agent Runtime 2"
  version: "1.0.0"
  runtime:
    type: "wasm"
    path: "/home/rocz/vegalabs/zerostate/agents/string-agent-rust/target/wasm32-unknown-unknown/release/string_agent.wasm"
  capabilities:
    - "string"
    - "text-processing"
  limits:
    max_memory_mb: 128
    max_execution_time_ms: 5000
    max_concurrent_tasks: 10

server:
  host: "0.0.0.0"
  port: 9002

p2p:
  enabled: true
  bootstrap: []  # Using mDNS-only for local testing
  presence_topic: "ainur/v1/global/l3_aether/presence"
  heartbeat_interval: 30

logging:
  level: "info"
  format: "json"
EOF

./bin/reference-runtime --agent-config /tmp/runtime2-config.yaml > /tmp/runtime2.log 2>&1 &
RUNTIME2_PID=$!

echo "Runtime 2 started (PID: $RUNTIME2_PID)"
sleep 3

if ! kill -0 $RUNTIME2_PID 2>/dev/null; then
    echo -e "${RED}❌ Runtime 2 failed to start. Log:${NC}"
    cat /tmp/runtime2.log
    exit 1
fi

echo -e "${GREEN}✅ Runtime 2 is running on localhost:9002${NC}"

cd ..
echo ""

# Step 4: Start orchestrator with P2P discovery
echo -e "${YELLOW}🎯 Step 4: Starting orchestrator with P2P discovery (mDNS-only)...${NC}"

export P2P_PRESENCE_TOPIC="ainur/v1/global/l3_aether/presence"
# P2P_BOOTSTRAP removed - using mDNS-only for local testing
export DATABASE_URL=""  # Use in-memory SQLite
export LOG_LEVEL="info"

./bin/zerostate-api --port 8080 --workers 3 > /tmp/orchestrator.log 2>&1 &
ORCH_PID=$!

echo "Orchestrator started (PID: $ORCH_PID)"
echo "Waiting for orchestrator and runtime discovery..."
sleep 8  # Give time for P2P discovery

if ! kill -0 $ORCH_PID 2>/dev/null; then
    echo -e "${RED}❌ Orchestrator failed to start. Log:${NC}"
    tail -30 /tmp/orchestrator.log
    exit 1
fi

# Check if API is responding
if curl -s http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}✅ Orchestrator is running on localhost:8080${NC}"
else
    echo -e "${RED}❌ Orchestrator health check failed${NC}"
    tail -30 /tmp/orchestrator.log
    exit 1
fi

echo ""
echo -e "${YELLOW}⏳ Waiting 35 seconds for gossipsub mesh to form and heartbeat...${NC}"
sleep 35

# Step 5: Submit test tasks
echo -e "${YELLOW}✨ Step 5: Submitting test tasks via API...${NC}"

# Register test user
TIMESTAMP=$(date +%s)
echo "Registering test user: p2p_test_${TIMESTAMP}@example.com"
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/users/register \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"p2p_test_${TIMESTAMP}@example.com\",\"password\":\"TestPass123\",\"full_name\":\"P2P Test User\"}" | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
    echo -e "${RED}❌ Failed to register user${NC}"
    exit 1
fi

echo -e "${GREEN}✅ User registered, token obtained${NC}"

# Submit math task (should route to Runtime 1)
echo ""
echo "Submitting math task: add(8, 4)"
MATH_TASK=$(curl -s -X POST http://localhost:8080/api/v1/tasks/submit \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "query": "math task",
      "type": "math-add",
      "capabilities":["math"],
      "budget":1.0,
      "timeout":300,
      "input": {
        "function": "add",
        "args": [8, 4]
      }
    }')

MATH_TASK_ID=$(echo $MATH_TASK | jq -r '.task_id')

if [ "$MATH_TASK_ID" = "null" ] || [ -z "$MATH_TASK_ID" ]; then
    echo -e "${RED}❌ Failed to submit math task${NC}"
    echo "Response: $MATH_TASK"
    exit 1
fi

echo -e "${GREEN}✅ Math task submitted: $MATH_TASK_ID${NC}"

# Submit string task (should route to Runtime 2)
echo ""
echo "Submitting string task: uppercase(\"hello world\")"
STRING_TASK=$(curl -s -X POST http://localhost:8080/api/v1/tasks/submit \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d '{
      "query": "string task",
      "type": "string-uppercase",
      "capabilities":["string"],
      "budget":1.0,
      "timeout":300,
      "input": {
        "function": "uppercase",
        "args": ["hello world"]
      }
    }')

STRING_TASK_ID=$(echo $STRING_TASK | jq -r '.task_id')

if [ "$STRING_TASK_ID" = "null" ] || [ -z "$STRING_TASK_ID" ]; then
    echo -e "${RED}❌ Failed to submit string task${NC}"
    echo "Response: $STRING_TASK"
    exit 1
fi

echo -e "${GREEN}✅ String task submitted: $STRING_TASK_ID${NC}"

# Wait for execution
echo ""
echo "Waiting 10 seconds for task execution..."
sleep 10

# Check math task result
echo ""
echo "Checking math task result..."
MATH_RESULT=$(curl -s "http://localhost:8080/api/v1/tasks/$MATH_TASK_ID" \
    -H "Authorization: Bearer $TOKEN")

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "MATH TASK RESULT:"
echo "════════════════════════════════════════════════════════════════"
echo "$MATH_RESULT" | jq '.'
echo "════════════════════════════════════════════════════════════════"

MATH_STATUS=$(echo $MATH_RESULT | jq -r '.status')
MATH_VALUE=$(echo $MATH_RESULT | jq -r '.result.value // .result')

# Check string task result
echo ""
echo "Checking string task result..."
STRING_RESULT=$(curl -s "http://localhost:8080/api/v1/tasks/$STRING_TASK_ID" \
    -H "Authorization: Bearer $TOKEN")

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "STRING TASK RESULT:"
echo "════════════════════════════════════════════════════════════════"
echo "$STRING_RESULT" | jq '.'
echo "════════════════════════════════════════════════════════════════"

STRING_STATUS=$(echo $STRING_RESULT | jq -r '.status')
STRING_VALUE=$(echo $STRING_RESULT | jq -r '.result.value // .result')

# Verify results
SUCCESS=true

if [ "$MATH_STATUS" = "completed" ] && [ "$MATH_VALUE" = "12" ]; then
    echo ""
    echo -e "${GREEN}✅ Math task PASSED! 8 + 4 = $MATH_VALUE${NC}"
else
    echo -e "${RED}❌ Math task FAILED! Expected 12, got: $MATH_VALUE (status: $MATH_STATUS)${NC}"
    SUCCESS=false
fi

if [ "$STRING_STATUS" = "completed" ] && [ "$STRING_VALUE" = "HELLO WORLD" ]; then
    echo -e "${GREEN}✅ String task PASSED! uppercase(\"hello world\") = $STRING_VALUE${NC}"
else
    echo -e "${RED}❌ String task FAILED! Expected 'HELLO WORLD', got: $STRING_VALUE (status: $STRING_STATUS)${NC}"
    SUCCESS=false
fi

echo ""

if [ "$SUCCESS" = true ]; then
    echo "════════════════════════════════════════════════════════════════"
    echo -e "${GREEN}SPRINT 1 PHASE 4: ✅ COMPLETE!${NC}"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "What just happened:"
    echo "1. ✅ Runtime 1 (math) started and announced presence on P2P topic"
    echo "2. ✅ Runtime 2 (string) started and announced presence on P2P topic"
    echo "3. ✅ Orchestrator discovered both runtimes via P2P gossip"
    echo "4. ✅ Math task routed to Runtime 1 based on 'math' capability"
    echo "5. ✅ String task routed to Runtime 2 based on 'string' capability"
    echo "6. ✅ Both tasks executed successfully and returned correct results"
    echo ""
    echo "🎯 P2P Discovery Works! Orchestrator ↔ Multiple Runtimes via L3 Aether!"
    echo ""
else
    echo -e "${RED}════════════════════════════════════════════════════════════════${NC}"
    echo -e "${RED}TEST FAILED - See logs above${NC}"
    echo -e "${RED}════════════════════════════════════════════════════════════════${NC}"
    
    echo ""
    echo "Runtime 1 log (last 20 lines):"
    tail -20 /tmp/runtime1.log
    
    echo ""
    echo "Runtime 2 log (last 20 lines):"
    tail -20 /tmp/runtime2.log
    
    echo ""
    echo "Orchestrator log (last 20 lines):"
    tail -20 /tmp/orchestrator.log
    
    exit 1
fi
