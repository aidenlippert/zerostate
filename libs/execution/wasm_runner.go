package execution

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

// WASMRunner executes WASM binaries in a sandboxed environment
type WASMRunner struct {
	logger  *zap.Logger
	timeout time.Duration
}

// WASMResult contains the execution result
type WASMResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
	Error    error
}

// NewWASMRunner creates a new WASM runner
func NewWASMRunner(logger *zap.Logger, timeout time.Duration) *WASMRunner {
	return &WASMRunner{
		logger:  logger,
		timeout: timeout,
	}
}

// Execute runs a WASM binary with the given input
func (r *WASMRunner) Execute(ctx context.Context, wasmBinary []byte, input []byte) (*WASMResult, error) {
	startTime := time.Now()

	r.logger.Info("starting WASM execution",
		zap.Int("binary_size", len(wasmBinary)),
		zap.Int("input_size", len(input)),
	)

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Create new runtime instance (sandboxed)
	runtime := wazero.NewRuntime(execCtx)
	defer runtime.Close(execCtx)

	// Instantiate WASI (provides filesystem, env, etc.)
	wasi_snapshot_preview1.MustInstantiate(execCtx, runtime)

	// Capture stdout and stderr
	stdoutBuf := &captureWriter{}
	stderrBuf := &captureWriter{}

	// Configure module with stdio
	config := wazero.NewModuleConfig().
		WithStdout(stdoutBuf).
		WithStderr(stderrBuf).
		WithStdin(nil). // No stdin for now
		WithStartFunctions("_start")

	// Compile and instantiate the WASM module
	compiled, err := runtime.CompileModule(execCtx, wasmBinary)
	if err != nil {
		r.logger.Error("failed to compile WASM module", zap.Error(err))
		return &WASMResult{
			ExitCode: -1,
			Error:    fmt.Errorf("compilation failed: %w", err),
			Duration: time.Since(startTime),
		}, err
	}
	defer compiled.Close(execCtx)

	// Instantiate and run
	module, err := runtime.InstantiateModule(execCtx, compiled, config)
	if err != nil {
		r.logger.Error("failed to instantiate WASM module", zap.Error(err))
		return &WASMResult{
			ExitCode: -1,
			Stdout:   stdoutBuf.Bytes(),
			Stderr:   stderrBuf.Bytes(),
			Error:    fmt.Errorf("instantiation failed: %w", err),
			Duration: time.Since(startTime),
		}, err
	}
	defer module.Close(execCtx)

	duration := time.Since(startTime)

	result := &WASMResult{
		ExitCode: 0, // If we got here, execution succeeded
		Stdout:   stdoutBuf.Bytes(),
		Stderr:   stderrBuf.Bytes(),
		Duration: duration,
	}

	r.logger.Info("WASM execution completed",
		zap.Int("exit_code", result.ExitCode),
		zap.Int("stdout_size", len(result.Stdout)),
		zap.Int("stderr_size", len(result.Stderr)),
		zap.Duration("duration", duration),
	)

	return result, nil
}

// ExecuteWithLimits runs WASM with resource limits
func (r *WASMRunner) ExecuteWithLimits(ctx context.Context, wasmBinary []byte, input []byte, limits ResourceLimits) (*WASMResult, error) {
	// TODO: Implement memory and CPU limits
	// For now, just use timeout
	return r.Execute(ctx, wasmBinary, input)
}

// ResourceLimits defines execution constraints
type ResourceLimits struct {
	MaxMemoryMB int
	MaxCPUCores int
	Timeout     time.Duration
}

// captureWriter captures bytes written to it
type captureWriter struct {
	buf []byte
}

func (w *captureWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	return len(p), nil
}

func (w *captureWriter) Bytes() []byte {
	return w.buf
}

var _ io.Writer = (*captureWriter)(nil)
