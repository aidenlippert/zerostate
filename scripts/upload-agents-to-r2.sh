#!/bin/bash
# Upload WASM agents to Cloudflare R2

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        📦 UPLOADING WASM AGENTS TO R2 STORAGE 📦            ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check for required environment variables
if [ -z "$R2_ACCESS_KEY_ID" ] || [ -z "$R2_SECRET_ACCESS_KEY" ]; then
    echo "❌ Error: R2 credentials not set"
    echo ""
    echo "Please set the following environment variables:"
    echo "  export R2_ACCESS_KEY_ID='your-access-key-id'"
    echo "  export R2_SECRET_ACCESS_KEY='your-secret-access-key'"
    echo ""
    echo "Get your credentials from:"
    echo "  https://dash.cloudflare.com/profile/api-tokens"
    echo ""
    exit 1
fi

# Configuration
ACCOUNT_ID="${R2_ACCOUNT_ID:-d4affab848a8f8e47b7930147fe1b43a}"
BUCKET_NAME="zerostate-agents"
ENDPOINT="https://${ACCOUNT_ID}.r2.cloudflarestorage.com"

echo "📋 Configuration:"
echo "   Bucket: $BUCKET_NAME"
echo "   Endpoint: $ENDPOINT"
echo ""

# Check if AWS CLI is installed
if ! command -v aws &> /dev/null; then
    echo "❌ Error: AWS CLI not installed"
    echo ""
    echo "Install it with:"
    echo "  Ubuntu/Debian: sudo apt install awscli"
    echo "  macOS: brew install awscli"
    echo ""
    exit 1
fi

# Configure AWS CLI for R2
export AWS_ACCESS_KEY_ID="$R2_ACCESS_KEY_ID"
export AWS_SECRET_ACCESS_KEY="$R2_SECRET_ACCESS_KEY"
export AWS_DEFAULT_REGION="auto"

echo "🔍 Finding WASM agents..."
echo ""

# Find and upload each agent
AGENTS_DIR="/home/rocz/vegalabs/zerostate/agents"
UPLOADED=0
FAILED=0

# Math Agent
if [ -f "$AGENTS_DIR/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm" ]; then
    echo "📤 Uploading math-agent.wasm..."
    if aws s3 cp "$AGENTS_DIR/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm" \
        "s3://$BUCKET_NAME/agents/math-agent.wasm" \
        --endpoint-url "$ENDPOINT" \
        --content-type "application/wasm"; then
        echo "   ✅ math-agent.wasm uploaded (986 bytes)"
        UPLOADED=$((UPLOADED + 1))
    else
        echo "   ❌ Failed to upload math-agent.wasm"
        FAILED=$((FAILED + 1))
    fi
else
    echo "   ⚠️  math-agent.wasm not found"
fi
echo ""

# String Agent
if [ -f "$AGENTS_DIR/string-agent-rust/target/wasm32-unknown-unknown/release/string_agent.wasm" ]; then
    echo "📤 Uploading string-agent.wasm..."
    if aws s3 cp "$AGENTS_DIR/string-agent-rust/target/wasm32-unknown-unknown/release/string_agent.wasm" \
        "s3://$BUCKET_NAME/agents/string-agent.wasm" \
        --endpoint-url "$ENDPOINT" \
        --content-type "application/wasm"; then
        SIZE=$(ls -lh "$AGENTS_DIR/string-agent-rust/target/wasm32-unknown-unknown/release/string_agent.wasm" | awk '{print $5}')
        echo "   ✅ string-agent.wasm uploaded ($SIZE)"
        UPLOADED=$((UPLOADED + 1))
    else
        echo "   ❌ Failed to upload string-agent.wasm"
        FAILED=$((FAILED + 1))
    fi
else
    echo "   ⚠️  string-agent.wasm not found"
fi
echo ""

# JSON Agent
if [ -f "$AGENTS_DIR/json-agent-rust/target/wasm32-unknown-unknown/release/json_agent.wasm" ]; then
    echo "📤 Uploading json-agent.wasm..."
    if aws s3 cp "$AGENTS_DIR/json-agent-rust/target/wasm32-unknown-unknown/release/json_agent.wasm" \
        "s3://$BUCKET_NAME/agents/json-agent.wasm" \
        --endpoint-url "$ENDPOINT" \
        --content-type "application/wasm"; then
        SIZE=$(ls -lh "$AGENTS_DIR/json-agent-rust/target/wasm32-unknown-unknown/release/json_agent.wasm" | awk '{print $5}')
        echo "   ✅ json-agent.wasm uploaded ($SIZE)"
        UPLOADED=$((UPLOADED + 1))
    else
        echo "   ❌ Failed to upload json-agent.wasm"
        FAILED=$((FAILED + 1))
    fi
else
    echo "   ⚠️  json-agent.wasm not found"
fi
echo ""

# Validation Agent
if [ -f "$AGENTS_DIR/validation-agent-rust/target/wasm32-unknown-unknown/release/validation_agent.wasm" ]; then
    echo "📤 Uploading validation-agent.wasm..."
    if aws s3 cp "$AGENTS_DIR/validation-agent-rust/target/wasm32-unknown-unknown/release/validation_agent.wasm" \
        "s3://$BUCKET_NAME/agents/validation-agent.wasm" \
        --endpoint-url "$ENDPOINT" \
        --content-type "application/wasm"; then
        SIZE=$(ls -lh "$AGENTS_DIR/validation-agent-rust/target/wasm32-unknown-unknown/release/validation_agent.wasm" | awk '{print $5}')
        echo "   ✅ validation-agent.wasm uploaded ($SIZE)"
        UPLOADED=$((UPLOADED + 1))
    else
        echo "   ❌ Failed to upload validation-agent.wasm"
        FAILED=$((FAILED + 1))
    fi
else
    echo "   ⚠️  validation-agent.wasm not found"
fi
echo ""

# Summary
echo "══════════════════════════════════════════════════════════════"
echo "📊 Upload Summary:"
echo "   ✅ Uploaded: $UPLOADED agents"
echo "   ❌ Failed: $FAILED agents"
echo "══════════════════════════════════════════════════════════════"
echo ""

if [ $UPLOADED -gt 0 ]; then
    echo "🎉 Success! Agents are now available in R2"
    echo ""
    echo "📋 Next Steps:"
    echo "   1. Set Fly.io secrets:"
    echo "      fly secrets set R2_ACCESS_KEY_ID='$R2_ACCESS_KEY_ID'"
    echo "      fly secrets set R2_SECRET_ACCESS_KEY='***'"
    echo "      fly secrets set R2_ACCOUNT_ID='$ACCOUNT_ID'"
    echo "      fly secrets set R2_ENDPOINT='$ENDPOINT'"
    echo "      fly secrets set R2_BUCKET_NAME='$BUCKET_NAME'"
    echo ""
    echo "   2. Deploy the updated API:"
    echo "      fly deploy"
    echo ""
    echo "   3. Test the integration:"
    echo "      ./test-groq-integration.sh"
    echo ""
fi

if [ $FAILED -gt 0 ]; then
    exit 1
fi
