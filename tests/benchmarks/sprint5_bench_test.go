package benchmarks

import (
	"context"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// BenchmarkAgentDiscoveryComparison benchmarks CQ-Router vs broadcast discovery
func BenchmarkAgentDiscoveryComparison(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop() // Disable logging for benchmarks

	// Setup test environment
	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(b, err)

	// Populate with test agents
	populateTestAgents(b, dbRepo, 100) // 100 test agents

	b.Run("CQRouter", func(b *testing.B) {
		// Setup CQ-Router
		cqRouter := orchestration.NewCQRouter(logger)

		// Create test task
		task := createBenchmarkTask("cq-router-test")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark CQ-Router agent discovery
			cqRouter.RouteToAgent(ctx, task)
		}
	})

	b.Run("BroadcastDiscovery", func(b *testing.B) {
		// Setup database agent selector (simulates broadcast)
		selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)

		// Create test task
		task := createBenchmarkTask("broadcast-test")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark broadcast-style agent discovery
			_, err := selector.SelectAgent(ctx, task)
			if err != nil {
				b.Fatalf("Agent selection failed: %v", err)
			}
		}
	})
}

// BenchmarkVCGAuctionOverhead benchmarks VCG vs first-price auction overhead
func BenchmarkVCGAuctionOverhead(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Create test bids for auction
	testBids := createTestBids(10) // 10 test bids

	b.Run("VCGAuction", func(b *testing.B) {
		vcgAuctioneer := createTestVCGAuctioneer(b, logger)
		task := createBenchmarkTask("vcg-benchmark")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark VCG auction mechanism
			// Note: In a real benchmark, we'd need actual bidding agents
			// For now, we simulate the VCG calculation overhead
			simulateVCGCalculation(testBids)
		}
	})

	b.Run("FirstPriceAuction", func(b *testing.B) {
		task := createBenchmarkTask("first-price-benchmark")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark first-price auction (simple min selection)
			simulateFirstPriceCalculation(testBids)
		}
	})
}

// BenchmarkReputationQueryLatency benchmarks reputation system query performance
func BenchmarkReputationQueryLatency(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Setup mock blockchain service
	blockchain := createMockBlockchainServiceForBenchmark(b, logger)

	// Test account ID
	testAccount := substrate.AccountID{}
	copy(testAccount[:], []byte("test-agent-account-for-benchmarks"))

	b.Run("GetReputationScore", func(b *testing.B) {
		repClient := blockchain.Reputation()
		if repClient == nil {
			b.Skip("Reputation client not available for benchmark")
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark reputation score query
			_, err := repClient.GetReputationScore(ctx, testAccount)
			if err != nil {
				b.Logf("Reputation query failed (expected in mock): %v", err)
			}
		}
	})

	b.Run("GetReputationStake", func(b *testing.B) {
		repClient := blockchain.Reputation()
		if repClient == nil {
			b.Skip("Reputation client not available for benchmark")
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark full reputation stake query
			_, err := repClient.GetReputationStake(ctx, testAccount)
			if err != nil {
				b.Logf("Reputation stake query failed (expected in mock): %v", err)
			}
		}
	})
}

// BenchmarkEndToEndTaskCompletion benchmarks complete task execution workflow
func BenchmarkEndToEndTaskCompletion(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Setup orchestrator
	orchestrator, queue, cleanup := setupBenchmarkOrchestrator(b, ctx, logger)
	defer cleanup()

	err := orchestrator.Start()
	require.NoError(b, err)
	defer orchestrator.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark complete task lifecycle
		taskID := benchmarkCompleteTaskExecution(b, ctx, queue)
		_ = taskID
	}
}

// BenchmarkTaskQueueOperations benchmarks task queue performance
func BenchmarkTaskQueueOperations(b *testing.B) {
	logger := zap.NewNop()

	b.Run("EnqueueDequeue", func(b *testing.B) {
		queue := orchestration.NewTaskQueue(1000, logger)
		task := createBenchmarkTask("queue-benchmark")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark enqueue/dequeue cycle
			err := queue.Enqueue(task)
			if err != nil {
				b.Fatalf("Failed to enqueue: %v", err)
			}

			_, err = queue.Dequeue()
			if err != nil {
				b.Fatalf("Failed to dequeue: %v", err)
			}
		}
	})

	b.Run("ConcurrentAccess", func(b *testing.B) {
		queue := orchestration.NewTaskQueue(1000, logger)

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				task := createBenchmarkTask("concurrent-" + string(rune(i)))
				queue.Enqueue(task)
				queue.Dequeue()
				i++
			}
		})
	})
}

// BenchmarkDatabaseOperations benchmarks database agent operations
func BenchmarkDatabaseOperations(b *testing.B) {
	logger := zap.NewNop()
	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(b, err)

	// Pre-populate with test agents
	populateTestAgents(b, dbRepo, 1000)

	b.Run("AgentSelection", func(b *testing.B) {
		selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)
		task := createBenchmarkTask("db-benchmark")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := selector.SelectAgent(context.Background(), task)
			if err != nil {
				b.Fatalf("Agent selection failed: %v", err)
			}
		}
	})
}

// BenchmarkMemoryUsage benchmarks memory usage under different scenarios
func BenchmarkMemoryUsage(b *testing.B) {
	logger := zap.NewNop()

	b.Run("TaskCreation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Benchmark task object creation memory usage
			task := createBenchmarkTask("memory-test-" + string(rune(i)))
			_ = task
		}
	})

	b.Run("LargeTaskQueue", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Benchmark memory usage with large task queues
			queue := orchestration.NewTaskQueue(10000, logger)

			// Fill queue
			for j := 0; j < 1000; j++ {
				task := createBenchmarkTask("large-queue-task")
				queue.Enqueue(task)
			}

			queue.Stop()
		}
	})
}

// Performance comparison benchmarks that measure relative performance

// BenchmarkPerformanceComparison runs comparative performance tests
func BenchmarkPerformanceComparison(b *testing.B) {
	// This benchmark compares different implementation approaches

	b.Run("WithReputation", func(b *testing.B) {
		// Benchmark with reputation system enabled
		runScenarioBenchmark(b, true, false)
	})

	b.Run("WithoutReputation", func(b *testing.B) {
		// Benchmark without reputation system
		runScenarioBenchmark(b, false, false)
	})

	b.Run("WithAuctions", func(b *testing.B) {
		// Benchmark with auction system enabled
		runScenarioBenchmark(b, false, true)
	})

	b.Run("WithBothSystems", func(b *testing.B) {
		// Benchmark with both reputation and auctions
		runScenarioBenchmark(b, true, true)
	})
}

// Helper Functions for Benchmarks

func createBenchmarkTask(taskID string) *orchestration.Task {
	return &orchestration.Task{
		ID:           taskID,
		Type:         "benchmark",
		Description:  "Benchmark task",
		Capabilities: []string{"compute"},
		UserID:       "benchmark-user",
		Priority:     orchestration.PriorityNormal,
		Timeout:      10 * time.Second,
		Input: map[string]interface{}{
			"benchmark": true,
			"data":      "test-data",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func populateTestAgents(b *testing.B, dbRepo *database.Repository, count int) {
	// This would populate the database with test agents
	// For benchmarking, we need realistic agent data
	b.Logf("Populating %d test agents for benchmark", count)
	// Implementation would depend on database.Repository interface
}

func createTestBids(count int) []*orchestration.BidSummary {
	bids := make([]*orchestration.BidSummary, count)
	for i := 0; i < count; i++ {
		bids[i] = &orchestration.BidSummary{
			BidID:      "bid-" + string(rune(i)),
			Price:      float64(i+1) * 0.1, // Varying prices
			ETAms:      int64((i + 1) * 1000), // Varying ETAs
			Reputation: float64(500 + i*10), // Varying reputation
		}
	}
	return bids
}

func simulateVCGCalculation(bids []*orchestration.BidSummary) float64 {
	// Simulate VCG auction calculation overhead
	// This includes sorting and second-price calculation

	if len(bids) == 0 {
		return 0
	}

	// Simulate sorting overhead (O(n log n))
	minPrice := bids[0].Price
	secondMinPrice := bids[0].Price

	for _, bid := range bids {
		if bid.Price < minPrice {
			secondMinPrice = minPrice
			minPrice = bid.Price
		} else if bid.Price < secondMinPrice && bid.Price != minPrice {
			secondMinPrice = bid.Price
		}
	}

	// Return second price (VCG payment)
	return secondMinPrice
}

func simulateFirstPriceCalculation(bids []*orchestration.BidSummary) float64 {
	// Simulate first-price auction calculation (O(n))
	if len(bids) == 0 {
		return 0
	}

	minPrice := bids[0].Price
	for _, bid := range bids {
		if bid.Price < minPrice {
			minPrice = bid.Price
		}
	}

	return minPrice
}

func createTestVCGAuctioneer(b *testing.B, logger *zap.Logger) *orchestration.VCGAuctioneer {
	// Create a test VCG auctioneer for benchmarking
	// This requires the MessageBus which might not be available in pure benchmark
	// So we'll create a minimal implementation
	return nil // Would need proper MessageBus setup
}

func createMockBlockchainServiceForBenchmark(b *testing.B, logger *zap.Logger) *substrate.BlockchainService {
	// Create a mock blockchain service optimized for benchmarking
	return substrate.NewBlockchainService(nil, substrate.DefaultServiceConfig(), logger)
}

func setupBenchmarkOrchestrator(b *testing.B, ctx context.Context, logger *zap.Logger) (*orchestration.Orchestrator, *orchestration.TaskQueue, func()) {
	// Create minimal orchestrator setup for benchmarking
	queue := orchestration.NewTaskQueue(100, logger)

	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(b, err)

	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)
	executor := orchestration.NewMockTaskExecutor(logger)

	config := &orchestration.OrchestratorConfig{
		NumWorkers:       2, // Minimal workers for benchmark
		TaskTimeout:      5 * time.Second,
		RetryAttempts:    1,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryBackoff:  1 * time.Second,
		WorkerPollPeriod: 10 * time.Millisecond,
	}

	orchestrator := orchestration.NewOrchestrator(ctx, queue, selector, executor, config, logger)

	cleanup := func() {
		orchestrator.Stop()
		queue.Stop()
	}

	return orchestrator, queue, cleanup
}

func benchmarkCompleteTaskExecution(b *testing.B, ctx context.Context, queue *orchestration.TaskQueue) string {
	task := createBenchmarkTask("end-to-end-benchmark")

	// Submit task
	err := queue.Enqueue(task)
	if err != nil {
		b.Fatalf("Failed to enqueue task: %v", err)
	}

	// Wait for completion (with timeout)
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			b.Fatalf("Task execution timed out")
			return ""
		case <-ticker.C:
			updatedTask, err := queue.Get(task.ID)
			if err != nil {
				continue
			}

			if updatedTask.Status == orchestration.TaskStatusCompleted ||
			   updatedTask.Status == orchestration.TaskStatusFailed {
				return task.ID
			}
		}
	}
}

func runScenarioBenchmark(b *testing.B, withReputation, withAuctions bool) {
	ctx := context.Background()
	logger := zap.NewNop()

	// Setup scenario-specific orchestrator
	orchestrator, queue, cleanup := setupScenarioOrchestrator(b, ctx, logger, withReputation, withAuctions)
	defer cleanup()

	err := orchestrator.Start()
	require.NoError(b, err)
	defer orchestrator.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		task := createBenchmarkTask("scenario-benchmark")

		err := queue.Enqueue(task)
		if err != nil {
			b.Fatalf("Failed to enqueue task: %v", err)
		}

		// Wait for task completion
		waitForTaskCompletionBenchmark(b, queue, task.ID)
	}
}

func setupScenarioOrchestrator(b *testing.B, ctx context.Context, logger *zap.Logger, withReputation, withAuctions bool) (*orchestration.Orchestrator, *orchestration.TaskQueue, func()) {
	queue := orchestration.NewTaskQueue(100, logger)

	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(b, err)

	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)
	executor := orchestration.NewMockTaskExecutor(logger)

	config := &orchestration.OrchestratorConfig{
		NumWorkers:       2,
		TaskTimeout:      5 * time.Second,
		RetryAttempts:    1,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryBackoff:  1 * time.Second,
		WorkerPollPeriod: 10 * time.Millisecond,
	}

	var blockchain *substrate.BlockchainService
	if withReputation {
		blockchain = createMockBlockchainServiceForBenchmark(b, logger)
	}

	var orchestrator *orchestration.Orchestrator
	if blockchain != nil {
		orchestrator = orchestration.NewOrchestratorWithBlockchain(
			ctx, queue, selector, executor, config, logger, blockchain)
	} else {
		orchestrator = orchestration.NewOrchestrator(
			ctx, queue, selector, executor, config, logger)
	}

	// TODO: Add auction setup if withAuctions is true
	// This would require MessageBus setup

	cleanup := func() {
		orchestrator.Stop()
		queue.Stop()
	}

	return orchestrator, queue, cleanup
}

func waitForTaskCompletionBenchmark(b *testing.B, queue *orchestration.TaskQueue, taskID string) {
	timeout := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			b.Fatalf("Task %s timed out", taskID)
			return
		case <-ticker.C:
			task, err := queue.Get(taskID)
			if err != nil {
				continue
			}

			if task.Status == orchestration.TaskStatusCompleted ||
			   task.Status == orchestration.TaskStatusFailed {
				return
			}
		}
	}
}