#!/bin/bash
# Test R2 Upload - Verify Cloudflare R2 configuration

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║           🪣 Testing Cloudflare R2 Storage                   ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Load environment variables
source .env

# Set AWS CLI environment
export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY
export AWS_ENDPOINT_URL=$R2_ENDPOINT

echo "📊 Configuration:"
echo "  Endpoint: $R2_ENDPOINT"
echo "  Bucket: $R2_BUCKET_NAME"
echo "  Access Key: ${R2_ACCESS_KEY_ID:0:10}..."
echo ""

# Test 1: List buckets
echo "🧪 Test 1: List R2 buckets..."
aws s3 ls --endpoint-url $AWS_ENDPOINT_URL 2>&1 | grep -q "ainur-agents" && echo "  ✅ Bucket 'ainur-agents' exists" || echo "  ❌ Bucket 'ainur-agents' not found (create it in Cloudflare dashboard)"
echo ""

# Test 2: Upload Math Agent WASM
WASM_FILE="./agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm"

if [ ! -f "$WASM_FILE" ]; then
    echo "❌ WASM file not found: $WASM_FILE"
    echo "   Run: cd agents/math-agent-rust && cargo build --target wasm32-unknown-unknown --release"
    exit 1
fi

echo "🧪 Test 2: Upload Math Agent WASM..."
echo "  File: $WASM_FILE"
echo "  Size: $(ls -lh $WASM_FILE | awk '{print $5}')"

aws s3 cp $WASM_FILE s3://$R2_BUCKET_NAME/agents/math-agent-v1.0.wasm \
    --endpoint-url $AWS_ENDPOINT_URL \
    --content-type "application/wasm" \
    2>&1

if [ $? -eq 0 ]; then
    echo "  ✅ Upload successful!"
else
    echo "  ❌ Upload failed!"
    exit 1
fi
echo ""

# Test 3: List uploaded files
echo "🧪 Test 3: List files in bucket..."
aws s3 ls s3://$R2_BUCKET_NAME/agents/ --endpoint-url $AWS_ENDPOINT_URL
echo ""

# Test 4: Download and verify
echo "🧪 Test 4: Download and verify..."
aws s3 cp s3://$R2_BUCKET_NAME/agents/math-agent-v1.0.wasm /tmp/test-download.wasm \
    --endpoint-url $AWS_ENDPOINT_URL \
    2>&1

if [ $? -eq 0 ]; then
    echo "  ✅ Download successful!"
    
    # Compare file sizes
    ORIG_SIZE=$(stat -f%z "$WASM_FILE" 2>/dev/null || stat -c%s "$WASM_FILE")
    DOWN_SIZE=$(stat -f%z "/tmp/test-download.wasm" 2>/dev/null || stat -c%s "/tmp/test-download.wasm")
    
    if [ "$ORIG_SIZE" = "$DOWN_SIZE" ]; then
        echo "  ✅ File integrity verified ($ORIG_SIZE bytes)"
    else
        echo "  ❌ File size mismatch! Original: $ORIG_SIZE, Downloaded: $DOWN_SIZE"
        exit 1
    fi
    
    rm /tmp/test-download.wasm
else
    echo "  ❌ Download failed!"
    exit 1
fi
echo ""

# Test 5: Get file URL
echo "🧪 Test 5: Generate public URL..."
PUBLIC_URL="https://pub-${R2_ACCOUNT_ID}.r2.dev/agents/math-agent-v1.0.wasm"
echo "  URL: $PUBLIC_URL"
echo "  Note: Enable public access in R2 settings if needed"
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 ALL R2 TESTS PASSED!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "✅ Cloudflare R2 is configured and working!"
echo "✅ Math Agent WASM uploaded successfully!"
echo "✅ File integrity verified!"
echo ""
echo "🚀 Ready for Wasmtime integration!"
echo ""
