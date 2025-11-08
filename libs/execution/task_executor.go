package execution

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// TaskExecutor manages the execution of tasks from the queue
type TaskExecutor struct {
	logger       *zap.Logger
	wasmRunner   *WASMRunner
	taskQueue    TaskQueue
	binaryStore  BinaryStore
	resultStore  ResultStore
	wsHub        WebSocketHub
	maxRetries   int
	retryDelay   time.Duration
}

// TaskQueue interface for Redis task queue operations
type TaskQueue interface {
	Dequeue(ctx context.Context) (*Task, error)
	UpdateStatus(ctx context.Context, taskID string, status string) error
}

// BinaryStore interface for S3 binary storage operations
type BinaryStore interface {
	GetBinary(ctx context.Context, agentID string) ([]byte, error)
}

// ResultStore interface for storing task results
type ResultStore interface {
	StoreResult(ctx context.Context, result *TaskResult) error
	GetResult(ctx context.Context, taskID string) (*TaskResult, error)
}

// WebSocketHub interface for real-time updates
type WebSocketHub interface {
	BroadcastTaskUpdate(taskID, status, message string) error
}

// Task represents a queued task
type Task struct {
	ID          string
	UserID      string
	AgentID     string
	Query       string
	Input       []byte
	Status      string
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID     string
	AgentID    string
	ExitCode   int
	Stdout     []byte
	Stderr     []byte
	Duration   time.Duration
	DurationMs int64  // Duration in milliseconds for database storage
	Error      string
	CreatedAt  time.Time
}

// NewTaskExecutor creates a new task executor
func NewTaskExecutor(
	logger *zap.Logger,
	wasmRunner *WASMRunner,
	taskQueue TaskQueue,
	binaryStore BinaryStore,
	resultStore ResultStore,
	wsHub WebSocketHub,
) *TaskExecutor {
	return &TaskExecutor{
		logger:      logger,
		wasmRunner:  wasmRunner,
		taskQueue:   taskQueue,
		binaryStore: binaryStore,
		resultStore: resultStore,
		wsHub:       wsHub,
		maxRetries:  3,
		retryDelay:  time.Second * 2,
	}
}

// Start begins processing tasks from the queue
func (e *TaskExecutor) Start(ctx context.Context) error {
	e.logger.Info("starting task executor")

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("task executor shutting down")
			return ctx.Err()
		default:
			if err := e.processNextTask(ctx); err != nil {
				e.logger.Error("failed to process task", zap.Error(err))
				time.Sleep(time.Second) // Backoff on error
			}
		}
	}
}

// processNextTask dequeues and executes a single task
func (e *TaskExecutor) processNextTask(ctx context.Context) error {
	// Dequeue task
	task, err := e.taskQueue.Dequeue(ctx)
	if err != nil {
		return fmt.Errorf("dequeue failed: %w", err)
	}
	if task == nil {
		// No tasks available, wait a bit
		time.Sleep(time.Second)
		return nil
	}

	e.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.String("agent_id", task.AgentID),
		zap.String("query", task.Query),
	)

	// Execute with retries
	var result *WASMResult
	var executeErr error

	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			e.logger.Warn("retrying task execution",
				zap.String("task_id", task.ID),
				zap.Int("attempt", attempt),
			)
			time.Sleep(e.retryDelay * time.Duration(attempt)) // Exponential backoff
		}

		result, executeErr = e.executeTask(ctx, task)
		if executeErr == nil {
			break
		}

		e.logger.Error("task execution failed",
			zap.String("task_id", task.ID),
			zap.Int("attempt", attempt),
			zap.Error(executeErr),
		)
	}

	// Store result
	taskResult := &TaskResult{
		TaskID:    task.ID,
		AgentID:   task.AgentID,
		CreatedAt: time.Now(),
	}

	if result != nil {
		taskResult.ExitCode = result.ExitCode
		taskResult.Stdout = result.Stdout
		taskResult.Stderr = result.Stderr
		taskResult.Duration = result.Duration
		if result.Error != nil {
			taskResult.Error = result.Error.Error()
		}
	} else if executeErr != nil {
		taskResult.Error = executeErr.Error()
		taskResult.ExitCode = -1
	}

	// Store result in database
	if err := e.resultStore.StoreResult(ctx, taskResult); err != nil {
		e.logger.Error("failed to store result",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
	}

	// Update task status
	status := "completed"
	if executeErr != nil || (result != nil && result.ExitCode != 0) {
		status = "failed"
	}

	if err := e.taskQueue.UpdateStatus(ctx, task.ID, status); err != nil {
		e.logger.Error("failed to update task status",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
	}

	// Send WebSocket update
	message := fmt.Sprintf("Task %s %s", task.Query, status)
	if err := e.wsHub.BroadcastTaskUpdate(task.ID, status, message); err != nil {
		e.logger.Error("failed to broadcast task update",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
	}

	e.logger.Info("task processing complete",
		zap.String("task_id", task.ID),
		zap.String("status", status),
		zap.Duration("duration", taskResult.Duration),
	)

	return nil
}

// executeTask executes a single task
func (e *TaskExecutor) executeTask(ctx context.Context, task *Task) (*WASMResult, error) {
	// Update status to running
	now := time.Now()
	task.StartedAt = &now
	task.Status = "running"

	if err := e.taskQueue.UpdateStatus(ctx, task.ID, "running"); err != nil {
		return nil, fmt.Errorf("failed to update status to running: %w", err)
	}

	// Broadcast running status
	e.wsHub.BroadcastTaskUpdate(task.ID, "running", fmt.Sprintf("Task %s started", task.Query))

	// Load WASM binary from S3
	e.logger.Info("loading WASM binary",
		zap.String("task_id", task.ID),
		zap.String("agent_id", task.AgentID),
	)

	wasmBinary, err := e.binaryStore.GetBinary(ctx, task.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load WASM binary: %w", err)
	}

	// Execute WASM
	e.logger.Info("executing WASM",
		zap.String("task_id", task.ID),
		zap.Int("binary_size", len(wasmBinary)),
		zap.Int("input_size", len(task.Input)),
	)

	result, err := e.wasmRunner.Execute(ctx, wasmBinary, task.Input)
	if err != nil {
		return result, fmt.Errorf("WASM execution failed: %w", err)
	}

	e.logger.Info("WASM execution succeeded",
		zap.String("task_id", task.ID),
		zap.Int("exit_code", result.ExitCode),
		zap.Int("stdout_size", len(result.Stdout)),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}
