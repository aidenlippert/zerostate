package integration

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockAgentSelector implements AgentSelector for testing
type mockAgentSelector struct {
	agents map[string]*identity.AgentCard
}

func newMockAgentSelector() *mockAgentSelector {
	return &mockAgentSelector{
		agents: make(map[string]*identity.AgentCard),
	}
}

func (m *mockAgentSelector) RegisterAgent(did string, capabilities []string) {
	m.agents[did] = &identity.AgentCard{
		DID:          did,
		Capabilities: capabilities,
	}
}

func (m *mockAgentSelector) SelectAgent(ctx context.Context, task *orchestration.Task) (*identity.AgentCard, error) {
	// Simple selection: return first agent with matching capabilities
	for _, agent := range m.agents {
		hasAllCapabilities := true
		for _, reqCap := range task.Capabilities {
			found := false
			for _, agentCap := range agent.Capabilities {
				if agentCap == reqCap {
					found = true
					break
				}
			}
			if !found {
				hasAllCapabilities = false
				break
			}
		}
		if hasAllCapabilities {
			return agent, nil
		}
	}
	return nil, orchestration.ErrNoSuitableAgent
}

// mockMessageBus implements simplified MessageBus for testing
type mockMessageBus struct {
	agents map[string]*mockAgent
}

type mockAgent struct {
	did     string
	handler func(req *p2p.TaskRequest) (*p2p.TaskResponse, error)
}

func newMockMessageBus() *mockMessageBus {
	return &mockMessageBus{
		agents: make(map[string]*mockAgent),
	}
}

func (m *mockMessageBus) RegisterAgent(did string, handler func(*p2p.TaskRequest) (*p2p.TaskResponse, error)) {
	m.agents[did] = &mockAgent{
		did:     did,
		handler: handler,
	}
}

func (m *mockMessageBus) SendRequest(ctx context.Context, agentDID string, req *p2p.TaskRequest, timeout time.Duration) (*p2p.TaskResponse, error) {
	agent, exists := m.agents[agentDID]
	if !exists {
		return nil, orchestration.ErrAgentNotFound
	}

	// Simulate network delay
	time.Sleep(10 * time.Millisecond)

	return agent.handler(req)
}

func (m *mockMessageBus) Broadcast(ctx context.Context, payload json.RawMessage, payloadType string) error {
	return nil
}

// TestTaskChainSequentialExecution tests basic sequential task chaining
func TestTaskChainSequentialExecution(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	// Setup mock components
	messageBus := newMockMessageBus()
	agentSelector := newMockAgentSelector()

	// Register mock agents
	agentSelector.RegisterAgent("agent_upload", []string{"file-upload"})
	agentSelector.RegisterAgent("agent_process", []string{"data-processing"})
	agentSelector.RegisterAgent("agent_store", []string{"data-storage"})

	// Register agent handlers
	messageBus.RegisterAgent("agent_upload", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		result := map[string]interface{}{
			"file_id":  "file_123",
			"file_url": "https://storage.example.com/file_123",
			"size":     1024,
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Price:    1.0,
			Duration: 100,
		}, nil
	})

	messageBus.RegisterAgent("agent_process", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		var input map[string]interface{}
		json.Unmarshal(req.Input, &input)

		result := map[string]interface{}{
			"processed_file_id": input["file_id"],
			"rows_processed":    1000,
			"status":            "success",
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Price:    2.0,
			Duration: 200,
		}, nil
	})

	messageBus.RegisterAgent("agent_store", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		var input map[string]interface{}
		json.Unmarshal(req.Input, &input)

		result := map[string]interface{}{
			"storage_id": "store_" + input["processed_file_id"].(string),
			"stored_at":  time.Now().Unix(),
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Price:    0.5,
			Duration: 50,
		}, nil
	})

	// Create chain executor
	executor := orchestration.NewChainExecutor(messageBus, agentSelector, logger)

	// Create task chain
	chain := orchestration.NewTaskChain("user_123", "data-pipeline")

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "upload",
		Capabilities: []string{"file-upload"},
		TaskType:     "upload-file",
		Input: map[string]interface{}{
			"source": "local",
		},
		Timeout: 5 * time.Second,
		Budget:  1.5,
	})

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "process",
		Capabilities: []string{"data-processing"},
		TaskType:     "process-data",
		InputMapping: map[string]string{
			"file_id":  "file_id",
			"file_url": "file_url",
		},
		Timeout: 5 * time.Second,
		Budget:  2.5,
	})

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "store",
		Capabilities: []string{"data-storage"},
		TaskType:     "store-result",
		InputMapping: map[string]string{
			"processed_file_id": "file_id",
		},
		Timeout: 5 * time.Second,
		Budget:  1.0,
	})

	// Execute chain
	err := executor.ExecuteChain(ctx, chain)

	// Assertions
	require.NoError(t, err)
	assert.Equal(t, orchestration.ChainStatusCompleted, chain.Status)
	assert.Equal(t, 3, len(chain.Steps))
	assert.Equal(t, 3.5, chain.TotalCost) // 1.0 + 2.0 + 0.5

	// Verify each step
	for _, step := range chain.Steps {
		assert.Equal(t, orchestration.StepStatusCompleted, step.Status)
		assert.NotEmpty(t, step.AssignedTo)
		assert.NotNil(t, step.Result)
	}

	// Verify final result
	finalResult := chain.Steps[2].Result
	assert.NotNil(t, finalResult["storage_id"])
}

// TestTaskChainConditionalBranching tests conditional step execution
func TestTaskChainConditionalBranching(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	messageBus := newMockMessageBus()
	agentSelector := newMockAgentSelector()

	agentSelector.RegisterAgent("agent_process", []string{"processing"})
	agentSelector.RegisterAgent("agent_success", []string{"success-handler"})
	agentSelector.RegisterAgent("agent_failure", []string{"failure-handler"})

	// Process agent that fails
	messageBus.RegisterAgent("agent_process", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "FAILED",
			Error:    "Processing error",
			Duration: 100,
		}, nil
	})

	messageBus.RegisterAgent("agent_success", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(map[string]interface{}{"status": "success"}),
			Duration: 50,
		}, nil
	})

	messageBus.RegisterAgent("agent_failure", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(map[string]interface{}{"status": "handled_failure"}),
			Duration: 50,
		}, nil
	})

	executor := orchestration.NewChainExecutor(messageBus, agentSelector, logger)
	chain := orchestration.NewTaskChain("user_123", "conditional-chain")

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "process",
		Capabilities: []string{"processing"},
		TaskType:     "process-data",
		Condition:    orchestration.BranchAlways,
		Timeout:      5 * time.Second,
	})

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "on-success",
		Capabilities: []string{"success-handler"},
		TaskType:     "handle-success",
		Condition:    orchestration.BranchOnSuccess,
		Timeout:      5 * time.Second,
	})

	chain.AddStep(&orchestration.TaskChainStep{
		Name:         "on-failure",
		Capabilities: []string{"failure-handler"},
		TaskType:     "handle-failure",
		Condition:    orchestration.BranchOnFailure,
		Timeout:      5 * time.Second,
	})

	err := executor.ExecuteChain(ctx, chain)

	// Chain should fail because first step failed
	require.Error(t, err)
	assert.Equal(t, orchestration.StepStatusFailed, chain.Steps[0].Status)
	// Success handler should be skipped
	assert.Equal(t, orchestration.StepStatusSkipped, chain.Steps[1].Status)
	// We won't reach failure handler in current implementation
}

// TestDAGParallelExecution tests parallel execution in DAG workflows
func TestDAGParallelExecution(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	messageBus := newMockMessageBus()
	agentSelector := newMockAgentSelector()

	// Register agents
	agentSelector.RegisterAgent("agent_fetch", []string{"data-fetch"})
	agentSelector.RegisterAgent("agent_process_1", []string{"data-processing"})
	agentSelector.RegisterAgent("agent_process_2", []string{"data-processing"})
	agentSelector.RegisterAgent("agent_combine", []string{"data-aggregation"})

	executionOrder := make([]string, 0)
	var orderMu sync.Mutex

	messageBus.RegisterAgent("agent_fetch", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		orderMu.Lock()
		executionOrder = append(executionOrder, "fetch")
		orderMu.Unlock()

		result := map[string]interface{}{
			"data": []int{1, 2, 3, 4, 5},
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Duration: 100,
		}, nil
	})

	messageBus.RegisterAgent("agent_process_1", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		orderMu.Lock()
		executionOrder = append(executionOrder, "process_1")
		orderMu.Unlock()

		time.Sleep(50 * time.Millisecond) // Simulate work

		result := map[string]interface{}{
			"sum": 15, // 1+2+3+4+5
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Duration: 50,
		}, nil
	})

	messageBus.RegisterAgent("agent_process_2", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		orderMu.Lock()
		executionOrder = append(executionOrder, "process_2")
		orderMu.Unlock()

		time.Sleep(50 * time.Millisecond) // Simulate work

		result := map[string]interface{}{
			"count": 5,
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Duration: 50,
		}, nil
	})

	messageBus.RegisterAgent("agent_combine", func(req *p2p.TaskRequest) (*p2p.TaskResponse, error) {
		orderMu.Lock()
		executionOrder = append(executionOrder, "combine")
		orderMu.Unlock()

		var input map[string]interface{}
		json.Unmarshal(req.Input, &input)

		result := map[string]interface{}{
			"combined": "success",
			"sum":      input["sum"],
			"count":    input["count"],
		}
		return &p2p.TaskResponse{
			TaskID:   req.TaskID,
			Status:   "COMPLETED",
			Result:   mustMarshal(result),
			Duration: 30,
		}, nil
	})

	executor := orchestration.NewDAGExecutor(messageBus, agentSelector, logger)
	workflow := orchestration.NewDAGWorkflow("user_123", "parallel-workflow")
	workflow.MaxParallelism = 5

	// Create DAG:
	//     fetch
	//    /     \
	// process_1  process_2  (parallel)
	//    \     /
	//    combine

	workflow.AddNode(&orchestration.DAGNode{
		ID:           "fetch",
		Name:         "Fetch Data",
		Capabilities: []string{"data-fetch"},
		TaskType:     "fetch-data",
		Dependencies: []string{},
		Timeout:      5 * time.Second,
	})

	workflow.AddNode(&orchestration.DAGNode{
		ID:           "process_1",
		Name:         "Process Part 1",
		Capabilities: []string{"data-processing"},
		TaskType:     "process-data",
		Dependencies: []string{"fetch"},
		InputMapping: map[string]string{"fetch.data": "data"},
		Timeout:      5 * time.Second,
	})

	workflow.AddNode(&orchestration.DAGNode{
		ID:           "process_2",
		Name:         "Process Part 2",
		Capabilities: []string{"data-processing"},
		TaskType:     "process-data",
		Dependencies: []string{"fetch"},
		InputMapping: map[string]string{"fetch.data": "data"},
		Timeout:      5 * time.Second,
	})

	workflow.AddNode(&orchestration.DAGNode{
		ID:           "combine",
		Name:         "Combine Results",
		Capabilities: []string{"data-aggregation"},
		TaskType:     "combine-data",
		Dependencies: []string{"process_1", "process_2"},
		InputMapping: map[string]string{
			"process_1.sum":   "sum",
			"process_2.count": "count",
		},
		Timeout: 5 * time.Second,
	})

	err := executor.ExecuteDAG(ctx, workflow)

	require.NoError(t, err)
	assert.Equal(t, orchestration.ChainStatusCompleted, workflow.Status)

	// Verify execution order: fetch first, then process_1 and process_2 in parallel, then combine
	assert.Equal(t, "fetch", executionOrder[0])
	assert.Contains(t, []string{"process_1", "process_2"}, executionOrder[1])
	assert.Contains(t, []string{"process_1", "process_2"}, executionOrder[2])
	assert.Equal(t, "combine", executionOrder[3])

	// Verify all nodes completed
	for _, node := range workflow.Nodes {
		assert.Equal(t, orchestration.DAGNodeStatusCompleted, node.Status)
	}
}

// TestCoordinationLocks tests distributed lock acquisition and release
func TestCoordinationLocks(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	messageBus := newMockMessageBus()
	coordService := orchestration.NewCoordinationService(messageBus, "agent_1", logger)
	defer coordService.Stop()

	// Test exclusive lock
	lock1, err := coordService.AcquireLock(ctx, "resource_1", orchestration.LockTypeExclusive, 5*time.Second)
	require.NoError(t, err)
	assert.NotEmpty(t, lock1.Token)
	assert.Equal(t, "resource_1", lock1.Resource)
	assert.Equal(t, "agent_1", lock1.Holder)

	// Try to acquire same lock (should wait/fail)
	ctx2, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	_, err = coordService.AcquireLock(ctx2, "resource_1", orchestration.LockTypeExclusive, 5*time.Second)
	assert.Error(t, err) // Should timeout

	// Release lock
	err = coordService.ReleaseLock(lock1.Token)
	require.NoError(t, err)

	// Now should be able to acquire
	lock2, err := coordService.AcquireLock(ctx, "resource_1", orchestration.LockTypeExclusive, 5*time.Second)
	require.NoError(t, err)
	assert.NotEqual(t, lock1.Token, lock2.Token)

	coordService.ReleaseLock(lock2.Token)
}

// TestCoordinationSharedState tests shared state with optimistic locking
func TestCoordinationSharedState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ctx := context.Background()

	messageBus := newMockMessageBus()
	coordService := orchestration.NewCoordinationService(messageBus, "agent_1", logger)
	defer coordService.Stop()

	// Create new state
	value1 := map[string]interface{}{
		"counter": 0,
		"status":  "initialized",
	}
	state1, err := coordService.SetState(ctx, "workflow_state", value1, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(1), state1.Version)

	// Update state
	value2 := map[string]interface{}{
		"counter": 1,
		"status":  "running",
	}
	state2, err := coordService.SetState(ctx, "workflow_state", value2, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(2), state2.Version)

	// Try to update with wrong version (conflict)
	value3 := map[string]interface{}{
		"counter": 999,
		"status":  "wrong",
	}
	_, err = coordService.SetState(ctx, "workflow_state", value3, 1) // Using old version
	assert.ErrorIs(t, err, orchestration.ErrStateConflict)

	// Atomic field update (with retry)
	_, err = coordService.UpdateState(ctx, "workflow_state", "counter", 5)
	require.NoError(t, err)

	// Verify update
	state4, err := coordService.GetState("workflow_state")
	require.NoError(t, err)
	assert.Equal(t, float64(5), state4.Value["counter"])
	assert.Equal(t, int64(3), state4.Version)
}

// mustMarshal marshals data to JSON, panicking on error
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
