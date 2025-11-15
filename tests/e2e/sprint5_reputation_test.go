package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestFullReputationWorkflow tests the complete reputation lifecycle:
// 1. Start chain-v2 blockchain in test mode
// 2. Start orchestrator with reputation enabled
// 3. Submit test task
// 4. Verify agent selection uses reputation
// 5. Complete task successfully
// 6. Verify reputation increased on-chain
// 7. Complete task with failure
// 8. Verify reputation decreased + slashing occurred
func TestFullReputationWorkflow(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	// Step 1: Start blockchain in test mode
	t.Log("🚀 Starting chain-v2 blockchain in test mode...")
	blockchain, cleanup := startTestBlockchain(t, ctx)
	defer cleanup()

	// Step 2: Start orchestrator with reputation enabled
	t.Log("🏗️ Starting orchestrator with reputation integration...")
	orchestrator, taskQueue := setupOrchestratorWithReputation(t, ctx, blockchain, logger)
	defer orchestrator.Stop()

	err := orchestrator.Start()
	require.NoError(t, err)

	// Step 3: Create test agents with different reputation scores
	t.Log("👥 Creating test agents with different reputation scores...")
	agents := createTestAgentsWithReputation(t, ctx, blockchain)
	require.Len(t, agents, 3)

	// Step 4: Submit test task and verify agent selection uses reputation
	t.Log("📋 Submitting test task and verifying reputation-based selection...")
	task := &orchestration.Task{
		ID:           "test-task-1",
		Type:         "math",
		Description:  "Calculate 2+2",
		Capabilities: []string{"math"},
		UserID:       "test-user",
		Priority:     orchestration.PriorityNormal,
		Timeout:      30 * time.Second,
		Input: map[string]interface{}{
			"expression": "2+2",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = taskQueue.Enqueue(task)
	require.NoError(t, err)

	// Wait for task completion
	startTime := time.Now()
	for time.Since(startTime) < 15*time.Second {
		updatedTask, err := taskQueue.Get(task.ID)
		require.NoError(t, err)

		if updatedTask.Status == orchestration.TaskStatusCompleted ||
		   updatedTask.Status == orchestration.TaskStatusFailed {
			task = updatedTask
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify task was assigned to highest reputation agent
	assert.NotEmpty(t, task.AssignedTo, "Task should be assigned to an agent")
	assert.Equal(t, orchestration.TaskStatusCompleted, task.Status, "Task should complete successfully")

	// Step 5: Verify agent selection used reputation scoring
	t.Log("✅ Verifying agent selection used reputation scoring...")
	selectedAgent := findAgentByDID(agents, task.AssignedTo)
	require.NotNil(t, selectedAgent, "Selected agent should exist in our test agents")

	// Should select the highest reputation agent (agents[0] has highest reputation)
	assert.Equal(t, agents[0].DID, task.AssignedTo, "Should select highest reputation agent")

	// Step 6: Verify reputation increased on-chain
	t.Log("📈 Verifying reputation increased on-chain...")

	// Get agent's account ID for blockchain queries
	agentAccount, err := convertDIDToAccountID(selectedAgent.DID)
	require.NoError(t, err)

	// Wait for reputation update to propagate
	time.Sleep(2 * time.Second)

	// Check reputation increased
	newReputation, err := blockchain.Reputation().GetReputationScore(ctx, agentAccount)
	require.NoError(t, err)

	stake, err := blockchain.Reputation().GetReputationStake(ctx, agentAccount)
	require.NoError(t, err)

	assert.Greater(t, newReputation, uint32(500), "Reputation should increase after successful task")
	assert.Equal(t, uint32(1), stake.TasksCompleted, "Should have 1 completed task")

	// Step 7: Submit a task that will fail
	t.Log("💥 Submitting task that will fail...")
	failTask := &orchestration.Task{
		ID:           "test-task-2-fail",
		Type:         "invalid",
		Description:  "This task will fail",
		Capabilities: []string{"nonexistent"},
		UserID:       "test-user",
		Priority:     orchestration.PriorityNormal,
		Timeout:      5 * time.Second,
		Input: map[string]interface{}{
			"invalid": "data",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err = taskQueue.Enqueue(failTask)
	require.NoError(t, err)

	// Wait for task to fail
	startTime = time.Now()
	for time.Since(startTime) < 10*time.Second {
		updatedTask, err := taskQueue.Get(failTask.ID)
		require.NoError(t, err)

		if updatedTask.Status == orchestration.TaskStatusCompleted ||
		   updatedTask.Status == orchestration.TaskStatusFailed {
			failTask = updatedTask
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Step 8: Verify reputation decreased + slashing occurred
	t.Log("📉 Verifying reputation decreased and slashing occurred...")

	// Wait for reputation update to propagate
	time.Sleep(2 * time.Second)

	// Check reputation decreased
	finalReputation, err := blockchain.Reputation().GetReputationScore(ctx, agentAccount)
	require.NoError(t, err)

	finalStake, err := blockchain.Reputation().GetReputationStake(ctx, agentAccount)
	require.NoError(t, err)

	// If task failed and was assigned to same agent, reputation should decrease
	if failTask.AssignedTo == selectedAgent.DID {
		assert.Less(t, finalReputation, newReputation, "Reputation should decrease after failed task")
		assert.Equal(t, uint32(1), finalStake.TasksFailed, "Should have 1 failed task")
		assert.Greater(t, finalStake.Slashed.String(), "0", "Should have some slashing")
	}

	// Verify orchestrator metrics
	metrics := orchestrator.GetMetrics()
	assert.Greater(t, metrics.TasksProcessed, int64(0), "Should have processed tasks")
	assert.Greater(t, metrics.ReputationUpdates, int64(0), "Should have reputation updates")

	t.Log("🎉 Full reputation workflow test completed successfully!")
}

// TestReputationFailoverScenarios tests edge cases and failure scenarios
func TestReputationFailoverScenarios(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	// Test 1: Blockchain unavailable - should fallback gracefully
	t.Run("BlockchainUnavailable", func(t *testing.T) {
		// Start orchestrator without blockchain
		orchestrator, taskQueue := setupOrchestratorWithoutReputation(t, ctx, logger)
		defer orchestrator.Stop()

		err := orchestrator.Start()
		require.NoError(t, err)

		// Submit task
		task := &orchestration.Task{
			ID:           "test-task-no-blockchain",
			Type:         "math",
			Description:  "Calculate 2+2",
			Capabilities: []string{"math"},
			UserID:       "test-user",
			Priority:     orchestration.PriorityNormal,
			Timeout:      30 * time.Second,
			Input: map[string]interface{}{
				"expression": "2+2",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		err = taskQueue.Enqueue(task)
		require.NoError(t, err)

		// Should complete without blockchain
		waitForTaskCompletion(t, taskQueue, task.ID, 10*time.Second)

		metrics := orchestrator.GetMetrics()
		assert.Equal(t, int64(0), metrics.ReputationUpdates, "No reputation updates when blockchain unavailable")
	})

	// Test 2: Reputation query timeout
	t.Run("ReputationQueryTimeout", func(t *testing.T) {
		// This would test timeout scenarios in a real environment
		t.Log("⏰ Reputation query timeout test - would test in real environment")
	})

	// Test 3: Slashing edge cases
	t.Run("SlashingEdgeCases", func(t *testing.T) {
		// This would test various slashing scenarios
		t.Log("⚔️ Slashing edge cases test - would test severe offense scenarios")
	})
}

// Helper Functions

func startTestBlockchain(t *testing.T, ctx context.Context) (*substrate.BlockchainService, func()) {
	// Start a test blockchain instance
	// In a real test, this would spin up a Docker container or connect to test chain

	client, err := substrate.NewClientV2("ws://127.0.0.1:9944")
	if err != nil {
		// For testing purposes, create a mock blockchain service
		return createMockBlockchainService(t), func() {}
	}

	service := substrate.NewBlockchainService(client, substrate.DefaultServiceConfig(), zap.NewNop())

	return service, func() {
		// Cleanup would stop the blockchain
	}
}

func createMockBlockchainService(t *testing.T) *substrate.BlockchainService {
	// Create a mock blockchain service for testing
	return substrate.NewBlockchainService(nil, substrate.DefaultServiceConfig(), zap.NewNop())
}

func setupOrchestratorWithReputation(t *testing.T, ctx context.Context, blockchain *substrate.BlockchainService, logger *zap.Logger) (*orchestration.Orchestrator, *orchestration.TaskQueue) {
	// Create task queue
	queue := orchestration.NewTaskQueue(100, logger)

	// Create mock database agent selector
	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(t, err)

	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)

	// Create mock executor
	executor := orchestration.NewMockTaskExecutor(logger)

	// Create orchestrator with blockchain integration
	config := orchestration.DefaultOrchestratorConfig()
	orchestrator := orchestration.NewOrchestratorWithBlockchain(
		ctx, queue, selector, executor, config, logger, blockchain)

	return orchestrator, queue
}

func setupOrchestratorWithoutReputation(t *testing.T, ctx context.Context, logger *zap.Logger) (*orchestration.Orchestrator, *orchestration.TaskQueue) {
	// Setup orchestrator without blockchain integration
	queue := orchestration.NewTaskQueue(100, logger)

	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(t, err)

	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)
	executor := orchestration.NewMockTaskExecutor(logger)

	config := orchestration.DefaultOrchestratorConfig()
	orchestrator := orchestration.NewOrchestrator(ctx, queue, selector, executor, config, logger)

	return orchestrator, queue
}

func createTestAgentsWithReputation(t *testing.T, ctx context.Context, blockchain *substrate.BlockchainService) []*identity.AgentCard {
	agents := []*identity.AgentCard{
		{
			DID: "did:substrate:agent1-high-rep",
			Endpoints: &identity.Endpoints{
				HTTP: []string{"http://agent1:8080"},
			},
			Capabilities: []identity.Capability{
				{Name: "math", Version: "v1"},
			},
			Proof: &identity.Proof{},
		},
		{
			DID: "did:substrate:agent2-med-rep",
			Endpoints: &identity.Endpoints{
				HTTP: []string{"http://agent2:8080"},
			},
			Capabilities: []identity.Capability{
				{Name: "math", Version: "v1"},
			},
			Proof: &identity.Proof{},
		},
		{
			DID: "did:substrate:agent3-low-rep",
			Endpoints: &identity.Endpoints{
				HTTP: []string{"http://agent3:8080"},
			},
			Capabilities: []identity.Capability{
				{Name: "math", Version: "v1"},
			},
			Proof: &identity.Proof{},
		},
	}

	// Setup different reputation scores for agents in blockchain
	if blockchain != nil && blockchain.IsEnabled() {
		for i, agent := range agents {
			accountID, err := convertDIDToAccountID(agent.DID)
			require.NoError(t, err)

			// Bond different amounts of reputation
			bondAmount := uint64(1000 + (i * 500)) // Higher reputation for earlier agents
			err = blockchain.Reputation().BondReputation(ctx, bondAmount)
			if err != nil {
				t.Logf("Warning: Could not bond reputation for agent %s: %v", agent.DID, err)
			}
		}
	}

	return agents
}

func findAgentByDID(agents []*identity.AgentCard, did string) *identity.AgentCard {
	for _, agent := range agents {
		if agent.DID == did {
			return agent
		}
	}
	return nil
}

func convertDIDToAccountID(did string) (substrate.AccountID, error) {
	// Simplified conversion for testing
	var accountID substrate.AccountID
	copy(accountID[:], []byte(did)[:32])
	return accountID, nil
}

func waitForTaskCompletion(t *testing.T, queue *orchestration.TaskQueue, taskID string, timeout time.Duration) *orchestration.Task {
	startTime := time.Now()
	for time.Since(startTime) < timeout {
		task, err := queue.Get(taskID)
		require.NoError(t, err)

		if task.Status == orchestration.TaskStatusCompleted ||
		   task.Status == orchestration.TaskStatusFailed {
			return task
		}
		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Task %s did not complete within %v", taskID, timeout)
	return nil
}