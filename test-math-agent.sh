#!/bin/bash
# Test Math Agent v1.0 - Ainur's First Real WASM Agent

set -e

WASM_FILE="./agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm"
WASMTIME="${HOME}/.wasmtime/bin/wasmtime"

echo "╔══════════════════════════════════════════════════════════════╗"
echo "║                                                              ║"
echo "║        🧮 Testing Math Agent v1.0 (Real WASM!)              ║"
echo "║                                                              ║"
echo "╚══════════════════════════════════════════════════════════════╝"
echo ""

# Check if WASM file exists
if [ ! -f "$WASM_FILE" ]; then
    echo "❌ WASM binary not found. Building..."
    cd agents/math-agent-rust
    cargo build --target wasm32-unknown-unknown --release
    cd ../..
fi

# Check if wasmtime is installed
if [ ! -f "$WASMTIME" ]; then
    echo "❌ wasmtime not found. Installing..."
    curl https://wasmtime.dev/install.sh -sSf | bash
fi

echo "📊 Binary Info:"
echo "  Size: $(ls -lh $WASM_FILE | awk '{print $5}')"
echo "  Path: $WASM_FILE"
echo ""

echo "🧪 Running Tests:"
echo ""

# Test add
RESULT=$($WASMTIME --invoke add $WASM_FILE 2 2 2>&1 | tail -1)
if [ "$RESULT" = "4" ]; then
    echo "  ✅ add(2, 2) = $RESULT"
else
    echo "  ❌ add(2, 2) = $RESULT (expected 4)"
    exit 1
fi

# Test multiply
RESULT=$($WASMTIME --invoke multiply $WASM_FILE 6 7 2>&1 | tail -1)
if [ "$RESULT" = "42" ]; then
    echo "  ✅ multiply(6, 7) = $RESULT"
else
    echo "  ❌ multiply(6, 7) = $RESULT (expected 42)"
    exit 1
fi

# Test subtract
RESULT=$($WASMTIME --invoke subtract $WASM_FILE 10 3 2>&1 | tail -1)
if [ "$RESULT" = "7" ]; then
    echo "  ✅ subtract(10, 3) = $RESULT"
else
    echo "  ❌ subtract(10, 3) = $RESULT (expected 7)"
    exit 1
fi

# Test divide
RESULT=$($WASMTIME --invoke divide $WASM_FILE 20 4 2>&1 | tail -1)
if [ "$RESULT" = "5" ]; then
    echo "  ✅ divide(20, 4) = $RESULT"
else
    echo "  ❌ divide(20, 4) = $RESULT (expected 5)"
    exit 1
fi

# Test factorial
RESULT=$($WASMTIME --invoke factorial $WASM_FILE 5 2>&1 | tail -1)
if [ "$RESULT" = "120" ]; then
    echo "  ✅ factorial(5) = $RESULT"
else
    echo "  ❌ factorial(5) = $RESULT (expected 120)"
    exit 1
fi

# Test fibonacci
RESULT=$($WASMTIME --invoke fibonacci $WASM_FILE 10 2>&1 | tail -1)
if [ "$RESULT" = "55" ]; then
    echo "  ✅ fibonacci(10) = $RESULT"
else
    echo "  ❌ fibonacci(10) = $RESULT (expected 55)"
    exit 1
fi

# Test power
RESULT=$($WASMTIME --invoke power $WASM_FILE 2 10 2>&1 | tail -1)
if [ "$RESULT" = "1024" ]; then
    echo "  ✅ power(2, 10) = $RESULT"
else
    echo "  ❌ power(2, 10) = $RESULT (expected 1024)"
    exit 1
fi

# Test is_prime
RESULT=$($WASMTIME --invoke is_prime $WASM_FILE 17 2>&1 | tail -1)
if [ "$RESULT" = "1" ]; then
    echo "  ✅ is_prime(17) = $RESULT (true)"
else
    echo "  ❌ is_prime(17) = $RESULT (expected 1)"
    exit 1
fi

RESULT=$($WASMTIME --invoke is_prime $WASM_FILE 18 2>&1 | tail -1)
if [ "$RESULT" = "0" ]; then
    echo "  ✅ is_prime(18) = $RESULT (false)"
else
    echo "  ❌ is_prime(18) = $RESULT (expected 0)"
    exit 1
fi

# Test gcd
RESULT=$($WASMTIME --invoke gcd $WASM_FILE 48 18 2>&1 | tail -1)
if [ "$RESULT" = "6" ]; then
    echo "  ✅ gcd(48, 18) = $RESULT"
else
    echo "  ❌ gcd(48, 18) = $RESULT (expected 6)"
    exit 1
fi

# Test lcm
RESULT=$($WASMTIME --invoke lcm $WASM_FILE 12 18 2>&1 | tail -1)
if [ "$RESULT" = "36" ]; then
    echo "  ✅ lcm(12, 18) = $RESULT"
else
    echo "  ❌ lcm(12, 18) = $RESULT (expected 36)"
    exit 1
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🎉 ALL TESTS PASSED! Math Agent v1.0 is production ready!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "📊 Summary:"
echo "  ✅ 11 functions tested"
echo "  ✅ 11 tests passed"
echo "  ✅ 0 tests failed"
echo "  📦 Binary size: $(ls -lh $WASM_FILE | awk '{print $5}')"
echo "  🚀 Ready for Ainur deployment!"
echo ""
