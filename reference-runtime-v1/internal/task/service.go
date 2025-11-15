package task

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ariv1 "github.com/aidenlippert/zerostate/reference-runtime-v1/pkg/ari/v1"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"go.uber.org/zap"
)

// Service implements the ARI v1 Task service
type Service struct {
	ariv1.UnimplementedTaskServer

	executor *WASMExecutor
	logger   *zap.Logger

	// Task tracking
	mu          sync.RWMutex
	activeTasks map[string]context.CancelFunc
}

// NewService creates a new Task service
func NewService(wasmPath string, logger *zap.Logger) (*Service, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	executor, err := NewWASMExecutor(wasmPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create WASM executor: %w", err)
	}

	return &Service{
		executor:    executor,
		logger:      logger,
		activeTasks: make(map[string]context.CancelFunc),
	}, nil
}

// Execute executes a task and streams the response
func (s *Service) Execute(req *ariv1.TaskExecuteRequest, stream ariv1.Task_ExecuteServer) error {
	startTime := time.Now()
	ctx := stream.Context()

	s.logger.Info("Task execution started",
		zap.String("task_id", req.TaskId),
		zap.String("input", req.Input),
	)

	// Register task
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	s.mu.Lock()
	s.activeTasks[req.TaskId] = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.activeTasks, req.TaskId)
		s.mu.Unlock()
	}()

	// Send initial status
	if err := stream.Send(&ariv1.TaskExecuteResponse{
		TaskId:          req.TaskId,
		Status:          ariv1.TaskStatus_TASK_STATUS_RUNNING,
		Progress:        0.0,
		ProgressMessage: "Task started",
	}); err != nil {
		return err
	}

	// Parse input
	var input TaskInput
	if err := json.Unmarshal([]byte(req.Input), &input); err != nil {
		s.logger.Error("Failed to parse task input",
			zap.String("task_id", req.TaskId),
			zap.Error(err),
		)

		return stream.Send(&ariv1.TaskExecuteResponse{
			TaskId:      req.TaskId,
			Status:      ariv1.TaskStatus_TASK_STATUS_FAILED,
			Error:       fmt.Sprintf("Invalid input: %v", err),
			ExecutionMs: time.Since(startTime).Milliseconds(),
		})
	}

	// Execute the task
	result, err := s.executor.Execute(taskCtx, &input)
	executionTime := time.Since(startTime)

	if err != nil {
		s.logger.Error("Task execution failed",
			zap.String("task_id", req.TaskId),
			zap.Error(err),
			zap.Duration("execution_time", executionTime),
		)

		return stream.Send(&ariv1.TaskExecuteResponse{
			TaskId:      req.TaskId,
			Status:      ariv1.TaskStatus_TASK_STATUS_FAILED,
			Error:       err.Error(),
			ExecutionMs: executionTime.Milliseconds(),
		})
	}

	// Marshal result
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return stream.Send(&ariv1.TaskExecuteResponse{
			TaskId:      req.TaskId,
			Status:      ariv1.TaskStatus_TASK_STATUS_FAILED,
			Error:       fmt.Sprintf("Failed to marshal result: %v", err),
			ExecutionMs: executionTime.Milliseconds(),
		})
	}

	// Send success response
	s.logger.Info("Task execution completed",
		zap.String("task_id", req.TaskId),
		zap.Duration("execution_time", executionTime),
	)

	return stream.Send(&ariv1.TaskExecuteResponse{
		TaskId:          req.TaskId,
		Status:          ariv1.TaskStatus_TASK_STATUS_COMPLETED,
		Result:          string(resultJSON),
		ExecutionMs:     executionTime.Milliseconds(),
		Progress:        1.0,
		ProgressMessage: "Task completed successfully",
	})
}

// CancelTask cancels a running task
func (s *Service) CancelTask(taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, exists := s.activeTasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	cancel()
	s.logger.Info("Task cancelled", zap.String("task_id", taskID))
	return nil
}

// TaskInput represents the input to a task
type TaskInput struct {
	Function string        `json:"function"`
	Args     []interface{} `json:"args"`
}

// WASMExecutor executes WASM modules
type WASMExecutor struct {
	engine *wasmtime.Engine
	module *wasmtime.Module
	logger *zap.Logger
}

// NewWASMExecutor creates a new WASM executor
func NewWASMExecutor(wasmPath string, logger *zap.Logger) (*WASMExecutor, error) {
	engine := wasmtime.NewEngine()

	module, err := wasmtime.NewModuleFromFile(engine, wasmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load WASM module: %w", err)
	}

	logger.Info("WASM module loaded successfully", zap.String("path", wasmPath))

	return &WASMExecutor{
		engine: engine,
		module: module,
		logger: logger,
	}, nil
}

// Execute executes a function in the WASM module
func (e *WASMExecutor) Execute(ctx context.Context, input *TaskInput) (interface{}, error) {
	store := wasmtime.NewStore(e.engine)
	linker := wasmtime.NewLinker(e.engine)

	// Define WASI (if needed)
	if err := linker.DefineWasi(); err != nil {
		return nil, fmt.Errorf("failed to define WASI: %w", err)
	}

	// Instantiate the module
	instance, err := linker.Instantiate(store, e.module)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}

	// Get the exported function
	fn := instance.GetFunc(store, input.Function)
	if fn == nil {
		return nil, fmt.Errorf("function not found: %s", input.Function)
	}

	// Convert args to int32 (simplified for math operations)
	wasmArgs := make([]interface{}, len(input.Args))
	for i, arg := range input.Args {
		switch v := arg.(type) {
		case float64:
			wasmArgs[i] = int32(v)
		case int:
			wasmArgs[i] = int32(v)
		case int32:
			wasmArgs[i] = v
		default:
			return nil, fmt.Errorf("unsupported argument type: %T", arg)
		}
	}

	// Call the function
	result, err := fn.Call(store, wasmArgs...)
	if err != nil {
		return nil, fmt.Errorf("function call failed: %w", err)
	}

	e.logger.Debug("WASM function executed",
		zap.String("function", input.Function),
		zap.Any("result", result),
	)

	return result, nil
}

// Close closes the WASM executor
func (e *WASMExecutor) Close() error {
	// Wasmtime resources are automatically freed
	return nil
}
