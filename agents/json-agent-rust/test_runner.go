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
	Data      string  `json:"data"`
	Path      *string `json:"path,omitempty"`
}

type Output struct {
	Result string          `json:"result"`
	Valid  *bool           `json:"valid,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

func main() {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)
	wasi_snapshot_preview1.Instantiate(ctx, runtime)

	wasmBytes, _ := os.ReadFile("target/wasm32-unknown-unknown/release/json_agent.wasm")
	mod, _ := runtime.InstantiateWithConfig(ctx, wasmBytes, wazero.NewModuleConfig().WithName("json_agent"))
	defer mod.Close(ctx)

	tests := []struct {
		name  string
		input Input
		check func(Output) bool
	}{
		{
			"Validate Valid JSON",
			Input{Operation: "validate", Data: `{"name":"test"}`},
			func(o Output) bool { return o.Valid != nil && *o.Valid },
		},
		{
			"Validate Invalid JSON",
			Input{Operation: "validate", Data: `{invalid`},
			func(o Output) bool { return o.Valid != nil && !*o.Valid },
		},
		{
			"Parse JSON",
			Input{Operation: "parse", Data: `{"name":"alice","age":30}`},
			func(o Output) bool { return o.Result == "parsed" },
		},
		{
			"Get Keys",
			Input{Operation: "keys", Data: `{"a":1,"b":2,"c":3}`},
			func(o Output) bool { return o.Result == "a, b, c" },
		},
		{
			"Get Type - Object",
			Input{Operation: "type", Data: `{"name":"test"}`},
			func(o Output) bool { return o.Result == "object" },
		},
		{
			"Get Type - Array",
			Input{Operation: "type", Data: `[1,2,3]`},
			func(o Output) bool { return o.Result == "array" },
		},
		{
			"Get Type - String",
			Input{Operation: "type", Data: `"hello"`},
			func(o Output) bool { return o.Result == "string" },
		},
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
		results, _ = execute.Call(ctx, inputPtr, uint64(len(inputJSON)))
		duration := time.Since(start)
		exitCode := results[0]

		getResultPtr := mod.ExportedFunction("get_result_ptr")
		getResultLen := mod.ExportedFunction("get_result_len")
		ptrResults, _ := getResultPtr.Call(ctx)
		lenResults, _ := getResultLen.Call(ctx)

		resultBytes, _ := mod.Memory().Read(uint32(ptrResults[0]), uint32(lenResults[0]))
		var output Output
		json.Unmarshal(resultBytes, &output)

		if exitCode == 0 && tt.check(output) {
			fmt.Printf("✅ %s (%.2fms)\n", tt.name, duration.Seconds()*1000)
			passed++
		} else {
			fmt.Printf("❌ %s: %s\n", tt.name, output.Result)
		}
	}

	fmt.Printf("\n📊 %d/%d tests passed\n", passed, len(tests))
}
