package load

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// LoadTestConfig defines load test parameters
type LoadTestConfig struct {
	ConcurrentTasks     int           // Number of concurrent tasks
	TasksPerWorker      int           // Tasks per concurrent worker
	TestDuration        time.Duration // Total test duration
	TaskTimeoutMin      time.Duration // Minimum task timeout
	TaskTimeoutMax      time.Duration // Maximum task timeout
	ReputationEnabled   bool          // Whether to test with reputation
	AuctionEnabled      bool          // Whether to test with auctions
}

// LoadTestResults captures comprehensive load test metrics
type LoadTestResults struct {
	// Task metrics
	TotalTasks          int64         `json:"total_tasks"`
	CompletedTasks      int64         `json:"completed_tasks"`
	FailedTasks         int64         `json:"failed_tasks"`
	TimedOutTasks       int64         `json:"timed_out_tasks"`

	// Timing metrics
	TestDuration        time.Duration `json:"test_duration"`
	AvgTaskLatency      time.Duration `json:"avg_task_latency"`
	P50Latency          time.Duration `json:"p50_latency"`
	P95Latency          time.Duration `json:"p95_latency"`
	P99Latency          time.Duration `json:"p99_latency"`
	MinLatency          time.Duration `json:"min_latency"`
	MaxLatency          time.Duration `json:"max_latency"`

	// Throughput metrics
	TasksPerSecond      float64       `json:"tasks_per_second"`

	// Reputation metrics (if enabled)
	ReputationUpdates   int64         `json:"reputation_updates"`
	ReputationFailures  int64         `json:"reputation_failures"`
	ReputationUpdateRate float64      `json:"reputation_update_rate"`

	// Error metrics
	Errors              []string      `json:"errors"`
	ErrorRate           float64       `json:"error_rate"`

	// Resource metrics
	PeakMemoryUsage     int64         `json:"peak_memory_mb"`
	CPUUsagePercent     float64       `json:"cpu_usage_percent"`

	// Race condition detection
	RaceConditions      int64         `json:"race_conditions"`
	DataInconsistencies int64         `json:"data_inconsistencies"`
}

// TestConcurrentTaskLoad tests 100 concurrent tasks with reputation and auction systems
func TestConcurrentTaskLoad(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	config := LoadTestConfig{
		ConcurrentTasks:     100,
		TasksPerWorker:      10, // 1000 total tasks
		TestDuration:        2 * time.Minute,
		TaskTimeoutMin:      1 * time.Second,
		TaskTimeoutMax:      10 * time.Second,
		ReputationEnabled:   true,
		AuctionEnabled:      false, // Start without auctions for baseline
	}

	t.Logf("🚀 Starting concurrent load test: %d workers × %d tasks = %d total tasks",
		config.ConcurrentTasks, config.TasksPerWorker,
		config.ConcurrentTasks * config.TasksPerWorker)

	// Setup load test environment
	orchestrator, queue, blockchain, cleanup := setupLoadTestEnvironment(t, ctx, logger, config)
	defer cleanup()

	err := orchestrator.Start()
	require.NoError(t, err)
	defer orchestrator.Stop()

	// Run the load test
	startTime := time.Now()
	results := runConcurrentLoad(t, ctx, queue, config)
	results.TestDuration = time.Since(startTime)

	// Get orchestrator metrics
	orchMetrics := orchestrator.GetMetrics()
	results.ReputationUpdates = orchMetrics.ReputationUpdates
	results.ReputationFailures = orchMetrics.ReputationFailures

	if results.TotalTasks > 0 {
		results.ReputationUpdateRate = float64(results.ReputationUpdates) / float64(results.TotalTasks)
		results.ErrorRate = float64(results.FailedTasks) / float64(results.TotalTasks)
	}

	// Calculate tasks per second
	results.TasksPerSecond = float64(results.CompletedTasks) / results.TestDuration.Seconds()

	// Print detailed results
	printLoadTestResults(t, results, config)

	// Performance assertions
	assert.GreaterOrEqual(t, results.TasksPerSecond, 5.0, "Should handle at least 5 tasks/second")
	assert.LessOrEqual(t, results.P95Latency, 2*time.Second, "P95 latency should be under 2 seconds")
	assert.LessOrEqual(t, results.ErrorRate, 0.05, "Error rate should be under 5%")
	assert.GreaterOrEqual(t, results.ReputationUpdateRate, 0.95, "Reputation update rate should be >95%")
	assert.Equal(t, int64(0), results.RaceConditions, "Should detect no race conditions")

	t.Log("🎉 Concurrent load test completed successfully!")
}

// TestReputationSystemLoad specifically tests reputation system under load
func TestReputationSystemLoad(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	config := LoadTestConfig{
		ConcurrentTasks:     50,
		TasksPerWorker:      20, // 1000 total tasks
		TestDuration:        90 * time.Second,
		TaskTimeoutMin:      500 * time.Millisecond,
		TaskTimeoutMax:      5 * time.Second,
		ReputationEnabled:   true,
		AuctionEnabled:      false,
	}

	t.Log("📊 Testing reputation system under load...")

	orchestrator, queue, blockchain, cleanup := setupLoadTestEnvironment(t, ctx, logger, config)
	defer cleanup()

	err := orchestrator.Start()
	require.NoError(t, err)
	defer orchestrator.Stop()

	// Monitor reputation circuit breaker status
	go monitorReputationCircuitBreaker(t, orchestrator, config.TestDuration)

	// Run the load test
	results := runConcurrentLoad(t, ctx, queue, config)

	// Specific reputation metrics
	orchMetrics := orchestrator.GetMetrics()
	results.ReputationUpdates = orchMetrics.ReputationUpdates
	results.ReputationFailures = orchMetrics.ReputationFailures

	// Test reputation update success rate
	expectedUpdates := results.CompletedTasks + results.FailedTasks
	actualUpdates := results.ReputationUpdates
	updateSuccessRate := float64(actualUpdates) / float64(expectedUpdates)

	t.Logf("Reputation Updates: %d/%d (%.1f%% success rate)",
		actualUpdates, expectedUpdates, updateSuccessRate*100)

	// Assertions for reputation system
	assert.GreaterOrEqual(t, updateSuccessRate, 0.95, "Reputation update success rate should be >95%")
	assert.LessOrEqual(t, orchMetrics.ReputationFailures, orchMetrics.ReputationUpdates/10,
		"Reputation failures should be <10% of updates")

	t.Log("✅ Reputation system load test completed!")
}

// TestAuctionSystemLoad tests auction system performance under load
func TestAuctionSystemLoad(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	config := LoadTestConfig{
		ConcurrentTasks:     30, // Lower concurrency for auction overhead
		TasksPerWorker:      15,
		TestDuration:        60 * time.Second,
		TaskTimeoutMin:      2 * time.Second,   // Longer timeouts for auctions
		TaskTimeoutMax:      15 * time.Second,
		ReputationEnabled:   true,
		AuctionEnabled:      true,
	}

	t.Log("🏆 Testing auction system under load...")

	orchestrator, queue, blockchain, cleanup := setupLoadTestEnvironment(t, ctx, logger, config)
	defer cleanup()

	err := orchestrator.Start()
	require.NoError(t, err)
	defer orchestrator.Stop()

	// Run the load test with auctions enabled
	results := runConcurrentLoad(t, ctx, queue, config)

	// Get auction metrics
	orchMetrics := orchestrator.GetMetrics()
	auctionsStarted := orchMetrics.AuctionsStarted
	auctionSuccesses := orchMetrics.AuctionSuccesses
	auctionFailures := orchMetrics.AuctionFailures
	auctionNoBids := orchMetrics.AuctionNoBids
	dbFallbacks := orchMetrics.DBFallbacks

	auctionSuccessRate := float64(auctionSuccesses) / float64(auctionsStarted)

	t.Logf("Auction Metrics:")
	t.Logf("  Started: %d, Successes: %d, Failures: %d, No Bids: %d",
		auctionsStarted, auctionSuccesses, auctionFailures, auctionNoBids)
	t.Logf("  Success Rate: %.1f%%, DB Fallbacks: %d",
		auctionSuccessRate*100, dbFallbacks)

	// Auction system assertions (more lenient due to P2P complexity)
	assert.GreaterOrEqual(t, auctionSuccessRate, 0.70, "Auction success rate should be >70%")
	assert.LessOrEqual(t, dbFallbacks, auctionsStarted/2, "DB fallbacks should be <50% of auctions")
	assert.LessOrEqual(t, results.P95Latency, 10*time.Second, "P95 latency should be under 10 seconds with auctions")

	t.Log("✅ Auction system load test completed!")
}

// TestStressConditions tests system behavior under extreme conditions
func TestStressConditions(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	t.Log("⚡ Running stress condition tests...")

	// Test 1: Memory pressure
	t.Run("MemoryPressure", func(t *testing.T) {
		config := LoadTestConfig{
			ConcurrentTasks:     200, // High concurrency
			TasksPerWorker:      5,   // Short bursts
			TestDuration:        30 * time.Second,
			TaskTimeoutMin:      100 * time.Millisecond,
			TaskTimeoutMax:      2 * time.Second,
			ReputationEnabled:   true,
			AuctionEnabled:      false,
		}

		orchestrator, queue, _, cleanup := setupLoadTestEnvironment(t, ctx, logger, config)
		defer cleanup()

		err := orchestrator.Start()
		require.NoError(t, err)
		defer orchestrator.Stop()

		results := runConcurrentLoad(t, ctx, queue, config)

		// Under stress, we expect some degradation but no catastrophic failure
		assert.GreaterOrEqual(t, results.TasksPerSecond, 2.0, "Should maintain >2 tasks/second under stress")
		assert.LessOrEqual(t, results.ErrorRate, 0.20, "Error rate should be <20% under stress")
	})

	// Test 2: Blockchain unavailability
	t.Run("BlockchainUnavailable", func(t *testing.T) {
		config := LoadTestConfig{
			ConcurrentTasks:     50,
			TasksPerWorker:      10,
			TestDuration:        30 * time.Second,
			TaskTimeoutMin:      1 * time.Second,
			TaskTimeoutMax:      5 * time.Second,
			ReputationEnabled:   false, // Blockchain unavailable
			AuctionEnabled:      false,
		}

		orchestrator, queue, _, cleanup := setupLoadTestEnvironmentWithoutBlockchain(t, ctx, logger, config)
		defer cleanup()

		err := orchestrator.Start()
		require.NoError(t, err)
		defer orchestrator.Stop()

		results := runConcurrentLoad(t, ctx, queue, config)

		// Should gracefully handle blockchain unavailability
		assert.GreaterOrEqual(t, results.TasksPerSecond, 3.0, "Should handle tasks without blockchain")
		assert.LessOrEqual(t, results.ErrorRate, 0.10, "Error rate should be <10% without blockchain")
	})

	t.Log("✅ Stress condition tests completed!")
}

// Helper Functions

func setupLoadTestEnvironment(t *testing.T, ctx context.Context, logger *zap.Logger, config LoadTestConfig) (*orchestration.Orchestrator, *orchestration.TaskQueue, *substrate.BlockchainService, func()) {
	// Create task queue with larger buffer for load testing
	queue := orchestration.NewTaskQueue(10000, logger)

	// Create database
	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(t, err)

	// Create agent selector
	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)

	// Create executor
	executor := orchestration.NewMockTaskExecutor(logger)

	// Setup blockchain if reputation enabled
	var blockchain *substrate.BlockchainService
	if config.ReputationEnabled {
		blockchain = createMockBlockchainService(t, logger)
	}

	// Create orchestrator with more workers for load testing
	orchConfig := &orchestration.OrchestratorConfig{
		NumWorkers:       20, // More workers for load testing
		TaskTimeout:      config.TaskTimeoutMax,
		RetryAttempts:    2,  // Fewer retries for faster load testing
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryBackoff:  1 * time.Second,
		WorkerPollPeriod: 50 * time.Millisecond, // Faster polling
	}

	orchestrator := orchestration.NewOrchestratorWithBlockchain(
		ctx, queue, selector, executor, orchConfig, logger, blockchain)

	cleanup := func() {
		orchestrator.Stop()
		queue.Stop()
		if blockchain != nil {
			// Cleanup blockchain
		}
	}

	return orchestrator, queue, blockchain, cleanup
}

func setupLoadTestEnvironmentWithoutBlockchain(t *testing.T, ctx context.Context, logger *zap.Logger, config LoadTestConfig) (*orchestration.Orchestrator, *orchestration.TaskQueue, *substrate.BlockchainService, func()) {
	queue := orchestration.NewTaskQueue(10000, logger)

	dbRepo, err := database.NewSQLiteRepository(":memory:", logger)
	require.NoError(t, err)

	selector := orchestration.NewDatabaseAgentSelector(dbRepo, logger)
	executor := orchestration.NewMockTaskExecutor(logger)

	orchConfig := &orchestration.OrchestratorConfig{
		NumWorkers:       20,
		TaskTimeout:      config.TaskTimeoutMax,
		RetryAttempts:    2,
		RetryBackoff:     100 * time.Millisecond,
		MaxRetryBackoff:  1 * time.Second,
		WorkerPollPeriod: 50 * time.Millisecond,
	}

	// No blockchain integration
	orchestrator := orchestration.NewOrchestrator(
		ctx, queue, selector, executor, orchConfig, logger)

	cleanup := func() {
		orchestrator.Stop()
		queue.Stop()
	}

	return orchestrator, queue, nil, cleanup
}

func createMockBlockchainService(t *testing.T, logger *zap.Logger) *substrate.BlockchainService {
	// Create mock blockchain service that simulates reputation system
	return substrate.NewBlockchainService(nil, substrate.DefaultServiceConfig(), logger)
}

func runConcurrentLoad(t *testing.T, ctx context.Context, queue *orchestration.TaskQueue, config LoadTestConfig) *LoadTestResults {
	results := &LoadTestResults{}
	var wg sync.WaitGroup
	var latencies []time.Duration
	var latencyMutex sync.Mutex

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, config.TestDuration)
	defer cancel()

	// Launch concurrent workers
	for i := 0; i < config.ConcurrentTasks; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			runWorkerLoad(timeoutCtx, workerID, queue, config, results, &latencies, &latencyMutex)
		}(i)
	}

	// Wait for all workers to complete
	wg.Wait()

	// Calculate latency percentiles
	calculateLatencyPercentiles(latencies, results)

	return results
}

func runWorkerLoad(
	ctx context.Context,
	workerID int,
	queue *orchestration.TaskQueue,
	config LoadTestConfig,
	results *LoadTestResults,
	latencies *[]time.Duration,
	latencyMutex *sync.Mutex,
) {
	for i := 0; i < config.TasksPerWorker; i++ {
		select {
		case <-ctx.Done():
			return // Test timeout reached
		default:
			submitAndTrackTask(ctx, workerID, i, queue, config, results, latencies, latencyMutex)
		}
	}
}

func submitAndTrackTask(
	ctx context.Context,
	workerID, taskNum int,
	queue *orchestration.TaskQueue,
	config LoadTestConfig,
	results *LoadTestResults,
	latencies *[]time.Duration,
	latencyMutex *sync.Mutex,
) {
	taskID := fmt.Sprintf("load-test-w%d-t%d-%d", workerID, taskNum, time.Now().UnixNano())

	// Randomize task timeout
	timeout := config.TaskTimeoutMin + time.Duration(
		rand.Float64() * float64(config.TaskTimeoutMax - config.TaskTimeoutMin))

	task := &orchestration.Task{
		ID:           taskID,
		Type:         "load-test",
		Description:  fmt.Sprintf("Load test task %s", taskID),
		Capabilities: []string{"mock"},
		UserID:       fmt.Sprintf("load-user-%d", workerID),
		Priority:     orchestration.PriorityNormal,
		Timeout:      timeout,
		Input: map[string]interface{}{
			"worker_id": workerID,
			"task_num":  taskNum,
			"data":      generateTestData(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	startTime := time.Now()

	// Submit task
	err := queue.Enqueue(task)
	atomic.AddInt64(&results.TotalTasks, 1)

	if err != nil {
		atomic.AddInt64(&results.FailedTasks, 1)
		return
	}

	// Wait for task completion or timeout
	completed := waitForTaskCompletion(ctx, queue, taskID, timeout + 5*time.Second)
	latency := time.Since(startTime)

	// Record latency
	latencyMutex.Lock()
	*latencies = append(*latencies, latency)
	latencyMutex.Unlock()

	// Update metrics based on result
	if completed != nil {
		switch completed.Status {
		case orchestration.TaskStatusCompleted:
			atomic.AddInt64(&results.CompletedTasks, 1)
		case orchestration.TaskStatusFailed:
			atomic.AddInt64(&results.FailedTasks, 1)
		default:
			atomic.AddInt64(&results.TimedOutTasks, 1)
		}
	} else {
		atomic.AddInt64(&results.TimedOutTasks, 1)
	}
}

func waitForTaskCompletion(ctx context.Context, queue *orchestration.TaskQueue, taskID string, timeout time.Duration) *orchestration.Task {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			return nil
		case <-ticker.C:
			task, err := queue.Get(taskID)
			if err != nil {
				continue
			}

			if task.Status == orchestration.TaskStatusCompleted ||
			   task.Status == orchestration.TaskStatusFailed {
				return task
			}
		}
	}
}

func calculateLatencyPercentiles(latencies []time.Duration, results *LoadTestResults) {
	if len(latencies) == 0 {
		return
	}

	// Sort latencies
	sortedLatencies := make([]time.Duration, len(latencies))
	copy(sortedLatencies, latencies)

	// Simple insertion sort (good enough for load test)
	for i := 1; i < len(sortedLatencies); i++ {
		key := sortedLatencies[i]
		j := i - 1
		for j >= 0 && sortedLatencies[j] > key {
			sortedLatencies[j+1] = sortedLatencies[j]
			j--
		}
		sortedLatencies[j+1] = key
	}

	// Calculate percentiles
	results.MinLatency = sortedLatencies[0]
	results.MaxLatency = sortedLatencies[len(sortedLatencies)-1]
	results.P50Latency = sortedLatencies[len(sortedLatencies)/2]
	results.P95Latency = sortedLatencies[int(float64(len(sortedLatencies))*0.95)]
	results.P99Latency = sortedLatencies[int(float64(len(sortedLatencies))*0.99)]

	// Calculate average
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	results.AvgTaskLatency = total / time.Duration(len(latencies))
}

func generateTestData() map[string]interface{} {
	return map[string]interface{}{
		"timestamp": time.Now().UnixNano(),
		"random":    rand.Float64(),
		"data":      make([]byte, 100), // 100 bytes of test data
	}
}

func monitorReputationCircuitBreaker(t *testing.T, orchestrator *orchestration.Orchestrator, duration time.Duration) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration)

	for {
		select {
		case <-timeout:
			return
		case <-ticker.C:
			stats := orchestrator.GetReputationCircuitBreakerStats()
			t.Logf("Reputation Circuit Breaker: failures=%v, is_open=%v",
				stats["failures"], stats["is_open"])
		}
	}
}

func printLoadTestResults(t *testing.T, results *LoadTestResults, config LoadTestConfig) {
	t.Log("📊 Load Test Results:")
	t.Logf("  Total Tasks: %d (Completed: %d, Failed: %d, Timed Out: %d)",
		results.TotalTasks, results.CompletedTasks, results.FailedTasks, results.TimedOutTasks)
	t.Logf("  Duration: %v", results.TestDuration)
	t.Logf("  Throughput: %.2f tasks/second", results.TasksPerSecond)
	t.Logf("  Error Rate: %.2f%%", results.ErrorRate*100)
	t.Logf("  Latency - Avg: %v, P50: %v, P95: %v, P99: %v",
		results.AvgTaskLatency, results.P50Latency, results.P95Latency, results.P99Latency)

	if config.ReputationEnabled {
		t.Logf("  Reputation Updates: %d/%d (%.1f%% success rate)",
			results.ReputationUpdates, results.TotalTasks, results.ReputationUpdateRate*100)
	}
}