package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/storage"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	fmt.Println("🧪 Testing WASM Execution Pipeline")
	fmt.Println("===================================")
	fmt.Println("")

	// Create logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Create R2 client
	r2, err := storage.NewR2StorageFromEnv()
	if err != nil {
		log.Fatalf("❌ Failed to create R2 client: %v\n   Make sure R2 environment variables are set", err)
	}

	fmt.Println("✅ R2 client created")
	fmt.Println("")

	ctx := context.Background()

	// Check if math-agent WASM exists
	mathAgentKey := "agents/math-agent-v1.0.wasm"
	exists, err := r2.Exists(ctx, mathAgentKey)
	if err != nil {
		log.Fatalf("❌ Failed to check if agent exists: %v", err)
	}

	if !exists {
		fmt.Println("⚠️  Math agent not found in R2")
		fmt.Println("   Please upload it first:")
		fmt.Println("   1. cd agents/math-agent-rust")
		fmt.Println("   2. cargo build --target wasm32-unknown-unknown --release")
		fmt.Println("   3. Run: go run scripts/upload-wasm-agents.go")
		fmt.Println("")
		fmt.Println("💡 Or use the pre-built binary from target/wasm32-unknown-unknown/release/")
		os.Exit(1)
	}

	fmt.Println("✅ Math agent found in R2")
	fmt.Println("")

	// Create WASM runner
	runner := execution.NewWASMRunnerV2(
		logger,
		r2,
		30*time.Second, // timeout
		64,             // 64MB memory limit
	)

	fmt.Println("✅ WASM runner created")
	fmt.Println("")

	// Test cases
	testCases := []struct {
		name     string
		function string
		args     []interface{}
		expected int64
	}{
		{"Add 2 + 2", "add", []interface{}{2, 2}, 4},
		{"Multiply 3 * 4", "multiply", []interface{}{3, 4}, 12},
		{"Subtract 10 - 3", "subtract", []interface{}{10, 3}, 7},
		{"Divide 20 / 5", "divide", []interface{}{20, 5}, 4},
		{"Factorial 5!", "factorial", []interface{}{5}, 120},
		{"Fibonacci F(10)", "fibonacci", []interface{}{10}, 55},
		{"Power 2^10", "power", []interface{}{2, 10}, 1024},
		{"Is Prime 17", "is_prime", []interface{}{17}, 1},
		{"GCD of 48,18", "gcd", []interface{}{48, 18}, 6},
		{"LCM of 4,6", "lcm", []interface{}{4, 6}, 12},
	}

	fmt.Println("🚀 Running Test Cases:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("")

	successCount := 0
	totalDuration := time.Duration(0)

	for i, tc := range testCases {
		fmt.Printf("Test %d/%d: %s\n", i+1, len(testCases), tc.name)
		fmt.Printf("  Function: %s(%v)\n", tc.function, tc.args)

		// Execute WASM
		req := &execution.WASMExecutionRequest{
			R2Key:    mathAgentKey,
			Function: tc.function,
			Args:     tc.args,
			Timeout:  5 * time.Second,
		}

		result, err := runner.Execute(ctx, req)
		if err != nil {
			fmt.Printf("  ❌ Execution failed: %v\n", err)
			fmt.Println("")
			continue
		}

		if !result.Success {
			fmt.Printf("  ❌ WASM execution failed: %s\n", result.Error)
			fmt.Println("")
			continue
		}

		// Get result value
		var actualValue int64
		switch v := result.Result.(type) {
		case float64:
			actualValue = int64(v)
		case int64:
			actualValue = v
		case int32:
			actualValue = int64(v)
		case int:
			actualValue = int64(v)
		case []uint64:
			if len(v) > 0 {
				actualValue = int64(v[0])
			}
		default:
			fmt.Printf("  ⚠️  Unknown result type: %T = %v\n", result.Result, result.Result)
			actualValue = -1
		}

		// Check result
		if actualValue == tc.expected {
			fmt.Printf("  ✅ Result: %d (correct!)\n", actualValue)
			successCount++
		} else {
			fmt.Printf("  ❌ Result: %d (expected %d)\n", actualValue, tc.expected)
		}

		fmt.Printf("  ⏱️  Duration: %v\n", result.Duration)
		fmt.Printf("  💾 Memory: %d bytes\n", result.MemoryUsed)
		totalDuration += result.Duration
		fmt.Println("")
	}

	// Summary
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("📊 Test Summary:")
	fmt.Printf("   Passed: %d/%d (%.1f%%)\n", successCount, len(testCases), float64(successCount)/float64(len(testCases))*100)
	fmt.Printf("   Total Duration: %v\n", totalDuration)
	fmt.Printf("   Avg Duration: %v\n", totalDuration/time.Duration(len(testCases)))
	fmt.Println("")

	if successCount == len(testCases) {
		fmt.Println("🎉 All tests passed!")
		fmt.Println("")
		fmt.Println("📚 Next Steps:")
		fmt.Println("   1. Create more WASM agents (string-agent, http-agent, etc.)")
		fmt.Println("   2. Test with Meta-Agent orchestrator")
		fmt.Println("   3. Deploy to production")
	} else {
		fmt.Printf("⚠️  Some tests failed (%d/%d)\n", len(testCases)-successCount, len(testCases))
		os.Exit(1)
	}
}
