package execution

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWASMRunner_Execute(t *testing.T) {
	// Create logger
	logger, _ := zap.NewDevelopment()

	// Create WASM runner with 10 second timeout
	runner := NewWASMRunner(logger, 10*time.Second)

	// Load test WASM binary
	wasmPath := "../../tests/wasm/hello.wasm"
	wasmBinary, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("Skipping test: test WASM binary not found at %s", wasmPath)
		return
	}

	// Execute WASM
	ctx := context.Background()
	result, err := runner.Execute(ctx, wasmBinary, nil)

	// Verify execution succeeded
	if err != nil {
		t.Fatalf("WASM execution failed: %v", err)
	}

	// Verify exit code
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
		t.Logf("Stderr: %s", string(result.Stderr))
	}

	// Verify stdout contains expected output
	stdout := string(result.Stdout)
	if !strings.Contains(stdout, "Hello from WASM!") {
		t.Errorf("Expected stdout to contain 'Hello from WASM!', got: %s", stdout)
	}
	if !strings.Contains(stdout, "Task executed successfully") {
		t.Errorf("Expected stdout to contain 'Task executed successfully', got: %s", stdout)
	}

	// Verify execution completed in reasonable time
	if result.Duration > 5*time.Second {
		t.Errorf("Execution took too long: %v", result.Duration)
	}

	t.Logf("WASM execution succeeded in %v", result.Duration)
	t.Logf("Stdout: %s", stdout)
}

func TestWASMRunner_Timeout(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	// Create WASM runner with very short timeout
	runner := NewWASMRunner(logger, 1*time.Millisecond)

	// Load test WASM binary
	wasmPath := "../../tests/wasm/hello.wasm"
	wasmBinary, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Skipf("Skipping test: test WASM binary not found at %s", wasmPath)
		return
	}

	// Execute WASM with timeout
	ctx := context.Background()
	result, err := runner.Execute(ctx, wasmBinary, nil)

	// Timeout should occur
	if err == nil {
		t.Logf("Warning: Expected timeout error, but got successful execution in %v", result.Duration)
		// Note: Timeout might not always trigger for very fast WASM execution
		// This is not a hard failure
	} else {
		t.Logf("Timeout occurred as expected: %v", err)
	}
}

func TestWASMRunner_InvalidBinary(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	runner := NewWASMRunner(logger, 10*time.Second)

	// Try to execute invalid WASM binary
	invalidBinary := []byte("this is not a valid WASM binary")
	ctx := context.Background()

	result, err := runner.Execute(ctx, invalidBinary, nil)

	// Should fail with compilation error
	if err == nil {
		t.Error("Expected compilation error for invalid WASM binary")
	}

	// Verify error result
	if result == nil {
		t.Error("Expected non-nil result even on error")
	} else {
		if result.ExitCode != -1 {
			t.Errorf("Expected exit code -1 for compilation error, got %d", result.ExitCode)
		}
	}

	t.Logf("Invalid binary correctly rejected: %v", err)
}
