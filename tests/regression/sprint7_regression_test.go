// Package regression provides comprehensive Sprint 7 performance regression tests
// Baseline: Sprint 6 performance (12.5 tasks/sec, 85ms P95) vs Sprint 7 performance
package regression

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/marketplace"
	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
	"github.com/aidenlippert/zerostate/libs/substrate"
)

// Sprint7RegressionTestSuite validates performance regression against Sprint 6 baselines
type Sprint7RegressionTestSuite struct {
	suite.Suite
	ctx                     context.Context
	cancel                  context.CancelFunc

	// System Components
	escrowClient           *substrate.EscrowClient
	reputationClient       *substrate.ReputationClient
	auctionClient          *substrate.VCGAuctionClient
	orchestrator           *orchestration.Orchestrator
	paymentService         *economic.PaymentChannelService
	reputationService      *reputation.ReputationService
	marketplaceService     *marketplace.MarketplaceService
	messageBus             *RegressionMockMessageBus
	metricsCollector       *metrics.Collector

	// Performance Tracking
	performanceMetrics     *PerformanceMetrics
	sprint6Baseline        *PerformanceBaseline
	regressionResults      *RegressionResults
	loadTestResults        chan LoadTestResult
}

// PerformanceMetrics tracks detailed performance data for Sprint 7
type PerformanceMetrics struct {
	mu                      sync.RWMutex
	StartTime              time.Time

	// Throughput Metrics
	TasksSubmitted         int64
	TasksCompleted         int64
	TasksPerSecond         float64
	PeakTasksPerSecond     float64

	// Latency Metrics (in milliseconds)
	TaskSubmissionLatencies    []float64
	VCGAuctionLatencies       []float64
	EscrowOperationLatencies  []float64
	PaymentLatencies          []float64
	EndToEndLatencies         []float64

	// Percentile Latencies
	LatencyP50             float64
	LatencyP95             float64
	LatencyP99             float64
	LatencyP99_9           float64

	// Resource Utilization
	MemoryUsageBytes       []int64
	CPUUtilization         []float64
	GoroutineCount         []int64
	NetworkBytesTransferred int64

	// Error Rates
	TimeoutErrors          int64
	ValidationErrors       int64
	NetworkErrors          int64
	TotalErrors           int64
	ErrorRate             float64

	// Concurrent Operations
	MaxConcurrentTasks     int64
	AverageConcurrentTasks float64
}

// PerformanceBaseline represents Sprint 6 performance baseline
type PerformanceBaseline struct {
	// Sprint 6 Established Baselines
	TasksPerSecond      float64   // 12.5 tasks/sec
	LatencyP50Ms       float64   // 45ms
	LatencyP95Ms       float64   // 85ms
	LatencyP99Ms       float64   // 120ms
	MemoryUsageMB      float64   // 180MB
	ErrorRatePercent   float64   // 0.1%
	MaxConcurrent      int64     // 75 concurrent tasks

	// Acceptable regression thresholds
	ThroughputTolerance      float64 // 5% degradation allowed
	LatencyTolerancePercent  float64 // 10% latency increase allowed
	MemoryTolerancePercent   float64 // 15% memory increase allowed
	ErrorRateThresholdPercent float64 // 0.5% max error rate
}

// RegressionResults stores comparison between Sprint 6 and Sprint 7
type RegressionResults struct {
	mu                        sync.RWMutex

	// Performance Comparison
	ThroughputChange         float64 // Percentage change from baseline
	LatencyP95Change         float64 // Percentage change in P95 latency
	LatencyP99Change         float64 // Percentage change in P99 latency
	MemoryUsageChange        float64 // Percentage change in memory usage
	ErrorRateChange          float64 // Change in error rate

	// Regression Status
	ThroughputRegression     bool
	LatencyRegression        bool
	MemoryRegression         bool
	ErrorRateRegression      bool
	OverallRegression        bool

	// Performance Score (0-100, higher is better)
	PerformanceScore         float64
	RegressionSeverity       string // "None", "Minor", "Moderate", "Severe"

	// Detailed Analysis
	RegressionImpacts        []RegressionImpact
	RecommendedActions       []string
}

// RegressionImpact represents a specific performance regression
type RegressionImpact struct {
	Component       string
	Metric          string
	BaselineValue   float64
	CurrentValue    float64
	ChangePercent   float64
	Severity        string
	Impact          string
}

// LoadTestResult represents results from load testing
type LoadTestResult struct {
	TestName         string
	Duration         time.Duration
	TasksCompleted   int64
	TasksPerSecond   float64
	LatencyP95       float64
	MemoryUsageMB    float64
	ErrorCount       int64
	Success          bool
}

// RegressionMockMessageBus simulates realistic production load
type RegressionMockMessageBus struct {
	mu                 sync.RWMutex
	messagesSent      int64
	messagesReceived  int64
	subscriptions     map[string][]p2p.MessageHandler
	requestHandlers   map[string]p2p.RequestHandler
	latencySimulation LatencySimulation
}

type LatencySimulation struct {
	BaseLatencyMs    int
	JitterRangeMs    int
	NetworkLoad      float64 // 0.0 to 1.0, affects latency
}

func NewRegressionMockMessageBus() *RegressionMockMessageBus {
	return &RegressionMockMessageBus{
		subscriptions:   make(map[string][]p2p.MessageHandler),
		requestHandlers: make(map[string]p2p.RequestHandler),
		latencySimulation: LatencySimulation{
			BaseLatencyMs: 3,
			JitterRangeMs: 2,
			NetworkLoad:   0.1, // Light load initially
		},
	}
}

func (m *RegressionMockMessageBus) Start(ctx context.Context) error {
	return nil
}

func (m *RegressionMockMessageBus) Stop() error {
	return nil
}

func (m *RegressionMockMessageBus) Publish(ctx context.Context, topic string, data []byte) error {
	atomic.AddInt64(&m.messagesSent, 1)

	// Simulate realistic network latency with load simulation
	latency := m.calculateRealisticLatency()
	time.Sleep(latency)

	// Deliver to subscribers
	m.mu.RLock()
	handlers := m.subscriptions[topic]
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h p2p.MessageHandler) {
			atomic.AddInt64(&m.messagesReceived, 1)
			h(data)
		}(handler)
	}

	return nil
}

func (m *RegressionMockMessageBus) Subscribe(ctx context.Context, topic string, handler p2p.MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[topic] = append(m.subscriptions[topic], handler)
	return nil
}

func (m *RegressionMockMessageBus) SendRequest(ctx context.Context, targetDID string, request []byte, timeout time.Duration) ([]byte, error) {
	atomic.AddInt64(&m.messagesSent, 1)

	// Simulate network latency
	latency := m.calculateRealisticLatency()
	time.Sleep(latency)

	// Generate response
	responseSize := len(request) + 128
	response := make([]byte, responseSize)
	copy(response, []byte(fmt.Sprintf("regression-test-response-%s", targetDID)))

	atomic.AddInt64(&m.messagesReceived, 1)
	return response, nil
}

func (m *RegressionMockMessageBus) RegisterRequestHandler(messageType string, handler p2p.RequestHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestHandlers[messageType] = handler
	return nil
}

func (m *RegressionMockMessageBus) GetPeerID() string {
	return "sprint7-regression-test-peer"
}

func (m *RegressionMockMessageBus) SetNetworkLoad(load float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.latencySimulation.NetworkLoad = load
}

func (m *RegressionMockMessageBus) calculateRealisticLatency() time.Duration {
	baseLatency := time.Duration(m.latencySimulation.BaseLatencyMs) * time.Millisecond
	jitter := time.Duration(rand.Intn(m.latencySimulation.JitterRangeMs*2)-m.latencySimulation.JitterRangeMs) * time.Millisecond

	// Add load-based latency increase
	loadMultiplier := 1.0 + (m.latencySimulation.NetworkLoad * 2.0) // Up to 3x latency under load
	finalLatency := time.Duration(float64(baseLatency + jitter) * loadMultiplier)

	return finalLatency
}

func TestSprint7Regression(t *testing.T) {
	suite.Run(t, new(Sprint7RegressionTestSuite))
}

func (s *Sprint7RegressionTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 30*time.Minute)

	// Initialize Sprint 6 baseline (established from previous sprint)
	s.sprint6Baseline = &PerformanceBaseline{
		TasksPerSecond:           12.5,
		LatencyP50Ms:            45.0,
		LatencyP95Ms:            85.0,
		LatencyP99Ms:            120.0,
		MemoryUsageMB:           180.0,
		ErrorRatePercent:        0.1,
		MaxConcurrent:           75,
		ThroughputTolerance:     5.0,  // 5% degradation allowed
		LatencyTolerancePercent: 10.0, // 10% latency increase allowed
		MemoryTolerancePercent:  15.0, // 15% memory increase allowed
		ErrorRateThresholdPercent: 0.5, // 0.5% max error rate
	}

	// Initialize performance metrics tracking
	s.performanceMetrics = &PerformanceMetrics{
		StartTime: time.Now(),
	}

	// Initialize regression results
	s.regressionResults = &RegressionResults{}

	// Setup load test results channel
	s.loadTestResults = make(chan LoadTestResult, 100)

	// Setup message bus with regression testing configuration
	s.messageBus = NewRegressionMockMessageBus()
	err := s.messageBus.Start(s.ctx)
	require.NoError(s.T(), err)

	// Initialize metrics collector
	s.metricsCollector = metrics.NewCollector()

	// Setup blockchain connection
	substrateClient, err := substrate.NewClientV2("ws://localhost:9944")
	require.NoError(s.T(), err, "Failed to connect to Substrate node for regression tests")

	keyring, err := substrate.CreateKeyringFromSeed("//Alice", substrate.Sr25519Type)
	require.NoError(s.T(), err)

	// Initialize blockchain clients
	s.escrowClient = substrate.NewEscrowClient(substrateClient, keyring)
	s.reputationClient = substrate.NewReputationClient(substrateClient, keyring)
	s.auctionClient = substrate.NewVCGAuctionClient(substrateClient, keyring)

	// Initialize services
	s.paymentService = economic.NewPaymentChannelService()
	s.reputationService = reputation.NewReputationService()

	// Setup marketplace
	discoveryService := marketplace.NewDiscoveryService(s.messageBus, s.reputationService)
	auctionService := marketplace.NewAuctionService(s.messageBus)
	s.marketplaceService = marketplace.NewMarketplaceService(
		discoveryService,
		auctionService,
		s.messageBus,
		s.reputationService,
	)

	// Initialize orchestrator with performance testing configuration
	s.orchestrator = orchestration.NewOrchestrator(
		orchestration.Config{
			MaxConcurrentTasks:    1000, // Higher than Sprint 6 to test scalability
			ReputationEnabled:     true,
			VCGEnabled:           true,
			PaymentEnabled:       true,
			CircuitBreakerEnabled: true,
			HealthCheckInterval:   10 * time.Second,
			TaskTimeoutSeconds:    180,
			PerformanceMode:      true, // Enable performance optimizations
		},
		s.messageBus,
		s.paymentService,
		s.reputationService,
	)

	// Allow services to stabilize
	time.Sleep(5 * time.Second)

	fmt.Println("📊 Sprint 7 Performance Regression Test Suite initialized")
	fmt.Printf("🎯 Sprint 6 Baseline: %.1f tasks/sec, P95: %.1fms, Memory: %.1fMB\n",
		s.sprint6Baseline.TasksPerSecond,
		s.sprint6Baseline.LatencyP95Ms,
		s.sprint6Baseline.MemoryUsageMB)
}

func (s *Sprint7RegressionTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
	close(s.loadTestResults)
	s.analyzeRegressionResults()
	s.printRegressionReport()
}

// TestBaselineThroughputRegression tests throughput against Sprint 6 baseline
func (s *Sprint7RegressionTestSuite) TestBaselineThroughputRegression() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("⚡ Testing Throughput Regression vs Sprint 6 Baseline")

	testStart := time.Now()
	testDuration := 60 * time.Second // 1-minute sustained load test

	var completedTasks int64
	var totalLatency int64
	var taskCounter int64

	// Setup agents for load testing
	agents := s.setupLoadTestAgents(10)

	// Execute sustained load test
	endTime := time.Now().Add(testDuration)
	var wg sync.WaitGroup

	// Launch concurrent task submissions
	for i := 0; i < 5; i++ { // 5 concurrent submission workers
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for time.Now().Before(endTime) {
				taskStart := time.Now()

				// Submit task
				userDID := generateRegressionDID("user", fmt.Sprintf("throughput_%d_%d", workerID, atomic.AddInt64(&taskCounter, 1)))
				taskReq := &orchestration.TaskRequest{
					UserDID:      userDID,
					TaskType:     "throughput-test",
					Description:  fmt.Sprintf("Throughput test task %d", taskCounter),
					MaxPayment:   50.0,
					Timeout:      30 * time.Second,
					Requirements: []string{"throughput-test"},
				}

				// Deposit funds
				err := s.paymentService.Deposit(ctx, userDID, 60.0)
				if err != nil {
					continue
				}

				// Submit task
				task, err := s.orchestrator.SubmitTask(ctx, taskReq)
				if err != nil {
					continue
				}

				// Process mini workflow for throughput measurement
				agent := agents[atomic.LoadInt64(&taskCounter)%int64(len(agents))]
				err = s.processRegressionWorkflow(ctx, task, agent)
				if err != nil {
					continue
				}

				// Record completion
				taskLatency := time.Since(taskStart)
				atomic.AddInt64(&completedTasks, 1)
				atomic.AddInt64(&totalLatency, int64(taskLatency.Milliseconds()))

				// Record latency
				s.recordLatency("throughput_test", taskLatency.Milliseconds())

				// Brief pause to avoid overwhelming system
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	actualDuration := time.Since(testStart)
	throughput := float64(completedTasks) / actualDuration.Seconds()
	averageLatency := float64(totalLatency) / float64(completedTasks)

	fmt.Printf("📊 Throughput Results:\n")
	fmt.Printf("  Tasks Completed: %d\n", completedTasks)
	fmt.Printf("  Duration: %v\n", actualDuration)
	fmt.Printf("  Sprint 7 Throughput: %.2f tasks/sec\n", throughput)
	fmt.Printf("  Sprint 6 Baseline: %.2f tasks/sec\n", s.sprint6Baseline.TasksPerSecond)
	fmt.Printf("  Average Latency: %.2fms\n", averageLatency)

	// Calculate regression
	throughputChange := ((throughput - s.sprint6Baseline.TasksPerSecond) / s.sprint6Baseline.TasksPerSecond) * 100

	s.performanceMetrics.TasksPerSecond = throughput
	s.regressionResults.ThroughputChange = throughputChange

	// Verify no significant regression
	assert.GreaterOrEqual(t, throughput, s.sprint6Baseline.TasksPerSecond*(1-s.sprint6Baseline.ThroughputTolerance/100),
		fmt.Sprintf("Throughput should not regress more than %.1f%% (Current: %.2f, Baseline: %.2f)",
			s.sprint6Baseline.ThroughputTolerance, throughput, s.sprint6Baseline.TasksPerSecond))

	if throughputChange < -s.sprint6Baseline.ThroughputTolerance {
		s.regressionResults.ThroughputRegression = true
		fmt.Printf("🚨 THROUGHPUT REGRESSION DETECTED: %.2f%% decrease\n", -throughputChange)
	} else {
		fmt.Printf("✅ Throughput: %.2f%% change (within tolerance)\n", throughputChange)
	}

	atomic.StoreInt64(&s.performanceMetrics.TasksCompleted, completedTasks)
}

// TestLatencyRegression tests latency percentiles against Sprint 6 baseline
func (s *Sprint7RegressionTestSuite) TestLatencyRegression() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("⏱️  Testing Latency Regression vs Sprint 6 Baseline")

	testStart := time.Now()
	numSamples := 200 // Collect 200 latency samples
	var latencies []float64
	var mu sync.Mutex

	agents := s.setupLoadTestAgents(5)

	// Collect latency samples with realistic load
	var wg sync.WaitGroup
	for i := 0; i < numSamples; i++ {
		wg.Add(1)
		go func(sampleID int) {
			defer wg.Done()

			sampleStart := time.Now()

			userDID := generateRegressionDID("user", fmt.Sprintf("latency_%d", sampleID))
			taskReq := &orchestration.TaskRequest{
				UserDID:      userDID,
				TaskType:     "latency-test",
				Description:  fmt.Sprintf("Latency test task %d", sampleID),
				MaxPayment:   40.0,
				Timeout:      25 * time.Second,
				Requirements: []string{"latency-test"},
			}

			// Process complete workflow and measure latency
			err := s.paymentService.Deposit(ctx, userDID, 50.0)
			if err != nil {
				return
			}

			task, err := s.orchestrator.SubmitTask(ctx, taskReq)
			if err != nil {
				return
			}

			agent := agents[sampleID%len(agents)]
			err = s.processRegressionWorkflow(ctx, task, agent)
			if err != nil {
				return
			}

			sampleLatency := time.Since(sampleStart).Milliseconds()

			mu.Lock()
			latencies = append(latencies, float64(sampleLatency))
			mu.Unlock()

			s.recordLatency("latency_test", sampleLatency)
		}(i)

		// Stagger requests to avoid overwhelming system
		if i%10 == 9 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	wg.Wait()

	// Calculate percentiles
	sort.Float64s(latencies)
	percentiles := s.calculateLatencyPercentiles(latencies)

	fmt.Printf("📊 Latency Results:\n")
	fmt.Printf("  Samples: %d\n", len(latencies))
	fmt.Printf("  Sprint 7 P50: %.2fms (baseline: %.2fms)\n", percentiles.P50, s.sprint6Baseline.LatencyP50Ms)
	fmt.Printf("  Sprint 7 P95: %.2fms (baseline: %.2fms)\n", percentiles.P95, s.sprint6Baseline.LatencyP95Ms)
	fmt.Printf("  Sprint 7 P99: %.2fms (baseline: %.2fms)\n", percentiles.P99, s.sprint6Baseline.LatencyP99Ms)

	// Calculate regression
	p95Change := ((percentiles.P95 - s.sprint6Baseline.LatencyP95Ms) / s.sprint6Baseline.LatencyP95Ms) * 100
	p99Change := ((percentiles.P99 - s.sprint6Baseline.LatencyP99Ms) / s.sprint6Baseline.LatencyP99Ms) * 100

	s.performanceMetrics.LatencyP50 = percentiles.P50
	s.performanceMetrics.LatencyP95 = percentiles.P95
	s.performanceMetrics.LatencyP99 = percentiles.P99
	s.regressionResults.LatencyP95Change = p95Change
	s.regressionResults.LatencyP99Change = p99Change

	// Verify no significant latency regression
	maxAcceptableP95 := s.sprint6Baseline.LatencyP95Ms * (1 + s.sprint6Baseline.LatencyTolerancePercent/100)
	assert.LessOrEqual(t, percentiles.P95, maxAcceptableP95,
		fmt.Sprintf("P95 latency should not regress more than %.1f%% (Current: %.2fms, Max acceptable: %.2fms)",
			s.sprint6Baseline.LatencyTolerancePercent, percentiles.P95, maxAcceptableP95))

	if p95Change > s.sprint6Baseline.LatencyTolerancePercent {
		s.regressionResults.LatencyRegression = true
		fmt.Printf("🚨 LATENCY REGRESSION DETECTED: P95 %.2f%% increase\n", p95Change)
	} else {
		fmt.Printf("✅ Latency P95: %.2f%% change (within tolerance)\n", p95Change)
	}

	testDuration := time.Since(testStart)
	fmt.Printf("Latency test completed in %v\n", testDuration)
}

// TestMemoryUsageRegression tests memory usage against Sprint 6 baseline
func (s *Sprint7RegressionTestSuite) TestMemoryUsageRegression() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🧠 Testing Memory Usage Regression vs Sprint 6 Baseline")

	// Record initial memory usage
	runtime.GC() // Force garbage collection for accurate measurement
	time.Sleep(100 * time.Millisecond)

	var initialMemStats, peakMemStats, finalMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)

	agents := s.setupLoadTestAgents(8)

	// Run sustained memory stress test
	var wg sync.WaitGroup
	numTasks := 100
	concurrentWorkers := 10

	memoryTracker := time.NewTicker(500 * time.Millisecond)
	var memoryReadings []int64
	var memoryMu sync.Mutex

	go func() {
		for range memoryTracker.C {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			memoryMu.Lock()
			memoryReadings = append(memoryReadings, int64(memStats.Alloc))
			memoryMu.Unlock()
		}
	}()

	// Execute memory stress workload
	for i := 0; i < concurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for taskID := 0; taskID < numTasks/concurrentWorkers; taskID++ {
				userDID := generateRegressionDID("user", fmt.Sprintf("memory_%d_%d", workerID, taskID))

				// Create larger payloads to test memory usage
				taskReq := &orchestration.TaskRequest{
					UserDID:      userDID,
					TaskType:     "memory-test",
					Description:  fmt.Sprintf("Memory stress task %d-%d with large payload", workerID, taskID),
					MaxPayment:   30.0,
					Timeout:      20 * time.Second,
					Requirements: []string{"memory-test"},
					Metadata:     generateLargeMetadata(1024), // 1KB metadata
				}

				err := s.paymentService.Deposit(ctx, userDID, 40.0)
				if err != nil {
					continue
				}

				task, err := s.orchestrator.SubmitTask(ctx, taskReq)
				if err != nil {
					continue
				}

				agent := agents[taskID%len(agents)]
				err = s.processRegressionWorkflow(ctx, task, agent)
				if err != nil {
					continue
				}

				// Record memory at peak usage
				var currentMemStats runtime.MemStats
				runtime.ReadMemStats(&currentMemStats)
				if currentMemStats.Alloc > peakMemStats.Alloc {
					peakMemStats = currentMemStats
				}
			}
		}(i)
	}

	wg.Wait()
	memoryTracker.Stop()

	// Final memory measurement
	runtime.GC()
	time.Sleep(100 * time.Millisecond)
	runtime.ReadMemStats(&finalMemStats)

	initialMemoryMB := float64(initialMemStats.Alloc) / 1024 / 1024
	peakMemoryMB := float64(peakMemStats.Alloc) / 1024 / 1024
	finalMemoryMB := float64(finalMemStats.Alloc) / 1024 / 1024

	// Calculate average memory during test
	memoryMu.Lock()
	var averageMemory float64
	if len(memoryReadings) > 0 {
		var total int64
		for _, reading := range memoryReadings {
			total += reading
		}
		averageMemory = float64(total) / float64(len(memoryReadings)) / 1024 / 1024
	}
	memoryMu.Unlock()

	fmt.Printf("📊 Memory Usage Results:\n")
	fmt.Printf("  Initial: %.2f MB\n", initialMemoryMB)
	fmt.Printf("  Peak: %.2f MB\n", peakMemoryMB)
	fmt.Printf("  Average: %.2f MB\n", averageMemory)
	fmt.Printf("  Final: %.2f MB\n", finalMemoryMB)
	fmt.Printf("  Sprint 6 Baseline: %.2f MB\n", s.sprint6Baseline.MemoryUsageMB)

	// Calculate memory regression
	memoryChange := ((peakMemoryMB - s.sprint6Baseline.MemoryUsageMB) / s.sprint6Baseline.MemoryUsageMB) * 100

	s.performanceMetrics.MemoryUsageBytes = append(s.performanceMetrics.MemoryUsageBytes, peakMemStats.Alloc)
	s.regressionResults.MemoryUsageChange = memoryChange

	// Verify no significant memory regression
	maxAcceptableMemory := s.sprint6Baseline.MemoryUsageMB * (1 + s.sprint6Baseline.MemoryTolerancePercent/100)
	assert.LessOrEqual(t, peakMemoryMB, maxAcceptableMemory,
		fmt.Sprintf("Memory usage should not regress more than %.1f%% (Current: %.2fMB, Max acceptable: %.2fMB)",
			s.sprint6Baseline.MemoryTolerancePercent, peakMemoryMB, maxAcceptableMemory))

	if memoryChange > s.sprint6Baseline.MemoryTolerancePercent {
		s.regressionResults.MemoryRegression = true
		fmt.Printf("🚨 MEMORY REGRESSION DETECTED: %.2f%% increase\n", memoryChange)
	} else {
		fmt.Printf("✅ Memory usage: %.2f%% change (within tolerance)\n", memoryChange)
	}

	// Check for memory leaks
	memoryGrowth := finalMemoryMB - initialMemoryMB
	if memoryGrowth > 50 { // More than 50MB growth after GC suggests leak
		fmt.Printf("⚠️  Potential memory leak detected: %.2fMB growth\n", memoryGrowth)
	}
}

// TestErrorRateRegression tests error rates against Sprint 6 baseline
func (s *Sprint7RegressionTestSuite) TestErrorRateRegression() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚨 Testing Error Rate Regression vs Sprint 6 Baseline")

	testStart := time.Now()
	var totalOperations int64
	var errorCount int64

	agents := s.setupLoadTestAgents(6)

	// Run error rate test with various failure scenarios
	numOperations := 500
	var wg sync.WaitGroup

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			atomic.AddInt64(&totalOperations, 1)

			userDID := generateRegressionDID("user", fmt.Sprintf("error_%d", opID))

			// Introduce various failure scenarios for realistic testing
			var shouldFail bool
			failureType := opID % 10

			switch failureType {
			case 0: // Invalid user DID
				userDID = "invalid-did-format"
				shouldFail = true
			case 1: // Insufficient funds
				err := s.paymentService.Deposit(ctx, userDID, 5.0) // Too little for task
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
				shouldFail = true
			case 2: // Timeout scenario
				// This would be handled by context timeouts in real scenarios
			default:
				// Normal operation
				err := s.paymentService.Deposit(ctx, userDID, 50.0)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
			}

			taskReq := &orchestration.TaskRequest{
				UserDID:      userDID,
				TaskType:     "error-test",
				Description:  fmt.Sprintf("Error test operation %d", opID),
				MaxPayment:   40.0,
				Timeout:      15 * time.Second,
				Requirements: []string{"error-test"},
			}

			task, err := s.orchestrator.SubmitTask(ctx, taskReq)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				if !shouldFail {
					s.recordError("task_submission", err.Error())
				}
				return
			}

			if shouldFail && failureType == 1 { // Insufficient funds case
				// Try to process workflow, should fail
				agent := agents[opID%len(agents)]
				err = s.processRegressionWorkflow(ctx, task, agent)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
				}
			} else if !shouldFail {
				// Normal processing
				agent := agents[opID%len(agents)]
				err = s.processRegressionWorkflow(ctx, task, agent)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					s.recordError("workflow_processing", err.Error())
				}
			}
		}(i)

		// Rate limit to avoid overwhelming system
		if i%20 == 19 {
			time.Sleep(50 * time.Millisecond)
		}
	}

	wg.Wait()

	// Calculate error rate
	actualErrorRate := float64(errorCount) / float64(totalOperations) * 100
	testDuration := time.Since(testStart)

	fmt.Printf("📊 Error Rate Results:\n")
	fmt.Printf("  Total Operations: %d\n", totalOperations)
	fmt.Printf("  Errors: %d\n", errorCount)
	fmt.Printf("  Sprint 7 Error Rate: %.3f%%\n", actualErrorRate)
	fmt.Printf("  Sprint 6 Baseline: %.3f%%\n", s.sprint6Baseline.ErrorRatePercent)
	fmt.Printf("  Test Duration: %v\n", testDuration)

	// Calculate regression
	errorRateChange := actualErrorRate - s.sprint6Baseline.ErrorRatePercent

	s.performanceMetrics.TotalErrors = errorCount
	s.performanceMetrics.ErrorRate = actualErrorRate
	s.regressionResults.ErrorRateChange = errorRateChange

	// Verify error rate is within acceptable limits
	assert.LessOrEqual(t, actualErrorRate, s.sprint6Baseline.ErrorRateThresholdPercent,
		fmt.Sprintf("Error rate should not exceed %.3f%% (Current: %.3f%%)",
			s.sprint6Baseline.ErrorRateThresholdPercent, actualErrorRate))

	if actualErrorRate > s.sprint6Baseline.ErrorRateThresholdPercent {
		s.regressionResults.ErrorRateRegression = true
		fmt.Printf("🚨 ERROR RATE REGRESSION DETECTED: %.3f%% (threshold: %.3f%%)\n",
			actualErrorRate, s.sprint6Baseline.ErrorRateThresholdPercent)
	} else {
		fmt.Printf("✅ Error rate: %.3f%% (within threshold)\n", actualErrorRate)
	}
}

// TestConcurrentLoadRegression tests concurrent load handling against Sprint 6 baseline
func (s *Sprint7RegressionTestSuite) TestConcurrentLoadRegression() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🔀 Testing Concurrent Load Regression vs Sprint 6 Baseline")

	testStart := time.Now()

	agents := s.setupLoadTestAgents(15)

	// Test increasing levels of concurrency
	concurrencyLevels := []int{10, 25, 50, 75, 100}
	results := make(map[int]LoadTestResult)

	for _, concurrency := range concurrencyLevels {
		fmt.Printf("Testing concurrency level: %d\n", concurrency)

		levelStart := time.Now()
		var completedTasks int64
		var errorCount int64

		// Increase network load simulation for higher concurrency
		networkLoad := float64(concurrency) / 100.0
		s.messageBus.SetNetworkLoad(networkLoad)

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(taskID int) {
				defer wg.Done()

				userDID := generateRegressionDID("user", fmt.Sprintf("concurrent_%d_%d", concurrency, taskID))

				err := s.paymentService.Deposit(ctx, userDID, 50.0)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}

				taskReq := &orchestration.TaskRequest{
					UserDID:      userDID,
					TaskType:     "concurrent-test",
					Description:  fmt.Sprintf("Concurrent test %d-%d", concurrency, taskID),
					MaxPayment:   35.0,
					Timeout:      20 * time.Second,
					Requirements: []string{"concurrent-test"},
				}

				task, err := s.orchestrator.SubmitTask(ctx, taskReq)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}

				agent := agents[taskID%len(agents)]
				err = s.processRegressionWorkflow(ctx, task, agent)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}

				atomic.AddInt64(&completedTasks, 1)
			}(i)
		}

		wg.Wait()
		levelDuration := time.Since(levelStart)
		tasksPerSecond := float64(completedTasks) / levelDuration.Seconds()

		// Record memory usage at this concurrency level
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		memoryMB := float64(memStats.Alloc) / 1024 / 1024

		result := LoadTestResult{
			TestName:       fmt.Sprintf("Concurrency_%d", concurrency),
			Duration:       levelDuration,
			TasksCompleted: completedTasks,
			TasksPerSecond: tasksPerSecond,
			MemoryUsageMB:  memoryMB,
			ErrorCount:     errorCount,
			Success:        completedTasks > int64(concurrency)*8/10, // 80% success rate
		}

		results[concurrency] = result

		fmt.Printf("  Concurrency %d: %.2f tasks/sec, %.2fMB memory, %d errors\n",
			concurrency, tasksPerSecond, memoryMB, errorCount)

		// Brief pause between concurrency levels
		time.Sleep(500 * time.Millisecond)
	}

	// Analyze concurrent performance against baseline
	maxConcurrency := s.sprint6Baseline.MaxConcurrent
	maxResult, exists := results[int(maxConcurrency)]

	if exists {
		fmt.Printf("\n📊 Sprint 6 Baseline Concurrency (%d):\n", int(maxConcurrency))
		fmt.Printf("  Tasks/sec: %.2f\n", maxResult.TasksPerSecond)
		fmt.Printf("  Memory: %.2f MB\n", maxResult.MemoryUsageMB)
		fmt.Printf("  Errors: %d\n", maxResult.ErrorCount)
		fmt.Printf("  Success: %t\n", maxResult.Success)

		assert.True(t, maxResult.Success, "Should handle Sprint 6 baseline concurrency level")
	}

	// Test higher concurrency (Sprint 7 improvement target)
	if higherResult, exists := results[100]; exists {
		fmt.Printf("\nSprint 7 Enhanced Concurrency (100):\n")
		fmt.Printf("  Tasks/sec: %.2f\n", higherResult.TasksPerSecond)
		fmt.Printf("  Memory: %.2f MB\n", higherResult.MemoryUsageMB)
		fmt.Printf("  Errors: %d\n", higherResult.ErrorCount)
		fmt.Printf("  Success: %t\n", higherResult.Success)

		// Sprint 7 should handle higher concurrency than Sprint 6
		if higherResult.Success {
			fmt.Printf("✅ Sprint 7 successfully handles higher concurrency than Sprint 6\n")
			s.performanceMetrics.MaxConcurrentTasks = 100
		}
	}

	totalTestDuration := time.Since(testStart)
	fmt.Printf("Total concurrent load test duration: %v\n", totalTestDuration)
}

// Helper methods

func (s *Sprint7RegressionTestSuite) setupLoadTestAgents(count int) []string {
	agents := make([]string, count)
	discoveryService := s.marketplaceService.GetDiscoveryService()

	for i := 0; i < count; i++ {
		agentDID := generateRegressionDID("agent", fmt.Sprintf("load_test_%d", i))
		agents[i] = agentDID

		// Register agent
		agentCard := &identity.AgentCard{
			DID:          agentDID,
			Name:         fmt.Sprintf("Load Test Agent %d", i),
			Capabilities: []string{"throughput-test", "latency-test", "memory-test", "error-test", "concurrent-test"},
			Reputation:   75.0 + float64(i%20), // Varying reputation 75-95
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		err := discoveryService.RegisterAgent(s.ctx, agentCard)
		if err != nil {
			fmt.Printf("Warning: Failed to register agent %s: %v\n", agentDID, err)
		}
	}

	return agents
}

func (s *Sprint7RegressionTestSuite) processRegressionWorkflow(ctx context.Context, task *orchestration.Task, agentDID string) error {
	escrowAmount := uint64(task.MaxPayment * 1_000_000)

	// Create escrow
	err := s.escrowClient.CreateEscrow(ctx, task.ID, escrowAmount, task.ID, nil)
	if err != nil {
		return err
	}

	// Accept task
	err = s.escrowClient.AcceptTask(ctx, task.ID, agentDID)
	if err != nil {
		return err
	}

	// Simulate brief execution
	time.Sleep(time.Duration(10+rand.Intn(40)) * time.Millisecond) // 10-50ms execution

	// Release payment
	return s.escrowClient.ReleasePayment(ctx, task.ID)
}

func (s *Sprint7RegressionTestSuite) recordLatency(operation string, latencyMs int64) {
	s.performanceMetrics.mu.Lock()
	defer s.performanceMetrics.mu.Unlock()

	latency := float64(latencyMs)

	switch operation {
	case "throughput_test":
		s.performanceMetrics.EndToEndLatencies = append(s.performanceMetrics.EndToEndLatencies, latency)
	case "latency_test":
		s.performanceMetrics.EndToEndLatencies = append(s.performanceMetrics.EndToEndLatencies, latency)
	default:
		s.performanceMetrics.EndToEndLatencies = append(s.performanceMetrics.EndToEndLatencies, latency)
	}
}

func (s *Sprint7RegressionTestSuite) recordError(component, message string) {
	atomic.AddInt64(&s.performanceMetrics.TotalErrors, 1)

	switch component {
	case "task_submission":
		atomic.AddInt64(&s.performanceMetrics.ValidationErrors, 1)
	case "workflow_processing":
		atomic.AddInt64(&s.performanceMetrics.NetworkErrors, 1)
	default:
		// General error
	}
}

type LatencyPercentiles struct {
	P50   float64
	P95   float64
	P99   float64
	P99_9 float64
}

func (s *Sprint7RegressionTestSuite) calculateLatencyPercentiles(latencies []float64) LatencyPercentiles {
	if len(latencies) == 0 {
		return LatencyPercentiles{}
	}

	n := len(latencies)
	return LatencyPercentiles{
		P50:   latencies[n*50/100],
		P95:   latencies[n*95/100],
		P99:   latencies[n*99/100],
		P99_9: latencies[n*999/1000],
	}
}

func (s *Sprint7RegressionTestSuite) analyzeRegressionResults() {
	// Calculate overall performance score
	score := 100.0

	// Throughput impact (25% weight)
	if s.regressionResults.ThroughputChange < 0 {
		score -= math.Abs(s.regressionResults.ThroughputChange) * 0.25 * 4 // 4x penalty for throughput loss
	} else {
		score += s.regressionResults.ThroughputChange * 0.25 * 0.5 // Half benefit for throughput gains
	}

	// Latency impact (30% weight)
	if s.regressionResults.LatencyP95Change > 0 {
		score -= s.regressionResults.LatencyP95Change * 0.30 * 3 // 3x penalty for latency increase
	} else {
		score += math.Abs(s.regressionResults.LatencyP95Change) * 0.30 * 0.5 // Half benefit for latency improvements
	}

	// Memory impact (20% weight)
	if s.regressionResults.MemoryUsageChange > 0 {
		score -= s.regressionResults.MemoryUsageChange * 0.20 * 2 // 2x penalty for memory increase
	}

	// Error rate impact (25% weight)
	if s.regressionResults.ErrorRateChange > 0 {
		score -= s.regressionResults.ErrorRateChange * 0.25 * 10 // 10x penalty for error rate increase
	}

	// Cap score between 0 and 100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	s.regressionResults.PerformanceScore = score

	// Determine regression severity
	if score >= 95 {
		s.regressionResults.RegressionSeverity = "None"
	} else if score >= 85 {
		s.regressionResults.RegressionSeverity = "Minor"
	} else if score >= 70 {
		s.regressionResults.RegressionSeverity = "Moderate"
	} else {
		s.regressionResults.RegressionSeverity = "Severe"
	}

	// Overall regression flag
	s.regressionResults.OverallRegression = s.regressionResults.ThroughputRegression ||
		s.regressionResults.LatencyRegression ||
		s.regressionResults.MemoryRegression ||
		s.regressionResults.ErrorRateRegression

	// Generate regression impacts
	if s.regressionResults.ThroughputRegression {
		impact := RegressionImpact{
			Component:     "Orchestrator",
			Metric:        "Throughput",
			BaselineValue: s.sprint6Baseline.TasksPerSecond,
			CurrentValue:  s.performanceMetrics.TasksPerSecond,
			ChangePercent: s.regressionResults.ThroughputChange,
			Severity:      "High",
			Impact:        fmt.Sprintf("%.2f%% decrease in task processing throughput", -s.regressionResults.ThroughputChange),
		}
		s.regressionResults.RegressionImpacts = append(s.regressionResults.RegressionImpacts, impact)
	}

	if s.regressionResults.LatencyRegression {
		impact := RegressionImpact{
			Component:     "System",
			Metric:        "Latency P95",
			BaselineValue: s.sprint6Baseline.LatencyP95Ms,
			CurrentValue:  s.performanceMetrics.LatencyP95,
			ChangePercent: s.regressionResults.LatencyP95Change,
			Severity:      "Medium",
			Impact:        fmt.Sprintf("%.2f%% increase in P95 latency", s.regressionResults.LatencyP95Change),
		}
		s.regressionResults.RegressionImpacts = append(s.regressionResults.RegressionImpacts, impact)
	}

	// Generate recommended actions
	if s.regressionResults.OverallRegression {
		if s.regressionResults.ThroughputRegression {
			s.regressionResults.RecommendedActions = append(s.regressionResults.RecommendedActions,
				"Profile orchestrator for bottlenecks and optimize task processing pipeline")
		}
		if s.regressionResults.LatencyRegression {
			s.regressionResults.RecommendedActions = append(s.regressionResults.RecommendedActions,
				"Analyze network and blockchain latency, implement caching where appropriate")
		}
		if s.regressionResults.MemoryRegression {
			s.regressionResults.RecommendedActions = append(s.regressionResults.RecommendedActions,
				"Review memory allocation patterns and implement object pooling")
		}
	} else {
		s.regressionResults.RecommendedActions = append(s.regressionResults.RecommendedActions,
			"Performance meets or exceeds Sprint 6 baseline - continue monitoring")
	}
}

func (s *Sprint7RegressionTestSuite) printRegressionReport() {
	duration := time.Since(s.performanceMetrics.StartTime)

	fmt.Printf("\n📊 SPRINT 7 PERFORMANCE REGRESSION REPORT\n")
	fmt.Printf("==========================================\n")
	fmt.Printf("Test Duration: %v\n", duration)
	fmt.Printf("Performance Score: %.1f/100\n", s.regressionResults.PerformanceScore)
	fmt.Printf("Regression Severity: %s\n", s.regressionResults.RegressionSeverity)

	fmt.Printf("\n🎯 Performance Comparison vs Sprint 6:\n")
	fmt.Printf("┌─────────────────┬──────────────┬──────────────┬─────────────┬──────────────┐\n")
	fmt.Printf("│     Metric      │   Sprint 6   │   Sprint 7   │   Change    │   Status     │\n")
	fmt.Printf("├─────────────────┼──────────────┼──────────────┼─────────────┼──────────────┤\n")

	// Throughput
	throughputStatus := "✅ PASS"
	if s.regressionResults.ThroughputRegression {
		throughputStatus = "🚨 FAIL"
	}
	fmt.Printf("│ Throughput/sec  │   %8.2f   │   %8.2f   │   %+6.2f%%  │ %s │\n",
		s.sprint6Baseline.TasksPerSecond, s.performanceMetrics.TasksPerSecond,
		s.regressionResults.ThroughputChange, throughputStatus)

	// Latency P95
	latencyStatus := "✅ PASS"
	if s.regressionResults.LatencyRegression {
		latencyStatus = "🚨 FAIL"
	}
	fmt.Printf("│ Latency P95 ms  │   %8.2f   │   %8.2f   │   %+6.2f%%  │ %s │\n",
		s.sprint6Baseline.LatencyP95Ms, s.performanceMetrics.LatencyP95,
		s.regressionResults.LatencyP95Change, latencyStatus)

	// Memory
	memoryStatus := "✅ PASS"
	if s.regressionResults.MemoryRegression {
		memoryStatus = "🚨 FAIL"
	}
	currentMemoryMB := float64(0)
	if len(s.performanceMetrics.MemoryUsageBytes) > 0 {
		currentMemoryMB = float64(s.performanceMetrics.MemoryUsageBytes[0]) / 1024 / 1024
	}
	fmt.Printf("│ Memory MB       │   %8.2f   │   %8.2f   │   %+6.2f%%  │ %s │\n",
		s.sprint6Baseline.MemoryUsageMB, currentMemoryMB,
		s.regressionResults.MemoryUsageChange, memoryStatus)

	// Error Rate
	errorStatus := "✅ PASS"
	if s.regressionResults.ErrorRateRegression {
		errorStatus = "🚨 FAIL"
	}
	fmt.Printf("│ Error Rate %%    │   %8.3f   │   %8.3f   │   %+6.3f   │ %s │\n",
		s.sprint6Baseline.ErrorRatePercent, s.performanceMetrics.ErrorRate,
		s.regressionResults.ErrorRateChange, errorStatus)

	fmt.Printf("└─────────────────┴──────────────┴──────────────┴─────────────┴──────────────┘\n")

	// Regression impacts
	if len(s.regressionResults.RegressionImpacts) > 0 {
		fmt.Printf("\n🚨 REGRESSION IMPACTS:\n")
		for i, impact := range s.regressionResults.RegressionImpacts {
			fmt.Printf("%d. %s - %s: %s\n", i+1, impact.Component, impact.Metric, impact.Impact)
		}
	}

	// Recommended actions
	if len(s.regressionResults.RecommendedActions) > 0 {
		fmt.Printf("\n💡 RECOMMENDED ACTIONS:\n")
		for i, action := range s.regressionResults.RecommendedActions {
			fmt.Printf("%d. %s\n", i+1, action)
		}
	}

	// Overall verdict
	fmt.Printf("\n🎯 OVERALL VERDICT:\n")
	if !s.regressionResults.OverallRegression {
		fmt.Printf("✅ Sprint 7 PASSES performance regression tests\n")
		fmt.Printf("   Performance meets or exceeds Sprint 6 baseline\n")
	} else {
		fmt.Printf("🚨 Sprint 7 has performance regressions that need attention\n")
		fmt.Printf("   Severity: %s (Score: %.1f/100)\n", s.regressionResults.RegressionSeverity, s.regressionResults.PerformanceScore)
	}
}

// Utility functions

func generateRegressionDID(entityType, suffix string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("did:zerostate:%s:regression_%s_%d", entityType, suffix, timestamp)
}

func generateLargeMetadata(sizeKB int) map[string]string {
	metadata := make(map[string]string)

	// Generate roughly sizeKB of metadata
	valueSize := sizeKB * 1024 / 10 // Distribute across 10 keys
	largeValue := make([]byte, valueSize)
	for i := range largeValue {
		largeValue[i] = byte('a' + (i % 26))
	}

	for i := 0; i < 10; i++ {
		metadata[fmt.Sprintf("large_key_%d", i)] = string(largeValue)
	}

	return metadata
}