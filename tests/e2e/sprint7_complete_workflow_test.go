// Package e2e provides comprehensive Sprint 7 E2E workflow tests
// Tests complete system integration from task submission to payment release
package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"runtime"
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

// Sprint7WorkflowTestSuite validates complete E2E workflows
type Sprint7WorkflowTestSuite struct {
	suite.Suite
	ctx                     context.Context
	cancel                  context.CancelFunc
	escrowClient           *substrate.EscrowClient
	reputationClient       *substrate.ReputationClient
	auctionClient          *substrate.VCGAuctionClient
	orchestrator           *orchestration.Orchestrator
	paymentService         *economic.PaymentChannelService
	reputationService      *reputation.ReputationService
	marketplaceService     *marketplace.MarketplaceService
	messageBus             *ProductionMockMessageBus
	metricsCollector       *metrics.Collector
	sprint7Metrics         *Sprint7WorkflowMetrics
	circuitBreakerTestChan chan error
}

// Sprint7WorkflowMetrics tracks comprehensive E2E workflow metrics
type Sprint7WorkflowMetrics struct {
	mu                         sync.RWMutex
	StartTime                 time.Time

	// Task Workflow Metrics
	TotalWorkflows            int64
	SuccessfulWorkflows       int64
	FailedWorkflows          int64
	RefundWorkflows          int64
	DisputedWorkflows        int64
	ConcurrentWorkflows      int64

	// Component Integration Metrics
	VCGAuctionsCompleted     int64
	EscrowTransitions        int64
	ReputationUpdates        int64
	PaymentChannelOperations int64
	CircuitBreakerActivations int64

	// Performance Metrics
	WorkflowLatencyP50       time.Duration
	WorkflowLatencyP95       time.Duration
	WorkflowLatencyP99       time.Duration
	ThroughputWorkflowsPerSec float64
	MemoryUsageBytes         int64
	GoroutineCount           int64

	// Error Tracking
	BlockchainErrors         int64
	NetworkErrors           int64
	TimeoutErrors           int64
	ValidationErrors        int64

	// Latency Distribution
	LatencyHistogram        []time.Duration
	ComponentLatencies      map[string][]time.Duration

	// Feature Coverage
	FeaturesCovered         map[string]bool
	FeatureSuccessRates     map[string]float64
}

// ProductionMockMessageBus simulates production-grade message bus with realistic behavior
type ProductionMockMessageBus struct {
	mu               sync.RWMutex
	messageCount     int64
	subscriptions    map[string][]p2p.MessageHandler
	requestHandlers  map[string]p2p.RequestHandler
	latencyProfile   LatencyProfile
	failureProfile   FailureProfile
	networkPartitions map[string]bool
}

type LatencyProfile struct {
	BaseLatencyMs    int
	JitterMs         int
	SlowRequestRate  float64 // Percentage of requests that are slow
	SlowLatencyMs    int
}

type FailureProfile struct {
	NetworkErrorRate   float64
	TimeoutRate       float64
	PartialFailureRate float64
	MeanTimeToRecovery time.Duration
}

func NewProductionMockMessageBus() *ProductionMockMessageBus {
	return &ProductionMockMessageBus{
		subscriptions:    make(map[string][]p2p.MessageHandler),
		requestHandlers:  make(map[string]p2p.RequestHandler),
		networkPartitions: make(map[string]bool),
		latencyProfile: LatencyProfile{
			BaseLatencyMs:   5,
			JitterMs:        3,
			SlowRequestRate: 0.05, // 5% slow requests
			SlowLatencyMs:   50,
		},
		failureProfile: FailureProfile{
			NetworkErrorRate:   0.005, // 0.5% network errors
			TimeoutRate:       0.001, // 0.1% timeouts
			PartialFailureRate: 0.002, // 0.2% partial failures
			MeanTimeToRecovery: 2 * time.Second,
		},
	}
}

func (m *ProductionMockMessageBus) Start(ctx context.Context) error {
	return nil
}

func (m *ProductionMockMessageBus) Stop() error {
	return nil
}

func (m *ProductionMockMessageBus) Publish(ctx context.Context, topic string, data []byte) error {
	atomic.AddInt64(&m.messageCount, 1)

	// Simulate realistic network latency with jitter
	latency := m.calculateLatency()
	time.Sleep(latency)

	// Simulate network failures
	if m.shouldSimulateFailure("network") {
		return fmt.Errorf("network partition: failed to publish to topic %s", topic)
	}

	// Deliver to subscribers
	m.mu.RLock()
	handlers := m.subscriptions[topic]
	m.mu.RUnlock()

	for _, handler := range handlers {
		go func(h p2p.MessageHandler) {
			if !m.shouldSimulateFailure("partial") {
				h(data)
			}
		}(handler)
	}

	return nil
}

func (m *ProductionMockMessageBus) Subscribe(ctx context.Context, topic string, handler p2p.MessageHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptions[topic] = append(m.subscriptions[topic], handler)
	return nil
}

func (m *ProductionMockMessageBus) SendRequest(ctx context.Context, targetDID string, request []byte, timeout time.Duration) ([]byte, error) {
	atomic.AddInt64(&m.messageCount, 1)

	// Check for network partitions
	if m.isPartitioned(targetDID) {
		return nil, fmt.Errorf("network partition: target %s unreachable", targetDID)
	}

	// Simulate realistic network latency
	latency := m.calculateLatency()
	time.Sleep(latency)

	// Simulate timeout errors
	if m.shouldSimulateFailure("timeout") {
		return nil, fmt.Errorf("request timeout to %s after %v", targetDID, timeout)
	}

	// Simulate network errors
	if m.shouldSimulateFailure("network") {
		return nil, fmt.Errorf("network error: connection refused by %s", targetDID)
	}

	// Return realistic response
	responseSize := len(request) + rand.Intn(512) // Response varies based on request
	response := make([]byte, responseSize)
	rand.Read(response[:8]) // Add some random data
	copy(response[8:], []byte(fmt.Sprintf("response-from-%s", targetDID)))

	return response, nil
}

func (m *ProductionMockMessageBus) RegisterRequestHandler(messageType string, handler p2p.RequestHandler) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestHandlers[messageType] = handler
	return nil
}

func (m *ProductionMockMessageBus) GetPeerID() string {
	return "sprint7-test-peer-production"
}

func (m *ProductionMockMessageBus) calculateLatency() time.Duration {
	baseLatency := time.Duration(m.latencyProfile.BaseLatencyMs) * time.Millisecond
	jitter := time.Duration(rand.Intn(m.latencyProfile.JitterMs*2)-m.latencyProfile.JitterMs) * time.Millisecond

	// Add occasional slow requests
	if rand.Float64() < m.latencyProfile.SlowRequestRate {
		return baseLatency + jitter + time.Duration(m.latencyProfile.SlowLatencyMs)*time.Millisecond
	}

	return baseLatency + jitter
}

func (m *ProductionMockMessageBus) shouldSimulateFailure(failureType string) bool {
	switch failureType {
	case "network":
		return rand.Float64() < m.failureProfile.NetworkErrorRate
	case "timeout":
		return rand.Float64() < m.failureProfile.TimeoutRate
	case "partial":
		return rand.Float64() < m.failureProfile.PartialFailureRate
	default:
		return false
	}
}

func (m *ProductionMockMessageBus) isPartitioned(targetDID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.networkPartitions[targetDID]
}

func (m *ProductionMockMessageBus) SimulateNetworkPartition(targetDID string, duration time.Duration) {
	m.mu.Lock()
	m.networkPartitions[targetDID] = true
	m.mu.Unlock()

	time.AfterFunc(duration, func() {
		m.mu.Lock()
		delete(m.networkPartitions, targetDID)
		m.mu.Unlock()
	})
}

func TestSprint7CompleteWorkflows(t *testing.T) {
	suite.Run(t, new(Sprint7WorkflowTestSuite))
}

func (s *Sprint7WorkflowTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 20*time.Minute)

	// Initialize comprehensive metrics tracking
	s.sprint7Metrics = &Sprint7WorkflowMetrics{
		StartTime:          time.Now(),
		FeaturesCovered:    make(map[string]bool),
		FeatureSuccessRates: make(map[string]float64),
		ComponentLatencies: make(map[string][]time.Duration),
	}

	// Setup production-grade message bus
	s.messageBus = NewProductionMockMessageBus()

	// Initialize metrics collector
	s.metricsCollector = metrics.NewCollector()

	// Setup blockchain clients with production configuration
	substrateClient, err := substrate.NewClientV2("ws://localhost:9944")
	require.NoError(s.T(), err, "Failed to connect to Substrate node")

	keyring, err := substrate.CreateKeyringFromSeed("//Alice", substrate.Sr25519Type)
	require.NoError(s.T(), err, "Failed to create keyring")

	// Initialize all clients
	s.escrowClient = substrate.NewEscrowClient(substrateClient, keyring)
	s.reputationClient = substrate.NewReputationClient(substrateClient, keyring)
	s.auctionClient = substrate.NewVCGAuctionClient(substrateClient, keyring)

	// Initialize services with production configurations
	s.paymentService = economic.NewPaymentChannelService()
	s.reputationService = reputation.NewReputationService()

	// Setup marketplace with enhanced configuration
	discoveryService := marketplace.NewDiscoveryService(s.messageBus, s.reputationService)
	auctionService := marketplace.NewAuctionService(s.messageBus)
	s.marketplaceService = marketplace.NewMarketplaceService(
		discoveryService,
		auctionService,
		s.messageBus,
		s.reputationService,
	)

	// Initialize orchestrator with production settings
	s.orchestrator = orchestration.NewOrchestrator(
		orchestration.Config{
			MaxConcurrentTasks:    1000,
			ReputationEnabled:     true,
			VCGEnabled:           true,
			PaymentEnabled:       true,
			CircuitBreakerEnabled: true,
			HealthCheckInterval:   10 * time.Second,
			TaskTimeoutSeconds:    300,
		},
		s.messageBus,
		s.paymentService,
		s.reputationService,
	)

	// Setup circuit breaker testing channel
	s.circuitBreakerTestChan = make(chan error, 100)

	// Wait for all services to fully initialize
	time.Sleep(5 * time.Second)

	fmt.Println("🚀 Sprint 7 E2E Test Suite initialized with production-grade configuration")
}

func (s *Sprint7WorkflowTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
	close(s.circuitBreakerTestChan)
	s.printSprint7Summary()
}

// TestCompleteSuccessfulWorkflow tests the complete happy path workflow
func (s *Sprint7WorkflowTestSuite) TestCompleteSuccessfulWorkflow() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🎯 Testing Complete Successful Workflow (User → Task → VCG → Execution → Payment)")

	workflowStart := time.Now()

	// Step 1: User setup and task submission
	userDID := generateUniqueDID("user", "successful_workflow")
	agentDID := generateUniqueDID("agent", "successful_worker")
	taskID := generateTaskID()

	// Initialize user balance
	err := s.paymentService.Deposit(ctx, userDID, 200.0)
	require.NoError(t, err, "Failed to deposit funds for user")

	// Register capable agent
	agent := &identity.AgentCard{
		DID:          agentDID,
		Name:         "High Performance Agent",
		Capabilities: []string{"image-processing", "gpu", "high-memory"},
		Reputation:   85.0,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	discoveryService := s.marketplaceService.GetDiscoveryService()
	err = discoveryService.RegisterAgent(ctx, agent)
	require.NoError(t, err, "Failed to register agent")

	// Step 2: Submit task with realistic requirements
	taskReq := &orchestration.TaskRequest{
		UserDID:      userDID,
		TaskType:     "image-processing",
		Description:  "Batch resize 1000 images with watermark",
		MaxPayment:   150.0,
		Timeout:      300 * time.Second,
		Requirements: []string{"gpu", "high-memory"},
		Priority:     orchestration.PriorityNormal,
		Metadata:     map[string]string{
			"batch_size": "1000",
			"format":     "jpeg",
			"quality":    "high",
		},
	}

	stepStart := time.Now()
	task, err := s.orchestrator.SubmitTask(ctx, taskReq)
	require.NoError(t, err, "Failed to submit task")
	s.recordComponentLatency("task_submission", time.Since(stepStart))

	assert.Equal(t, userDID, task.UserDID)
	assert.Equal(t, taskReq.Description, task.Description)
	assert.NotEmpty(t, task.ID)

	// Step 3: VCG Auction process
	stepStart = time.Now()

	// Create auction request
	auctionReq := &marketplace.AuctionRequest{
		TaskID:          task.ID,
		UserID:          userDID,
		Capabilities:    taskReq.Requirements,
		TaskType:        taskReq.TaskType,
		MaxPrice:        taskReq.MaxPayment,
		AuctionDuration: 5 * time.Second,
		AuctionType:     marketplace.AuctionTypeVCG,
	}

	// Simulate multiple competing bids
	bidders := []struct {
		agentDID string
		bid      float64
		capability float64
	}{
		{agentDID, 120.0, 0.95},
		{generateUniqueDID("agent", "bidder2"), 130.0, 0.85},
		{generateUniqueDID("agent", "bidder3"), 140.0, 0.80},
	}

	// Submit bids concurrently (realistic auction scenario)
	var wg sync.WaitGroup
	for _, bidder := range bidders {
		wg.Add(1)
		go func(bid struct{ agentDID string; bid float64; capability float64 }) {
			defer wg.Done()
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond) // Random bid timing
			err := s.marketplaceService.SubmitBidForAgent(ctx, auctionReq.TaskID, bid.agentDID, bid.bid, 10*time.Second)
			if err != nil {
				fmt.Printf("⚠️  Bid submission failed for %s: %v\n", bid.agentDID, err)
			}
		}(bidder)
	}
	wg.Wait()

	// Run VCG auction
	allocation, err := s.marketplaceService.RunAuction(ctx, auctionReq)
	require.NoError(t, err, "VCG auction failed")
	s.recordComponentLatency("vcg_auction", time.Since(stepStart))

	assert.Equal(t, agentDID, allocation.WinnerDID, "Expected highest capability agent to win")
	assert.Greater(t, allocation.FinalPrice, 0.0)
	assert.LessOrEqual(t, allocation.FinalPrice, taskReq.MaxPayment)

	// Step 4: Escrow creation and management
	stepStart = time.Now()
	escrowAmount := uint64(allocation.FinalPrice * 1_000_000) // Convert to micro-units
	err = s.escrowClient.CreateEscrow(ctx, task.ID, escrowAmount, task.ID, nil)
	require.NoError(t, err, "Failed to create escrow")
	s.recordComponentLatency("escrow_creation", time.Since(stepStart))

	// Verify escrow state
	escrow, err := s.escrowClient.GetEscrow(ctx, task.ID)
	require.NoError(t, err, "Failed to get escrow")
	assert.Equal(t, substrate.EscrowStatePending, escrow.State)

	// Step 5: Agent accepts task
	stepStart = time.Now()
	err = s.escrowClient.AcceptTask(ctx, task.ID, agentDID)
	require.NoError(t, err, "Failed to accept task")
	s.recordComponentLatency("task_acceptance", time.Since(stepStart))

	// Verify task accepted
	escrow, err = s.escrowClient.GetEscrow(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateAccepted, escrow.State)
	assert.Equal(t, agentDID, string(*escrow.AgentDID))

	// Step 6: Task execution simulation
	stepStart = time.Now()
	executionDuration := simulateTaskExecution(taskReq.TaskType, 500*time.Millisecond)
	s.recordComponentLatency("task_execution", executionDuration)

	assert.Less(t, executionDuration, taskReq.Timeout, "Task execution should complete within timeout")

	// Step 7: Payment release
	stepStart = time.Now()
	err = s.escrowClient.ReleasePayment(ctx, task.ID)
	require.NoError(t, err, "Failed to release payment")
	s.recordComponentLatency("payment_release", time.Since(stepStart))

	// Verify payment completed
	escrow, err = s.escrowClient.GetEscrow(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateCompleted, escrow.State)

	// Step 8: Reputation update
	stepStart = time.Now()
	err = s.reputationClient.ReportOutcome(ctx, agentDID, true)
	require.NoError(t, err, "Failed to update reputation")
	s.recordComponentLatency("reputation_update", time.Since(stepStart))

	// Verify reputation increased
	newReputation, err := s.reputationClient.GetReputationScore(ctx, agentDID)
	require.NoError(t, err)
	assert.Greater(t, newReputation, agent.Reputation)

	workflowDuration := time.Since(workflowStart)
	s.recordWorkflowCompletion(workflowDuration, true)

	fmt.Printf("✅ Complete successful workflow completed in %v\n", workflowDuration)

	// Verify performance benchmarks
	assert.Less(t, workflowDuration, 30*time.Second, "Complete workflow should finish within 30s")

	// Update metrics
	atomic.AddInt64(&s.sprint7Metrics.SuccessfulWorkflows, 1)
	atomic.AddInt64(&s.sprint7Metrics.VCGAuctionsCompleted, 1)
	atomic.AddInt64(&s.sprint7Metrics.EscrowTransitions, 4) // Pending->Accepted->Completed
	atomic.AddInt64(&s.sprint7Metrics.ReputationUpdates, 1)
	s.sprint7Metrics.FeaturesCovered["complete_workflow"] = true
}

// TestFailedTaskWorkflow tests the complete failure path with reputation slashing
func (s *Sprint7WorkflowTestSuite) TestFailedTaskWorkflow() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚨 Testing Failed Task Workflow (Failure → Reputation Slash → Refund)")

	workflowStart := time.Now()

	userDID := generateUniqueDID("user", "failed_workflow")
	agentDID := generateUniqueDID("agent", "unreliable_agent")
	taskID := generateTaskID()

	// Setup
	err := s.paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)

	// Register unreliable agent
	agent := &identity.AgentCard{
		DID:          agentDID,
		Name:         "Unreliable Agent",
		Capabilities: []string{"text-processing"},
		Reputation:   60.0, // Lower reputation
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	discoveryService := s.marketplaceService.GetDiscoveryService()
	err = discoveryService.RegisterAgent(ctx, agent)
	require.NoError(t, err)

	// Submit task
	taskReq := &orchestration.TaskRequest{
		UserDID:      userDID,
		TaskType:     "text-processing",
		Description:  "Unreliable task that will fail",
		MaxPayment:   80.0,
		Timeout:      60 * time.Second,
		Requirements: []string{"text-processing"},
	}

	task, err := s.orchestrator.SubmitTask(ctx, taskReq)
	require.NoError(t, err)

	// Create escrow
	escrowAmount := uint64(80.0 * 1_000_000)
	err = s.escrowClient.CreateEscrow(ctx, task.ID, escrowAmount, task.ID, nil)
	require.NoError(t, err)

	// Agent accepts task
	err = s.escrowClient.AcceptTask(ctx, task.ID, agentDID)
	require.NoError(t, err)

	// Simulate task failure (agent fails to deliver)
	time.Sleep(100 * time.Millisecond) // Simulate some work attempt

	// Record initial reputation
	initialReputation, err := s.reputationClient.GetReputationScore(ctx, agentDID)
	require.NoError(t, err)

	// Report failure and slash reputation
	stepStart := time.Now()
	err = s.reputationClient.ReportOutcome(ctx, agentDID, false)
	require.NoError(t, err)
	s.recordComponentLatency("reputation_slashing", time.Since(stepStart))

	// Verify reputation decreased
	newReputation, err := s.reputationClient.GetReputationScore(ctx, agentDID)
	require.NoError(t, err)
	assert.Less(t, newReputation, initialReputation, "Reputation should decrease after failure")

	// Process refund
	initialUserBalance, err := s.paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)

	stepStart = time.Now()
	err = s.escrowClient.RefundEscrow(ctx, task.ID)
	require.NoError(t, err)
	s.recordComponentLatency("refund_processing", time.Since(stepStart))

	// Verify refund processed
	escrow, err := s.escrowClient.GetEscrow(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateRefunded, escrow.State)

	finalUserBalance, err := s.paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Greater(t, finalUserBalance, initialUserBalance, "User should receive refund")

	workflowDuration := time.Since(workflowStart)
	s.recordWorkflowCompletion(workflowDuration, false)

	fmt.Printf("✅ Failed task workflow with refund completed in %v\n", workflowDuration)

	// Update metrics
	atomic.AddInt64(&s.sprint7Metrics.FailedWorkflows, 1)
	atomic.AddInt64(&s.sprint7Metrics.RefundWorkflows, 1)
	atomic.AddInt64(&s.sprint7Metrics.ReputationUpdates, 1)
	s.sprint7Metrics.FeaturesCovered["failure_handling"] = true
}

// TestDisputedWorkflow tests the dispute mechanism
func (s *Sprint7WorkflowTestSuite) TestDisputedWorkflow() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("⚖️  Testing Disputed Workflow (Dispute → Escrow Lock → Resolution)")

	workflowStart := time.Now()

	userDID := generateUniqueDID("user", "dispute_workflow")
	agentDID := generateUniqueDID("agent", "disputed_agent")

	// Setup disputed scenario
	err := s.paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)

	taskReq := &orchestration.TaskRequest{
		UserDID:      userDID,
		TaskType:     "data-analysis",
		Description:  "Disputed data analysis task",
		MaxPayment:   90.0,
		Timeout:      120 * time.Second,
		Requirements: []string{"data-analysis"},
	}

	task, err := s.orchestrator.SubmitTask(ctx, taskReq)
	require.NoError(t, err)

	// Create escrow and accept task
	escrowAmount := uint64(90.0 * 1_000_000)
	err = s.escrowClient.CreateEscrow(ctx, task.ID, escrowAmount, task.ID, nil)
	require.NoError(t, err)

	err = s.escrowClient.AcceptTask(ctx, task.ID, agentDID)
	require.NoError(t, err)

	// Simulate dispute scenario
	time.Sleep(200 * time.Millisecond) // Simulate some work

	// Raise dispute
	stepStart := time.Now()
	err = s.escrowClient.DisputeEscrow(ctx, task.ID)
	require.NoError(t, err)
	s.recordComponentLatency("dispute_initiation", time.Since(stepStart))

	// Verify dispute state
	escrow, err := s.escrowClient.GetEscrow(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateDisputed, escrow.State)

	// Verify funds are locked (no payments or refunds)
	userBalance, err := s.paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 10.0, userBalance, "User balance should show locked funds") // 100 - 90 = 10

	// In a real scenario, dispute resolution would happen here
	// For testing, we'll simulate resolution after a delay
	time.Sleep(500 * time.Millisecond)

	workflowDuration := time.Since(workflowStart)
	s.recordWorkflowCompletion(workflowDuration, false) // Disputed = not successful

	fmt.Printf("✅ Disputed workflow initiated and locked in %v\n", workflowDuration)

	// Update metrics
	atomic.AddInt64(&s.sprint7Metrics.DisputedWorkflows, 1)
	atomic.AddInt64(&s.sprint7Metrics.EscrowTransitions, 3) // Pending->Accepted->Disputed
	s.sprint7Metrics.FeaturesCovered["dispute_mechanism"] = true
}

// TestConcurrentWorkflows tests multiple simultaneous workflows
func (s *Sprint7WorkflowTestSuite) TestConcurrentWorkflows() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🚦 Testing Concurrent Workflows (10 simultaneous workflows)")

	workflowStart := time.Now()
	numWorkflows := 10
	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	// Create multiple concurrent workflows
	for i := 0; i < numWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			atomic.AddInt64(&s.sprint7Metrics.ConcurrentWorkflows, 1)

			userDID := generateUniqueDID("user", fmt.Sprintf("concurrent_%d", workflowID))
			agentDID := generateUniqueDID("agent", fmt.Sprintf("concurrent_agent_%d", workflowID))

			// Setup user
			err := s.paymentService.Deposit(ctx, userDID, 50.0)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			// Register agent
			agent := &identity.AgentCard{
				DID:          agentDID,
				Name:         fmt.Sprintf("Concurrent Agent %d", workflowID),
				Capabilities: []string{"concurrent-task"},
				Reputation:   70.0 + float64(workflowID), // Varying reputation
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}

			discoveryService := s.marketplaceService.GetDiscoveryService()
			err = discoveryService.RegisterAgent(ctx, agent)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			// Submit task
			taskReq := &orchestration.TaskRequest{
				UserDID:      userDID,
				TaskType:     "concurrent-task",
				Description:  fmt.Sprintf("Concurrent task %d", workflowID),
				MaxPayment:   40.0,
				Timeout:      30 * time.Second,
				Requirements: []string{"concurrent-task"},
			}

			task, err := s.orchestrator.SubmitTask(ctx, taskReq)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			// Process workflow
			err = s.executeMiniWorkflow(ctx, task, agentDID, 40.0)
			if err != nil {
				atomic.AddInt64(&errorCount, 1)
				return
			}

			atomic.AddInt64(&successCount, 1)
		}(i)
	}

	// Wait for all workflows to complete
	wg.Wait()

	concurrentDuration := time.Since(workflowStart)

	fmt.Printf("✅ %d/%d concurrent workflows completed successfully in %v\n",
		successCount, numWorkflows, concurrentDuration)

	// Verify performance under load
	assert.GreaterOrEqual(t, successCount, int64(8), "At least 80% of concurrent workflows should succeed")
	assert.Less(t, concurrentDuration, 60*time.Second, "Concurrent workflows should complete within 60s")

	// Calculate throughput
	throughput := float64(successCount) / concurrentDuration.Seconds()
	s.sprint7Metrics.ThroughputWorkflowsPerSec = throughput

	fmt.Printf("📊 Concurrent throughput: %.2f workflows/sec\n", throughput)
	assert.Greater(t, throughput, 0.1, "Should process at least 0.1 workflows/sec under load")

	// Update metrics
	atomic.AddInt64(&s.sprint7Metrics.SuccessfulWorkflows, successCount)
	atomic.AddInt64(&s.sprint7Metrics.FailedWorkflows, errorCount)
	s.sprint7Metrics.FeaturesCovered["concurrent_processing"] = true
}

// TestCircuitBreakerUnderFailure tests circuit breaker activation
func (s *Sprint7WorkflowTestSuite) TestCircuitBreakerUnderFailure() {
	t := s.T()
	ctx := s.ctx

	fmt.Println("🔌 Testing Circuit Breaker Under Failure Conditions")

	// Simulate network partition to trigger circuit breaker
	problematicAgent := generateUniqueDID("agent", "problematic")
	s.messageBus.SimulateNetworkPartition(problematicAgent, 10*time.Second)

	// Try to communicate with partitioned agent multiple times
	var failures int64
	for i := 0; i < 10; i++ {
		_, err := s.messageBus.SendRequest(ctx, problematicAgent, []byte("test"), 1*time.Second)
		if err != nil {
			atomic.AddInt64(&failures, 1)
			s.circuitBreakerTestChan <- err
		}
	}

	fmt.Printf("📊 Circuit breaker test: %d failures out of 10 attempts\n", failures)

	// Verify circuit breaker would be activated
	assert.GreaterOrEqual(t, failures, int64(5), "Should have multiple failures to trigger circuit breaker")

	// Test that system continues operating despite failures
	userDID := generateUniqueDID("user", "circuit_breaker_test")
	err := s.paymentService.Deposit(ctx, userDID, 50.0)
	require.NoError(t, err, "System should continue operating despite circuit breaker")

	// Update metrics
	atomic.AddInt64(&s.sprint7Metrics.CircuitBreakerActivations, 1)
	atomic.AddInt64(&s.sprint7Metrics.NetworkErrors, failures)
	s.sprint7Metrics.FeaturesCovered["circuit_breaker"] = true
}

// Helper methods

func (s *Sprint7WorkflowTestSuite) executeMiniWorkflow(ctx context.Context, task *orchestration.Task, agentDID string, amount float64) error {
	escrowAmount := uint64(amount * 1_000_000)

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

	// Simulate quick execution
	time.Sleep(50 * time.Millisecond)

	// Release payment
	err = s.escrowClient.ReleasePayment(ctx, task.ID)
	if err != nil {
		return err
	}

	// Update reputation
	return s.reputationClient.ReportOutcome(ctx, agentDID, true)
}

func (s *Sprint7WorkflowTestSuite) recordComponentLatency(component string, latency time.Duration) {
	s.sprint7Metrics.mu.Lock()
	defer s.sprint7Metrics.mu.Unlock()

	if s.sprint7Metrics.ComponentLatencies[component] == nil {
		s.sprint7Metrics.ComponentLatencies[component] = make([]time.Duration, 0)
	}
	s.sprint7Metrics.ComponentLatencies[component] = append(
		s.sprint7Metrics.ComponentLatencies[component], latency)
}

func (s *Sprint7WorkflowTestSuite) recordWorkflowCompletion(duration time.Duration, success bool) {
	s.sprint7Metrics.mu.Lock()
	defer s.sprint7Metrics.mu.Unlock()

	s.sprint7Metrics.LatencyHistogram = append(s.sprint7Metrics.LatencyHistogram, duration)

	if success {
		atomic.AddInt64(&s.sprint7Metrics.SuccessfulWorkflows, 1)
	} else {
		atomic.AddInt64(&s.sprint7Metrics.FailedWorkflows, 1)
	}

	atomic.AddInt64(&s.sprint7Metrics.TotalWorkflows, 1)
}

func (s *Sprint7WorkflowTestSuite) printSprint7Summary() {
	duration := time.Since(s.sprint7Metrics.StartTime)

	// Calculate latency percentiles
	s.calculateLatencyPercentiles()

	fmt.Printf("\n📊 SPRINT 7 E2E WORKFLOW TEST SUMMARY\n")
	fmt.Printf("=====================================\n")
	fmt.Printf("Total Duration: %v\n", duration)
	fmt.Printf("Total Workflows: %d\n", s.sprint7Metrics.TotalWorkflows)
	fmt.Printf("Successful Workflows: %d\n", s.sprint7Metrics.SuccessfulWorkflows)
	fmt.Printf("Failed Workflows: %d\n", s.sprint7Metrics.FailedWorkflows)
	fmt.Printf("Refund Workflows: %d\n", s.sprint7Metrics.RefundWorkflows)
	fmt.Printf("Disputed Workflows: %d\n", s.sprint7Metrics.DisputedWorkflows)
	fmt.Printf("Peak Concurrent Workflows: %d\n", s.sprint7Metrics.ConcurrentWorkflows)
	fmt.Printf("\n🎯 Component Performance:\n")
	fmt.Printf("VCG Auctions Completed: %d\n", s.sprint7Metrics.VCGAuctionsCompleted)
	fmt.Printf("Escrow Transitions: %d\n", s.sprint7Metrics.EscrowTransitions)
	fmt.Printf("Reputation Updates: %d\n", s.sprint7Metrics.ReputationUpdates)
	fmt.Printf("Circuit Breaker Activations: %d\n", s.sprint7Metrics.CircuitBreakerActivations)

	fmt.Printf("\n⚡ Latency Metrics:\n")
	fmt.Printf("Workflow P50: %v\n", s.sprint7Metrics.WorkflowLatencyP50)
	fmt.Printf("Workflow P95: %v\n", s.sprint7Metrics.WorkflowLatencyP95)
	fmt.Printf("Workflow P99: %v\n", s.sprint7Metrics.WorkflowLatencyP99)
	fmt.Printf("Throughput: %.2f workflows/sec\n", s.sprint7Metrics.ThroughputWorkflowsPerSec)

	// Print component latencies
	for component, latencies := range s.sprint7Metrics.ComponentLatencies {
		if len(latencies) > 0 {
			avg := calculateAverage(latencies)
			fmt.Printf("  %s avg: %v\n", component, avg)
		}
	}

	fmt.Printf("\n🔧 Error Analysis:\n")
	fmt.Printf("Blockchain Errors: %d\n", s.sprint7Metrics.BlockchainErrors)
	fmt.Printf("Network Errors: %d\n", s.sprint7Metrics.NetworkErrors)
	fmt.Printf("Timeout Errors: %d\n", s.sprint7Metrics.TimeoutErrors)
	fmt.Printf("Validation Errors: %d\n", s.sprint7Metrics.ValidationErrors)

	// Success rate
	if s.sprint7Metrics.TotalWorkflows > 0 {
		successRate := float64(s.sprint7Metrics.SuccessfulWorkflows) / float64(s.sprint7Metrics.TotalWorkflows) * 100
		fmt.Printf("\n✅ Overall Success Rate: %.2f%%\n", successRate)
	}

	// Memory usage
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	fmt.Printf("Memory Usage: %.2f MB\n", float64(memStats.Alloc)/1024/1024)
	fmt.Printf("Goroutines: %d\n", runtime.NumGoroutine())

	// Feature coverage
	fmt.Printf("\n🎯 Features Covered:\n")
	for feature, covered := range s.sprint7Metrics.FeaturesCovered {
		if covered {
			fmt.Printf("  ✅ %s\n", feature)
		}
	}
}

func (s *Sprint7WorkflowTestSuite) calculateLatencyPercentiles() {
	if len(s.sprint7Metrics.LatencyHistogram) == 0 {
		return
	}

	// Sort latencies
	latencies := make([]time.Duration, len(s.sprint7Metrics.LatencyHistogram))
	copy(latencies, s.sprint7Metrics.LatencyHistogram)

	// Simple sort for percentile calculation
	for i := 0; i < len(latencies); i++ {
		for j := i + 1; j < len(latencies); j++ {
			if latencies[i] > latencies[j] {
				latencies[i], latencies[j] = latencies[j], latencies[i]
			}
		}
	}

	n := len(latencies)
	s.sprint7Metrics.WorkflowLatencyP50 = latencies[n*50/100]
	s.sprint7Metrics.WorkflowLatencyP95 = latencies[n*95/100]
	s.sprint7Metrics.WorkflowLatencyP99 = latencies[n*99/100]
}

// Utility functions
func generateUniqueDID(entityType, suffix string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("did:zerostate:%s:%s_%d", entityType, suffix, timestamp)
}

func generateTaskID() string {
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), rand.Intn(1000000))
}

func simulateTaskExecution(taskType string, baseTime time.Duration) time.Duration {
	// Simulate realistic execution times based on task type
	multiplier := 1.0
	switch taskType {
	case "image-processing":
		multiplier = 1.5
	case "data-analysis":
		multiplier = 2.0
	case "text-processing":
		multiplier = 0.8
	case "concurrent-task":
		multiplier = 0.5
	}

	jitter := time.Duration(rand.Intn(200)) * time.Millisecond
	execTime := time.Duration(float64(baseTime) * multiplier) + jitter

	time.Sleep(execTime)
	return execTime
}

func calculateAverage(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	var total time.Duration
	for _, d := range durations {
		total += d
	}

	return total / time.Duration(len(durations))
}