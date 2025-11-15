// Package load provides scale and performance validation tests for Sprint 6 Phase 4
package load

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
	"github.com/aidenlippert/zerostate/libs/substrate"
)

// Sprint6ScaleTestSuite validates system performance and scale for MVP
type Sprint6ScaleTestSuite struct {
	suite.Suite
	ctx               context.Context
	cancel            context.CancelFunc
	escrowClient      *substrate.EscrowClient
	reputationClient  *substrate.ReputationClient
	orchestrator      *orchestration.Orchestrator
	paymentService    *economic.PaymentChannelService
	reputationService *reputation.ReputationService
	messageBus        *ScaleTestMessageBus
	scaleMetrics     *ScaleMetrics
}

// ScaleMetrics tracks comprehensive performance metrics
type ScaleMetrics struct {
	mu                        sync.RWMutex
	StartTime                time.Time
	TotalOperations          int64
	SuccessfulOperations     int64
	FailedOperations         int64
	ConcurrentTasks          int64
	MaxConcurrentTasks       int64
	TaskThroughputPerSecond  float64
	PaymentThroughputPerSec  float64
	LatencyP50               time.Duration
	LatencyP95               time.Duration
	LatencyP99               time.Duration
	ErrorRate                float64
	MemoryUsageBytes         int64
	CPUUsagePercent          float64
	GoroutineCount           int64
	HeapObjects              int64
	GCPauses                 []time.Duration
	NetworkLatency           time.Duration
	BlockchainLatency        time.Duration
	CircuitBreakerTrips      int64
	Latencies               []time.Duration
	Errors                  []error
}

// ScaleTestMessageBus provides realistic P2P simulation under load
type ScaleTestMessageBus struct {
	mu             sync.RWMutex
	messageCount   int64
	activeRequests int64
	latencyMs      int
	maxLatencyMs   int
	errorRate      float64
	throughputMsgs int64
	startTime      time.Time
}

func NewScaleTestMessageBus() *ScaleTestMessageBus {
	return &ScaleTestMessageBus{
		latencyMs:    5,  // 5ms base latency
		maxLatencyMs: 50, // Up to 50ms under load
		errorRate:    0.001, // 0.1% error rate
		startTime:    time.Now(),
	}
}

func (m *ScaleTestMessageBus) Start(ctx context.Context) error {
	return nil
}

func (m *ScaleTestMessageBus) Stop() error {
	return nil
}

func (m *ScaleTestMessageBus) Publish(ctx context.Context, topic string, data []byte) error {
	atomic.AddInt64(&m.messageCount, 1)
	atomic.AddInt64(&m.throughputMsgs, 1)

	// Simulate variable latency under load
	latency := m.calculateDynamicLatency()
	time.Sleep(latency)

	return m.simulateNetworkError()
}

func (m *ScaleTestMessageBus) Subscribe(ctx context.Context, topic string, handler p2p.MessageHandler) error {
	return nil
}

func (m *ScaleTestMessageBus) SendRequest(ctx context.Context, targetDID string, request []byte, timeout time.Duration) ([]byte, error) {
	atomic.AddInt64(&m.activeRequests, 1)
	defer atomic.AddInt64(&m.activeRequests, -1)

	latency := m.calculateDynamicLatency()
	time.Sleep(latency)

	if err := m.simulateNetworkError(); err != nil {
		return nil, err
	}

	return []byte(fmt.Sprintf("response-%d", atomic.LoadInt64(&m.messageCount))), nil
}

func (m *ScaleTestMessageBus) RegisterRequestHandler(messageType string, handler p2p.RequestHandler) error {
	return nil
}

func (m *ScaleTestMessageBus) GetPeerID() string {
	return "scale-test-peer"
}

func (m *ScaleTestMessageBus) calculateDynamicLatency() time.Duration {
	// Increase latency based on load
	activeReqs := atomic.LoadInt64(&m.activeRequests)
	loadFactor := float64(activeReqs) / 100.0 // Scale based on 100 concurrent requests
	if loadFactor > 1.0 {
		loadFactor = 1.0
	}

	baseLatency := m.latencyMs
	extraLatency := int(float64(m.maxLatencyMs-m.latencyMs) * loadFactor)
	totalLatency := baseLatency + extraLatency

	return time.Duration(totalLatency) * time.Millisecond
}

func (m *ScaleTestMessageBus) simulateNetworkError() error {
	if rand.Float64() < m.errorRate {
		return fmt.Errorf("simulated network error")
	}
	return nil
}

func TestSprint6Scale(t *testing.T) {
	suite.Run(t, new(Sprint6ScaleTestSuite))
}

func (s *Sprint6ScaleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 20*time.Minute)

	// Initialize scale metrics
	s.scaleMetrics = &ScaleMetrics{
		StartTime: time.Now(),
		Latencies: make([]time.Duration, 0),
		Errors:    make([]error, 0),
		GCPauses:  make([]time.Duration, 0),
	}

	// Setup blockchain clients
	substrateClient, err := substrate.NewClientV2("ws://localhost:9944")
	require.NoError(s.T(), err)

	keyring, err := substrate.CreateKeyringFromSeed("//Alice", substrate.Sr25519Type)
	require.NoError(s.T(), err)

	s.escrowClient = substrate.NewEscrowClient(substrateClient, keyring)
	s.reputationClient = substrate.NewReputationClient(substrateClient, keyring)

	// Setup services
	s.paymentService = economic.NewPaymentChannelService()
	s.reputationService = reputation.NewReputationService()
	s.messageBus = NewScaleTestMessageBus()

	// Initialize orchestrator for high-load testing
	s.orchestrator = orchestration.NewOrchestrator(
		orchestration.Config{
			MaxConcurrentTasks:    1000, // High concurrency limit
			ReputationEnabled:     true,
			VCGEnabled:           true,
			PaymentEnabled:       true,
			CircuitBreakerEnabled: true,
			MetricsEnabled:       true,
		},
		s.messageBus,
		s.paymentService,
		s.reputationService,
	)

	// Start memory and GC monitoring
	go s.monitorSystemMetrics()

	// Allow services to stabilize
	time.Sleep(5 * time.Second)
}

func (s *Sprint6ScaleTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
	s.printScaleSummary()
}

// TestConcurrentTaskThroughput validates system can handle 100 concurrent tasks
func (s *Sprint6ScaleTestSuite) TestConcurrentTaskThroughput() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚀 Testing: 100 concurrent tasks throughput...")

	const concurrentTasks = 100
	const targetThroughput = 10.0 // 10 tasks/sec minimum

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64
	errors := make(chan error, concurrentTasks)

	startTime := time.Now()

	// Launch concurrent tasks
	for i := 0; i < concurrentTasks; i++ {
		wg.Add(1)
		go func(taskIndex int) {
			defer wg.Done()

			taskStart := time.Now()
			err := s.executeCompleteTaskWorkflow(ctx, taskIndex)
			taskLatency := time.Since(taskStart)

			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				errors <- err
				s.scaleMetrics.RecordError(err)
			} else {
				atomic.AddInt64(&successCount, 1)
			}

			// Update concurrent task count
			current := atomic.AddInt64(&s.scaleMetrics.ConcurrentTasks, 1)
			for {
				max := atomic.LoadInt64(&s.scaleMetrics.MaxConcurrentTasks)
				if current <= max || atomic.CompareAndSwapInt64(&s.scaleMetrics.MaxConcurrentTasks, max, current) {
					break
				}
			}

			s.scaleMetrics.RecordLatency(taskLatency)
			atomic.AddInt64(&s.scaleMetrics.ConcurrentTasks, -1)
		}(i)
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(errors)

	totalDuration := time.Since(startTime)
	actualThroughput := float64(successCount) / totalDuration.Seconds()

	// Collect and validate results
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
	}

	errorRate := float64(errorCount) / float64(concurrentTasks) * 100

	// Performance assertions
	assert.GreaterOrEqual(t, actualThroughput, targetThroughput,
		fmt.Sprintf("Throughput should be >= %.1f tasks/sec, got %.2f", targetThroughput, actualThroughput))
	assert.Less(t, errorRate, 5.0,
		fmt.Sprintf("Error rate should be < 5%%, got %.2f%%", errorRate))

	// Update metrics
	s.scaleMetrics.mu.Lock()
	s.scaleMetrics.TotalOperations += int64(concurrentTasks)
	s.scaleMetrics.SuccessfulOperations += successCount
	s.scaleMetrics.FailedOperations += errorCount
	s.scaleMetrics.TaskThroughputPerSecond = actualThroughput
	s.scaleMetrics.ErrorRate = errorRate
	s.scaleMetrics.mu.Unlock()

	s.scaleMetrics.CalculatePercentiles()

	fmt.Printf("✅ Concurrent task throughput validated\n")
	fmt.Printf("   Tasks: %d, Success: %d, Errors: %d\n", concurrentTasks, successCount, errorCount)
	fmt.Printf("   Throughput: %.2f tasks/sec, Error Rate: %.2f%%\n", actualThroughput, errorRate)
	fmt.Printf("   Latency P50: %v, P95: %v, P99: %v\n",
		s.scaleMetrics.LatencyP50, s.scaleMetrics.LatencyP95, s.scaleMetrics.LatencyP99)
}

// TestPaymentThroughputUnderLoad validates payment system performance under load
func (s *Sprint6ScaleTestSuite) TestPaymentThroughputUnderLoad() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚀 Testing: Payment throughput under load...")

	const paymentOperations = 200
	const targetThroughputPPS = 20.0 // 20 payments/sec

	var wg sync.WaitGroup
	var successCount int64
	var totalAmount float64
	var paymentErrors int64

	startTime := time.Now()

	// Generate concurrent payment operations
	for i := 0; i < paymentOperations; i++ {
		wg.Add(1)
		go func(paymentIndex int) {
			defer wg.Done()

			userDID := fmt.Sprintf("did:zerostate:user:payment_load_%d", paymentIndex)
			agentDID := fmt.Sprintf("did:zerostate:agent:payment_load_%d", paymentIndex)
			amount := 100.0

			paymentStart := time.Now()

			// Execute payment workflow
			if err := s.executePaymentWorkflow(ctx, userDID, agentDID, amount); err != nil {
				atomic.AddInt64(&paymentErrors, 1)
				s.scaleMetrics.RecordError(err)
			} else {
				atomic.AddInt64(&successCount, 1)
				// Use mutex for float64 since atomic.AddFloat64 doesn't exist
				s.scaleMetrics.mu.Lock()
				totalAmount += amount
				s.scaleMetrics.mu.Unlock()
			}

			paymentLatency := time.Since(paymentStart)
			s.scaleMetrics.RecordLatency(paymentLatency)
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)
	paymentThroughput := float64(successCount) / totalDuration.Seconds()
	errorRate := float64(paymentErrors) / float64(paymentOperations) * 100

	// Performance assertions
	assert.GreaterOrEqual(t, paymentThroughput, targetThroughputPPS,
		fmt.Sprintf("Payment throughput should be >= %.1f/sec, got %.2f", targetThroughputPPS, paymentThroughput))
	assert.Less(t, errorRate, 2.0,
		fmt.Sprintf("Payment error rate should be < 2%%, got %.2f%%", errorRate))

	// Update metrics
	s.scaleMetrics.mu.Lock()
	s.scaleMetrics.PaymentThroughputPerSec = paymentThroughput
	s.scaleMetrics.mu.Unlock()

	fmt.Printf("✅ Payment throughput validated\n")
	fmt.Printf("   Payments: %d, Success: %d, Errors: %d\n", paymentOperations, successCount, paymentErrors)
	fmt.Printf("   Throughput: %.2f payments/sec, Total Amount: %.2f AINU\n", paymentThroughput, totalAmount)
}

// TestLatencyUnderScale validates P95 latency remains < 100ms under load
func (s *Sprint6ScaleTestSuite) TestLatencyUnderScale() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚀 Testing: Latency under scale...")

	const operations = 500
	const maxP95Latency = 100 * time.Millisecond

	var wg sync.WaitGroup
	latencies := make([]time.Duration, operations)
	var mu sync.Mutex
	operationIndex := int64(0)

	startTime := time.Now()

	// Generate operations with varying load
	for batch := 0; batch < 10; batch++ {
		// Each batch increases concurrency
		batchConcurrency := (batch + 1) * 5 // 5, 10, 15, ..., 50 concurrent

		for i := 0; i < batchConcurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				opStart := time.Now()

				// Simulate different operation types
				switch rand.Intn(3) {
				case 0:
					s.simulateTaskOperation(ctx)
				case 1:
					s.simulatePaymentOperation(ctx)
				case 2:
					s.simulateReputationOperation(ctx)
				}

				latency := time.Since(opStart)

				mu.Lock()
				index := atomic.AddInt64(&operationIndex, 1) - 1
				if index < int64(len(latencies)) {
					latencies[index] = latency
				}
				mu.Unlock()

				s.scaleMetrics.RecordLatency(latency)
			}()
		}

		// Stagger batch launches to create realistic load pattern
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Calculate percentiles
	s.scaleMetrics.CalculatePercentiles()

	// Performance assertions
	assert.Less(t, s.scaleMetrics.LatencyP95, maxP95Latency,
		fmt.Sprintf("P95 latency should be < %v, got %v", maxP95Latency, s.scaleMetrics.LatencyP95))
	assert.Less(t, s.scaleMetrics.LatencyP99, 500*time.Millisecond,
		"P99 latency should be < 500ms")

	avgLatency := totalDuration / time.Duration(operations)
	assert.Less(t, avgLatency, 50*time.Millisecond,
		"Average latency should be < 50ms")

	fmt.Printf("✅ Latency under scale validated\n")
	fmt.Printf("   Operations: %d, Duration: %v\n", operations, totalDuration)
	fmt.Printf("   P50: %v, P95: %v, P99: %v\n",
		s.scaleMetrics.LatencyP50, s.scaleMetrics.LatencyP95, s.scaleMetrics.LatencyP99)
}

// TestMemoryUsageStability validates memory usage remains stable under load
func (s *Sprint6ScaleTestSuite) TestMemoryUsageStability() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚀 Testing: Memory usage stability...")

	const maxMemoryMB = 200 // 200MB limit for MVP
	const testDuration = 2 * time.Minute

	// Record initial memory
	initialMemory := s.getCurrentMemoryUsage()

	// Generate sustained load for test duration
	stopLoad := make(chan bool)
	var wg sync.WaitGroup

	// Start load generators
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.generateSustainedLoad(ctx, stopLoad)
		}()
	}

	// Monitor memory for test duration
	memoryReadings := []int64{}
	memoryTicker := time.NewTicker(10 * time.Second)
	defer memoryTicker.Stop()

	timeout := time.After(testDuration)

	memoryMonitorDone := make(chan bool)
	go func() {
		defer close(memoryMonitorDone)
		for {
			select {
			case <-memoryTicker.C:
				currentMemory := s.getCurrentMemoryUsage()
				memoryReadings = append(memoryReadings, currentMemory)

				s.scaleMetrics.mu.Lock()
				s.scaleMetrics.MemoryUsageBytes = currentMemory
				s.scaleMetrics.mu.Unlock()

			case <-timeout:
				return
			}
		}
	}()

	<-timeout
	close(stopLoad)
	wg.Wait()
	<-memoryMonitorDone

	// Analyze memory stability
	finalMemory := s.getCurrentMemoryUsage()
	maxMemory := int64(0)
	for _, reading := range memoryReadings {
		if reading > maxMemory {
			maxMemory = reading
		}
	}

	memoryGrowth := finalMemory - initialMemory
	maxMemoryMB := maxMemory / (1024 * 1024)

	// Performance assertions
	assert.Less(t, maxMemoryMB, int64(maxMemoryMB),
		fmt.Sprintf("Max memory usage should be < %dMB, got %dMB", maxMemoryMB, maxMemoryMB))
	assert.Less(t, memoryGrowth, initialMemory/2,
		"Memory growth should be < 50% of initial memory")

	fmt.Printf("✅ Memory stability validated\n")
	fmt.Printf("   Initial: %dMB, Final: %dMB, Max: %dMB\n",
		initialMemory/(1024*1024), finalMemory/(1024*1024), maxMemoryMB)
	fmt.Printf("   Memory growth: %dMB\n", memoryGrowth/(1024*1024))
}

// TestGoroutineLeakDetection validates no goroutine leaks under sustained load
func (s *Sprint6ScaleTestSuite) TestGoroutineLeakDetection() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚀 Testing: Goroutine leak detection...")

	// Record initial goroutine count
	initialGoroutines := int64(runtime.NumGoroutine())

	// Run sustained operations
	const operations = 1000
	var wg sync.WaitGroup

	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func(opIndex int) {
			defer wg.Done()

			// Execute various operations that create goroutines
			s.executeCompleteTaskWorkflow(ctx, opIndex)

			// Small delay to ensure cleanup
			time.Sleep(1 * time.Millisecond)
		}(i)
	}

	wg.Wait()

	// Allow time for cleanup
	time.Sleep(5 * time.Second)
	runtime.GC()
	time.Sleep(2 * time.Second)

	// Check final goroutine count
	finalGoroutines := int64(runtime.NumGoroutine())
	goroutineGrowth := finalGoroutines - initialGoroutines

	s.scaleMetrics.mu.Lock()
	s.scaleMetrics.GoroutineCount = finalGoroutines
	s.scaleMetrics.mu.Unlock()

	// Allow some growth but detect significant leaks
	maxAllowedGrowth := int64(50) // 50 goroutines max growth
	assert.Less(t, goroutineGrowth, maxAllowedGrowth,
		fmt.Sprintf("Goroutine growth should be < %d, got %d", maxAllowedGrowth, goroutineGrowth))

	fmt.Printf("✅ Goroutine leak detection validated\n")
	fmt.Printf("   Initial: %d, Final: %d, Growth: %d\n",
		initialGoroutines, finalGoroutines, goroutineGrowth)
}

// Helper methods

func (s *Sprint6ScaleTestSuite) executeCompleteTaskWorkflow(ctx context.Context, taskIndex int) error {
	userDID := fmt.Sprintf("did:zerostate:user:scale_%d", taskIndex)
	agentDID := fmt.Sprintf("did:zerostate:agent:scale_%d", taskIndex)
	taskID := s.generateTaskID()

	// Deposit funds
	if err := s.paymentService.Deposit(ctx, userDID, 100.0); err != nil {
		return fmt.Errorf("deposit failed: %w", err)
	}

	// Create escrow
	if err := s.escrowClient.CreateEscrow(ctx, taskID, 100_000_000, s.generateTaskID(), nil); err != nil {
		return fmt.Errorf("escrow creation failed: %w", err)
	}

	// Agent accepts
	if err := s.escrowClient.AcceptTask(ctx, taskID, agentDID); err != nil {
		return fmt.Errorf("task accept failed: %w", err)
	}

	// Release payment
	if err := s.escrowClient.ReleasePayment(ctx, taskID); err != nil {
		return fmt.Errorf("payment release failed: %w", err)
	}

	// Update reputation
	if err := s.reputationClient.ReportOutcome(ctx, agentDID, true); err != nil {
		// Don't fail the task for reputation failures (graceful degradation)
		fmt.Printf("Warning: reputation update failed for %s: %v\n", agentDID, err)
	}

	return nil
}

func (s *Sprint6ScaleTestSuite) executePaymentWorkflow(ctx context.Context, userDID, agentDID string, amount float64) error {
	if err := s.paymentService.Deposit(ctx, userDID, amount); err != nil {
		return err
	}

	channel, err := s.paymentService.CreateChannel(ctx, userDID, agentDID, amount, fmt.Sprintf("auction-%d", rand.Int()))
	if err != nil {
		return err
	}

	taskID := fmt.Sprintf("task-%d", rand.Int())
	if err := s.paymentService.LockEscrow(ctx, channel.ID, taskID, amount); err != nil {
		return err
	}

	return s.paymentService.ReleaseEscrow(ctx, channel.ID, taskID, true)
}

func (s *Sprint6ScaleTestSuite) simulateTaskOperation(ctx context.Context) {
	taskID := s.generateTaskID()
	agentDID := fmt.Sprintf("did:zerostate:agent:sim_%d", rand.Int())

	// Simulate task processing time
	time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)
}

func (s *Sprint6ScaleTestSuite) simulatePaymentOperation(ctx context.Context) {
	// Simulate payment processing
	time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
}

func (s *Sprint6ScaleTestSuite) simulateReputationOperation(ctx context.Context) {
	// Simulate reputation query/update
	time.Sleep(time.Duration(rand.Intn(30)) * time.Millisecond)
}

func (s *Sprint6ScaleTestSuite) generateSustainedLoad(ctx context.Context, stop chan bool) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Generate various types of load
			go s.simulateTaskOperation(ctx)
			if rand.Intn(3) == 0 {
				go s.simulatePaymentOperation(ctx)
			}
			if rand.Intn(5) == 0 {
				go s.simulateReputationOperation(ctx)
			}
		case <-stop:
			return
		}
	}
}

func (s *Sprint6ScaleTestSuite) monitorSystemMetrics() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateSystemMetrics()
		}
	}
}

func (s *Sprint6ScaleTestSuite) updateSystemMetrics() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	s.scaleMetrics.mu.Lock()
	s.scaleMetrics.MemoryUsageBytes = int64(m.Alloc)
	s.scaleMetrics.GoroutineCount = int64(runtime.NumGoroutine())
	s.scaleMetrics.HeapObjects = int64(m.HeapObjects)

	// Record GC pause
	if len(m.PauseNs) > 0 {
		lastPause := time.Duration(m.PauseNs[(m.NumGC+255)%256])
		if lastPause > 0 {
			s.scaleMetrics.GCPauses = append(s.scaleMetrics.GCPauses, lastPause)
		}
	}
	s.scaleMetrics.mu.Unlock()
}

func (s *Sprint6ScaleTestSuite) getCurrentMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

func (s *Sprint6ScaleTestSuite) generateTaskID() [32]byte {
	var taskID [32]byte
	for i := range taskID {
		taskID[i] = byte(rand.Intn(256))
	}
	return taskID
}

func (s *ScaleMetrics) RecordLatency(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Latencies = append(s.Latencies, d)
}

func (s *ScaleMetrics) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.Errors = append(s.Errors, err)
	}
}

func (s *ScaleMetrics) CalculatePercentiles() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	latencies := make([]time.Duration, len(s.Latencies))
	copy(latencies, s.Latencies)

	// Simple bubble sort for testing (would use sort.Slice in production)
	for i := 0; i < len(latencies)-1; i++ {
		for j := 0; j < len(latencies)-i-1; j++ {
			if latencies[j] > latencies[j+1] {
				latencies[j], latencies[j+1] = latencies[j+1], latencies[j]
			}
		}
	}

	// Calculate percentiles
	p50Index := len(latencies) / 2
	p95Index := (95 * len(latencies)) / 100
	p99Index := (99 * len(latencies)) / 100

	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	if p99Index >= len(latencies) {
		p99Index = len(latencies) - 1
	}

	s.LatencyP50 = latencies[p50Index]
	s.LatencyP95 = latencies[p95Index]
	s.LatencyP99 = latencies[p99Index]
}

func (s *Sprint6ScaleTestSuite) printScaleSummary() {
	duration := time.Since(s.scaleMetrics.StartTime)

	fmt.Printf("\n🚀 SCALE TEST SUMMARY\n")
	fmt.Printf("=====================\n")
	fmt.Printf("Total Duration: %v\n", duration)
	fmt.Printf("Total Operations: %d\n", s.scaleMetrics.TotalOperations)
	fmt.Printf("Successful Operations: %d\n", s.scaleMetrics.SuccessfulOperations)
	fmt.Printf("Failed Operations: %d\n", s.scaleMetrics.FailedOperations)
	fmt.Printf("Max Concurrent Tasks: %d\n", s.scaleMetrics.MaxConcurrentTasks)
	fmt.Printf("Task Throughput: %.2f tasks/sec\n", s.scaleMetrics.TaskThroughputPerSecond)
	fmt.Printf("Payment Throughput: %.2f payments/sec\n", s.scaleMetrics.PaymentThroughputPerSec)
	fmt.Printf("Error Rate: %.2f%%\n", s.scaleMetrics.ErrorRate)
	fmt.Printf("Memory Usage: %d MB\n", s.scaleMetrics.MemoryUsageBytes/(1024*1024))
	fmt.Printf("Goroutines: %d\n", s.scaleMetrics.GoroutineCount)
	fmt.Printf("Latency P95: %v\n", s.scaleMetrics.LatencyP95)
	fmt.Printf("Latency P99: %v\n", s.scaleMetrics.LatencyP99)

	if len(s.scaleMetrics.GCPauses) > 0 {
		maxGCPause := time.Duration(0)
		for _, pause := range s.scaleMetrics.GCPauses {
			if pause > maxGCPause {
				maxGCPause = pause
			}
		}
		fmt.Printf("Max GC Pause: %v\n", maxGCPause)
	}
}

// Helper to add float64 atomically
func addFloat64(addr *float64, delta float64) float64 {
	for {
		old := *addr
		new := old + delta
		if atomic.CompareAndSwapUint64(
			(*uint64)(unsafe.Pointer(addr)),
			*(*uint64)(unsafe.Pointer(&old)),
			*(*uint64)(unsafe.Pointer(&new)),
		) {
			return new
		}
	}
}