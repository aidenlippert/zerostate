package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	ErrChainFailed     = errors.New("task chain execution failed")
	ErrStepFailed      = errors.New("chain step execution failed")
	ErrInvalidChain    = errors.New("invalid task chain configuration")
	ErrChainTimeout    = errors.New("chain execution timeout")
	ErrBranchCondition = errors.New("branch condition evaluation failed")
	ErrAgentNotFound   = errors.New("agent not found for chain step")
)

// ChainStatus represents the status of a task chain execution
type ChainStatus string

const (
	ChainStatusPending   ChainStatus = "pending"
	ChainStatusRunning   ChainStatus = "running"
	ChainStatusCompleted ChainStatus = "completed"
	ChainStatusFailed    ChainStatus = "failed"
	ChainStatusCanceled  ChainStatus = "canceled"
)

// StepStatus represents the status of a single chain step
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"
	StepStatusRunning   StepStatus = "running"
	StepStatusCompleted StepStatus = "completed"
	StepStatusFailed    StepStatus = "failed"
	StepStatusSkipped   StepStatus = "skipped"
)

// BranchCondition defines when to execute a conditional step
type BranchCondition string

const (
	BranchOnSuccess BranchCondition = "on_success" // Execute if previous step succeeded
	BranchOnFailure BranchCondition = "on_failure" // Execute if previous step failed
	BranchAlways    BranchCondition = "always"     // Execute regardless of previous step result
)

// TaskChainStep represents a single step in a task chain
type TaskChainStep struct {
	// Identity
	ID      string `json:"id"`
	Name    string `json:"name"`
	StepNum int    `json:"step_num"`

	// Agent Selection
	AgentID      string            `json:"agent_id,omitempty"`     // Specific agent ID (optional)
	Capabilities []string          `json:"capabilities,omitempty"` // Required capabilities (if no specific agent)
	Requirements map[string]string `json:"requirements,omitempty"` // Agent requirements

	// Task Configuration
	TaskType   string                 `json:"task_type"`
	Input      map[string]interface{} `json:"input"`
	Timeout    time.Duration          `json:"timeout"`
	MaxRetries int                    `json:"max_retries"`
	Budget     float64                `json:"budget"`

	// Input Transformation
	// Maps previous step output fields to this step's input fields
	// e.g., {"prev_output_field": "this_input_field"}
	InputMapping map[string]string `json:"input_mapping,omitempty"`

	// Conditional Execution
	Condition BranchCondition `json:"condition"` // When to execute this step

	// Execution State
	Status      StepStatus             `json:"status"`
	AssignedTo  string                 `json:"assigned_to,omitempty"` // Assigned agent DID
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ExecutionMS int64                  `json:"execution_ms,omitempty"` // Execution time in milliseconds
}

// TaskChain represents a sequence of tasks executed across multiple agents
type TaskChain struct {
	// Identity
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Chain Configuration
	Steps       []*TaskChainStep       `json:"steps"`
	Metadata    map[string]interface{} `json:"metadata"`
	TotalBudget float64                `json:"total_budget"` // Total budget for all steps

	// Execution State
	Status      ChainStatus `json:"status"`
	CurrentStep int         `json:"current_step"`
	TotalCost   float64     `json:"total_cost"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`

	// Retry Configuration
	MaxRetries int `json:"max_retries"` // Max retries for entire chain
	RetryCount int `json:"retry_count"` // Current retry count for chain
}

// ChainExecutor executes task chains across multiple agents
type ChainExecutor struct {
	mu            sync.RWMutex
	messageBus    *p2p.MessageBus
	agentSelector AgentSelector
	logger        *zap.Logger

	// Active chains
	activeChains map[string]*chainExecution

	// Metrics
	metricsChainTotal    prometheus.Counter
	metricsChainSuccess  prometheus.Counter
	metricsChainFailure  prometheus.Counter
	metricsChainDuration prometheus.Histogram
	metricsStepDuration  prometheus.Histogram
	metricsChainSteps    prometheus.Histogram
}

// chainExecution tracks the execution state of a chain
type chainExecution struct {
	chain       *TaskChain
	ctx         context.Context
	cancel      context.CancelFunc
	stepResults []map[string]interface{} // Results from each step
	mu          sync.RWMutex
}

// NewChainExecutor creates a new chain executor
func NewChainExecutor(
	messageBus *p2p.MessageBus,
	agentSelector AgentSelector,
	logger *zap.Logger,
) *ChainExecutor {
	return &ChainExecutor{
		messageBus:    messageBus,
		agentSelector: agentSelector,
		logger:        logger,
		activeChains:  make(map[string]*chainExecution),

		metricsChainTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_chain_executions_total",
			Help: "Total number of chain executions started",
		}),
		metricsChainSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_chain_executions_success",
			Help: "Total number of successful chain executions",
		}),
		metricsChainFailure: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_chain_executions_failure",
			Help: "Total number of failed chain executions",
		}),
		metricsChainDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_chain_execution_duration_seconds",
			Help:    "Duration of chain execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		}),
		metricsStepDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_chain_step_duration_seconds",
			Help:    "Duration of individual chain step execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 10), // 0.05s to ~50s
		}),
		metricsChainSteps: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_chain_steps_count",
			Help:    "Number of steps in executed chains",
			Buckets: prometheus.LinearBuckets(1, 1, 20), // 1 to 20 steps
		}),
	}
}

// ExecuteChain executes a task chain
func (ce *ChainExecutor) ExecuteChain(ctx context.Context, chain *TaskChain) error {
	ce.metricsChainTotal.Inc()
	ce.metricsChainSteps.Observe(float64(len(chain.Steps)))

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		ce.metricsChainDuration.Observe(duration.Seconds())
	}()

	// Validate chain
	if err := ce.validateChain(chain); err != nil {
		ce.logger.Error("invalid chain", zap.String("chain_id", chain.ID), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrInvalidChain, err)
	}

	// Create execution context
	execCtx, cancel := context.WithCancel(ctx)
	execution := &chainExecution{
		chain:       chain,
		ctx:         execCtx,
		cancel:      cancel,
		stepResults: make([]map[string]interface{}, len(chain.Steps)),
	}

	// Register active chain
	ce.mu.Lock()
	ce.activeChains[chain.ID] = execution
	ce.mu.Unlock()

	// Clean up on completion
	defer func() {
		ce.mu.Lock()
		delete(ce.activeChains, chain.ID)
		ce.mu.Unlock()
		cancel()
	}()

	// Update chain status
	chain.Status = ChainStatusRunning
	now := time.Now()
	chain.StartedAt = &now
	chain.UpdatedAt = now

	ce.logger.Info("starting chain execution",
		zap.String("chain_id", chain.ID),
		zap.String("chain_name", chain.Name),
		zap.Int("num_steps", len(chain.Steps)),
	)

	// Execute steps sequentially
	for i, step := range chain.Steps {
		chain.CurrentStep = i

		// Check if we should execute this step based on condition
		shouldExecute := ce.shouldExecuteStep(step, i, execution)
		if !shouldExecute {
			step.Status = StepStatusSkipped
			ce.logger.Info("skipping step",
				zap.String("chain_id", chain.ID),
				zap.String("step_id", step.ID),
				zap.String("condition", string(step.Condition)),
			)
			continue
		}

		// Execute step
		if err := ce.executeStep(execCtx, chain, step, i, execution); err != nil {
			ce.logger.Error("step execution failed",
				zap.String("chain_id", chain.ID),
				zap.String("step_id", step.ID),
				zap.Error(err),
			)

			// Mark chain as failed
			chain.Status = ChainStatusFailed
			chain.Error = fmt.Sprintf("step %d failed: %v", i, err)
			completedAt := time.Now()
			chain.CompletedAt = &completedAt
			chain.UpdatedAt = completedAt

			ce.metricsChainFailure.Inc()
			return fmt.Errorf("%w at step %d: %v", ErrStepFailed, i, err)
		}
	}

	// Chain completed successfully
	chain.Status = ChainStatusCompleted
	completedAt := time.Now()
	chain.CompletedAt = &completedAt
	chain.UpdatedAt = completedAt

	ce.metricsChainSuccess.Inc()
	ce.logger.Info("chain execution completed",
		zap.String("chain_id", chain.ID),
		zap.Duration("duration", time.Since(startTime)),
		zap.Float64("total_cost", chain.TotalCost),
	)

	return nil
}

// validateChain validates a task chain configuration
func (ce *ChainExecutor) validateChain(chain *TaskChain) error {
	if chain.ID == "" {
		return errors.New("chain ID is required")
	}

	if len(chain.Steps) == 0 {
		return errors.New("chain must have at least one step")
	}

	// Validate each step
	for i, step := range chain.Steps {
		if step.ID == "" {
			return fmt.Errorf("step %d: ID is required", i)
		}

		if step.TaskType == "" {
			return fmt.Errorf("step %d: task type is required", i)
		}

		if step.AgentID == "" && len(step.Capabilities) == 0 {
			return fmt.Errorf("step %d: must specify either agent_id or capabilities", i)
		}

		if step.Timeout == 0 {
			step.Timeout = 30 * time.Second // Default timeout
		}

		if step.Condition == "" {
			step.Condition = BranchAlways // Default condition
		}

		step.StepNum = i
		step.Status = StepStatusPending
	}

	return nil
}

// shouldExecuteStep determines if a step should be executed based on its condition
func (ce *ChainExecutor) shouldExecuteStep(step *TaskChainStep, stepNum int, execution *chainExecution) bool {
	// First step always executes (if condition is not on_failure)
	if stepNum == 0 {
		return step.Condition != BranchOnFailure
	}

	// Get previous step
	prevStep := execution.chain.Steps[stepNum-1]

	// Check condition
	switch step.Condition {
	case BranchAlways:
		return true
	case BranchOnSuccess:
		return prevStep.Status == StepStatusCompleted
	case BranchOnFailure:
		return prevStep.Status == StepStatusFailed
	default:
		ce.logger.Warn("unknown branch condition, defaulting to always",
			zap.String("condition", string(step.Condition)),
		)
		return true
	}
}

// executeStep executes a single step in the chain
func (ce *ChainExecutor) executeStep(
	ctx context.Context,
	chain *TaskChain,
	step *TaskChainStep,
	stepNum int,
	execution *chainExecution,
) error {
	stepStartTime := time.Now()
	defer func() {
		duration := time.Since(stepStartTime)
		ce.metricsStepDuration.Observe(duration.Seconds())
		step.ExecutionMS = duration.Milliseconds()
	}()

	step.Status = StepStatusRunning
	stepStart := time.Now()
	step.StartedAt = &stepStart

	ce.logger.Info("executing step",
		zap.String("chain_id", chain.ID),
		zap.String("step_id", step.ID),
		zap.String("step_name", step.Name),
		zap.Int("step_num", stepNum),
	)

	// Build step input from previous results and input mapping
	stepInput, err := ce.buildStepInput(step, stepNum, execution)
	if err != nil {
		step.Status = StepStatusFailed
		step.Error = err.Error()
		return fmt.Errorf("failed to build step input: %w", err)
	}

	// Select agent for this step
	var agentCard *identity.AgentCard
	if step.AgentID != "" {
		// Use specific agent
		agentCard = &identity.AgentCard{
			DID: step.AgentID,
		}
	} else {
		// Select agent based on capabilities
		task := &Task{
			Type:         step.TaskType,
			Capabilities: step.Capabilities,
			Input:        stepInput,
			Budget:       step.Budget,
		}
		agentCard, err = ce.agentSelector.SelectAgent(ctx, task)
		if err != nil {
			step.Status = StepStatusFailed
			step.Error = err.Error()
			return fmt.Errorf("%w: %v", ErrAgentNotFound, err)
		}
	}

	step.AssignedTo = agentCard.DID

	// Create task request
	taskReq := &p2p.TaskRequest{
		TaskID:       step.ID,
		AgentID:      agentCard.DID,
		Input:        mustMarshal(stepInput),
		Deadline:     time.Now().Add(step.Timeout),
		Budget:       step.Budget,
		Priority:     int(PriorityNormal),
		Requirements: step.Requirements,
	}

	// Send request to agent via message bus
	ce.logger.Debug("sending task request to agent",
		zap.String("step_id", step.ID),
		zap.String("agent_id", agentCard.DID),
	)

	resp, err := ce.messageBus.SendRequest(ctx, agentCard.DID, taskReq, step.Timeout)
	if err != nil {
		step.Status = StepStatusFailed
		step.Error = err.Error()
		return fmt.Errorf("failed to send task request: %w", err)
	}

	// Process response
	if resp.Status == "COMPLETED" {
		step.Status = StepStatusCompleted

		// Parse result
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			ce.logger.Warn("failed to parse step result",
				zap.String("step_id", step.ID),
				zap.Error(err),
			)
			result = map[string]interface{}{
				"raw_result": string(resp.Result),
			}
		}
		step.Result = result

		// Store result for next steps
		execution.mu.Lock()
		execution.stepResults[stepNum] = result
		execution.mu.Unlock()

		// Update chain cost
		chain.TotalCost += resp.Price

		stepCompleted := time.Now()
		step.CompletedAt = &stepCompleted

		ce.logger.Info("step completed successfully",
			zap.String("step_id", step.ID),
			zap.Duration("duration", time.Since(stepStartTime)),
			zap.Float64("cost", resp.Price),
		)

		return nil
	}

	// Step failed
	step.Status = StepStatusFailed
	step.Error = resp.Error
	stepCompleted := time.Now()
	step.CompletedAt = &stepCompleted

	return fmt.Errorf("step failed: %s", resp.Error)
}

// buildStepInput builds the input for a step by combining:
// 1. The step's configured input
// 2. Mapped outputs from previous steps
func (ce *ChainExecutor) buildStepInput(
	step *TaskChainStep,
	stepNum int,
	execution *chainExecution,
) (map[string]interface{}, error) {
	execution.mu.RLock()
	defer execution.mu.RUnlock()

	// Start with configured input
	input := make(map[string]interface{})
	for k, v := range step.Input {
		input[k] = v
	}

	// Apply input mappings from previous steps
	if step.InputMapping != nil && stepNum > 0 {
		// Get previous step result
		prevResult := execution.stepResults[stepNum-1]
		if prevResult == nil {
			return input, nil // No previous result to map
		}

		// Map fields
		for sourceField, targetField := range step.InputMapping {
			if value, ok := prevResult[sourceField]; ok {
				input[targetField] = value
			} else {
				ce.logger.Warn("input mapping source field not found",
					zap.String("step_id", step.ID),
					zap.String("source_field", sourceField),
				)
			}
		}
	}

	return input, nil
}

// CancelChain cancels a running chain
func (ce *ChainExecutor) CancelChain(chainID string) error {
	ce.mu.RLock()
	execution, ok := ce.activeChains[chainID]
	ce.mu.RUnlock()

	if !ok {
		return fmt.Errorf("chain %s not found or not running", chainID)
	}

	execution.cancel()
	execution.chain.Status = ChainStatusCanceled
	now := time.Now()
	execution.chain.CompletedAt = &now
	execution.chain.UpdatedAt = now

	ce.logger.Info("chain canceled", zap.String("chain_id", chainID))
	return nil
}

// GetChainStatus returns the current status of a chain
func (ce *ChainExecutor) GetChainStatus(chainID string) (*TaskChain, error) {
	ce.mu.RLock()
	execution, ok := ce.activeChains[chainID]
	ce.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("chain %s not found", chainID)
	}

	execution.mu.RLock()
	defer execution.mu.RUnlock()

	// Return a copy of the chain
	return execution.chain, nil
}

// NewTaskChain creates a new task chain
func NewTaskChain(userID, name string) *TaskChain {
	return &TaskChain{
		ID:          uuid.New().String(),
		UserID:      userID,
		Name:        name,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Steps:       make([]*TaskChainStep, 0),
		Metadata:    make(map[string]interface{}),
		Status:      ChainStatusPending,
		CurrentStep: 0,
		MaxRetries:  1,
		RetryCount:  0,
	}
}

// AddStep adds a step to the chain
func (tc *TaskChain) AddStep(step *TaskChainStep) {
	if step.ID == "" {
		step.ID = uuid.New().String()
	}
	step.StepNum = len(tc.Steps)
	step.Status = StepStatusPending
	tc.Steps = append(tc.Steps, step)
	tc.UpdatedAt = time.Now()
}

// mustMarshal marshals data to JSON, panicking on error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal: %v", err))
	}
	return data
}
