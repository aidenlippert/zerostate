package integration

import (
	"context"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOrchestratorWorkflow(t *testing.T) {
	// Setup
	ctx := context.Background()
	logger := zap.NewNop()

	// Create HNSW index
	hnsw := search.NewHNSWIndex(16, 200)

	// Register some mock agents
	registerMockAgents(t, hnsw, logger)

	// Create task queue
	queue := orchestration.NewTaskQueue(ctx, 100, logger)
	defer queue.Close()

	// Create agent selector
	selector := orchestration.NewHNSWAgentSelector(hnsw, logger)

	// Create task executor
	executor := orchestration.NewMockTaskExecutor(logger)

	// Create orchestrator
	config := orchestration.DefaultOrchestratorConfig()
	config.NumWorkers = 3
	orch := orchestration.NewOrchestrator(ctx, queue, selector, executor, config, logger)

	// Start orchestrator
	err := orch.Start()
	require.NoError(t, err)
	defer orch.Stop()

	t.Run("SingleTaskExecution", func(t *testing.T) {
		// Submit a task
		task := orchestration.NewTask(
			"test-user",
			"text-processing",
			[]string{"text-analysis", "nlp"},
			map[string]interface{}{
				"text": "Hello, world!",
			},
		)
		task.Timeout = 5 * time.Second

		err := queue.Enqueue(task)
		require.NoError(t, err)

		// Wait for task to complete
		waitForTaskCompletion(t, queue, task.ID, 10*time.Second)

		// Verify task completed
		completedTask, err := queue.Get(task.ID)
		require.NoError(t, err)
		assert.Equal(t, orchestration.TaskStatusCompleted, completedTask.Status)
		assert.NotEmpty(t, completedTask.AssignedTo)
		assert.NotNil(t, completedTask.Result)
	})

	t.Run("MultipleTasksWithPriority", func(t *testing.T) {
		// Submit tasks with different priorities
		tasks := make([]*orchestration.Task, 5)
		priorities := []orchestration.TaskPriority{
			orchestration.PriorityLow,
			orchestration.PriorityNormal,
			orchestration.PriorityHigh,
			orchestration.PriorityCritical,
			orchestration.PriorityNormal,
		}

		for i := 0; i < 5; i++ {
			tasks[i] = orchestration.NewTask(
				"test-user",
				"computation",
				[]string{"compute"},
				map[string]interface{}{"index": i},
			)
			tasks[i].Priority = priorities[i]
			tasks[i].Timeout = 5 * time.Second

			err := queue.Enqueue(tasks[i])
			require.NoError(t, err)
		}

		// Wait for all tasks to complete
		for _, task := range tasks {
			waitForTaskCompletion(t, queue, task.ID, 10*time.Second)
		}

		// Verify all tasks completed
		for _, task := range tasks {
			completedTask, err := queue.Get(task.ID)
			require.NoError(t, err)
			assert.Equal(t, orchestration.TaskStatusCompleted, completedTask.Status)
		}
	})

	t.Run("ConcurrentTaskExecution", func(t *testing.T) {
		// Submit 10 tasks concurrently
		numTasks := 10
		taskIDs := make([]string, numTasks)

		for i := 0; i < numTasks; i++ {
			task := orchestration.NewTask(
				"test-user",
				"parallel-task",
				[]string{"computation"},
				map[string]interface{}{"index": i},
			)
			task.Timeout = 5 * time.Second

			err := queue.Enqueue(task)
			require.NoError(t, err)
			taskIDs[i] = task.ID
		}

		// Wait for all tasks to complete
		for _, taskID := range taskIDs {
			waitForTaskCompletion(t, queue, taskID, 15*time.Second)
		}

		// Verify all tasks completed
		for _, taskID := range taskIDs {
			task, err := queue.Get(taskID)
			require.NoError(t, err)
			assert.Equal(t, orchestration.TaskStatusCompleted, task.Status)
		}
	})

	t.Run("OrchestratorMetrics", func(t *testing.T) {
		// Get initial metrics
		initialMetrics := orch.GetMetrics()

		// Submit a task
		task := orchestration.NewTask(
			"test-user",
			"metrics-test",
			[]string{"analysis"},
			map[string]interface{}{},
		)
		task.Timeout = 5 * time.Second

		err := queue.Enqueue(task)
		require.NoError(t, err)

		// Wait for completion
		waitForTaskCompletion(t, queue, task.ID, 10*time.Second)

		// Check metrics updated
		finalMetrics := orch.GetMetrics()
		assert.Greater(t, finalMetrics.TasksProcessed, initialMetrics.TasksProcessed)
		assert.Greater(t, finalMetrics.TasksSucceeded, initialMetrics.TasksSucceeded)
	})
}

func TestAgentSelection(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create HNSW index
	hnsw := search.NewHNSWIndex(16, 200)

	// Register agents with different capabilities
	agents := []struct {
		name         string
		capabilities []string
	}{
		{"text-agent", []string{"text-analysis", "nlp", "sentiment"}},
		{"image-agent", []string{"image-processing", "ocr", "classification"}},
		{"compute-agent", []string{"computation", "math", "statistics"}},
	}

	for _, a := range agents {
		signer, err := identity.NewSigner(logger)
		require.NoError(t, err)

		card := &identity.AgentCard{
			DID: signer.DID(),
			Keys: &identity.Keys{
				Signing: signer.PublicKeyBase58(),
			},
			Capabilities: make([]identity.Capability, len(a.capabilities)),
		}

		for i, cap := range a.capabilities {
			card.Capabilities[i] = identity.Capability{
				Name:    cap,
				Version: "1.0",
			}
		}

		// Add to HNSW index
		embeddingGen := search.NewEmbedding(128)
		vector := embeddingGen.EncodeCapabilities(a.capabilities, nil)
		hnsw.Add(vector, card)
	}

	// Create selector
	selector := orchestration.NewHNSWAgentSelector(hnsw, logger)

	t.Run("SelectTextAgent", func(t *testing.T) {
		task := orchestration.NewTask(
			"user",
			"text-task",
			[]string{"text-analysis", "nlp"},
			map[string]interface{}{},
		)

		agent, err := selector.SelectAgent(ctx, task)
		require.NoError(t, err)
		assert.NotNil(t, agent)
		assert.NotEmpty(t, agent.DID)
	})

	t.Run("SelectImageAgent", func(t *testing.T) {
		task := orchestration.NewTask(
			"user",
			"image-task",
			[]string{"image-processing", "ocr"},
			map[string]interface{}{},
		)

		agent, err := selector.SelectAgent(ctx, task)
		require.NoError(t, err)
		assert.NotNil(t, agent)
	})

	t.Run("SelectComputeAgent", func(t *testing.T) {
		task := orchestration.NewTask(
			"user",
			"compute-task",
			[]string{"computation", "math"},
			map[string]interface{}{},
		)

		agent, err := selector.SelectAgent(ctx, task)
		require.NoError(t, err)
		assert.NotNil(t, agent)
	})
}

func TestOrchestratorGracefulShutdown(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create components
	hnsw := search.NewHNSWIndex(16, 200)
	registerMockAgents(t, hnsw, logger)

	queue := orchestration.NewTaskQueue(ctx, 100, logger)
	defer queue.Close()

	selector := orchestration.NewHNSWAgentSelector(hnsw, logger)
	executor := orchestration.NewMockTaskExecutor(logger)

	// Create orchestrator with single worker
	config := orchestration.DefaultOrchestratorConfig()
	config.NumWorkers = 1
	orch := orchestration.NewOrchestrator(ctx, queue, selector, executor, config, logger)

	// Start orchestrator
	err := orch.Start()
	require.NoError(t, err)

	// Submit a task
	task := orchestration.NewTask(
		"user",
		"shutdown-test",
		[]string{"test"},
		map[string]interface{}{},
	)
	err = queue.Enqueue(task)
	require.NoError(t, err)

	// Wait a bit for worker to pick up task
	time.Sleep(100 * time.Millisecond)

	// Stop orchestrator
	err = orch.Stop()
	require.NoError(t, err)

	// Verify orchestrator stopped cleanly
	// (no panics or deadlocks)
}

// Helper functions

func registerMockAgents(t *testing.T, hnsw *search.HNSWIndex, logger *zap.Logger) {
	capabilities := [][]string{
		{"text-analysis", "nlp", "sentiment"},
		{"image-processing", "classification"},
		{"computation", "math"},
		{"data-processing", "analytics"},
	}

	for _, caps := range capabilities {
		signer, err := identity.NewSigner(logger)
		require.NoError(t, err)

		card := &identity.AgentCard{
			DID: signer.DID(),
			Keys: &identity.Keys{
				Signing: signer.PublicKeyBase58(),
			},
			Capabilities: make([]identity.Capability, len(caps)),
		}

		for i, cap := range caps {
			card.Capabilities[i] = identity.Capability{
				Name:    cap,
				Version: "1.0",
			}
		}

		// Add to HNSW index
		embeddingGen := search.NewEmbedding(128)
		vector := embeddingGen.EncodeCapabilities(caps, nil)
		hnsw.Add(vector, card)
	}
}

func waitForTaskCompletion(t *testing.T, queue *orchestration.TaskQueue, taskID string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		task, err := queue.Get(taskID)
		require.NoError(t, err)

		if task.IsTerminal() {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("task %s did not complete within %v", taskID, timeout)
}
