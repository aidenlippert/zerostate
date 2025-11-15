#!/bin/bash
# Test Wasmtime + R2 End-to-End Integration

set -e

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║       🚀 WASMTIME + R2 E2E TEST 🚀                           ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Load environment
source .env

export AWS_ACCESS_KEY_ID=$R2_ACCESS_KEY_ID
export AWS_SECRET_ACCESS_KEY=$R2_SECRET_ACCESS_KEY
export AWS_ENDPOINT_URL=$R2_ENDPOINT

WASMTIME="${HOME}/.wasmtime/bin/wasmtime"

echo "📊 Configuration:"
echo "  R2 Bucket: $R2_BUCKET_NAME"
echo "  R2 Endpoint: $R2_ENDPOINT"
echo "  Wasmtime: $WASMTIME"
echo ""

# Step 1: Download WASM from R2
echo "🧪 Step 1: Download Math Agent from R2..."
aws s3 cp s3://$R2_BUCKET_NAME/agents/math-agent-v1.0.wasm /tmp/ainur-test.wasm \
    --endpoint-url $AWS_ENDPOINT_URL --quiet
echo "  ✅ Downloaded: $(ls -lh /tmp/ainur-test.wasm | awk '{print $5}')"
echo ""

# Step 2: Execute WASM with Wasmtime
echo "🧪 Step 2: Execute WASM functions..."
echo ""

# Test 1: add(2, 2)
echo "  Test 1: add(2, 2)"
RESULT=$($WASMTIME --invoke add /tmp/ainur-test.wasm 2 2 2>&1 | tail -1)
if [ "$RESULT" = "4" ]; then
    echo "    ✅ Result: $RESULT"
else
    echo "    ❌ Failed: got $RESULT, expected 4"
    exit 1
fi

# Test 2: multiply(6, 7)
echo "  Test 2: multiply(6, 7)"
RESULT=$($WASMTIME --invoke multiply /tmp/ainur-test.wasm 6 7 2>&1 | tail -1)
if [ "$RESULT" = "42" ]; then
    echo "    ✅ Result: $RESULT"
else
    echo "    ❌ Failed: got $RESULT, expected 42"
    exit 1
fi

# Test 3: factorial(5)
echo "  Test 3: factorial(5)"
RESULT=$($WASMTIME --invoke factorial /tmp/ainur-test.wasm 5 2>&1 | tail -1)
if [ "$RESULT" = "120" ]; then
    echo "    ✅ Result: $RESULT"
else
    echo "    ❌ Failed: got $RESULT, expected 120"
    exit 1
fi

# Test 4: fibonacci(10)
echo "  Test 4: fibonacci(10)"
RESULT=$($WASMTIME --invoke fibonacci /tmp/ainur-test.wasm 10 2>&1 | tail -1)
if [ "$RESULT" = "55" ]; then
    echo "    ✅ Result: $RESULT"
else
    echo "    ❌ Failed: got $RESULT, expected 55"
    exit 1
fi

# Test 5: is_prime(17)
echo "  Test 5: is_prime(17)"
RESULT=$($WASMTIME --invoke is_prime /tmp/ainur-test.wasm 17 2>&1 | tail -1)
if [ "$RESULT" = "1" ]; then
    echo "    ✅ Result: $RESULT (prime)"
else
    echo "    ❌ Failed: got $RESULT, expected 1"
    exit 1
fi

# Test 6: gcd(48, 18)
echo "  Test 6: gcd(48, 18)"
RESULT=$($WASMTIME --invoke gcd /tmp/ainur-test.wasm 48 18 2>&1 | tail -1)
if [ "$RESULT" = "6" ]; then
    echo "    ✅ Result: $RESULT"
else
    echo "    ❌ Failed: got $RESULT, expected 6"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 ALL E2E TESTS PASSED!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "✅ COMPLETE E2E WORKFLOW:"
echo "   1. WASM stored in Cloudflare R2"
echo "   2. Downloaded from R2 on-demand"
echo "   3. Executed with Wasmtime runtime"
echo "   4. All 6 functions working correctly"
echo ""
echo "🚀 THIS IS PRODUCTION-READY!"
echo ""
echo "Next: Integrate into Ainur orchestrator"
echo ""

# Cleanup
rm /tmp/ainur-test.wasm
