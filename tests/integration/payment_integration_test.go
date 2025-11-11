package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/marketplace"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
)

// TestPaymentChannelBasics tests basic payment channel operations
func TestPaymentChannelBasics(t *testing.T) {
	ctx := context.Background()

	// Create payment service
	paymentService := economic.NewPaymentChannelService()

	// Test 1: Deposit
	userDID := "did:zerostate:user:test1"
	depositAmount := 100.0

	err := paymentService.Deposit(ctx, userDID, depositAmount)
	require.NoError(t, err)

	balance, err := paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, depositAmount, balance)

	// Test 2: Create payment channel
	agentDID := "did:zerostate:agent:test1"
	channelAmount := 50.0

	channel, err := paymentService.CreateChannel(ctx, userDID, agentDID, channelAmount, "auction-1")
	require.NoError(t, err)
	assert.NotEmpty(t, channel.ID)
	assert.Equal(t, userDID, channel.PayerDID)
	assert.Equal(t, agentDID, channel.PayeeDID)
	assert.Equal(t, channelAmount, channel.TotalDeposit)
	assert.Equal(t, economic.ChannelStateOpen, channel.State)

	// Verify balance deducted
	balance, err = paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, depositAmount-channelAmount, balance)

	// Test 3: Lock escrow
	taskID := "task-1"
	err = paymentService.LockEscrow(ctx, channel.ID, taskID, channelAmount)
	require.NoError(t, err)

	// Verify channel state
	channel, err = paymentService.GetChannel(ctx, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, economic.ChannelStateEscrowed, channel.State)
	assert.Equal(t, channelAmount, channel.EscrowedAmount)

	// Test 4: Release escrow (success)
	err = paymentService.ReleaseEscrow(ctx, channel.ID, taskID, true)
	require.NoError(t, err)

	// Verify agent received payment
	agentBalance, err := paymentService.GetBalance(ctx, agentDID)
	require.NoError(t, err)
	assert.Equal(t, channelAmount, agentBalance)

	// Test 5: Close channel
	err = paymentService.CloseChannel(ctx, channel.ID)
	require.NoError(t, err)

	channel, err = paymentService.GetChannel(ctx, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, economic.ChannelStateClosed, channel.State)
}

// TestPaymentChannelRefund tests escrow release with task failure (refund)
func TestPaymentChannelRefund(t *testing.T) {
	ctx := context.Background()

	paymentService := economic.NewPaymentChannelService()

	// Setup
	userDID := "did:zerostate:user:test2"
	agentDID := "did:zerostate:agent:test2"

	err := paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)

	channel, err := paymentService.CreateChannel(ctx, userDID, agentDID, 50.0, "auction-2")
	require.NoError(t, err)

	err = paymentService.LockEscrow(ctx, channel.ID, "task-2", 50.0)
	require.NoError(t, err)

	// Release escrow with failure (refund user)
	err = paymentService.ReleaseEscrow(ctx, channel.ID, "task-2", false)
	require.NoError(t, err)

	// Verify user received refund
	channel, err = paymentService.GetChannel(ctx, channel.ID)
	require.NoError(t, err)
	assert.Equal(t, 50.0, channel.PendingRefund)

	// Close channel
	err = paymentService.CloseChannel(ctx, channel.ID)
	require.NoError(t, err)

	// Verify user balance restored
	balance, err := paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, balance)

	// Verify agent received nothing
	agentBalance, err := paymentService.GetBalance(ctx, agentDID)
	require.NoError(t, err)
	assert.Equal(t, 0.0, agentBalance)
}

// TestPaymentIdempotency tests idempotency of escrow release
func TestPaymentIdempotency(t *testing.T) {
	ctx := context.Background()

	paymentService := economic.NewPaymentChannelService()

	// Setup
	userDID := "did:zerostate:user:test3"
	agentDID := "did:zerostate:agent:test3"

	err := paymentService.Deposit(ctx, userDID, 100.0)
	require.NoError(t, err)

	channel, err := paymentService.CreateChannel(ctx, userDID, agentDID, 50.0, "auction-3")
	require.NoError(t, err)

	err = paymentService.LockEscrow(ctx, channel.ID, "task-3", 50.0)
	require.NoError(t, err)

	// Release escrow (first time)
	err = paymentService.ReleaseEscrow(ctx, channel.ID, "task-3", true)
	require.NoError(t, err)

	// Try to release again (should fail with idempotency error)
	err = paymentService.ReleaseEscrow(ctx, channel.ID, "task-3", true)
	assert.ErrorIs(t, err, economic.ErrEscrowAlreadyReleased)

	// Verify agent balance didn't double
	agentBalance, err := paymentService.GetBalance(ctx, agentDID)
	require.NoError(t, err)
	assert.Equal(t, 50.0, agentBalance)
}

// TestBalanceInvariant tests balance invariant verification
func TestBalanceInvariant(t *testing.T) {
	ctx := context.Background()

	paymentService := economic.NewPaymentChannelService()

	// Perform various operations
	err := paymentService.Deposit(ctx, "user-1", 100.0)
	require.NoError(t, err)

	err = paymentService.Deposit(ctx, "user-2", 200.0)
	require.NoError(t, err)

	channel1, err := paymentService.CreateChannel(ctx, "user-1", "agent-1", 50.0, "auction-1")
	require.NoError(t, err)

	channel2, err := paymentService.CreateChannel(ctx, "user-2", "agent-2", 100.0, "auction-2")
	require.NoError(t, err)

	err = paymentService.LockEscrow(ctx, channel1.ID, "task-1", 50.0)
	require.NoError(t, err)

	err = paymentService.ReleaseEscrow(ctx, channel1.ID, "task-1", true)
	require.NoError(t, err)

	// Verify balance invariant holds
	err = paymentService.VerifyBalanceInvariant()
	assert.NoError(t, err)
}

// TestMarketplacePaymentIntegration tests marketplace with payment integration
func TestMarketplacePaymentIntegration(t *testing.T) {
	ctx := context.Background()

	// Setup services
	paymentService := economic.NewPaymentChannelService()
	messageBus := &mockPaymentMessageBus{}
	reputationService := reputation.NewReputationService()
	discoveryService := marketplace.NewDiscoveryService(messageBus, reputationService)
	auctionService := marketplace.NewAuctionService(messageBus)
	marketplaceService := marketplace.NewMarketplaceService(
		discoveryService,
		auctionService,
		messageBus,
		reputationService,
	)
	paymentMarketplace := marketplace.NewPaymentMarketplaceService(
		marketplaceService,
		paymentService,
		reputationService,
	)

	// Setup user and agents
	userDID := "did:zerostate:user:marketplace1"
	agent1DID := "did:zerostate:agent:marketplace1"
	agent2DID := "did:zerostate:agent:marketplace2"

	// Deposit funds
	err := paymentService.Deposit(ctx, userDID, 1000.0)
	require.NoError(t, err)

	// Register agents
	agent1 := &identity.AgentCard{
		DID:          agent1DID,
		Name:         "Test Agent 1",
		Capabilities: []string{"image-processing"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = discoveryService.RegisterAgent(ctx, agent1)
	require.NoError(t, err)

	agent2 := &identity.AgentCard{
		DID:          agent2DID,
		Name:         "Test Agent 2",
		Capabilities: []string{"image-processing"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	err = discoveryService.RegisterAgent(ctx, agent2)
	require.NoError(t, err)

	// Create auction request
	req := &marketplace.AuctionRequest{
		TaskID:          "task-marketplace-1",
		UserID:          userDID,
		Capabilities:    []string{"image-processing"},
		TaskType:        "batch-resize",
		MaxPrice:        100.0,
		Timeout:         5 * time.Second,
		AuctionDuration: 2 * time.Second,
		AuctionType:     marketplace.AuctionTypeSecondPrice,
	}

	// Submit bids (simulated)
	go func() {
		time.Sleep(500 * time.Millisecond)
		marketplaceService.SubmitBidForAgent(ctx, req.TaskID, agent1DID, 80.0, 5*time.Second)
		marketplaceService.SubmitBidForAgent(ctx, req.TaskID, agent2DID, 70.0, 6*time.Second)
	}()

	// Allocate task with payment
	allocation, channelID, err := paymentMarketplace.AllocateTaskWithPayment(ctx, req)
	require.NoError(t, err)
	assert.NotEmpty(t, allocation.AuctionID)
	assert.NotEmpty(t, allocation.WinnerDID)
	assert.NotEmpty(t, channelID)

	// Verify user balance deducted
	balance, err := paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 1000.0-allocation.FinalPrice, balance)

	// Verify payment channel created
	channel, err := paymentService.GetChannel(ctx, channelID)
	require.NoError(t, err)
	assert.Equal(t, economic.ChannelStateEscrowed, channel.State)

	// Complete task with success
	err = paymentMarketplace.CompleteTaskWithPayment(ctx, req.TaskID, allocation.WinnerDID, true)
	require.NoError(t, err)

	// Verify agent received payment
	winnerBalance, err := paymentService.GetBalance(ctx, allocation.WinnerDID)
	require.NoError(t, err)
	assert.Equal(t, allocation.FinalPrice, winnerBalance)

	// Verify channel closed
	channel, err = paymentService.GetChannel(ctx, channelID)
	require.NoError(t, err)
	assert.Equal(t, economic.ChannelStateClosed, channel.State)
}

// TestDAGPaymentSplitting tests payment splitting for DAG workflows
func TestDAGPaymentSplitting(t *testing.T) {
	ctx := context.Background()

	// Setup services
	paymentService := economic.NewPaymentChannelService()
	reputationService := reputation.NewReputationService()
	paymentSplitting := marketplace.NewPaymentSplittingService(paymentService, reputationService)

	// Setup user and agents
	userDID := "did:zerostate:user:dag1"
	agent1DID := "did:zerostate:agent:dag1"
	agent2DID := "did:zerostate:agent:dag2"
	agent3DID := "did:zerostate:agent:dag3"

	// Deposit funds
	err := paymentService.Deposit(ctx, userDID, 1000.0)
	require.NoError(t, err)

	// Create payment splits (3 agents, equal split)
	totalPayment := 300.0
	splits := []marketplace.PaymentSplit{
		{AgentDID: agent1DID, Ratio: 0.333, Amount: 100.0, TaskID: "task-1", Success: true},
		{AgentDID: agent2DID, Ratio: 0.333, Amount: 100.0, TaskID: "task-2", Success: true},
		{AgentDID: agent3DID, Ratio: 0.334, Amount: 100.0, TaskID: "task-3", Success: true},
	}

	req := &marketplace.DAGPaymentRequest{
		WorkflowID:   "workflow-1",
		UserID:       userDID,
		TotalPayment: totalPayment,
		Splits:       splits,
	}

	// Execute DAG payment
	result, err := paymentSplitting.ExecuteDAGPayment(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, 3, result.SuccessfulSplits)
	assert.Equal(t, 0, result.FailedSplits)
	assert.Equal(t, totalPayment, result.TotalPaid)

	// Verify each agent received their split
	balance1, err := paymentService.GetBalance(ctx, agent1DID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, balance1)

	balance2, err := paymentService.GetBalance(ctx, agent2DID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, balance2)

	balance3, err := paymentService.GetBalance(ctx, agent3DID)
	require.NoError(t, err)
	assert.Equal(t, 100.0, balance3)

	// Verify user balance
	userBalance, err := paymentService.GetBalance(ctx, userDID)
	require.NoError(t, err)
	assert.Equal(t, 1000.0-totalPayment, userBalance)
}

// TestAtomicDAGPayment tests atomic DAG payment (all-or-nothing)
func TestAtomicDAGPayment(t *testing.T) {
	ctx := context.Background()

	// Setup services
	paymentService := economic.NewPaymentChannelService()
	reputationService := reputation.NewReputationService()
	paymentSplitting := marketplace.NewPaymentSplittingService(paymentService, reputationService)

	// Test 1: All tasks succeed - all agents get paid
	{
		userDID := "did:zerostate:user:atomic1"
		agent1DID := "did:zerostate:agent:atomic1"
		agent2DID := "did:zerostate:agent:atomic2"

		err := paymentService.Deposit(ctx, userDID, 1000.0)
		require.NoError(t, err)

		splits := []marketplace.PaymentSplit{
			{AgentDID: agent1DID, Ratio: 0.5, Amount: 50.0, TaskID: "task-1", Success: true},
			{AgentDID: agent2DID, Ratio: 0.5, Amount: 50.0, TaskID: "task-2", Success: true},
		}

		req := &marketplace.DAGPaymentRequest{
			WorkflowID:   "workflow-atomic-1",
			UserID:       userDID,
			TotalPayment: 100.0,
			Splits:       splits,
		}

		result, err := paymentSplitting.ExecuteAtomicDAGPayment(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 2, result.SuccessfulSplits)
		assert.Equal(t, 100.0, result.TotalPaid)

		// Verify both agents paid
		balance1, _ := paymentService.GetBalance(ctx, agent1DID)
		assert.Equal(t, 50.0, balance1)

		balance2, _ := paymentService.GetBalance(ctx, agent2DID)
		assert.Equal(t, 50.0, balance2)
	}

	// Test 2: One task fails - no agents get paid
	{
		userDID := "did:zerostate:user:atomic2"
		agent1DID := "did:zerostate:agent:atomic3"
		agent2DID := "did:zerostate:agent:atomic4"

		err := paymentService.Deposit(ctx, userDID, 1000.0)
		require.NoError(t, err)

		splits := []marketplace.PaymentSplit{
			{AgentDID: agent1DID, Ratio: 0.5, Amount: 50.0, TaskID: "task-3", Success: true},
			{AgentDID: agent2DID, Ratio: 0.5, Amount: 50.0, TaskID: "task-4", Success: false}, // FAILURE
		}

		req := &marketplace.DAGPaymentRequest{
			WorkflowID:   "workflow-atomic-2",
			UserID:       userDID,
			TotalPayment: 100.0,
			Splits:       splits,
		}

		result, err := paymentSplitting.ExecuteAtomicDAGPayment(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, 0, result.SuccessfulSplits)
		assert.Equal(t, 2, result.FailedSplits)
		assert.Equal(t, 0.0, result.TotalPaid)

		// Verify no agents paid
		balance1, _ := paymentService.GetBalance(ctx, agent1DID)
		assert.Equal(t, 0.0, balance1)

		balance2, _ := paymentService.GetBalance(ctx, agent2DID)
		assert.Equal(t, 0.0, balance2)

		// Verify user refunded
		userBalance, _ := paymentService.GetBalance(ctx, userDID)
		assert.Equal(t, 1000.0, userBalance)
	}
}

// TestCalculateSplitsFromDAG tests split calculation from DAG workflow
func TestCalculateSplitsFromDAG(t *testing.T) {
	ctx := context.Background()

	paymentService := economic.NewPaymentChannelService()
	reputationService := reputation.NewReputationService()
	paymentSplitting := marketplace.NewPaymentSplittingService(paymentService, reputationService)

	// Create mock DAG workflow result
	workflow := &orchestration.DAGWorkflow{
		Tasks: map[string]*orchestration.DAGTask{
			"task-1": {ID: "task-1"},
			"task-2": {ID: "task-2"},
			"task-3": {ID: "task-3"},
		},
	}

	result := &orchestration.WorkflowResult{
		WorkflowID: "workflow-1",
		Success:    true,
		TaskResults: map[string]*orchestration.TaskResult{
			"task-1": {AgentDID: "agent-1", Success: true},
			"task-2": {AgentDID: "agent-2", Success: true},
			"task-3": {AgentDID: "agent-3", Success: true},
		},
	}

	totalPayment := 300.0

	splits, err := paymentSplitting.CalculateSplitsFromDAG(ctx, workflow, result, totalPayment)
	require.NoError(t, err)
	assert.Equal(t, 3, len(splits))

	// Verify equal split (simple strategy)
	expectedRatio := 1.0 / 3.0
	for _, split := range splits {
		assert.InDelta(t, expectedRatio, split.Ratio, 0.001)
		assert.InDelta(t, 100.0, split.Amount, 0.1)
		assert.True(t, split.Success)
	}

	// Verify ratios sum to 1.0
	totalRatio := 0.0
	for _, split := range splits {
		totalRatio += split.Ratio
	}
	assert.InDelta(t, 1.0, totalRatio, 0.001)
}

// Mock message bus for payment tests
type mockPaymentMessageBus struct{}

func (m *mockPaymentMessageBus) Start(ctx context.Context) error { return nil }
func (m *mockPaymentMessageBus) Stop() error                     { return nil }
func (m *mockPaymentMessageBus) Publish(ctx context.Context, topic string, data []byte) error {
	return nil
}
func (m *mockPaymentMessageBus) Subscribe(ctx context.Context, topic string, handler p2p.MessageHandler) error {
	return nil
}
func (m *mockPaymentMessageBus) SendRequest(ctx context.Context, targetDID string, request []byte, timeout time.Duration) ([]byte, error) {
	return []byte("pong"), nil
}
func (m *mockPaymentMessageBus) RegisterRequestHandler(messageType string, handler p2p.RequestHandler) error {
	return nil
}
func (m *mockPaymentMessageBus) GetPeerID() string {
	return "test-peer-id"
}
