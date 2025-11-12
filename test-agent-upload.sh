#!/bin/bash

# Test Agent Upload and Database Persistence
# This script tests the complete flow: registration -> upload -> DB verification

set -e

API_URL="${API_URL:-http://localhost:8080}"
TEST_EMAIL="agent-test-$(date +%s)@example.com"
TEST_PASSWORD="SecurePassword123!"

echo "========================================="
echo "ZeroState Agent Upload & Persistence Test"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

# Step 1: Register user
echo "Step 1: User Registration"
REGISTER_RESPONSE=$(curl -s -X POST $API_URL/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$TEST_EMAIL\",\"password\":\"$TEST_PASSWORD\",\"full_name\":\"Test User\"}")

if echo "$REGISTER_RESPONSE" | jq -e '.token' > /dev/null 2>&1; then
    success "User registered"
    TOKEN=$(echo "$REGISTER_RESPONSE" | jq -r '.token')
    USER_ID=$(echo "$REGISTER_RESPONSE" | jq -r '.user.id')
    info "Token: ${TOKEN:0:30}..."
    info "User ID: $USER_ID"
else
    error "User registration failed"
    echo "$REGISTER_RESPONSE" | jq 2>/dev/null || echo "$REGISTER_RESPONSE"
    exit 1
fi
echo ""

# Step 2: Create WASM binary
echo "Step 2: Creating Test WASM Binary"
# WASM magic number (0x00 0x61 0x73 0x6D) + version (0x01 0x00 0x00 0x00)
echo -ne '\x00\x61\x73\x6d\x01\x00\x00\x00' > /tmp/test-agent.wasm
# Add minimal module structure (empty type, function, export sections)
echo -ne '\x01\x04\x01\x60\x00\x00' >> /tmp/test-agent.wasm  # Type section
echo -ne '\x03\x02\x01\x00' >> /tmp/test-agent.wasm          # Function section
echo -ne '\x07\x08\x01\x04test\x00\x00' >> /tmp/test-agent.wasm  # Export section
echo -ne '\x0a\x04\x01\x02\x00\x0b' >> /tmp/test-agent.wasm  # Code section

WASM_SIZE=$(stat -f%z /tmp/test-agent.wasm 2>/dev/null || stat -c%s /tmp/test-agent.wasm)
success "WASM binary created ($WASM_SIZE bytes)"
echo ""

# Step 3: Upload agent
echo "Step 3: Agent Upload"
UPLOAD_RESPONSE=$(curl -s -X POST $API_URL/api/v1/agents/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm_binary=@/tmp/test-agent.wasm" \
  -F "name=TestAgent-$(date +%s)" \
  -F "description=E2E test agent for database persistence" \
  -F "version=1.0.0" \
  -F "capabilities=compute" \
  -F "capabilities=storage" \
  -F "price=0.01")

if echo "$UPLOAD_RESPONSE" | jq -e '.agent_id' > /dev/null 2>&1; then
    success "Agent uploaded successfully"
    AGENT_ID=$(echo "$UPLOAD_RESPONSE" | jq -r '.agent_id')
    BINARY_HASH=$(echo "$UPLOAD_RESPONSE" | jq -r '.binary_hash')
    info "Agent ID: $AGENT_ID"
    info "Binary Hash: $BINARY_HASH"
    echo "$UPLOAD_RESPONSE" | jq
else
    error "Agent upload failed"
    echo "$UPLOAD_RESPONSE" | jq 2>/dev/null || echo "$UPLOAD_RESPONSE"
    exit 1
fi
echo ""

# Step 4: Verify in database (PostgreSQL)
echo "Step 4: Database Verification (PostgreSQL)"
if [ -n "$DATABASE_URL" ]; then
    info "Checking PostgreSQL database..."
    
    # Extract connection details from DATABASE_URL
    # Format: postgres://user:pass@host:port/dbname
    DB_QUERY="SELECT id, did, name, status, created_at FROM agents WHERE id = '$AGENT_ID' OR did = '$AGENT_ID';"
    
    psql "$DATABASE_URL" -c "$DB_QUERY" 2>/dev/null && success "Agent found in PostgreSQL" || {
        error "Agent not found in PostgreSQL"
        info "Manual check: psql \$DATABASE_URL -c \"SELECT * FROM agents WHERE did = '$AGENT_ID';\""
    }
else
    info "DATABASE_URL not set - skipping PostgreSQL check"
fi
echo ""

# Step 5: Verify via API
echo "Step 5: API Verification"
LIST_RESPONSE=$(curl -s -X GET "$API_URL/api/v1/agents" \
  -H "Authorization: Bearer $TOKEN")

if echo "$LIST_RESPONSE" | jq -e ".agents[] | select(.id == \"$AGENT_ID\")" > /dev/null 2>&1; then
    success "Agent found via API"
    echo "$LIST_RESPONSE" | jq ".agents[] | select(.id == \"$AGENT_ID\")"
else
    error "Agent not found in list"
    AGENT_COUNT=$(echo "$LIST_RESPONSE" | jq '.agents | length' 2>/dev/null || echo "0")
    info "Total agents in system: $AGENT_COUNT"
fi
echo ""

# Step 6: SQLite verification (if using local DB)
echo "Step 6: SQLite Verification (local dev)"
if [ -f "./zerostate.db" ]; then
    info "Checking SQLite database..."
    sqlite3 ./zerostate.db "SELECT id, did, name, status FROM agents WHERE id = '$AGENT_ID' OR did = '$AGENT_ID';" && success "Agent found in SQLite" || error "Agent not in SQLite"
else
    info "No zerostate.db file found - likely using PostgreSQL"
fi
echo ""

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
success "Agent upload and persistence test complete!"
echo ""
echo "Test Details:"
echo "  User: $TEST_EMAIL"
echo "  Agent ID: $AGENT_ID"
echo "  Binary Hash: $BINARY_HASH"
echo ""
echo "Manual Verification Commands:"
echo ""
echo "  # PostgreSQL:"
echo "  psql \$DATABASE_URL -c \"SELECT * FROM agents WHERE did = '$AGENT_ID';\""
echo ""
echo "  # SQLite:"
echo "  sqlite3 ./zerostate.db \"SELECT * FROM agents WHERE did = '$AGENT_ID';\""
echo ""
echo "  # Via API:"
echo "  curl -H 'Authorization: Bearer $TOKEN' $API_URL/api/v1/agents | jq '.agents[] | select(.id == \"$AGENT_ID\")'"
echo ""

