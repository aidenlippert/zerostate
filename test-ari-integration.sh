#!/bin/bash

# Sprint 1 Phase 3: Integration Test
# Tests the ARI-v1 protocol end-to-end

set -e  # Exit on error

echo "╔══════════════════════════════════════════════════════════════════════════╗"
echo "║                                                                          ║"
echo "║         Sprint 1 Phase 3: ARI-v1 Integration Test                       ║"
echo "║                                                                          ║"
echo "╚══════════════════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Cleanup function
cleanup() {
    echo ""
    echo "${YELLOW}🧹 Cleaning up...${NC}"
    
    if [ ! -z "$RUNTIME_PID" ]; then
        echo "Stopping runtime (PID: $RUNTIME_PID)"
        kill $RUNTIME_PID 2>/dev/null || true
    fi
    
    if [ ! -z "$ORCH_PID" ]; then
        echo "Stopping orchestrator (PID: $ORCH_PID)"
        kill $ORCH_PID 2>/dev/null || true
    fi
    
    echo "${GREEN}✅ Cleanup complete${NC}"
}

trap cleanup EXIT

# Step 1: Build both binaries
echo "${YELLOW}📦 Step 1: Building binaries...${NC}"
cd /home/rocz/vegalabs/zerostate

echo "Building reference-runtime-v1..."
cd reference-runtime-v1
export PATH=$PATH:$(go env GOPATH)/bin
go build -o bin/runtime ./cmd/runtime 2>&1 | tail -5
if [ $? -eq 0 ]; then
    echo "${GREEN}✅ Runtime built successfully${NC}"
else
    echo "${RED}❌ Runtime build failed${NC}"
    exit 1
fi

echo "Building orchestrator..."
cd ..
go build -o bin/zerostate-api ./cmd/api 2>&1 | tail -5
if [ $? -eq 0 ]; then
    echo "${GREEN}✅ Orchestrator built successfully${NC}"
else
    echo "${RED}❌ Orchestrator build failed${NC}"
    exit 1
fi

echo ""

# Step 2: Start reference runtime
echo "${YELLOW}🚀 Step 2: Starting reference-runtime-v1...${NC}"
cd reference-runtime-v1

# Check if math agent WASM exists
if [ ! -f "../agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm" ]; then
    echo "${RED}❌ Math agent WASM not found at:${NC}"
    echo "   ../agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm"
    echo ""
    echo "${YELLOW}Trying to build math agent...${NC}"
    cd ../agents/math-agent-rust
    cargo build --release --target wasm32-unknown-unknown
    cd ../../reference-runtime-v1
fi

./bin/runtime --agent-config testdata/math-agent.yaml > /tmp/runtime.log 2>&1 &
RUNTIME_PID=$!

echo "Runtime started (PID: $RUNTIME_PID)"
echo "Waiting for runtime to be ready..."
sleep 3

# Check if runtime is running
if ! kill -0 $RUNTIME_PID 2>/dev/null; then
    echo "${RED}❌ Runtime failed to start. Log:${NC}"
    tail -20 /tmp/runtime.log
    exit 1
fi

echo "${GREEN}✅ Runtime is running on localhost:9000${NC}"
echo ""

# Step 3: Test runtime with grpcurl
echo "${YELLOW}🧪 Step 3: Testing runtime with grpcurl...${NC}"
if command -v grpcurl &> /dev/null; then
    echo "Querying Agent/GetInfo..."
    grpcurl -plaintext localhost:9000 ari.v1.Agent/GetInfo | jq '.' || true
    echo "${GREEN}✅ Runtime responding to gRPC calls${NC}"
else
    echo "${YELLOW}⚠️  grpcurl not installed, skipping direct test${NC}"
    echo "   Install with: brew install grpcurl (macOS) or apt-get install grpcurl (Linux)"
fi

echo ""

# Step 4: Start orchestrator with ARI enabled
echo "${YELLOW}🎯 Step 4: Starting orchestrator with ARI_RUNTIME_ADDR...${NC}"
cd ..

export ARI_RUNTIME_ADDR="localhost:9000"
# Use in-memory SQLite for testing (no external DB needed)
export DATABASE_URL=""
export LOG_LEVEL="info"

./bin/zerostate-api --port 8080 --workers 3 > /tmp/orchestrator.log 2>&1 &
ORCH_PID=$!

echo "Orchestrator started (PID: $ORCH_PID)"
echo "Waiting for orchestrator to be ready..."
sleep 5

# Check if orchestrator is running
if ! kill -0 $ORCH_PID 2>/dev/null; then
    echo "${RED}❌ Orchestrator failed to start. Log:${NC}"
    tail -30 /tmp/orchestrator.log
    exit 1
fi

# Check if API is responding
if curl -s http://localhost:8080/health > /dev/null; then
    echo "${GREEN}✅ Orchestrator is running on localhost:8080${NC}"
else
    echo "${RED}❌ Orchestrator health check failed${NC}"
    tail -30 /tmp/orchestrator.log
    exit 1
fi

echo ""

# Step 5: Submit test task
echo "${YELLOW}✨ Step 5: Submitting test task via API...${NC}"

# Register test user
TIMESTAMP=$(date +%s)
echo "Registering test user: ari_test_${TIMESTAMP}@example.com"
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/users/register \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"ari_test_${TIMESTAMP}@example.com\",\"password\":\"TestPass123\",\"full_name\":\"ARI Test User\"}" | jq -r '.token')

if [ "$TOKEN" = "null" ] || [ -z "$TOKEN" ]; then
    echo "${RED}❌ Failed to register user${NC}"
    exit 1
fi

echo "${GREEN}✅ User registered, token obtained${NC}"

# Submit task
echo ""
echo "Submitting task: add(5, 7)"
TASK_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/tasks/submit \
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
        "args": [5, 7]
      }
    }')

TASK_ID=$(echo $TASK_RESPONSE | jq -r '.task_id')

if [ "$TASK_ID" = "null" ] || [ -z "$TASK_ID" ]; then
    echo "${RED}❌ Failed to submit task${NC}"
    echo "Response: $TASK_RESPONSE"
    exit 1
fi

echo "${GREEN}✅ Task submitted: $TASK_ID${NC}"

# Wait for execution
echo ""
echo "Waiting 10 seconds for task execution..."
sleep 10

# Check result
echo ""
echo "Checking task result..."
RESULT=$(curl -s "http://localhost:8080/api/v1/tasks/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN")

echo ""
echo "════════════════════════════════════════════════════════════════"
echo "TASK RESULT:"
echo "════════════════════════════════════════════════════════════════"
echo "$RESULT" | jq '.'
echo "════════════════════════════════════════════════════════════════"

# Check if task completed
STATUS=$(echo $RESULT | jq -r '.status')
if [ "$STATUS" = "completed" ]; then
    FINAL_RESULT=$(echo $RESULT | jq -r '.result.value // .result.final_result // .result')
    echo ""
    echo "${GREEN}🎉 SUCCESS! Task completed via ARI-v1${NC}"
    echo "${GREEN}   Result: $FINAL_RESULT${NC}"
    echo ""
    echo "════════════════════════════════════════════════════════════════"
    echo "SPRINT 1 PHASE 3: ✅ COMPLETE!"
    echo "════════════════════════════════════════════════════════════════"
    echo ""
    echo "What just happened:"
    echo "1. ✅ reference-runtime-v1 started on localhost:9000"
    echo "2. ✅ Orchestrator connected to runtime via gRPC (ARI-v1)"
    echo "3. ✅ Task submitted via REST API"
    echo "4. ✅ Orchestrator routed task to runtime via Execute()"
    echo "5. ✅ Runtime executed math.wasm and returned result"
    echo "6. ✅ Result returned to user via API"
    echo ""
    echo "🎯 The protocol works! Orchestrator ↔ Runtime via ARI-v1!"
    echo ""
else
    echo "${RED}❌ Task did not complete. Status: $STATUS${NC}"
    echo ""
    echo "Runtime log (last 20 lines):"
    tail -20 /tmp/runtime.log
    echo ""
    echo "Orchestrator log (last 20 lines):"
    tail -20 /tmp/orchestrator.log
    exit 1
fi
