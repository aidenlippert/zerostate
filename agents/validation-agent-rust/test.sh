#!/bin/bash
# Test validation agent - 12 operations

WASM="target/wasm32-unknown-unknown/release/validation_agent.wasm"

echo "Testing Data Validation Agent..."
echo "================================"

# Test 1: Email validation
echo -n "1. Email (valid): "
echo '{"operation":"email","value":"test@example.com"}' | wasmtime "$WASM" --invoke execute

echo -n "2. Email (invalid): "
echo '{"operation":"email","value":"notanemail"}' | wasmtime "$WASM" --invoke execute

# Test 3: URL validation
echo -n "3. URL (valid): "
echo '{"operation":"url","value":"https://example.com"}' | wasmtime "$WASM" --invoke execute

echo -n "4. URL (invalid): "
echo '{"operation":"url","value":"notaurl"}' | wasmtime "$WASM" --invoke execute

# Test 5: Phone validation
echo -n "5. Phone (international): "
echo '{"operation":"phone","value":"+12345678901"}' | wasmtime "$WASM" --invoke execute

echo -n "6. Phone (domestic): "
echo '{"operation":"phone","value":"1234567890"}' | wasmtime "$WASM" --invoke execute

# Test 7: Credit card validation (Luhn algorithm)
echo -n "7. Credit card (valid Luhn): "
echo '{"operation":"credit_card","value":"4532015112830366"}' | wasmtime "$WASM" --invoke execute

echo -n "8. Credit card (invalid): "
echo '{"operation":"credit_card","value":"1234567890123456"}' | wasmtime "$WASM" --invoke execute

# Test 9: IPv4 validation
echo -n "9. IPv4 (valid): "
echo '{"operation":"ipv4","value":"192.168.1.1"}' | wasmtime "$WASM" --invoke execute

echo -n "10. IPv4 (invalid): "
echo '{"operation":"ipv4","value":"256.1.1.1"}' | wasmtime "$WASM" --invoke execute

# Test 11: IPv6 validation
echo -n "11. IPv6 (valid): "
echo '{"operation":"ipv6","value":"2001:0db8:85a3::8a2e:0370:7334"}' | wasmtime "$WASM" --invoke execute

# Test 12: Not empty
echo -n "12. Not empty (valid): "
echo '{"operation":"not_empty","value":"hello"}' | wasmtime "$WASM" --invoke execute

echo -n "13. Not empty (invalid): "
echo '{"operation":"not_empty","value":"   "}' | wasmtime "$WASM" --invoke execute

# Test 14: Length validation
echo -n "14. Length min:3 (valid): "
echo '{"operation":"length","value":"hello","pattern":"min:3"}' | wasmtime "$WASM" --invoke execute

echo -n "15. Length max:5 (invalid): "
echo '{"operation":"length","value":"toolong","pattern":"max:5"}' | wasmtime "$WASM" --invoke execute

# Test 16: Numeric validation
echo -n "16. Numeric (valid): "
echo '{"operation":"numeric","value":"123.45"}' | wasmtime "$WASM" --invoke execute

echo -n "17. Numeric (invalid): "
echo '{"operation":"numeric","value":"abc"}' | wasmtime "$WASM" --invoke execute

# Test 18: Alpha validation
echo -n "18. Alpha (valid): "
echo '{"operation":"alpha","value":"abc"}' | wasmtime "$WASM" --invoke execute

# Test 19: Alphanumeric validation
echo -n "19. Alphanumeric (valid): "
echo '{"operation":"alphanumeric","value":"abc123"}' | wasmtime "$WASM" --invoke execute

# Test 20: Regex validation
echo -n "20. Regex contains 'ell': "
echo '{"operation":"regex","value":"hello","pattern":"ell"}' | wasmtime "$WASM" --invoke execute

echo ""
echo "All tests complete!"
