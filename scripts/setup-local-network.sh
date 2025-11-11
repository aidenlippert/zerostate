#!/bin/bash

# ZeroState Local Network Setup
# Sets up a complete local development environment for agent testing

set -e

echo "🚀 ZeroState Local Network Setup"
echo "================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
POSTGRES_PASSWORD="${POSTGRES_PASSWORD:-zerostate_dev}"
JWT_SECRET="${JWT_SECRET:-dev_secret_change_in_production}"
API_PORT="${API_PORT:-8080}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to wait for service
wait_for_service() {
    local host=$1
    local port=$2
    local service=$3
    local max_attempts=30
    local attempt=0

    echo -n "Waiting for $service to be ready..."
    while ! nc -z $host $port 2>/dev/null; do
        attempt=$((attempt + 1))
        if [ $attempt -ge $max_attempts ]; then
            echo -e " ${YELLOW}TIMEOUT${NC}"
            return 1
        fi
        echo -n "."
        sleep 1
    done
    echo -e " ${GREEN}✓${NC}"
    return 0
}

# Check prerequisites
echo -e "${BLUE}Checking prerequisites...${NC}"

if ! command_exists docker; then
    echo -e "${YELLOW}⚠ Docker not found. Please install Docker first.${NC}"
    exit 1
fi

if ! command_exists docker-compose; then
    echo -e "${YELLOW}⚠ docker-compose not found. Please install docker-compose first.${NC}"
    exit 1
fi

if ! command_exists go; then
    echo -e "${YELLOW}⚠ Go not found. Please install Go 1.21+ first.${NC}"
    exit 1
fi

if ! command_exists nc; then
    echo -e "${YELLOW}⚠ netcat (nc) not found. Installing...${NC}"
    # Try to install netcat
    if command_exists apt-get; then
        sudo apt-get install -y netcat
    elif command_exists yum; then
        sudo yum install -y nc
    else
        echo "Please install netcat manually"
        exit 1
    fi
fi

echo -e "${GREEN}✓ All prerequisites satisfied${NC}"
echo ""

# Step 1: Start infrastructure
echo -e "${BLUE}Step 1: Starting infrastructure (PostgreSQL, Redis)...${NC}"
docker-compose up -d postgres redis

# Wait for PostgreSQL
wait_for_service localhost 5432 "PostgreSQL" || exit 1

# Wait for Redis
wait_for_service localhost 6379 "Redis" || exit 1

echo -e "${GREEN}✓ Infrastructure started${NC}"
echo ""

# Step 2: Build API
echo -e "${BLUE}Step 2: Checking ZeroState API binary...${NC}"
if [ ! -f "bin/zerostate-api" ]; then
    echo "Building API binary..."
    go build -o bin/zerostate-api cmd/api/main.go || {
        echo -e "${YELLOW}⚠ Build failed, using existing binary if available${NC}"
    }
fi
if [ -f "bin/zerostate-api" ]; then
    echo -e "${GREEN}✓ API binary ready ($(du -h bin/zerostate-api | cut -f1))${NC}"
else
    echo -e "${RED}❌ No API binary found. Please fix build issues.${NC}"
    exit 1
fi
echo ""

# Step 3: Start API
echo -e "${BLUE}Step 3: Starting ZeroState API on port ${API_PORT}...${NC}"

# Export environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=zerostate
export DB_PASSWORD=$POSTGRES_PASSWORD
export DB_NAME=zerostate
export DB_SSLMODE=disable
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=$JWT_SECRET
export SERVER_PORT=$API_PORT
export LOG_LEVEL=info

# Kill any existing API process
pkill -f "zerostate-api" 2>/dev/null || true
sleep 1

# Start API in background
./bin/zerostate-api > logs/api.log 2>&1 &
API_PID=$!
echo $API_PID > /tmp/zerostate-api.pid

# Wait for API to be ready
wait_for_service localhost $API_PORT "ZeroState API" || {
    echo -e "${YELLOW}API failed to start. Check logs/api.log for details${NC}"
    exit 1
}

echo -e "${GREEN}✓ API started (PID: $API_PID)${NC}"
echo ""

# Step 4: Create test user
echo -e "${BLUE}Step 4: Creating test user...${NC}"

SIGNUP_RESPONSE=$(curl -s -X POST http://localhost:$API_PORT/api/v1/auth/signup \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "TestPassword123!",
        "name": "Test User"
    }' || echo '{"error": "signup failed"}')

if echo "$SIGNUP_RESPONSE" | grep -q "error"; then
    echo -e "${YELLOW}⚠ User might already exist, trying login...${NC}"
else
    echo -e "${GREEN}✓ Test user created${NC}"
fi

# Login to get token
echo -e "${BLUE}Logging in...${NC}"
LOGIN_RESPONSE=$(curl -s -X POST http://localhost:$API_PORT/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{
        "email": "test@example.com",
        "password": "TestPassword123!"
    }')

TOKEN=$(echo $LOGIN_RESPONSE | jq -r '.access_token' 2>/dev/null || echo "")

if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo -e "${YELLOW}⚠ Login failed. Response: $LOGIN_RESPONSE${NC}"
    exit 1
fi

# Save token for other scripts
echo $TOKEN > /tmp/zerostate-token.txt

echo -e "${GREEN}✓ Logged in successfully${NC}"
echo ""

# Step 5: Summary
echo -e "${GREEN}✅ Local network setup complete!${NC}"
echo ""
echo "================================================"
echo "🎯 Network Information"
echo "================================================"
echo "API Endpoint:    http://localhost:$API_PORT"
echo "PostgreSQL:      localhost:5432"
echo "Redis:           localhost:6379"
echo "API Logs:        logs/api.log"
echo "API PID:         $API_PID"
echo ""
echo "📝 Test User Credentials"
echo "================================================"
echo "Email:           test@example.com"
echo "Password:        TestPassword123!"
echo "Token:           $TOKEN"
echo "Token saved to:  /tmp/zerostate-token.txt"
echo ""
echo "🔧 Useful Commands"
echo "================================================"
echo "View API logs:    tail -f logs/api.log"
echo "Stop API:         kill $API_PID"
echo "Stop all:         docker-compose down"
echo "Get token:        cat /tmp/zerostate-token.txt"
echo ""
echo "📚 Next Steps"
echo "================================================"
echo "1. Register an agent:  ./scripts/register-agent.sh examples/agents/echo-agent/dist/echo-agent.wasm"
echo "2. Test agent:         ./scripts/test-agent.sh"
echo "3. View metrics:       curl http://localhost:$API_PORT/metrics"
echo ""
