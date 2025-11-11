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
	ErrDAGCycleDetected   = errors.New("cycle detected in DAG")
	ErrDAGInvalidNode     = errors.New("invalid DAG node")
	ErrDAGExecutionFailed = errors.New("DAG execution failed")
	ErrDAGTimeout         = errors.New("DAG execution timeout")
	ErrDAGNodeFailed      = errors.New("DAG node execution failed")
)

// DAGNodeStatus represents the status of a DAG node execution
type DAGNodeStatus string

const (
	DAGNodeStatusPending   DAGNodeStatus = "pending"
	DAGNodeStatusReady     DAGNodeStatus = "ready" // Dependencies satisfied, ready to execute
	DAGNodeStatusRunning   DAGNodeStatus = "running"
	DAGNodeStatusCompleted DAGNodeStatus = "completed"
	DAGNodeStatusFailed    DAGNodeStatus = "failed"
	DAGNodeStatusSkipped   DAGNodeStatus = "skipped"
)

// DAGNode represents a single node in the workflow DAG
type DAGNode struct {
	// Identity
	ID   string `json:"id"`
	Name string `json:"name"`

	// Agent Selection
	AgentID      string            `json:"agent_id,omitempty"`     // Specific agent ID (optional)
	Capabilities []string          `json:"capabilities,omitempty"` // Required capabilities
	Requirements map[string]string `json:"requirements,omitempty"` // Agent requirements

	// Task Configuration
	TaskType string                 `json:"task_type"`
	Input    map[string]interface{} `json:"input"`
	Timeout  time.Duration          `json:"timeout"`
	Budget   float64                `json:"budget"`

	// Input Mapping from Dependencies
	// Maps dependency node outputs to this node's inputs
	// Key: "dependency_node_id.output_field"
	// Value: "this_input_field"
	InputMapping map[string]string `json:"input_mapping,omitempty"`

	// Graph Structure
	Dependencies []string `json:"dependencies"` // Node IDs this node depends on

	// Execution State
	Status      DAGNodeStatus          `json:"status"`
	AssignedTo  string                 `json:"assigned_to,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	ExecutionMS int64                  `json:"execution_ms,omitempty"`
}

// DAGWorkflow represents a DAG-based multi-agent workflow
type DAGWorkflow struct {
	// Identity
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Graph Configuration
	Nodes       map[string]*DAGNode    `json:"nodes"` // Map of node ID to node
	Metadata    map[string]interface{} `json:"metadata"`
	TotalBudget float64                `json:"total_budget"`

	// Execution State
	Status      ChainStatus `json:"status"`
	TotalCost   float64     `json:"total_cost"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`

	// Execution Configuration
	MaxParallelism int           `json:"max_parallelism"` // Max concurrent nodes (0 = unlimited)
	Timeout        time.Duration `json:"timeout"`         // Total workflow timeout
}

// DAGExecutor executes DAG-based workflows with parallel execution
type DAGExecutor struct {
	mu            sync.RWMutex
	messageBus    *p2p.MessageBus
	agentSelector AgentSelector
	logger        *zap.Logger

	// Active workflows
	activeWorkflows map[string]*dagExecution

	// Metrics
	metricsDAGTotal       prometheus.Counter
	metricsDAGSuccess     prometheus.Counter
	metricsDAGFailure     prometheus.Counter
	metricsDAGDuration    prometheus.Histogram
	metricsDAGNodes       prometheus.Histogram
	metricsDAGParallelism prometheus.Histogram
	metricsNodeDuration   prometheus.Histogram
}

// dagExecution tracks the execution state of a DAG workflow
type dagExecution struct {
	workflow        *DAGWorkflow
	ctx             context.Context
	cancel          context.CancelFunc
	nodeResults     map[string]map[string]interface{} // Node ID -> results
	mu              sync.RWMutex
	semaphore       chan struct{} // Limits parallelism
	wg              sync.WaitGroup
	executionErrors []error
}

// NewDAGExecutor creates a new DAG executor
func NewDAGExecutor(
	messageBus *p2p.MessageBus,
	agentSelector AgentSelector,
	logger *zap.Logger,
) *DAGExecutor {
	return &DAGExecutor{
		messageBus:      messageBus,
		agentSelector:   agentSelector,
		logger:          logger,
		activeWorkflows: make(map[string]*dagExecution),

		metricsDAGTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_dag_executions_total",
			Help: "Total number of DAG workflow executions started",
		}),
		metricsDAGSuccess: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_dag_executions_success",
			Help: "Total number of successful DAG workflow executions",
		}),
		metricsDAGFailure: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_dag_executions_failure",
			Help: "Total number of failed DAG workflow executions",
		}),
		metricsDAGDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_dag_execution_duration_seconds",
			Help:    "Duration of DAG workflow execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}),
		metricsDAGNodes: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_dag_nodes_count",
			Help:    "Number of nodes in executed DAG workflows",
			Buckets: prometheus.LinearBuckets(1, 1, 50),
		}),
		metricsDAGParallelism: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_dag_parallelism",
			Help:    "Maximum parallelism achieved during DAG execution",
			Buckets: prometheus.LinearBuckets(1, 1, 20),
		}),
		metricsNodeDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_dag_node_duration_seconds",
			Help:    "Duration of individual DAG node execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.05, 2, 10),
		}),
	}
}

// ExecuteDAG executes a DAG workflow with parallel execution
func (de *DAGExecutor) ExecuteDAG(ctx context.Context, workflow *DAGWorkflow) error {
	de.metricsDAGTotal.Inc()
	de.metricsDAGNodes.Observe(float64(len(workflow.Nodes)))

	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		de.metricsDAGDuration.Observe(duration.Seconds())
	}()

	// Validate DAG
	if err := de.validateDAG(workflow); err != nil {
		de.logger.Error("invalid DAG", zap.String("workflow_id", workflow.ID), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrDAGInvalidNode, err)
	}

	// Create execution context with timeout
	execCtx := ctx
	if workflow.Timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, workflow.Timeout)
		defer cancel()
	} else {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithCancel(ctx)
		defer cancel()
	}

	// Create semaphore for parallelism control
	var semaphore chan struct{}
	if workflow.MaxParallelism > 0 {
		semaphore = make(chan struct{}, workflow.MaxParallelism)
	}

	execution := &dagExecution{
		workflow:        workflow,
		ctx:             execCtx,
		nodeResults:     make(map[string]map[string]interface{}),
		semaphore:       semaphore,
		executionErrors: make([]error, 0),
	}

	// Register active workflow
	de.mu.Lock()
	de.activeWorkflows[workflow.ID] = execution
	de.mu.Unlock()

	// Clean up on completion
	defer func() {
		de.mu.Lock()
		delete(de.activeWorkflows, workflow.ID)
		de.mu.Unlock()
	}()

	// Update workflow status
	workflow.Status = ChainStatusRunning
	now := time.Now()
	workflow.StartedAt = &now
	workflow.UpdatedAt = now

	de.logger.Info("starting DAG execution",
		zap.String("workflow_id", workflow.ID),
		zap.String("workflow_name", workflow.Name),
		zap.Int("num_nodes", len(workflow.Nodes)),
		zap.Int("max_parallelism", workflow.MaxParallelism),
	)

	// Execute DAG using topological sort and parallel execution
	if err := de.executeDAGNodes(execution); err != nil {
		workflow.Status = ChainStatusFailed
		workflow.Error = err.Error()
		completedAt := time.Now()
		workflow.CompletedAt = &completedAt
		workflow.UpdatedAt = completedAt

		de.metricsDAGFailure.Inc()
		de.logger.Error("DAG execution failed",
			zap.String("workflow_id", workflow.ID),
			zap.Error(err),
		)
		return err
	}

	// Workflow completed successfully
	workflow.Status = ChainStatusCompleted
	completedAt := time.Now()
	workflow.CompletedAt = &completedAt
	workflow.UpdatedAt = completedAt

	de.metricsDAGSuccess.Inc()
	de.logger.Info("DAG execution completed",
		zap.String("workflow_id", workflow.ID),
		zap.Duration("duration", time.Since(startTime)),
		zap.Float64("total_cost", workflow.TotalCost),
	)

	return nil
}

// validateDAG validates the DAG workflow configuration
func (de *DAGExecutor) validateDAG(workflow *DAGWorkflow) error {
	if workflow.ID == "" {
		return errors.New("workflow ID is required")
	}

	if len(workflow.Nodes) == 0 {
		return errors.New("workflow must have at least one node")
	}

	// Validate nodes and check for cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeID, node := range workflow.Nodes {
		if node.ID == "" {
			node.ID = nodeID
		}

		if node.TaskType == "" {
			return fmt.Errorf("node %s: task type is required", nodeID)
		}

		if node.AgentID == "" && len(node.Capabilities) == 0 {
			return fmt.Errorf("node %s: must specify either agent_id or capabilities", nodeID)
		}

		if node.Timeout == 0 {
			node.Timeout = 30 * time.Second // Default timeout
		}

		node.Status = DAGNodeStatusPending

		// Check dependencies exist
		for _, depID := range node.Dependencies {
			if _, exists := workflow.Nodes[depID]; !exists {
				return fmt.Errorf("node %s: dependency %s does not exist", nodeID, depID)
			}
		}
	}

	// Check for cycles using DFS
	for nodeID := range workflow.Nodes {
		if !visited[nodeID] {
			if de.detectCycle(nodeID, workflow.Nodes, visited, recStack) {
				return fmt.Errorf("%w: cycle detected starting at node %s", ErrDAGCycleDetected, nodeID)
			}
		}
	}

	return nil
}

// detectCycle performs DFS to detect cycles in the DAG
func (de *DAGExecutor) detectCycle(
	nodeID string,
	nodes map[string]*DAGNode,
	visited map[string]bool,
	recStack map[string]bool,
) bool {
	visited[nodeID] = true
	recStack[nodeID] = true

	node := nodes[nodeID]
	for _, depID := range node.Dependencies {
		if !visited[depID] {
			if de.detectCycle(depID, nodes, visited, recStack) {
				return true
			}
		} else if recStack[depID] {
			return true // Cycle detected
		}
	}

	recStack[nodeID] = false
	return false
}

// executeDAGNodes executes DAG nodes with parallel execution and dependency management
func (de *DAGExecutor) executeDAGNodes(execution *dagExecution) error {
	workflow := execution.workflow

	// Build dependency graph
	dependents := make(map[string][]string) // Maps node ID to nodes that depend on it
	inDegree := make(map[string]int)        // Remaining dependencies for each node

	for nodeID, node := range workflow.Nodes {
		inDegree[nodeID] = len(node.Dependencies)
		for _, depID := range node.Dependencies {
			dependents[depID] = append(dependents[depID], nodeID)
		}
	}

	// Find nodes with no dependencies (ready to execute)
	readyQueue := make(chan string, len(workflow.Nodes))
	for nodeID, degree := range inDegree {
		if degree == 0 {
			readyQueue <- nodeID
			workflow.Nodes[nodeID].Status = DAGNodeStatusReady
		}
	}

	// Track completion
	completed := make(map[string]bool)
	failed := make(map[string]bool)
	var completedMu sync.Mutex

	// Execute nodes as they become ready
	for len(completed)+len(failed) < len(workflow.Nodes) {
		select {
		case <-execution.ctx.Done():
			return fmt.Errorf("%w: %v", ErrDAGTimeout, execution.ctx.Err())

		case nodeID := <-readyQueue:
			// Acquire semaphore if parallelism is limited
			if execution.semaphore != nil {
				execution.semaphore <- struct{}{}
			}

			execution.wg.Add(1)
			go func(nid string) {
				defer execution.wg.Done()
				defer func() {
					if execution.semaphore != nil {
						<-execution.semaphore
					}
				}()

				node := workflow.Nodes[nid]

				// Execute node
				err := de.executeNode(execution, node)

				completedMu.Lock()
				defer completedMu.Unlock()

				if err != nil {
					failed[nid] = true
					de.logger.Error("DAG node failed",
						zap.String("workflow_id", workflow.ID),
						zap.String("node_id", nid),
						zap.Error(err),
					)
					// Don't queue dependents
					return
				}

				completed[nid] = true

				// Queue dependent nodes that are now ready
				for _, depNodeID := range dependents[nid] {
					inDegree[depNodeID]--
					if inDegree[depNodeID] == 0 {
						// All dependencies satisfied
						workflow.Nodes[depNodeID].Status = DAGNodeStatusReady
						readyQueue <- depNodeID
					}
				}
			}(nodeID)

		default:
			// No nodes ready, wait for ongoing executions
			if len(completed)+len(failed) < len(workflow.Nodes) {
				time.Sleep(10 * time.Millisecond)
			}
		}
	}

	// Wait for all nodes to complete
	execution.wg.Wait()

	// Check if any nodes failed
	if len(failed) > 0 {
		return fmt.Errorf("%w: %d nodes failed", ErrDAGNodeFailed, len(failed))
	}

	return nil
}

// executeNode executes a single DAG node
func (de *DAGExecutor) executeNode(execution *dagExecution, node *DAGNode) error {
	nodeStartTime := time.Now()
	defer func() {
		duration := time.Since(nodeStartTime)
		de.metricsNodeDuration.Observe(duration.Seconds())
		node.ExecutionMS = duration.Milliseconds()
	}()

	node.Status = DAGNodeStatusRunning
	nodeStart := time.Now()
	node.StartedAt = &nodeStart

	de.logger.Info("executing DAG node",
		zap.String("workflow_id", execution.workflow.ID),
		zap.String("node_id", node.ID),
		zap.String("node_name", node.Name),
	)

	// Build node input from dependencies and input mapping
	nodeInput, err := de.buildNodeInput(node, execution)
	if err != nil {
		node.Status = DAGNodeStatusFailed
		node.Error = err.Error()
		return fmt.Errorf("failed to build node input: %w", err)
	}

	// Select agent for this node
	var agentCard *identity.AgentCard
	if node.AgentID != "" {
		agentCard = &identity.AgentCard{
			DID: node.AgentID,
		}
	} else {
		task := &Task{
			Type:         node.TaskType,
			Capabilities: node.Capabilities,
			Input:        nodeInput,
			Budget:       node.Budget,
		}
		agentCard, err = de.agentSelector.SelectAgent(execution.ctx, task)
		if err != nil {
			node.Status = DAGNodeStatusFailed
			node.Error = err.Error()
			return fmt.Errorf("%w: %v", ErrAgentNotFound, err)
		}
	}

	node.AssignedTo = agentCard.DID

	// Create task request
	taskReq := &p2p.TaskRequest{
		TaskID:       node.ID,
		AgentID:      agentCard.DID,
		Input:        mustMarshal(nodeInput),
		Deadline:     time.Now().Add(node.Timeout),
		Budget:       node.Budget,
		Priority:     int(PriorityNormal),
		Requirements: node.Requirements,
	}

	// Send request to agent
	resp, err := de.messageBus.SendRequest(execution.ctx, agentCard.DID, taskReq, node.Timeout)
	if err != nil {
		node.Status = DAGNodeStatusFailed
		node.Error = err.Error()
		return fmt.Errorf("failed to send task request: %w", err)
	}

	// Process response
	if resp.Status == "COMPLETED" {
		node.Status = DAGNodeStatusCompleted

		// Parse result
		var result map[string]interface{}
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			de.logger.Warn("failed to parse node result",
				zap.String("node_id", node.ID),
				zap.Error(err),
			)
			result = map[string]interface{}{
				"raw_result": string(resp.Result),
			}
		}
		node.Result = result

		// Store result for dependent nodes
		execution.mu.Lock()
		execution.nodeResults[node.ID] = result
		execution.workflow.TotalCost += resp.Price
		execution.mu.Unlock()

		nodeCompleted := time.Now()
		node.CompletedAt = &nodeCompleted

		de.logger.Info("DAG node completed",
			zap.String("node_id", node.ID),
			zap.Duration("duration", time.Since(nodeStartTime)),
			zap.Float64("cost", resp.Price),
		)

		return nil
	}

	// Node failed
	node.Status = DAGNodeStatusFailed
	node.Error = resp.Error
	nodeCompleted := time.Now()
	node.CompletedAt = &nodeCompleted

	return fmt.Errorf("node failed: %s", resp.Error)
}

// buildNodeInput builds the input for a node by combining:
// 1. The node's configured input
// 2. Mapped outputs from dependency nodes
func (de *DAGExecutor) buildNodeInput(
	node *DAGNode,
	execution *dagExecution,
) (map[string]interface{}, error) {
	execution.mu.RLock()
	defer execution.mu.RUnlock()

	// Start with configured input
	input := make(map[string]interface{})
	for k, v := range node.Input {
		input[k] = v
	}

	// Apply input mappings from dependencies
	if node.InputMapping != nil {
		for sourceKey, targetField := range node.InputMapping {
			// Parse source key: "dependency_node_id.output_field"
			// For simplicity, we'll support direct dependency references
			// Format: "node_id.field_name" or just "node_id" for entire result

			// Find the dependency node and get its result
			for _, depID := range node.Dependencies {
				depResult, ok := execution.nodeResults[depID]
				if !ok {
					continue // Dependency not yet completed
				}

				// Check if source key matches this dependency
				if sourceKey == depID {
					// Use entire result
					input[targetField] = depResult
				} else if len(sourceKey) > len(depID) && sourceKey[:len(depID)] == depID {
					// Extract specific field: "depID.fieldName"
					fieldName := sourceKey[len(depID)+1:]
					if value, ok := depResult[fieldName]; ok {
						input[targetField] = value
					}
				}
			}
		}
	}

	return input, nil
}

// CancelDAG cancels a running DAG workflow
func (de *DAGExecutor) CancelDAG(workflowID string) error {
	de.mu.RLock()
	execution, ok := de.activeWorkflows[workflowID]
	de.mu.RUnlock()

	if !ok {
		return fmt.Errorf("workflow %s not found or not running", workflowID)
	}

	execution.cancel()
	execution.workflow.Status = ChainStatusCanceled
	now := time.Now()
	execution.workflow.CompletedAt = &now
	execution.workflow.UpdatedAt = now

	de.logger.Info("DAG workflow canceled", zap.String("workflow_id", workflowID))
	return nil
}

// GetDAGStatus returns the current status of a DAG workflow
func (de *DAGExecutor) GetDAGStatus(workflowID string) (*DAGWorkflow, error) {
	de.mu.RLock()
	execution, ok := de.activeWorkflows[workflowID]
	de.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	execution.mu.RLock()
	defer execution.mu.RUnlock()

	return execution.workflow, nil
}

// NewDAGWorkflow creates a new DAG workflow
func NewDAGWorkflow(userID, name string) *DAGWorkflow {
	return &DAGWorkflow{
		ID:             uuid.New().String(),
		UserID:         userID,
		Name:           name,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Nodes:          make(map[string]*DAGNode),
		Metadata:       make(map[string]interface{}),
		Status:         ChainStatusPending,
		MaxParallelism: 0, // Unlimited by default
		Timeout:        0, // No timeout by default
	}
}

// AddNode adds a node to the DAG workflow
func (dw *DAGWorkflow) AddNode(node *DAGNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}

	if _, exists := dw.Nodes[node.ID]; exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}

	node.Status = DAGNodeStatusPending
	dw.Nodes[node.ID] = node
	dw.UpdatedAt = time.Now()

	return nil
}
