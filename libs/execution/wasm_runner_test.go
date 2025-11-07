package execution

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Minimal valid WASM module that exports a simple function
// (module (func (export "add") (result i32) i32.const 42))
var simpleWASM = []byte{
	0x00, 0x61, 0x73, 0x6d, // WASM magic number
	0x01, 0x00, 0x00, 0x00, // WASM version 1
	0x01, 0x05, 0x01, 0x60, 0x00, 0x01, 0x7f, // Type section: function type () -> i32
	0x03, 0x02, 0x01, 0x00, // Function section: 1 function of type 0
	0x07, 0x07, 0x01, 0x03, 0x61, 0x64, 0x64, 0x00, 0x00, // Export section: export function 0 as "add"
	0x0a, 0x06, 0x01, 0x04, 0x00, 0x41, 0x2a, 0x0b, // Code section: function returns i32.const 42
}

func TestNewWASMRunner(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := DefaultExecutionConfig()

	runner, err := NewWASMRunner(ctx, config, logger)
	require.NoError(t, err)
	require.NotNil(t, runner)
	defer runner.Close(ctx)

	assert.Equal(t, config.MaxMemory, runner.config.MaxMemory)
	assert.Equal(t, config.MaxExecutionTime, runner.config.MaxExecutionTime)
	assert.True(t, runner.config.EnableWASI)
}

func TestDefaultExecutionConfig(t *testing.T) {
	config := DefaultExecutionConfig()

	assert.Equal(t, uint64(DefaultMaxMemory), config.MaxMemory)
	assert.Equal(t, DefaultMaxExecutionTime, config.MaxExecutionTime)
	assert.Equal(t, uint64(DefaultMaxStackSize), config.MaxStackSize)
	assert.True(t, config.EnableMetrics)
	assert.True(t, config.EnableWASI)
}

func TestExecuteSimpleWASM(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	require.NotNil(t, runner)
	defer runner.Close(ctx)

	var stdout, stderr bytes.Buffer
	result, err := runner.Execute(ctx, simpleWASM, "add", nil, &stdout, &stderr)
	
	t.Logf("Execute returned error: %v", err)
	if result != nil {
		t.Logf("Result: %+v", result)
	} else {
		t.Logf("Result is nil!")
	}
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(0), result.ExitCode)
	assert.Greater(t, result.Duration, time.Duration(0))
	assert.False(t, result.StartTime.IsZero())
	assert.False(t, result.EndTime.IsZero())
}

func TestExecuteInvalidWASM(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	invalidWASM := []byte{0x00, 0x00, 0x00, 0x00} // Invalid WASM

	result, err := runner.Execute(ctx, invalidWASM, "main", nil, nil, nil)
	
	assert.Error(t, err)
	assert.ErrorIs(t, result.Error, ErrInvalidModule)
}

func TestExecuteFunctionNotFound(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	result, err := runner.Execute(ctx, simpleWASM, "nonexistent", nil, nil, nil)
	
	assert.Error(t, err)
	assert.ErrorIs(t, result.Error, ErrInvalidModule)
	assert.Contains(t, err.Error(), "function nonexistent not found")
}

func TestExecuteWithTimeout(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := DefaultExecutionConfig()
	config.MaxExecutionTime = 10 * time.Millisecond // Very short timeout

	runner, err := NewWASMRunner(ctx, config, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	// Verify the timeout configuration is set
	assert.Equal(t, 10*time.Millisecond, runner.config.MaxExecutionTime)
	
	// Test with simple WASM - should complete before timeout
	var stdout, stderr bytes.Buffer
	result, err := runner.Execute(ctx, simpleWASM, "add", nil, &stdout, &stderr)
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestExecuteWithStdin(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	stdin := strings.NewReader("input data\n")
	var stdout, stderr bytes.Buffer

	result, err := runner.Execute(ctx, simpleWASM, "add", stdin, &stdout, &stderr)
	
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(0), result.ExitCode)
}

func TestMemoryLimit(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := DefaultExecutionConfig()
	config.MaxMemory = 1024 * 1024 // 1MB limit

	runner, err := NewWASMRunner(ctx, config, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	result, err := runner.Execute(ctx, simpleWASM, "add", nil, nil, nil)
	
	require.NoError(t, err)
	// Memory should be within limits
	assert.LessOrEqual(t, result.MemoryUsed, config.MaxMemory)
}

func TestConcurrentExecutions(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	// Run 10 concurrent executions - each will compile independently
	var wg sync.WaitGroup
	errors := make(chan error, 10)
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var stdout, stderr bytes.Buffer
			result, err := runner.Execute(ctx, simpleWASM, "add", nil, &stdout, &stderr)
			if err != nil {
				errors <- err
				return
			}
			if result.ExitCode != 0 {
				errors <- fmt.Errorf("unexpected exit code: %d", result.ExitCode)
			}
		}()
	}

	wg.Wait()
	close(errors)
	
	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent execution failed: %v", err)
	}
}

func TestClose(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)

	err = runner.Close(ctx)
	assert.NoError(t, err)
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, err := NewWASMRunner(ctx, nil, logger)
	require.NoError(t, err)
	defer runner.Close(ctx)

	stats := runner.Stats()
	// Stats is a placeholder for now
	assert.NotNil(t, stats)
}

func BenchmarkExecuteSimpleWASM(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	runner, _ := NewWASMRunner(ctx, nil, logger)
	defer runner.Close(ctx)

	var stdout, stderr bytes.Buffer

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stdout.Reset()
		stderr.Reset()
		runner.Execute(ctx, simpleWASM, "add", nil, &stdout, &stderr)
	}
}

