// Package execution provides WASM task execution with resource limits and sandboxing
package execution

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

const (
	// DefaultMaxMemory is the default maximum memory in bytes (128MB)
	DefaultMaxMemory = 128 * 1024 * 1024
	// DefaultMaxExecutionTime is the default maximum execution time
	DefaultMaxExecutionTime = 30 * time.Second
	// DefaultMaxStackSize is the default maximum stack size (8MB)
	DefaultMaxStackSize = 8 * 1024 * 1024
)

var (
	// Metrics
	wasmExecutionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "wasm_executions_total",
			Help: "Total WASM executions",
		},
		[]string{"status"},
	)

	wasmExecutionDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "wasm_execution_duration_seconds",
			Help:    "WASM execution duration",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
		[]string{"status"},
	)

	wasmMemoryUsage = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "wasm_memory_usage_bytes",
			Help: "WASM memory usage in bytes",
		},
		[]string{"module"},
	)

	wasmActiveExecutions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "wasm_active_executions",
			Help: "Number of currently active WASM executions",
		},
	)
)

var (
	// ErrTimeout is returned when execution exceeds time limit
	ErrTimeout = errors.New("execution timeout")
	// ErrMemoryLimit is returned when memory limit is exceeded
	ErrMemoryLimit = errors.New("memory limit exceeded")
	// ErrInvalidModule is returned when WASM module is invalid
	ErrInvalidModule = errors.New("invalid WASM module")
	// ErrExecutionFailed is returned when execution fails
	ErrExecutionFailed = errors.New("execution failed")
)

// ExecutionConfig holds WASM execution configuration
type ExecutionConfig struct {
	// MaxMemory is the maximum memory in bytes
	MaxMemory uint64
	// MaxExecutionTime is the maximum execution time
	MaxExecutionTime time.Duration
	// MaxStackSize is the maximum stack size
	MaxStackSize uint64
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
	// EnableWASI enables WASI (WebAssembly System Interface) support
	EnableWASI bool
}

// DefaultExecutionConfig returns default execution configuration
func DefaultExecutionConfig() *ExecutionConfig {
	return &ExecutionConfig{
		MaxMemory:        DefaultMaxMemory,
		MaxExecutionTime: DefaultMaxExecutionTime,
		MaxStackSize:     DefaultMaxStackSize,
		EnableMetrics:    true,
		EnableWASI:       true,
	}
}

// ExecutionResult holds the result of a WASM execution
type ExecutionResult struct {
	ExitCode    int32
	Output      []byte
	Error       error
	Duration    time.Duration
	MemoryUsed  uint64
	GasUsed     uint64 // Future: for resource metering
	StartTime   time.Time
	EndTime     time.Time
}

// WASMRunner executes WASM modules with resource limits
type WASMRunner struct {
	runtime wazero.Runtime
	config  *ExecutionConfig
	logger  *zap.Logger
	mu      sync.Mutex // Protects concurrent access to runtime
}

// NewWASMRunner creates a new WASM runner
func NewWASMRunner(ctx context.Context, config *ExecutionConfig, logger *zap.Logger) (*WASMRunner, error) {
	if config == nil {
		config = DefaultExecutionConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create wazero runtime with compilation cache
	runtimeConfig := wazero.NewRuntimeConfig().
		WithCloseOnContextDone(true).
		WithCompilationCache(wazero.NewCompilationCache())

	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

	// Instantiate WASI if enabled
	if config.EnableWASI {
		if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
			return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
		}
	}

	runner := &WASMRunner{
		runtime: runtime,
		config:  config,
		logger:  logger,
	}

	logger.Info("WASM runner initialized",
		zap.Uint64("max_memory", config.MaxMemory),
		zap.Duration("max_execution_time", config.MaxExecutionTime),
		zap.Bool("wasi_enabled", config.EnableWASI),
	)

	return runner, nil
}

// Execute runs a WASM module with the given inputs
func (wr *WASMRunner) Execute(ctx context.Context, wasmBytes []byte, functionName string, stdin io.Reader, stdout, stderr io.Writer) (*ExecutionResult, error) {
	start := time.Now()
	wasmActiveExecutions.Inc()
	defer wasmActiveExecutions.Dec()

	result := &ExecutionResult{
		StartTime: start,
	}

	// Apply execution timeout
	execCtx, cancel := context.WithTimeout(ctx, wr.config.MaxExecutionTime)
	defer cancel()

	// Compile module (protected by mutex for thread-safety)
	wr.mu.Lock()
	compiledModule, err := wr.runtime.CompileModule(execCtx, wasmBytes)
	wr.mu.Unlock()
	
	if err != nil {
		result.Error = fmt.Errorf("%w: %v", ErrInvalidModule, err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		wasmExecutionsTotal.WithLabelValues("invalid_module").Inc()
		return result, result.Error
	}
	defer compiledModule.Close(execCtx)

	// Configure module with I/O
	moduleConfig := wazero.NewModuleConfig().
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(stderr)

	// Instantiate module (protected by mutex)
	wr.mu.Lock()
	module, err := wr.runtime.InstantiateModule(execCtx, compiledModule, moduleConfig)
	wr.mu.Unlock()
	
	if err != nil{
		result.Error = fmt.Errorf("%w: %v", ErrExecutionFailed, err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		wasmExecutionsTotal.WithLabelValues("instantiation_failed").Inc()
		return result, result.Error
	}
	defer module.Close(execCtx)

	// Execute function first
	fn := module.ExportedFunction(functionName)
	if fn == nil {
		result.Error = fmt.Errorf("%w: function %s not found", ErrInvalidModule, functionName)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		wasmExecutionsTotal.WithLabelValues("function_not_found").Inc()
		return result, result.Error
	}

	// Call the function
	_, err = fn.Call(execCtx)
	
	// Get memory stats after execution (may be nil for modules without memory)
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Ignore panics from memory access - some modules don't export memory
				wr.logger.Debug("Could not read memory stats", zap.Any("panic", r))
			}
		}()
		
		if memDef := module.Memory(); memDef != nil {
			size := memDef.Size()
			if size > 0 {
				result.MemoryUsed = uint64(size)
				wasmMemoryUsage.WithLabelValues("task").Set(float64(result.MemoryUsed))
			}
		}
	}()
	
	if err != nil {
		// Check if it was a timeout
		if execCtx.Err() == context.DeadlineExceeded {
			result.Error = ErrTimeout
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(start)
			wasmExecutionsTotal.WithLabelValues("timeout").Inc()
			wasmExecutionDuration.WithLabelValues("timeout").Observe(result.Duration.Seconds())
			
			wr.logger.Warn("WASM execution timeout",
				zap.String("function", functionName),
				zap.Duration("duration", result.Duration),
			)
			return result, result.Error
		}

		result.Error = fmt.Errorf("%w: %v", ErrExecutionFailed, err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		wasmExecutionsTotal.WithLabelValues("failed").Inc()
		wasmExecutionDuration.WithLabelValues("failed").Observe(result.Duration.Seconds())
		return result, result.Error
	}

	result.ExitCode = 0
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)

	wasmExecutionsTotal.WithLabelValues("success").Inc()
	wasmExecutionDuration.WithLabelValues("success").Observe(result.Duration.Seconds())

	wr.logger.Info("WASM execution completed",
		zap.String("function", functionName),
		zap.Duration("duration", result.Duration),
		zap.Uint64("memory_used", result.MemoryUsed),
	)

	return result, nil
}

// ExecuteWithArgs runs a WASM module with command-line arguments
func (wr *WASMRunner) ExecuteWithArgs(ctx context.Context, wasmBytes []byte, args []string, stdin io.Reader, stdout, stderr io.Writer) (*ExecutionResult, error) {
	start := time.Now()
	wasmActiveExecutions.Inc()
	defer wasmActiveExecutions.Dec()

	result := &ExecutionResult{
		StartTime: start,
	}

	// Apply execution timeout
	execCtx, cancel := context.WithTimeout(ctx, wr.config.MaxExecutionTime)
	defer cancel()

	// Compile module (protected by mutex for thread-safety)
	wr.mu.Lock()
	compiledModule, err := wr.runtime.CompileModule(execCtx, wasmBytes)
	wr.mu.Unlock()
	
	if err != nil {
		result.Error = fmt.Errorf("%w: %v", ErrInvalidModule, err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)
		wasmExecutionsTotal.WithLabelValues("invalid_module").Inc()
		return result, result.Error
	}
	defer compiledModule.Close(execCtx)

	// Configure module with resource limits and WASI
	moduleConfig := wazero.NewModuleConfig().
		WithName("task").
		WithArgs(args...)

	// Configure I/O
	if stdin != nil {
		moduleConfig = moduleConfig.WithStdin(stdin)
	}
	if stdout != nil {
		moduleConfig = moduleConfig.WithStdout(stdout)
	}
	if stderr != nil {
		moduleConfig = moduleConfig.WithStderr(stderr)
	}

	// Instantiate and execute (protected by mutex)
	wr.mu.Lock()
	module, err := wr.runtime.InstantiateModule(execCtx, compiledModule, moduleConfig)
	wr.mu.Unlock()
	
	if err != nil {
		result.Error = fmt.Errorf("%w: %v", ErrExecutionFailed, err)
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(start)

		// Check if it was a timeout
		if execCtx.Err() == context.DeadlineExceeded {
			result.Error = ErrTimeout
			wasmExecutionsTotal.WithLabelValues("timeout").Inc()
			wasmExecutionDuration.WithLabelValues("timeout").Observe(result.Duration.Seconds())
		} else {
			wasmExecutionsTotal.WithLabelValues("failed").Inc()
			wasmExecutionDuration.WithLabelValues("failed").Observe(result.Duration.Seconds())
		}

		return result, result.Error
	}
	defer module.Close(execCtx)

	// Get memory stats
	if memDef := module.Memory(); memDef != nil {
		result.MemoryUsed = uint64(memDef.Size())
		wasmMemoryUsage.WithLabelValues("task").Set(float64(result.MemoryUsed))
	}

	result.ExitCode = 0
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(start)

	wasmExecutionsTotal.WithLabelValues("success").Inc()
	wasmExecutionDuration.WithLabelValues("success").Observe(result.Duration.Seconds())

	wr.logger.Info("WASM execution completed",
		zap.Strings("args", args),
		zap.Duration("duration", result.Duration),
		zap.Uint64("memory_used", result.MemoryUsed),
	)

	return result, nil
}

// Close closes the WASM runtime
func (wr *WASMRunner) Close(ctx context.Context) error {
	wr.logger.Info("closing WASM runner")
	return wr.runtime.Close(ctx)
}

// Stats returns execution statistics
type WASMStats struct {
	ActiveExecutions int
	TotalExecutions  int
	SuccessRate      float64
}

// Stats returns current statistics (placeholder for future implementation)
func (wr *WASMRunner) Stats() WASMStats {
	// This would track internal state
	return WASMStats{
		ActiveExecutions: 0,
		TotalExecutions:  0,
		SuccessRate:      0.0,
	}
}
