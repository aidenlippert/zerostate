// Package e2e provides end-to-end tests for Sprint 6 Phase 4 - Payment Lifecycle MVP validation
package e2e

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
	"github.com/aidenlippert/zerostate/libs/substrate"
)

// Sprint6PaymentLifecycleTestSuite validates complete payment lifecycle for MVP
type Sprint6PaymentLifecycleTestSuite struct {
	suite.Suite
	ctx                 context.Context
	cancel              context.CancelFunc
	escrowClient       *substrate.EscrowClient
	reputationClient   *substrate.ReputationClient
	orchestrator       *orchestration.Orchestrator
	paymentService     *economic.PaymentChannelService
	reputationService  *reputation.ReputationService
	metrics           *TestMetrics
}

// TestMetrics tracks performance and quality metrics during MVP testing
type TestMetrics struct {
	mu                   sync.RWMutex
	TasksSubmitted       int64
	TasksCompleted       int64
	TasksRefunded        int64
	TasksDisputed        int64
	PaymentsReleased     int64
	EscrowsCreated       int64
	ReputationUpdates    int64
	AuctionsCompleted    int64
	LatencyP50           time.Duration
	LatencyP95           time.Duration
	ErrorRate            float64
	StartTime           time.Time
	Latencies           []time.Duration
	Errors              []error
}

func (m *TestMetrics) RecordLatency(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Latencies = append(m.Latencies, d)
}

func (m *TestMetrics) RecordError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err != nil {
		m.Errors = append(m.Errors, err)
	}
}

func (m *TestMetrics) CalculatePercentiles() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.Latencies) == 0 {
		return
	}

	// Sort latencies for percentile calculation
	latencies := make([]time.Duration, len(m.Latencies))
	copy(latencies, m.Latencies)

	for i := 0; i < len(latencies)-1; i++ {
		for j := 0; j < len(latencies)-i-1; j++ {
			if latencies[j] > latencies[j+1] {
				latencies[j], latencies[j+1] = latencies[j+1], latencies[j]
			}
		}
	}

	// Calculate P50 and P95
	p50Index := len(latencies) / 2
	p95Index := (95 * len(latencies)) / 100

	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}

	m.LatencyP50 = latencies[p50Index]
	m.LatencyP95 = latencies[p95Index]

	// Calculate error rate
	if m.TasksSubmitted > 0 {
		m.ErrorRate = float64(len(m.Errors)) / float64(m.TasksSubmitted) * 100
	}
}

func TestSprint6PaymentLifecycle(t *testing.T) {
	suite.Run(t, new(Sprint6PaymentLifecycleTestSuite))
}

func (s *Sprint6PaymentLifecycleTestSuite) SetupSuite() {
	s.ctx, s.cancel = context.WithTimeout(context.Background(), 10*time.Minute)

	// Initialize metrics
	s.metrics = &TestMetrics{
		StartTime: time.Now(),
		Latencies: make([]time.Duration, 0),
		Errors:    make([]error, 0),
	}

	// Setup blockchain client (chain-v2)
	substrateClient, err := substrate.NewClientV2("ws://localhost:9944")
	require.NoError(s.T(), err)

	// Setup keyring for test transactions
	keyring, err := substrate.CreateKeyringFromSeed("//Alice", substrate.Sr25519Type)
	require.NoError(s.T(), err)

	// Initialize escrow and reputation clients
	s.escrowClient = substrate.NewEscrowClient(substrateClient, keyring)
	s.reputationClient = substrate.NewReputationClient(substrateClient, keyring)

	// Setup services
	s.paymentService = economic.NewPaymentChannelService()
	s.reputationService = reputation.NewReputationService()

	// Initialize orchestrator with all integrations
	messageBus := &mockMessageBus{}
	s.orchestrator = orchestration.NewOrchestrator(
		orchestration.Config{
			MaxConcurrentTasks: 100,
			ReputationEnabled:  true,
			VCGEnabled:         true,
			PaymentEnabled:     true,
		},
		messageBus,
		s.paymentService,
		s.reputationService,
	)

	// Wait for blockchain to be ready
	time.Sleep(2 * time.Second)
}

func (s *Sprint6PaymentLifecycleTestSuite) TearDownSuite() {
	if s.cancel != nil {
		s.cancel()
	}
}

// TestCompletePaymentLifecycleSuccess validates the complete successful payment flow
func (s *Sprint6PaymentLifecycleTestSuite) TestCompletePaymentLifecycleSuccess() {
	t := s.T()
	ctx := s.ctx
	startTime := time.Now()

	// Step 1: User submits task with escrow (100 AINU)
	userDID := "did:zerostate:user:test_lifecycle_1"
	agentDID := "did:zerostate:agent:test_lifecycle_1"
	taskID := generateTaskID()
	escrowAmount := uint64(100_000_000) // 100 AINU (with decimals)

	// Deposit funds for user
	err := s.paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.TasksSubmitted++
	s.metrics.mu.Unlock()

	// Create escrow on blockchain
	err = s.escrowClient.CreateEscrow(ctx, taskID, escrowAmount, 100) // 100 block timeout
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.EscrowsCreated++
	s.metrics.mu.Unlock()

	// Verify escrow created
	escrow, err := s.escrowClient.GetEscrow(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStatePending, escrow.State)
	assert.Equal(t, fmt.Sprintf("%d", escrowAmount), string(escrow.Amount))

	// Step 2: Agent accepts task
	err = s.escrowClient.AcceptTask(ctx, taskID, agentDID)
	require.NoError(t, err)

	// Verify escrow state updated
	escrow, err = s.escrowClient.GetEscrow(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateAccepted, escrow.State)
	assert.NotNil(t, escrow.AgentDID)
	assert.Equal(t, agentDID, string(*escrow.AgentDID))

	// Step 3: Agent completes task (simulated)
	time.Sleep(500 * time.Millisecond) // Simulate task execution time

	// Step 4: Release payment (95 AINU to agent, 5 AINU fee)
	err = s.escrowClient.ReleasePayment(ctx, taskID)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.PaymentsReleased++
	s.metrics.mu.Unlock()

	// Verify escrow completed
	escrow, err = s.escrowClient.GetEscrow(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateCompleted, escrow.State)

	// Step 5: Update reputation for successful completion
	err = s.reputationClient.ReportOutcome(ctx, agentDID, true)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.ReputationUpdates++
	s.metrics.mu.Unlock()

	// Verify reputation increased
	score, err := s.reputationClient.GetReputationScore(ctx, agentDID)
	require.NoError(t, err)
	assert.Greater(t, score, uint64(0))

	// Step 6: Verify payment received (on local payment service)
	agentBalance, err := s.paymentService.GetBalance(ctx, agentDID)
	require.NoError(t, err)
	assert.Greater(t, agentBalance, 0.0) // Agent should have received payment

	// Record metrics
	latency := time.Since(startTime)
	s.metrics.RecordLatency(latency)
	s.metrics.mu.Lock()
	s.metrics.TasksCompleted++
	s.metrics.mu.Unlock()

	// Performance assertions
	assert.Less(t, latency, 5*time.Second, "Payment lifecycle should complete within 5 seconds")

	fmt.Printf("✅ Complete payment lifecycle success test passed in %v\n", latency)
}

// TestPaymentRefundFlow validates the refund flow when no agent accepts or timeout occurs
func (s *Sprint6PaymentLifecycleTestSuite) TestPaymentRefundFlow() {
	t := s.T()
	ctx := s.ctx
	startTime := time.Now()

	// Step 1: User submits task with escrow
	userDID := "did:zerostate:user:test_refund_1"
	taskID := generateTaskID()
	escrowAmount := uint64(100_000_000) // 100 AINU

	// Deposit funds for user
	err := s.paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.TasksSubmitted++
	s.metrics.mu.Unlock()

	// Create escrow with short timeout (10 blocks)
	err = s.escrowClient.CreateEscrow(ctx, taskID, escrowAmount, 10)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.EscrowsCreated++
	s.metrics.mu.Unlock()

	// Step 2: Simulate timeout - no agent accepts
	time.Sleep(2 * time.Second) // Wait for timeout simulation

	// Step 3: Trigger refund (timeout or manual refund)
	err = s.escrowClient.RefundEscrow(ctx, taskID)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.TasksRefunded++
	s.metrics.mu.Unlock()

	// Verify escrow refunded
	escrow, err := s.escrowClient.GetEscrow(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateRefunded, escrow.State)

	// Step 4: Verify user received refund (100 AINU back)
	userBalance, err := s.paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, userBalance) // Full refund

	// Record metrics
	latency := time.Since(startTime)
	s.metrics.RecordLatency(latency)

	fmt.Printf("✅ Payment refund flow test passed in %v\n", latency)
}

// TestPaymentDisputeFlow validates the dispute flow
func (s *Sprint6PaymentLifecycleTestSuite) TestPaymentDisputeFlow() {
	t := s.T()
	ctx := s.ctx
	startTime := time.Now()

	// Step 1: Setup task and escrow
	userDID := "did:zerostate:user:test_dispute_1"
	agentDID := "did:zerostate:agent:test_dispute_1"
	taskID := generateTaskID()
	escrowAmount := uint64(100_000_000) // 100 AINU

	// Deposit funds and create escrow
	err := s.paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)

	err = s.escrowClient.CreateEscrow(ctx, taskID, escrowAmount, 100)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.TasksSubmitted++
	s.metrics.EscrowsCreated++
	s.metrics.mu.Unlock()

	// Step 2: Agent accepts task
	err = s.escrowClient.AcceptTask(ctx, taskID, agentDID)
	require.NoError(t, err)

	// Step 3: Agent completes task with disputed result
	time.Sleep(300 * time.Millisecond) // Simulate task execution

	// Step 4: User raises dispute instead of approving payment
	err = s.escrowClient.DisputeEscrow(ctx, taskID)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.TasksDisputed++
	s.metrics.mu.Unlock()

	// Verify escrow disputed
	escrow, err := s.escrowClient.GetEscrow(ctx, taskID)
	require.NoError(t, err)
	assert.Equal(t, substrate.EscrowStateDisputed, escrow.State)

	// Step 5: Verify escrow is locked (no payments released)
	agentBalance, err := s.paymentService.GetBalance(ctx, agentDID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, agentBalance) // No payment to agent yet

	userBalance, err := s.paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, userBalance) // No refund yet

	// Record metrics
	latency := time.Since(startTime)
	s.metrics.RecordLatency(latency)

	fmt.Printf("✅ Payment dispute flow test passed in %v\n", latency)
}

// TestReputationPaymentIntegration validates reputation system integration with payments
func (s *Sprint6PaymentLifecycleTestSuite) TestReputationPaymentIntegration() {
	t := s.T()
	ctx := s.ctx

	agentDID := "did:zerostate:agent:reputation_test_1"

	// Test 1: Successful task increases reputation
	{
		userDID := "did:zerostate:user:reputation_test_1"
		taskID := generateTaskID()

		// Get initial reputation
		initialScore, err := s.reputationClient.GetReputationScore(ctx, agentDID)
		require.NoError(t, err)

		// Complete successful task
		err = s.paymentService.Deposit(ctx, userDID, 100.0)
		require.NoError(t, err)

		err = s.escrowClient.CreateEscrow(ctx, taskID, 100_000_000, 100)
		require.NoError(t, err)

		err = s.escrowClient.AcceptTask(ctx, taskID, agentDID)
		require.NoError(t, err)

		err = s.escrowClient.ReleasePayment(ctx, taskID)
		require.NoError(t, err)

		// Update reputation for success
		err = s.reputationClient.ReportOutcome(ctx, agentDID, true)
		require.NoError(t, err)
		s.metrics.mu.Lock()
		s.metrics.ReputationUpdates++
		s.metrics.mu.Unlock()

		// Verify reputation increased
		newScore, err := s.reputationClient.GetReputationScore(ctx, agentDID)
		require.NoError(t, err)
		assert.Greater(t, newScore, initialScore)
	}

	// Test 2: Failed task decreases reputation and triggers slash
	{
		userDID := "did:zerostate:user:reputation_test_2"
		taskID := generateTaskID()

		// Get reputation before failure
		beforeFailure, err := s.reputationClient.GetReputationScore(ctx, agentDID)
		require.NoError(t, err)

		// Setup failed task
		err = s.paymentService.Deposit(ctx, userDID, 100.0)
		require.NoError(t, err)

		err = s.escrowClient.CreateEscrow(ctx, taskID, 100_000_000, 100)
		require.NoError(t, err)

		err = s.escrowClient.AcceptTask(ctx, taskID, agentDID)
		require.NoError(t, err)

		// Report failure (no payment release)
		err = s.reputationClient.ReportOutcome(ctx, agentDID, false)
		require.NoError(t, err)

		// Trigger 1% slash for failure
		err = s.reputationClient.SlashSevere(ctx, agentDID, 1) // 1% slash
		require.NoError(t, err)
		s.metrics.mu.Lock()
		s.metrics.ReputationUpdates += 2 // Report + Slash
		s.metrics.mu.Unlock()

		// Verify reputation decreased
		afterFailure, err := s.reputationClient.GetReputationScore(ctx, agentDID)
		require.NoError(t, err)
		assert.Less(t, afterFailure, beforeFailure)

		// Refund user for failed task
		err = s.escrowClient.RefundEscrow(ctx, taskID)
		require.NoError(t, err)

		userBalance, err := s.paymentService.GetBalance(ctx, userDID)
		require.NoError(t, err)
		assert.Equal(t, 100.0, userBalance) // Full refund
	}

	fmt.Printf("✅ Reputation-payment integration test passed\n")
}

// TestVCGAuctionPaymentIntegration validates VCG auction with payment integration
func (s *Sprint6PaymentLifecycleTestSuite) TestVCGAuctionPaymentIntegration() {
	t := s.T()
	ctx := s.ctx
	startTime := time.Now()

	// Setup 3 agents with different bids
	userDID := "did:zerostate:user:vcg_test_1"
	agent1DID := "did:zerostate:agent:vcg_1"
	agent2DID := "did:zerostate:agent:vcg_2"
	agent3DID := "did:zerostate:agent:vcg_3"
	taskID := generateTaskID()

	// User deposits funds
	err := s.paymentService.Deposit(ctx, userDID, 200.0)
	require.NoError(t, err)

	// Simulate VCG auction with bids: [100, 150, 200] AINU
	// Expected: 100 AINU agent wins, pays second-price (150 AINU)
	bids := []orchestration.AgentBid{
		{AgentDID: agent1DID, BidAmount: 100.0, Capability: 0.9},
		{AgentDID: agent2DID, BidAmount: 150.0, Capability: 0.8},
		{AgentDID: agent3DID, BidAmount: 200.0, Capability: 0.7},
	}

	// Run VCG auction
	auction := orchestration.NewVCGAuction()
	winner, finalPrice, err := auction.RunAuction(ctx, taskID, bids)
	require.NoError(t, err)
	s.metrics.mu.Lock()
	s.metrics.AuctionsCompleted++
	s.metrics.mu.Unlock()

	// Verify VCG mechanism: lowest bidder wins, pays second price
	assert.Equal(t, agent1DID, winner.AgentDID) // Lowest bid wins
	assert.Equal(t, 150.0, finalPrice)          // Pays second-lowest price

	// Create escrow for VCG result
	escrowAmount := uint64(finalPrice * 1_000_000) // Convert to blockchain units
	err = s.escrowClient.CreateEscrow(ctx, taskID, escrowAmount, 100)
	require.NoError(t, err)

	// Winner accepts task
	err = s.escrowClient.AcceptTask(ctx, taskID, winner.AgentDID)
	require.NoError(t, err)

	// Complete task and release payment
	err = s.escrowClient.ReleasePayment(ctx, taskID)
	require.NoError(t, err)

	// Verify agent received VCG payment (150 AINU)
	agentBalance, err := s.paymentService.GetBalance(ctx, winner.AgentDID)
	require.NoError(t, err)
	assert.Equal(t, finalPrice, agentBalance)

	// Record metrics
	latency := time.Since(startTime)
	s.metrics.RecordLatency(latency)

	fmt.Printf("✅ VCG auction payment integration test passed in %v\n", latency)
	fmt.Printf("   Winner: %s, Final Price: %.2f AINU\n", winner.AgentDID, finalPrice)
}

// TestConcurrentPaymentLifecycles validates system under concurrent load
func (s *Sprint6PaymentLifecycleTestSuite) TestConcurrentPaymentLifecycles() {
	t := s.T()
	ctx := s.ctx
	startTime := time.Now()

	const concurrentTasks = 50
	var wg sync.WaitGroup
	errors := make(chan error, concurrentTasks)

	// Run concurrent payment lifecycles
	for i := 0; i < concurrentTasks; i++ {
		wg.Add(1)
		go func(taskIndex int) {
			defer wg.Done()

			userDID := fmt.Sprintf("did:zerostate:user:concurrent_%d", taskIndex)
			agentDID := fmt.Sprintf("did:zerostate:agent:concurrent_%d", taskIndex)
			taskID := generateTaskID()
			escrowAmount := uint64(100_000_000)

			taskStart := time.Now()

			// Complete payment lifecycle
			if err := s.paymentService.Deposit(ctx, userDID, 100.0); err != nil {
				errors <- fmt.Errorf("task %d deposit failed: %w", taskIndex, err)
				return
			}

			if err := s.escrowClient.CreateEscrow(ctx, taskID, escrowAmount, 100); err != nil {
				errors <- fmt.Errorf("task %d escrow creation failed: %w", taskIndex, err)
				return
			}

			if err := s.escrowClient.AcceptTask(ctx, taskID, agentDID); err != nil {
				errors <- fmt.Errorf("task %d accept failed: %w", taskIndex, err)
				return
			}

			// Simulate random task execution time
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)

			if err := s.escrowClient.ReleasePayment(ctx, taskID); err != nil {
				errors <- fmt.Errorf("task %d release failed: %w", taskIndex, err)
				return
			}

			// Record task completion
			taskLatency := time.Since(taskStart)
			s.metrics.RecordLatency(taskLatency)
			s.metrics.mu.Lock()
			s.metrics.TasksSubmitted++
			s.metrics.TasksCompleted++
			s.metrics.EscrowsCreated++
			s.metrics.PaymentsReleased++
			s.metrics.mu.Unlock()

		}(i)
	}

	// Wait for all tasks to complete
	wg.Wait()
	close(errors)

	// Check for errors
	var errorList []error
	for err := range errors {
		errorList = append(errorList, err)
		s.metrics.RecordError(err)
	}

	// Calculate metrics
	s.metrics.CalculatePercentiles()
	totalDuration := time.Since(startTime)

	// Performance assertions
	assert.Less(t, len(errorList), concurrentTasks/10, "Error rate should be < 10%") // < 10% error rate
	assert.Less(t, s.metrics.LatencyP95, 2*time.Second, "P95 latency should be < 2s")
	assert.Greater(t, float64(s.metrics.TasksCompleted)/totalDuration.Seconds(), 5.0, "Throughput should be > 5 tasks/sec")

	fmt.Printf("✅ Concurrent payment lifecycles test passed\n")
	fmt.Printf("   Tasks: %d, Completed: %d, Errors: %d\n", concurrentTasks, s.metrics.TasksCompleted, len(errorList))
	fmt.Printf("   P50: %v, P95: %v, Error Rate: %.2f%%\n",
		s.metrics.LatencyP50, s.metrics.LatencyP95, s.metrics.ErrorRate)
	fmt.Printf("   Throughput: %.2f tasks/sec\n",
		float64(s.metrics.TasksCompleted)/totalDuration.Seconds())
}

// TestFailureScenarios validates system resilience under various failure conditions
func (s *Sprint6PaymentLifecycleTestSuite) TestFailureScenarios() {
	t := s.T()
	ctx := s.ctx

	// Test 1: Blockchain disconnection during payment
	{
		// This test would require mocking blockchain disconnection
		// For now, we validate error handling exists
		userDID := "did:zerostate:user:failure_test_1"
		taskID := generateTaskID()

		err := s.paymentService.Deposit(ctx, userDID, 100.0)
		require.NoError(t, err)

		// Try to create escrow with invalid parameters to simulate failure
		err = s.escrowClient.CreateEscrow(ctx, taskID, 0, 0) // Invalid amount
		assert.Error(t, err) // Should handle error gracefully
		s.metrics.RecordError(err)
	}

	// Test 2: Double payment prevention (idempotency)
	{
		userDID := "did:zerostate:user:idempotency_test"
		agentDID := "did:zerostate:agent:idempotency_test"
		taskID := generateTaskID()

		// Setup successful payment
		err := s.paymentService.Deposit(ctx, userDID, 100.0)
		require.NoError(t, err)

		err = s.escrowClient.CreateEscrow(ctx, taskID, 100_000_000, 100)
		require.NoError(t, err)

		err = s.escrowClient.AcceptTask(ctx, taskID, agentDID)
		require.NoError(t, err)

		err = s.escrowClient.ReleasePayment(ctx, taskID)
		require.NoError(t, err)

		// Try to release payment again (should fail)
		err = s.escrowClient.ReleasePayment(ctx, taskID)
		assert.Error(t, err) // Should prevent double payment
	}

	// Test 3: Insufficient balance handling
	{
		userDID := "did:zerostate:user:insufficient_balance"

		// Try to create escrow without sufficient balance
		// This would be handled at the payment service level
		balance, err := s.paymentService.GetBalance(ctx, userDID)
		require.NoError(t, err)
		assert.Equal(t, 0.0, balance) // No balance deposited

		// Attempting to create payment channel should fail
		_, err = s.paymentService.CreateChannel(ctx, userDID, "agent-test", 100.0, "auction-test")
		assert.Error(t, err) // Should fail due to insufficient balance
		s.metrics.RecordError(err)
	}

	fmt.Printf("✅ Failure scenarios test passed\n")
}

// Utility functions

func generateTaskID() [32]byte {
	var taskID [32]byte
	for i := range taskID {
		taskID[i] = byte(rand.Intn(256))
	}
	return taskID
}

// mockMessageBus for testing
type mockMessageBus struct{}

func (m *mockMessageBus) Start(ctx context.Context) error { return nil }
func (m *mockMessageBus) Stop() error                     { return nil }
func (m *mockMessageBus) Publish(ctx context.Context, topic string, data []byte) error { return nil }
func (m *mockMessageBus) Subscribe(ctx context.Context, topic string, handler p2p.MessageHandler) error { return nil }
func (m *mockMessageBus) SendRequest(ctx context.Context, targetDID string, request []byte, timeout time.Duration) ([]byte, error) {
	return []byte("mock-response"), nil
}
func (m *mockMessageBus) RegisterRequestHandler(messageType string, handler p2p.RequestHandler) error { return nil }
func (m *mockMessageBus) GetPeerID() string { return "mock-peer-id" }