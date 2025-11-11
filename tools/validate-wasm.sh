#!/bin/bash
# Validate WASM Binary for ZeroState Upload
# Usage: ./validate-wasm.sh agent.wasm

set -e

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <wasm-file>"
  exit 1
fi

WASM_FILE="$1"

if [[ ! -f "$WASM_FILE" ]]; then
  echo "❌ File not found: $WASM_FILE"
  exit 1
fi

echo "🔍 Validating WASM binary: $WASM_FILE"
echo ""

# Check magic bytes
echo -n "Checking WASM magic bytes... "
MAGIC=$(xxd -p -l 4 "$WASM_FILE")
if [[ "$MAGIC" == "0061736d" ]]; then
  echo "✅ Valid"
else
  echo "❌ Invalid (expected: 0061736d, got: $MAGIC)"
  exit 1
fi

# Check file size
echo -n "Checking file size... "
SIZE_BYTES=$(wc -c < "$WASM_FILE")
SIZE_MB=$((SIZE_BYTES / 1024 / 1024))
SIZE_HUMAN=$(du -h "$WASM_FILE" | cut -f1)

if [[ $SIZE_BYTES -gt 52428800 ]]; then  # 50 MB in bytes
  echo "❌ Too large: $SIZE_HUMAN (max: 50 MB)"
  exit 1
elif [[ $SIZE_BYTES -lt 1024 ]]; then  # 1 KB minimum
  echo "❌ Too small: $SIZE_HUMAN (min: 1 KB)"
  exit 1
else
  echo "✅ $SIZE_HUMAN (within 50 MB limit)"
fi

# Check WASM version
echo -n "Checking WASM version... "
VERSION=$(xxd -p -s 4 -l 4 "$WASM_FILE")
if [[ "$VERSION" == "01000000" ]]; then
  echo "✅ Version 1"
else
  echo "⚠️  Unexpected version: $VERSION (may still work)"
fi

# Use wasm-objdump if available for detailed analysis
if command -v wasm-objdump &> /dev/null; then
  echo ""
  echo "📋 WASM Module Details:"

  # Check exports
  echo -n "  Exports: "
  EXPORTS=$(wasm-objdump -x "$WASM_FILE" 2>/dev/null | grep -A 100 "Export\[" | grep "func" | wc -l)
  if [[ $EXPORTS -gt 0 ]]; then
    echo "✅ $EXPORTS function(s)"
    wasm-objdump -x "$WASM_FILE" 2>/dev/null | grep -A 100 "Export\[" | grep "func" | head -5
  else
    echo "⚠️  No exports found (may fail at runtime)"
  fi

  # Check imports
  echo -n "  Imports: "
  IMPORTS=$(wasm-objdump -x "$WASM_FILE" 2>/dev/null | grep -A 100 "Import\[" | grep "func" | wc -l)
  if [[ $IMPORTS -gt 0 ]]; then
    echo "⚠️  $IMPORTS function(s) (requires runtime support)"
  else
    echo "✅ None (self-contained)"
  fi

  # Check memory
  echo -n "  Memory: "
  MEMORY=$(wasm-objdump -x "$WASM_FILE" 2>/dev/null | grep "memory" | head -1 || echo "unknown")
  echo "$MEMORY"

else
  echo ""
  echo "💡 Install wabt for detailed analysis: apt-get install wabt"
fi

echo ""
echo "✅ Validation complete! WASM binary is ready for upload."
echo ""
echo "📤 Upload with:"
echo "  curl -X POST https://zerostate-api.fly.dev/api/v1/agents/upload \\"
echo "    -H 'Authorization: Bearer YOUR_JWT_TOKEN' \\"
echo "    -F 'wasm_binary=@$WASM_FILE' \\"
echo "    -F 'name=Your Agent Name' \\"
echo "    -F 'description=Agent description' \\"
echo "    -F 'version=1.0.0' \\"
echo "    -F 'capabilities=[\"capability1\",\"capability2\"]' \\"
echo "    -F 'price=0.05'"
