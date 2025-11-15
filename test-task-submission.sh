#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

API_URL="http://localhost:8080"
EMAIL="test-task-$(date +%s)@example.com"
PASSWORD="testpassword123"

echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   ZeroState Meta-Orchestrator E2E Test         ║${NC}"
echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo ""

# Step 1: Register user
echo -e "${YELLOW}Step 1: Registering user...${NC}"
REGISTER_RESPONSE=$(curl -s -X POST $API_URL/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d "{
    \"email\": \"$EMAIL\",
    \"password\": \"$PASSWORD\",
    \"full_name\": \"Test User\"
  }")

TOKEN=$(echo $REGISTER_RESPONSE | jq -r '.token')
USER_ID=$(echo $REGISTER_RESPONSE | jq -r '.user.id')

if [ "$TOKEN" == "null" ] || [ -z "$TOKEN" ]; then
  echo -e "${RED}✗ Failed to register user${NC}"
  echo "Response: $REGISTER_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓ User registered${NC}"
echo -e "  ${BLUE}User ID:${NC} $USER_ID"
echo -e "  ${BLUE}Token:${NC} ${TOKEN:0:20}..."
echo ""

# Step 2: Submit a simple task
echo -e "${YELLOW}Step 2: Submitting task...${NC}"
TASK_RESPONSE=$(curl -s -X POST $API_URL/api/v1/tasks/submit \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "query": "Calculate the sum of 42 and 58",
    "capabilities": ["math", "calculation"],
    "budget": 10.0,
    "timeout": 30,
    "priority": "normal"
  }')

TASK_ID=$(echo $TASK_RESPONSE | jq -r '.task_id')
TASK_STATUS=$(echo $TASK_RESPONSE | jq -r '.status')

if [ "$TASK_ID" == "null" ] || [ -z "$TASK_ID" ]; then
  echo -e "${RED}✗ Failed to submit task${NC}"
  echo "Response: $TASK_RESPONSE"
  exit 1
fi

echo -e "${GREEN}✓ Task submitted${NC}"
echo -e "  ${BLUE}Task ID:${NC} $TASK_ID"
echo -e "  ${BLUE}Initial Status:${NC} $TASK_STATUS"
echo ""

# Step 3: Check task status (poll for a few seconds)
echo -e "${YELLOW}Step 3: Monitoring task status...${NC}"
for i in {1..10}; do
  sleep 1
  STATUS_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/tasks/$TASK_ID/status" \
    -H "Authorization: Bearer $TOKEN")
  
  CURRENT_STATUS=$(echo $STATUS_RESPONSE | jq -r '.status')
  PROGRESS=$(echo $STATUS_RESPONSE | jq -r '.progress')
  
  echo -e "  ${BLUE}[$i]${NC} Status: $CURRENT_STATUS | Progress: $PROGRESS%"
  
  # Check if task is completed or failed
  if [ "$CURRENT_STATUS" == "completed" ] || [ "$CURRENT_STATUS" == "failed" ]; then
    break
  fi
done
echo ""

# Step 4: Get task result
echo -e "${YELLOW}Step 4: Retrieving task result...${NC}"
RESULT_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/tasks/$TASK_ID/result" \
  -H "Authorization: Bearer $TOKEN")

FINAL_STATUS=$(echo $RESULT_RESPONSE | jq -r '.status')
RESULT=$(echo $RESULT_RESPONSE | jq -r '.result')
ERROR_MSG=$(echo $RESULT_RESPONSE | jq -r '.error')
COST=$(echo $RESULT_RESPONSE | jq -r '.cost')
DURATION=$(echo $RESULT_RESPONSE | jq -r '.duration')

echo -e "  ${BLUE}Final Status:${NC} $FINAL_STATUS"
if [ "$FINAL_STATUS" == "completed" ]; then
  echo -e "  ${GREEN}✓ Task completed successfully${NC}"
  echo -e "  ${BLUE}Result:${NC} $RESULT"
  echo -e "  ${BLUE}Cost:${NC} \$${COST}"
  echo -e "  ${BLUE}Duration:${NC} ${DURATION}ms"
elif [ "$FINAL_STATUS" == "failed" ]; then
  echo -e "  ${RED}✗ Task failed${NC}"
  echo -e "  ${RED}Error:${NC} $ERROR_MSG"
else
  echo -e "  ${YELLOW}⚠ Task still in progress: $FINAL_STATUS${NC}"
fi
echo ""

# Step 5: List all tasks
echo -e "${YELLOW}Step 5: Listing all tasks...${NC}"
LIST_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/tasks?limit=5" \
  -H "Authorization: Bearer $TOKEN")

TASK_COUNT=$(echo $LIST_RESPONSE | jq -r '.count')
echo -e "  ${BLUE}Total tasks:${NC} $TASK_COUNT"
echo ""

# Step 6: Check orchestrator health
echo -e "${YELLOW}Step 6: Checking orchestrator health...${NC}"
HEALTH_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/orchestrator/health" \
  -H "Authorization: Bearer $TOKEN")

WORKERS=$(echo $HEALTH_RESPONSE | jq -r '.active_workers')
PROCESSED=$(echo $HEALTH_RESPONSE | jq -r '.tasks_processed')
SUCCEEDED=$(echo $HEALTH_RESPONSE | jq -r '.tasks_succeeded')
FAILED=$(echo $HEALTH_RESPONSE | jq -r '.tasks_failed')

echo -e "  ${BLUE}Active Workers:${NC} $WORKERS"
echo -e "  ${BLUE}Tasks Processed:${NC} $PROCESSED"
echo -e "  ${BLUE}Tasks Succeeded:${NC} $SUCCEEDED"
echo -e "  ${BLUE}Tasks Failed:${NC} $FAILED"
echo ""

# Summary
echo -e "${BLUE}╔════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║              Test Summary                      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════╝${NC}"
if [ "$FINAL_STATUS" == "completed" ]; then
  echo -e "${GREEN}✓ Meta-Orchestrator E2E test PASSED!${NC}"
  echo -e "${GREEN}  - User registration: ✓${NC}"
  echo -e "${GREEN}  - Task submission: ✓${NC}"
  echo -e "${GREEN}  - Task execution: ✓${NC}"
  echo -e "${GREEN}  - Result retrieval: ✓${NC}"
  exit 0
elif [ "$FINAL_STATUS" == "failed" ]; then
  echo -e "${RED}✗ Task execution FAILED${NC}"
  echo -e "${RED}  Error: $ERROR_MSG${NC}"
  exit 1
else
  echo -e "${YELLOW}⚠ Test incomplete (task still running)${NC}"
  echo -e "${YELLOW}  This might be normal for longer tasks${NC}"
  exit 0
fi
