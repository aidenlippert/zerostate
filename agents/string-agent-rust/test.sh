#!/bin/bash

set -e

cd "$(dirname "$0")"

WASM_FILE="target/wasm32-unknown-unknown/release/string_agent.wasm"

echo "🧪 Testing String Manipulation Agent"
echo "======================================"
echo ""
echo "WASM file: $WASM_FILE"
echo "Size: $(ls -lh $WASM_FILE | awk '{print $5}')"
echo ""

# Create test runner
cat > test_runner.go <<'EOF'
package main

import (
"context"
"encoding/json"
"fmt"
"os"
"time"

"github.com/tetratelabs/wazero"
"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Input struct {
Operation string  `json:"operation"`
Text      string  `json:"text"`
Separator *string `json:"separator,omitempty"`
}

type Output struct {
Result string   `json:"result"`
Parts  []string `json:"parts,omitempty"`
}

func main() {
ctx := context.Background()
runtime := wazero.NewRuntime(ctx)
defer runtime.Close(ctx)
wasi_snapshot_preview1.Instantiate(ctx, runtime)

wasmBytes, _ := os.ReadFile("target/wasm32-unknown-unknown/release/string_agent.wasm")
mod, _ := runtime.InstantiateWithConfig(ctx, wasmBytes, wazero.NewModuleConfig().WithName("string_agent"))
defer mod.Close(ctx)

tests := []struct {
name  string
input Input
want  string
}{
{"Uppercase", Input{Operation: "uppercase", Text: "hello world"}, "HELLO WORLD"},
{"Lowercase", Input{Operation: "lowercase", Text: "HELLO WORLD"}, "hello world"},
{"Reverse", Input{Operation: "reverse", Text: "hello"}, "olleh"},
{"Title Case", Input{Operation: "title_case", Text: "hello world"}, "Hello World"},
{"Count Words", Input{Operation: "count_words", Text: "the quick brown fox"}, "4"},
}

passed := 0
for _, tt := range tests {
inputJSON, _ := json.Marshal(tt.input)
allocMem := mod.ExportedFunction("alloc_memory")
results, _ := allocMem.Call(ctx, uint64(len(inputJSON)))
inputPtr := results[0]
mod.Memory().Write(uint32(inputPtr), inputJSON)

execute := mod.ExportedFunction("execute")
start := time.Now()
execute.Call(ctx, inputPtr, uint64(len(inputJSON)))
duration := time.Since(start)

getResultPtr := mod.ExportedFunction("get_result_ptr")
getResultLen := mod.ExportedFunction("get_result_len")
ptrResults, _ := getResultPtr.Call(ctx)
lenResults, _ := getResultLen.Call(ctx)

resultBytes, _ := mod.Memory().Read(uint32(ptrResults[0]), uint32(lenResults[0]))
var output Output
json.Unmarshal(resultBytes, &output)

if output.Result == tt.want {
fmt.Printf("✅ %s (%.2fms)\n", tt.name, duration.Seconds()*1000)
passed++
} else {
fmt.Printf("❌ %s: got '%s', want '%s'\n", tt.name, output.Result, tt.want)
}
}

fmt.Printf("\n📊 %d/5 tests passed\n", passed)
}
EOF

go run test_runner.go
echo ""
echo "✅ Tests complete!"
