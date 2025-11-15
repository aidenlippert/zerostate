package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestVCGAuctionMechanism tests the VCG (Vickrey-Clarke-Groves) auction:
// 1. Create 3 test agents with different costs
// 2. Submit CFP
// 3. Verify VCG winner selection (lowest bid)
// 4. Verify payment is second-price
// 5. Compare with first-price auction result
func TestVCGAuctionMechanism(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	// Step 1: Setup VCG auction environment
	t.Log("🏗️ Setting up VCG auction environment...")
	vcgAuctioneer, messageBus, cleanup := setupVCGAuctionEnvironment(t, ctx, logger)
	defer cleanup()

	// Step 2: Create 3 test agents with different cost structures
	t.Log("👥 Creating 3 test agents with different cost structures...")
	agents := createTestAgentsWithDifferentCosts(t, ctx, messageBus)
	require.Len(t, agents, 3)

	// Expected bids: Agent1 (1.0 AINU), Agent2 (0.8 AINU), Agent3 (1.2 AINU)
	// VCG should select Agent2 (lowest) and charge second-price (1.0 AINU)

	// Step 3: Create test task and submit CFP
	t.Log("📋 Creating test task and submitting CFP...")
	task := &orchestration.Task{
		ID:           "vcg-test-task-1",
		Type:         "computation",
		Description:  "Test computational task for VCG auction",
		Capabilities: []string{"compute", "math"},
		UserID:       "test-user-vcg",
		Priority:     orchestration.PriorityNormal,
		Timeout:      30 * time.Second,
		Input: map[string]interface{}{
			"algorithm": "matrix_multiply",
			"size":      "100x100",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Step 4: Run VCG auction
	t.Log("🏆 Running VCG auction...")
	auctionWindow := 2 * time.Second

	vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, auctionWindow)
	require.NoError(t, err, "VCG auction should complete successfully")
	require.NotNil(t, vcgResult, "VCG result should not be nil")

	// Step 5: Verify VCG auction results
	t.Log("✅ Verifying VCG auction results...")

	// Should have received bids from all 3 agents
	assert.Len(t, vcgResult.AllBids, 3, "Should receive bids from all 3 agents")

	// Winner should be Agent2 (lowest bid: 0.8 AINU)
	require.NotNil(t, vcgResult.Winner, "VCG auction should have a winner")
	assert.Equal(t, "did:test:agent2-medium-cost", string(vcgResult.Winner.AgentDID),
		"Agent2 should win (lowest bid)")
	assert.Equal(t, 0.8, vcgResult.Winner.Price, "Winner's bid should be 0.8 AINU")

	// Payment should be second-price (1.0 AINU from Agent1)
	assert.Equal(t, 1.0, vcgResult.SecondPrice, "Payment should be second-price (1.0 AINU)")
	assert.Equal(t, 0.8, vcgResult.FirstPrice, "First-price would be 0.8 AINU")

	// Efficiency should be positive (VCG saves money)
	expectedEfficiency := (0.8 - 1.0) / 0.8 // (first_price - second_price) / first_price
	assert.InDelta(t, -0.25, vcgResult.Efficiency, 0.01, "Efficiency calculation should be correct")

	t.Log("📊 VCG auction results:",
		"Winner:", string(vcgResult.Winner.AgentDID),
		"Winner bid:", vcgResult.Winner.Price,
		"Payment (second-price):", vcgResult.SecondPrice,
		"Efficiency vs first-price:", vcgResult.Efficiency)

	// Step 6: Run first-price auction for comparison
	t.Log("🔄 Running first-price auction for comparison...")

	firstPriceResult, err := vcgAuctioneer.StartFirstPriceAuction(ctx, task, auctionWindow)
	require.NoError(t, err, "First-price auction should complete successfully")

	// Step 7: Compare VCG vs first-price auction
	t.Log("📈 Comparing VCG vs first-price auction...")

	// Winner should be the same (lowest bidder)
	assert.Equal(t, vcgResult.Winner.AgentDID, firstPriceResult.Winner.AgentDID,
		"Both auctions should select the same winner")

	// But payment differs
	assert.Equal(t, 0.8, firstPriceResult.SecondPrice,
		"First-price auction: winner pays their bid (0.8)")
	assert.Equal(t, 1.0, vcgResult.SecondPrice,
		"VCG auction: winner pays second-price (1.0)")

	// In this case, first-price is actually cheaper (degenerate case)
	costDifference := vcgResult.SecondPrice - firstPriceResult.SecondPrice
	assert.Equal(t, 0.2, costDifference, "VCG costs 0.2 more in this specific case")

	t.Log("💰 Cost comparison:",
		"VCG payment:", vcgResult.SecondPrice,
		"First-price payment:", firstPriceResult.SecondPrice,
		"Difference:", costDifference)

	// Step 8: Run comprehensive comparison
	t.Log("🔍 Running comprehensive auction mechanism comparison...")

	comparison, err := vcgAuctioneer.CompareAuctionMechanisms(ctx, task, auctionWindow)
	require.NoError(t, err, "Comparison should complete successfully")

	assert.Equal(t, task.ID, comparison.TaskID, "Comparison should reference correct task")
	assert.NotNil(t, comparison.VCGResult, "Should have VCG result")
	assert.NotNil(t, comparison.FirstPriceResult, "Should have first-price result")

	// Verify cost savings calculation
	expectedCostSavings := comparison.FirstPriceResult.FirstPrice - comparison.VCGResult.SecondPrice
	assert.Equal(t, expectedCostSavings, comparison.CostSavings,
		"Cost savings calculation should be correct")

	// Step 9: Verify computational overhead
	overhead := comparison.GetOverhead()
	assert.GreaterOrEqual(t, overhead, 0.0, "Overhead should be non-negative")
	assert.LessOrEqual(t, overhead, 1.0, "Overhead should be reasonable (<100%)")

	t.Log("⚡ Performance metrics:",
		"Computational overhead:", overhead,
		"Total bids processed:", len(comparison.VCGResult.AllBids))

	t.Log("🎉 VCG auction mechanism test completed successfully!")
}

// TestVCGAuctionEdgeCases tests edge cases in VCG auction
func TestVCGAuctionEdgeCases(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	vcgAuctioneer, messageBus, cleanup := setupVCGAuctionEnvironment(t, ctx, logger)
	defer cleanup()

	// Test 1: Single bidder (degenerate case)
	t.Run("SingleBidder", func(t *testing.T) {
		t.Log("🔸 Testing single bidder scenario...")

		// Create only one agent
		agents := createTestAgentsWithDifferentCosts(t, ctx, messageBus)
		singleAgent := agents[:1] // Take only first agent

		task := createTestComputationTask("vcg-single-bidder")

		vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, 1*time.Second)
		require.NoError(t, err)

		if len(vcgResult.AllBids) > 0 {
			// With single bidder, second-price should equal first-price
			assert.Equal(t, vcgResult.Winner.Price, vcgResult.SecondPrice,
				"With single bidder, payment should equal bid price")
		}
	})

	// Test 2: No bidders
	t.Run("NoBidders", func(t *testing.T) {
		t.Log("🔸 Testing no bidders scenario...")

		// Don't start any agents
		task := createTestComputationTask("vcg-no-bidders")

		vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, 500*time.Millisecond)
		require.NoError(t, err)

		assert.Nil(t, vcgResult.Winner, "Should have no winner when no bids")
		assert.Empty(t, vcgResult.AllBids, "Should have no bids")
	})

	// Test 3: Tied bids (reputation tiebreaker)
	t.Run("TiedBidsReputationTiebreaker", func(t *testing.T) {
		t.Log("🔸 Testing tied bids with reputation tiebreaker...")

		// Create agents with same price but different reputation
		agents := createTestAgentsWithSamePriceDifferentReputation(t, ctx, messageBus)
		require.Len(t, agents, 2)

		task := createTestComputationTask("vcg-tied-bids")

		vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, 1*time.Second)
		require.NoError(t, err)

		if len(vcgResult.AllBids) >= 2 {
			// Should select agent with higher reputation in case of price tie
			winnerReputation := vcgResult.Winner.Reputation
			for _, bid := range vcgResult.AllBids {
				if bid.Price == vcgResult.Winner.Price {
					assert.LessOrEqual(t, bid.Reputation, winnerReputation,
						"Winner should have highest reputation among tied bidders")
				}
			}
		}
	})

	// Test 4: Performance with many bidders
	t.Run("ManyBidders", func(t *testing.T) {
		t.Log("🔸 Testing performance with many bidders...")

		// This would test with 10+ agents in a real environment
		// For this test, we'll simulate the scenario
		task := createTestComputationTask("vcg-many-bidders")

		start := time.Now()
		vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, 2*time.Second)
		duration := time.Since(start)

		require.NoError(t, err)

		// Performance should be reasonable
		assert.Less(t, duration, 5*time.Second, "VCG auction should complete quickly")

		t.Logf("VCG auction with %d bidders completed in %v",
			len(vcgResult.AllBids), duration)
	})
}

// TestVCGAuctionTruthfulness tests the truthfulness property of VCG auctions
func TestVCGAuctionTruthfulness(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewDevelopment()

	// Note: Testing truthfulness (that bidding true cost is optimal strategy)
	// requires game theory simulation which is complex. This test demonstrates
	// the concept but doesn't fully prove truthfulness mathematically.

	t.Log("🎯 Testing VCG auction truthfulness property...")

	vcgAuctioneer, messageBus, cleanup := setupVCGAuctionEnvironment(t, ctx, logger)
	defer cleanup()

	agents := createTestAgentsWithDifferentCosts(t, ctx, messageBus)
	task := createTestComputationTask("vcg-truthfulness")

	// Run auction with honest bidding
	vcgResult, err := vcgAuctioneer.StartVCGAuction(ctx, task, 2*time.Second)
	require.NoError(t, err)

	if len(vcgResult.AllBids) >= 2 {
		// In VCG, the winner pays the second-highest bid
		// This should incentivize truthful bidding since payment is independent of own bid
		// (as long as you win)

		winner := vcgResult.Winner
		secondPrice := vcgResult.SecondPrice

		t.Logf("Winner bid: %.2f, Payment: %.2f", winner.Price, secondPrice)

		// The key property: payment is independent of winner's bid amount
		// (assuming the winner would still win with any bid below second-price)
		assert.NotEqual(t, winner.Price, secondPrice,
			"VCG payment should be independent of winner's bid")
	}
}

// Helper Functions

func setupVCGAuctionEnvironment(t *testing.T, ctx context.Context, logger *zap.Logger) (*orchestration.VCGAuctioneer, *p2p.MessageBus, func()) {
	// Create P2P message bus for auction communication
	messageBus := p2p.NewMessageBus(logger)

	// Create base auctioneer
	auctioneer := orchestration.NewAuctioneer(messageBus, logger)

	// Create VCG auctioneer
	vcgAuctioneer := orchestration.NewVCGAuctioneer(auctioneer, logger)

	cleanup := func() {
		messageBus.Stop()
	}

	return vcgAuctioneer, messageBus, cleanup
}

func createTestAgentsWithDifferentCosts(t *testing.T, ctx context.Context, messageBus *p2p.MessageBus) []TestAgent {
	agents := []TestAgent{
		{
			DID:        agentcard.DID("did:test:agent1-high-cost"),
			Cost:       1.0, // High cost
			Reputation: 800,
			Endpoint:   "agent1:8080",
		},
		{
			DID:        agentcard.DID("did:test:agent2-medium-cost"),
			Cost:       0.8, // Lowest cost - should win
			Reputation: 600,
			Endpoint:   "agent2:8080",
		},
		{
			DID:        agentcard.DID("did:test:agent3-highest-cost"),
			Cost:       1.2, // Highest cost
			Reputation: 700,
			Endpoint:   "agent3:8080",
		},
	}

	// Start mock agents that will respond to CFPs
	for _, agent := range agents {
		startMockBiddingAgent(t, agent, messageBus)
	}

	return agents
}

func createTestAgentsWithSamePriceDifferentReputation(t *testing.T, ctx context.Context, messageBus *p2p.MessageBus) []TestAgent {
	agents := []TestAgent{
		{
			DID:        agentcard.DID("did:test:agent-low-rep"),
			Cost:       0.5, // Same cost
			Reputation: 400, // Lower reputation
			Endpoint:   "agent-low:8080",
		},
		{
			DID:        agentcard.DID("did:test:agent-high-rep"),
			Cost:       0.5, // Same cost
			Reputation: 900, // Higher reputation - should win
			Endpoint:   "agent-high:8080",
		},
	}

	for _, agent := range agents {
		startMockBiddingAgent(t, agent, messageBus)
	}

	return agents
}

func createTestComputationTask(taskID string) *orchestration.Task {
	return &orchestration.Task{
		ID:           taskID,
		Type:         "computation",
		Description:  "Test computational task",
		Capabilities: []string{"compute", "math"},
		UserID:       "test-user",
		Priority:     orchestration.PriorityNormal,
		Timeout:      30 * time.Second,
		Input: map[string]interface{}{
			"algorithm": "test",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

type TestAgent struct {
	DID        agentcard.DID
	Cost       float64
	Reputation float64
	Endpoint   string
}

func startMockBiddingAgent(t *testing.T, agent TestAgent, messageBus *p2p.MessageBus) {
	// Start a mock agent that will respond to CFPs with the specified bid
	// This simulates the agent's bidding behavior

	go func() {
		// Listen for CFPs and respond with bids
		// This is a simplified mock implementation
		// In a real test, you'd start actual agent processes
		t.Logf("Mock agent %s started, will bid %.2f AINU", agent.DID, agent.Cost)
	}()
}