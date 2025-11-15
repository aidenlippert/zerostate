const fs = require('fs');
const path = require('path');

// Load WASM module
const wasmPath = path.join(__dirname, 'target/wasm32-unknown-unknown/release/validation_agent.wasm');
const wasmBuffer = fs.readFileSync(wasmPath);

async function testValidationAgent() {
    const wasmModule = await WebAssembly.instantiate(wasmBuffer);
    const { alloc_memory, dealloc_memory, execute, get_result_ptr, get_result_len, memory } = wasmModule.instance.exports;

    function callAgent(input) {
        const inputStr = JSON.stringify(input);
        const inputBytes = Buffer.from(inputStr, 'utf-8');
        
        // Allocate memory
        const inputPtr = alloc_memory(inputBytes.length);
        const inputView = new Uint8Array(memory.buffer, inputPtr, inputBytes.length);
        inputView.set(inputBytes);
        
        // Execute
        const result = execute(inputPtr, inputBytes.length);
        
        // Get result
        const resultPtr = get_result_ptr();
        const resultLen = get_result_len();
        const resultBytes = new Uint8Array(memory.buffer, resultPtr, resultLen);
        const resultStr = Buffer.from(resultBytes).toString('utf-8');
        
        // Cleanup
        dealloc_memory(inputPtr, inputBytes.length);
        
        return JSON.parse(resultStr);
    }

    console.log('Testing Data Validation Agent...');
    console.log('=================================\n');

    let passed = 0;
    let failed = 0;

    // Test 1: Email validation
    console.log('1. Email (valid):');
    let result = callAgent({ operation: 'email', value: 'test@example.com' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('2. Email (invalid):');
    result = callAgent({ operation: 'email', value: 'notanemail' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 3: URL validation
    console.log('3. URL (valid):');
    result = callAgent({ operation: 'url', value: 'https://example.com' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('4. URL (invalid):');
    result = callAgent({ operation: 'url', value: 'notaurl' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 5: Phone validation
    console.log('5. Phone (international):');
    result = callAgent({ operation: 'phone', value: '+12345678901' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('6. Phone (domestic):');
    result = callAgent({ operation: 'phone', value: '1234567890' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    // Test 7: Credit card (Luhn algorithm)
    console.log('7. Credit card (valid Luhn):');
    result = callAgent({ operation: 'credit_card', value: '4532015112830366' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('8. Credit card (invalid):');
    result = callAgent({ operation: 'credit_card', value: '1234567890123456' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 9: IPv4
    console.log('9. IPv4 (valid):');
    result = callAgent({ operation: 'ipv4', value: '192.168.1.1' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('10. IPv4 (invalid):');
    result = callAgent({ operation: 'ipv4', value: '256.1.1.1' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 11: IPv6
    console.log('11. IPv6 (valid):');
    result = callAgent({ operation: 'ipv6', value: '2001:0db8:85a3::8a2e:0370:7334' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    // Test 12: Not empty
    console.log('12. Not empty (valid):');
    result = callAgent({ operation: 'not_empty', value: 'hello' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('13. Not empty (invalid):');
    result = callAgent({ operation: 'not_empty', value: '   ' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 14: Length validation
    console.log('14. Length min:3 (valid):');
    result = callAgent({ operation: 'length', value: 'hello', pattern: 'min:3' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('15. Length max:5 (invalid):');
    result = callAgent({ operation: 'length', value: 'toolong', pattern: 'max:5' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 16: Numeric
    console.log('16. Numeric (valid):');
    result = callAgent({ operation: 'numeric', value: '123.45' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('17. Numeric (invalid):');
    result = callAgent({ operation: 'numeric', value: 'abc' });
    console.log(`   Result: ${!result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (!result.valid) passed++; else failed++;

    // Test 18: Alpha
    console.log('18. Alpha (valid):');
    result = callAgent({ operation: 'alpha', value: 'abc' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    // Test 19: Alphanumeric
    console.log('19. Alphanumeric (valid):');
    result = callAgent({ operation: 'alphanumeric', value: 'abc123' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    // Test 20: Regex
    console.log('20. Regex contains "ell":');
    result = callAgent({ operation: 'regex', value: 'hello', pattern: 'ell' });
    console.log(`   Result: ${result.valid ? '✅' : '❌'} ${JSON.stringify(result)}`);
    if (result.valid) passed++; else failed++;

    console.log('\n=================================');
    console.log(`Tests passed: ${passed}/20`);
    console.log(`Tests failed: ${failed}/20`);
    console.log('=================================');
}

testValidationAgent().catch(console.error);
