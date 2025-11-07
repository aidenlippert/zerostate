package orchestration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/search"
	"go.uber.org/zap"
)

var (
	ErrNoSuitableAgent = errors.New("no suitable agent found for task")
	ErrAgentUnavailable = errors.New("agent is unavailable")
	ErrExecutionTimeout = errors.New("task execution timeout")
	ErrOrchestratorStopped = errors.New("orchestrator has been stopped")
)

// AgentSelector selects the best agent for a given task
type AgentSelector interface {
	SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error)
}

// TaskExecutor executes tasks on agents
type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error)
}

// Orchestrator manages task routing and execution
type Orchestrator struct {
	// Core components
	queue    *TaskQueue
	selector AgentSelector
	executor TaskExecutor
	logger   *zap.Logger

	// Worker pool
	numWorkers int
	workers    []*worker
	workerWg   sync.WaitGroup

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	stopCh chan struct{}

	// Metrics
	metrics *OrchestratorMetrics
	mu      sync.RWMutex
}

// OrchestratorMetrics tracks orchestration statistics
type OrchestratorMetrics struct {
	TasksProcessed   int64
	TasksSucceeded   int64
	TasksFailed      int64
	TasksTimedOut    int64
	AvgExecutionTime time.Duration
	ActiveWorkers    int
}

// OrchestratorConfig configures the orchestrator
type OrchestratorConfig struct {
	NumWorkers       int           // Number of worker goroutines
	TaskTimeout      time.Duration // Default task timeout
	RetryAttempts    int           // Max retry attempts for failed tasks
	RetryBackoff     time.Duration // Initial retry backoff
	MaxRetryBackoff  time.Duration // Maximum retry backoff
	WorkerPollPeriod time.Duration // How often workers poll for tasks
}

// DefaultOrchestratorConfig returns default configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		NumWorkers:       5,
		TaskTimeout:      30 * time.Second,
		RetryAttempts:    3,
		RetryBackoff:     1 * time.Second,
		MaxRetryBackoff:  10 * time.Second,
		WorkerPollPeriod: 100 * time.Millisecond,
	}
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(
	ctx context.Context,
	queue *TaskQueue,
	selector AgentSelector,
	executor TaskExecutor,
	config *OrchestratorConfig,
	logger *zap.Logger,
) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	orchCtx, cancel := context.WithCancel(ctx)

	return &Orchestrator{
		queue:      queue,
		selector:   selector,
		executor:   executor,
		logger:     logger,
		numWorkers: config.NumWorkers,
		ctx:        orchCtx,
		cancel:     cancel,
		stopCh:     make(chan struct{}),
		metrics:    &OrchestratorMetrics{},
	}
}

// Start starts the orchestrator workers
func (o *Orchestrator) Start() error {
	o.logger.Info("starting orchestrator",
		zap.Int("num_workers", o.numWorkers),
	)

	// Start worker goroutines
	o.workers = make([]*worker, o.numWorkers)
	for i := 0; i < o.numWorkers; i++ {
		w := &worker{
			id:           i,
			orchestrator: o,
			logger:       o.logger.With(zap.Int("worker_id", i)),
		}
		o.workers[i] = w

		o.workerWg.Add(1)
		go w.run()
	}

	o.logger.Info("orchestrator started successfully",
		zap.Int("workers", o.numWorkers),
	)

	return nil
}

// Stop gracefully stops the orchestrator
func (o *Orchestrator) Stop() error {
	o.logger.Info("stopping orchestrator")

	// Signal workers to stop
	o.cancel()
	close(o.stopCh)

	// Wait for all workers to finish
	o.workerWg.Wait()

	o.logger.Info("orchestrator stopped")
	return nil
}

// GetMetrics returns current orchestrator metrics
func (o *Orchestrator) GetMetrics() *OrchestratorMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return &OrchestratorMetrics{
		TasksProcessed:   o.metrics.TasksProcessed,
		TasksSucceeded:   o.metrics.TasksSucceeded,
		TasksFailed:      o.metrics.TasksFailed,
		TasksTimedOut:    o.metrics.TasksTimedOut,
		AvgExecutionTime: o.metrics.AvgExecutionTime,
		ActiveWorkers:    o.metrics.ActiveWorkers,
	}
}

// updateMetrics updates orchestrator metrics
func (o *Orchestrator) updateMetrics(result *TaskResult, executionTime time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.metrics.TasksProcessed++

	switch result.Status {
	case TaskStatusCompleted:
		o.metrics.TasksSucceeded++
	case TaskStatusFailed:
		o.metrics.TasksFailed++
	}

	// Update average execution time
	if o.metrics.AvgExecutionTime == 0 {
		o.metrics.AvgExecutionTime = executionTime
	} else {
		// Simple moving average
		o.metrics.AvgExecutionTime = (o.metrics.AvgExecutionTime + executionTime) / 2
	}
}

// worker represents a task processing worker
type worker struct {
	id           int
	orchestrator *Orchestrator
	logger       *zap.Logger
}

// run is the main worker loop
func (w *worker) run() {
	defer w.orchestrator.workerWg.Done()

	w.logger.Info("worker started")

	for {
		select {
		case <-w.orchestrator.stopCh:
			w.logger.Info("worker stopping")
			return
		case <-w.orchestrator.ctx.Done():
			w.logger.Info("worker context canceled")
			return
		default:
			// Try to get a task
			task, err := w.orchestrator.queue.DequeueWait(w.orchestrator.ctx)
			if err != nil {
				if err == context.Canceled || err == ErrQueueClosed {
					w.logger.Info("worker stopping due to context/queue closed")
					return
				}
				w.logger.Error("failed to dequeue task", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if task == nil {
				// No task available, continue
				continue
			}

			// Process the task
			w.processTask(task)
		}
	}
}

// processTask processes a single task
func (w *worker) processTask(task *Task) {
	startTime := time.Now()

	w.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
	)

	// Update task status to assigned
	task.UpdateStatus(TaskStatusAssigned)
	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task status", zap.Error(err))
	}

	// Select agent for task
	agent, err := w.orchestrator.selector.SelectAgent(w.orchestrator.ctx, task)
	if err != nil {
		w.logger.Error("failed to select agent",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		w.handleTaskFailure(task, err)
		return
	}

	// Assign agent to task
	task.AssignedTo = agent.DID
	task.UpdateStatus(TaskStatusRunning)
	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task with agent assignment", zap.Error(err))
	}

	w.logger.Info("agent selected for task",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
	)

	// Execute task with timeout
	execCtx, cancel := context.WithTimeout(w.orchestrator.ctx, task.Timeout)
	defer cancel()

	result, err := w.orchestrator.executor.ExecuteTask(execCtx, task, agent)
	executionTime := time.Since(startTime)

	if err != nil {
		w.logger.Error("task execution failed",
			zap.String("task_id", task.ID),
			zap.String("agent_id", agent.DID),
			zap.Error(err),
			zap.Duration("execution_time", executionTime),
		)
		w.handleTaskFailure(task, err)
		return
	}

	// Update task with result
	task.Result = result.Result
	task.ActualCost = 0 // TODO: Calculate actual cost
	task.UpdateStatus(result.Status)

	if result.Status == TaskStatusFailed {
		task.Error = result.Error
	}

	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task with result", zap.Error(err))
	}

	// Update metrics
	w.orchestrator.updateMetrics(result, executionTime)

	w.logger.Info("task completed",
		zap.String("task_id", task.ID),
		zap.String("status", string(result.Status)),
		zap.Duration("execution_time", executionTime),
	)
}

// handleTaskFailure handles task execution failures
func (w *worker) handleTaskFailure(task *Task, err error) {
	task.Error = err.Error()

	// Check if task can be retried
	if task.CanRetry() {
		task.RetryCount++
		task.UpdateStatus(TaskStatusPending)

		// Re-enqueue with exponential backoff
		backoff := time.Duration(task.RetryCount) * time.Second
		time.Sleep(backoff)

		if err := w.orchestrator.queue.Enqueue(task); err != nil {
			w.logger.Error("failed to re-enqueue task for retry",
				zap.String("task_id", task.ID),
				zap.Error(err),
			)
			task.UpdateStatus(TaskStatusFailed)
		} else {
			w.logger.Info("task re-enqueued for retry",
				zap.String("task_id", task.ID),
				zap.Int("retry_count", task.RetryCount),
			)
			return
		}
	} else {
		task.UpdateStatus(TaskStatusFailed)
	}

	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update failed task", zap.Error(err))
	}
}

// HNSWAgentSelector uses HNSW semantic search to find best agent
type HNSWAgentSelector struct {
	hnsw   *search.HNSWIndex
	logger *zap.Logger
}

// NewHNSWAgentSelector creates a new HNSW-based agent selector
func NewHNSWAgentSelector(hnsw *search.HNSWIndex, logger *zap.Logger) *HNSWAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HNSWAgentSelector{
		hnsw:   hnsw,
		logger: logger,
	}
}

// SelectAgent selects the best agent for a task using semantic search
func (s *HNSWAgentSelector) SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	if s.hnsw == nil {
		return nil, fmt.Errorf("HNSW index not initialized")
	}

	// Generate embedding for task capabilities
	embeddingGen := search.NewEmbedding(128)
	taskVector := embeddingGen.EncodeCapabilities(task.Capabilities, nil)

	// Search for similar agents (k=5)
	results := s.hnsw.Search(taskVector, 5)

	if len(results) == 0 {
		s.logger.Warn("no agents found for task",
			zap.String("task_id", task.ID),
			zap.Strings("capabilities", task.Capabilities),
		)
		return nil, ErrNoSuitableAgent
	}

	// Get the best matching agent (highest similarity)
	bestResult := results[0]
	agent, ok := bestResult.Payload.(*identity.AgentCard)
	if !ok {
		return nil, fmt.Errorf("invalid agent card in HNSW result")
	}

	s.logger.Info("agent selected",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
		zap.Float64("similarity", bestResult.Distance),
	)

	return agent, nil
}

// MockTaskExecutor is a simple executor for testing
type MockTaskExecutor struct {
	logger *zap.Logger
}

// NewMockTaskExecutor creates a new mock executor
func NewMockTaskExecutor(logger *zap.Logger) *MockTaskExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MockTaskExecutor{
		logger: logger,
	}
}

// ExecuteTask executes a task (mock implementation)
func (e *MockTaskExecutor) ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error) {
	e.logger.Info("executing task (mock)",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
	)

	// Simulate execution time
	select {
	case <-time.After(100 * time.Millisecond):
		// Success
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Return mock result
	return &TaskResult{
		TaskID: task.ID,
		Status: TaskStatusCompleted,
		Result: map[string]interface{}{
			"message": "Task completed successfully (mock)",
			"input":   task.Input,
		},
		ExecutionMS: 100,
		AgentDID:    agent.DID,
		Timestamp:   time.Now(),
	}, nil
}
