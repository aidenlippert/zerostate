package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/execution"
	"github.com/aidenlippert/zerostate/libs/identity"
	"go.uber.org/zap"
)

// WASMTaskExecutor executes tasks using real WASM binaries
type WASMTaskExecutor struct {
	wasmRunner  *execution.WASMRunner
	binaryStore execution.BinaryStore
	logger      *zap.Logger
}

// NewWASMTaskExecutor creates a new WASM task executor
func NewWASMTaskExecutor(wasmRunner *execution.WASMRunner, binaryStore execution.BinaryStore, logger *zap.Logger) *WASMTaskExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &WASMTaskExecutor{
		wasmRunner:  wasmRunner,
		binaryStore: binaryStore,
		logger:      logger,
	}
}

// ExecuteTask executes a task using real WASM execution
func (e *WASMTaskExecutor) ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error) {
	e.logger.Info("executing task with WASM",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
	)

	start := time.Now()

	// Get WASM binary from store
	wasmBinary, err := e.binaryStore.GetBinary(ctx, agent.DID)
	if err != nil {
		e.logger.Error("failed to get WASM binary",
			zap.String("agent_id", agent.DID),
			zap.Error(err),
		)
		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("failed to get WASM binary: %v", err),
			ExecutionMS: time.Since(start).Milliseconds(),
		}, nil
	}

	// Convert task input to bytes
	var inputBytes []byte
	if task.Input != nil {
		inputBytes, err = json.Marshal(task.Input)
		if err != nil {
			e.logger.Error("failed to marshal task input",
				zap.Error(err),
			)
			return &TaskResult{
				TaskID:      task.ID,
				Status:      TaskStatusFailed,
				Error:       fmt.Sprintf("failed to marshal input: %v", err),
				ExecutionMS: time.Since(start).Milliseconds(),
			}, nil
		}
	}

	// Set timeout from task or use default
	timeout := 30 * time.Second
	if task.Timeout > 0 {
		timeout = time.Duration(task.Timeout) * time.Second
	}

	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute WASM with resource limits
	limits := execution.ResourceLimits{
		MaxMemoryMB: 512, // Default 512MB limit
		MaxCPUCores: 1,   // Single core
		Timeout:     timeout,
	}

	wasmResult, err := e.wasmRunner.ExecuteWithLimits(execCtx, wasmBinary, inputBytes, limits)
	if err != nil {
		e.logger.Error("WASM execution failed",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       fmt.Sprintf("execution failed: %v", err),
			ExecutionMS: time.Since(start).Milliseconds(),
		}, nil
	}

	// Parse output
	var result map[string]interface{}
	if len(wasmResult.Stdout) > 0 {
		if err := json.Unmarshal(wasmResult.Stdout, &result); err != nil {
			// If not JSON, store as raw string
			result = map[string]interface{}{
				"output": string(wasmResult.Stdout),
			}
		}
	} else {
		result = map[string]interface{}{
			"output": "",
		}
	}

	// Add stderr if present
	if len(wasmResult.Stderr) > 0 {
		result["stderr"] = string(wasmResult.Stderr)
	}

	// Determine status based on exit code
	status := TaskStatusCompleted
	if wasmResult.ExitCode != 0 {
		status = TaskStatusFailed
		result["exit_code"] = wasmResult.ExitCode
	}

	executionTime := time.Since(start).Milliseconds()
	e.logger.Info("task execution complete",
		zap.String("task_id", task.ID),
		zap.String("status", string(status)),
		zap.Int64("execution_ms", executionTime),
	)

	return &TaskResult{
		TaskID:      task.ID,
		Status:      status,
		Result:      result,
		ExecutionMS: executionTime,
	}, nil
}
