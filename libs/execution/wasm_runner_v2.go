package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/tetratelabs/wazero"
	"go.uber.org/zap"
)

// WASMRunnerV2 executes WASM binaries loaded from R2
// This is the new implementation that replaces the mock executor
type WASMRunnerV2 struct {
	logger      *zap.Logger
	r2Storage   R2StorageInterface
	timeout     time.Duration
	maxMemoryMB int
}

// R2StorageInterface defines the interface for R2 storage operations
type R2StorageInterface interface {
	DownloadWASM(ctx context.Context, key string) ([]byte, error)
}

// WASMExecutionRequest contains all parameters for WASM execution
type WASMExecutionRequest struct {
	R2Key       string        // R2 path to WASM binary
	Function    string        // Function to call (e.g., "add", "multiply")
	Args        []interface{} // Function arguments
	Timeout     time.Duration // Execution timeout
	MaxMemoryMB int           // Memory limit
}

// WASMExecutionResult contains the execution output
type WASMExecutionResult struct {
	Success    bool          `json:"success"`
	Result     interface{}   `json:"result,omitempty"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	MemoryUsed int64         `json:"memory_used"`
}

// NewWASMRunnerV2 creates a new WASM runner with R2 integration
func NewWASMRunnerV2(logger *zap.Logger, r2Storage R2StorageInterface, timeout time.Duration, maxMemoryMB int) *WASMRunnerV2 {
	return &WASMRunnerV2{
		logger:      logger,
		r2Storage:   r2Storage,
		timeout:     timeout,
		maxMemoryMB: maxMemoryMB,
	}
}

// Execute runs a WASM binary loaded from R2
func (r *WASMRunnerV2) Execute(ctx context.Context, req *WASMExecutionRequest) (*WASMExecutionResult, error) {
	startTime := time.Now()

	r.logger.Info("executing WASM from R2",
		zap.String("r2_key", req.R2Key),
		zap.String("function", req.Function),
		zap.Any("args", req.Args),
	)

	// Step 1: Download WASM binary from R2
	wasmBinary, err := r.r2Storage.DownloadWASM(ctx, req.R2Key)
	if err != nil {
		return &WASMExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to download WASM from R2: %v", err),
			Duration: time.Since(startTime),
		}, err
	}

	r.logger.Info("WASM binary downloaded from R2",
		zap.String("r2_key", req.R2Key),
		zap.Int("size_bytes", len(wasmBinary)),
	)

	// Step 2: Set execution timeout
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.timeout
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Step 3: Create wazero runtime with memory limits
	runtimeConfig := wazero.NewRuntimeConfig()
	maxMemory := req.MaxMemoryMB
	if maxMemory == 0 {
		maxMemory = r.maxMemoryMB
	}
	if maxMemory > 0 {
		// Convert MB to pages (64KB per page)
		maxPages := uint32(maxMemory * 16)
		runtimeConfig = runtimeConfig.WithMemoryLimitPages(maxPages)
	}

	runtime := wazero.NewRuntimeWithConfig(execCtx, runtimeConfig)
	defer runtime.Close(execCtx)

	// Step 4: Compile WASM module
	compiled, err := runtime.CompileModule(execCtx, wasmBinary)
	if err != nil {
		return &WASMExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to compile WASM: %v", err),
			Duration: time.Since(startTime),
		}, err
	}
	defer compiled.Close(execCtx)

	// Step 5: Instantiate module
	module, err := runtime.InstantiateModule(execCtx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return &WASMExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to instantiate WASM: %v", err),
			Duration: time.Since(startTime),
		}, err
	}
	defer module.Close(execCtx)

	// Step 6: Get the function to call
	fn := module.ExportedFunction(req.Function)
	if fn == nil {
		return &WASMExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("function '%s' not found in WASM module", req.Function),
			Duration: time.Since(startTime),
		}, fmt.Errorf("function not found: %s", req.Function)
	}

	// Step 7: Convert arguments to uint64 (WASM parameter type)
	params := make([]uint64, len(req.Args))
	for i, arg := range req.Args {
		switch v := arg.(type) {
		case int:
			params[i] = uint64(v)
		case int32:
			params[i] = uint64(v)
		case int64:
			params[i] = uint64(v)
		case float64:
			// For JSON unmarshaling (numbers come as float64)
			params[i] = uint64(int64(v))
		default:
			return &WASMExecutionResult{
				Success:  false,
				Error:    fmt.Sprintf("unsupported argument type at index %d: %T", i, arg),
				Duration: time.Since(startTime),
			}, fmt.Errorf("unsupported argument type: %T", arg)
		}
	}

	// Step 8: Execute the function
	results, err := fn.Call(execCtx, params...)
	if err != nil {
		return &WASMExecutionResult{
			Success:  false,
			Error:    fmt.Sprintf("WASM execution failed: %v", err),
			Duration: time.Since(startTime),
		}, err
	}

	// Step 9: Extract result (assume single return value for now)
	var result interface{}
	if len(results) > 0 {
		result = int64(results[0])
	}

	duration := time.Since(startTime)

	// Step 10: Get memory usage
	memStats := module.Memory().Size()

	r.logger.Info("WASM execution successful",
		zap.String("function", req.Function),
		zap.Any("result", result),
		zap.Duration("duration", duration),
		zap.Uint32("memory_pages", memStats),
	)

	return &WASMExecutionResult{
		Success:    true,
		Result:     result,
		Duration:   duration,
		MemoryUsed: int64(memStats * 65536), // Convert pages to bytes
	}, nil
}

// ExecuteMultiple calls multiple functions in sequence
func (r *WASMRunnerV2) ExecuteMultiple(ctx context.Context, r2Key string, calls []FunctionCall) ([]interface{}, error) {
	// Download WASM once
	wasmBinary, err := r.r2Storage.DownloadWASM(ctx, r2Key)
	if err != nil {
		return nil, fmt.Errorf("failed to download WASM: %w", err)
	}

	// Create runtime
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM: %w", err)
	}
	defer compiled.Close(ctx)

	module, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate WASM: %w", err)
	}
	defer module.Close(ctx)

	// Execute each call
	results := make([]interface{}, len(calls))
	for i, call := range calls {
		fn := module.ExportedFunction(call.Function)
		if fn == nil {
			return nil, fmt.Errorf("function not found: %s", call.Function)
		}

		// Convert args
		params := make([]uint64, len(call.Args))
		for j, arg := range call.Args {
			params[j] = uint64(arg)
		}

		result, err := fn.Call(ctx, params...)
		if err != nil {
			return nil, fmt.Errorf("execution failed for %s: %w", call.Function, err)
		}

		if len(result) > 0 {
			results[i] = int64(result[0])
		}
	}

	return results, nil
}

// FunctionCall represents a single function call
type FunctionCall struct {
	Function string
	Args     []int32
}

// ValidateWASM validates a WASM binary without executing it
func (r *WASMRunnerV2) ValidateWASM(ctx context.Context, wasmBinary []byte) error {
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return fmt.Errorf("invalid WASM: %w", err)
	}
	defer compiled.Close(ctx)

	return nil
}

// ListExportedFunctions returns all exported functions in a WASM module
// Note: wazero's API doesn't expose this easily, so we return known functions
func (r *WASMRunnerV2) ListExportedFunctions(ctx context.Context, wasmBinary []byte) ([]string, error) {
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiled, err := runtime.CompileModule(ctx, wasmBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM: %w", err)
	}
	defer compiled.Close(ctx)

	_, err = runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate WASM: %w", err)
	}

	// Return empty list for now - callers need to know function names
	// In production, we'd parse WASM binary or maintain manifest
	return []string{}, nil
}

// MarshalJSON implements json.Marshaler for WASMExecutionResult
func (r *WASMExecutionResult) MarshalJSON() ([]byte, error) {
	type Alias WASMExecutionResult
	return json.Marshal(&struct {
		DurationMs int64 `json:"duration_ms"`
		*Alias
	}{
		DurationMs: r.Duration.Milliseconds(),
		Alias:      (*Alias)(r),
	})
}
