package p2p

import (
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/routing"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestQRoutingIntegration tests the Q-routing integration with P2P
func TestQRoutingIntegration(t *testing.T) {
	logger := zap.NewNop()

	// Create mock peer IDs
	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHDiVX8K8nDjgXxGD9W8F9ZGWbNj3eaBN5BK8HY6M3Mzn")
	peer3, _ := peer.Decode("12D3KooWKnDdG89FNZgMLGKfBnEm4GRUVKMgdv8kN9TnwF3nCY4K")

	candidates := []peer.ID{peer1, peer2, peer3}

	// Create Q-table
	qtable := routing.NewQTable()

	// Simulate routing history (peer2 has best performance)
	qtable.UpdateRoute(peer1, 100*time.Millisecond, true, 1000) // Average
	qtable.UpdateRoute(peer2, 30*time.Millisecond, true, 5000)  // Best (low latency, high bandwidth)
	qtable.UpdateRoute(peer3, 200*time.Millisecond, false, 500) // Worst (high latency, failure)

	// Add more samples to establish clear winner
	for i := 0; i < 10; i++ {
		qtable.UpdateRoute(peer1, 95*time.Millisecond, true, 1200)
		qtable.UpdateRoute(peer2, 28*time.Millisecond, true, 4800)
		qtable.UpdateRoute(peer3, 180*time.Millisecond, false, 600)
	}

	// Test 1: SelectBestPeer should choose peer2
	best, ok := qtable.SelectBestPeer(candidates)
	assert.True(t, ok, "SelectBestPeer should succeed")
	assert.Equal(t, peer2, best, "Should select peer2 (best Q-score)")

	// Test 2: GetTopPeers should return peers in Q-score order
	topPeers := qtable.GetTopPeers(3)
	assert.Len(t, topPeers, 3, "Should return 3 peers")
	assert.Equal(t, peer2, topPeers[0], "peer2 should be ranked #1")
	assert.Equal(t, peer1, topPeers[1], "peer1 should be ranked #2")
	assert.Equal(t, peer3, topPeers[2], "peer3 should be ranked #3")

	// Test 3: GetQValue should return peer metrics
	qval, exists := qtable.GetQValue(peer2)
	assert.True(t, exists, "peer2 should exist in Q-table")
	assert.Greater(t, qval.SuccessRate, 0.9, "peer2 should have high success rate")
	assert.Less(t, qval.AvgLatency, 0.05, "peer2 should have low latency (<50ms)")

	logger.Info("Q-routing integration test passed")
}

// TestQRoutingLearning tests that Q-values improve with positive feedback
func TestQRoutingLearning(t *testing.T) {
	qtable := routing.NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")

	// Initial Q-score (neutral for unknown peer)
	initialQVal, _ := qtable.GetQValue(peer1)
	initialScore := 0.0
	if initialQVal != nil {
		initialScore = initialQVal.QScore
	}

	// Simulate successful routing (should increase Q-score)
	for i := 0; i < 20; i++ {
		qtable.UpdateRoute(peer1, 50*time.Millisecond, true, 10000)
	}

	// Q-score should have improved
	finalQVal, exists := qtable.GetQValue(peer1)
	assert.True(t, exists, "peer1 should exist after updates")
	assert.Greater(t, finalQVal.QScore, initialScore, "Q-score should increase with positive feedback")
	assert.Greater(t, finalQVal.SuccessRate, 0.9, "Success rate should be high")
	assert.Less(t, finalQVal.AvgLatency, 0.1, "Latency should be stable")

	t.Logf("Q-score improved from %.3f to %.3f", initialScore, finalQVal.QScore)
}

// TestQRoutingFailureHandling tests Q-values decrease with failures
func TestQRoutingFailureHandling(t *testing.T) {
	qtable := routing.NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")

	// Start with good performance
	for i := 0; i < 10; i++ {
		qtable.UpdateRoute(peer1, 50*time.Millisecond, true, 5000)
	}

	goodQVal, _ := qtable.GetQValue(peer1)
	goodScore := goodQVal.QScore

	// Simulate failures (should decrease Q-score)
	for i := 0; i < 10; i++ {
		qtable.UpdateRoute(peer1, 500*time.Millisecond, false, 0)
	}

	badQVal, exists := qtable.GetQValue(peer1)
	assert.True(t, exists, "peer1 should still exist")
	assert.Less(t, badQVal.QScore, goodScore, "Q-score should decrease with failures")
	assert.Less(t, badQVal.SuccessRate, 0.6, "Success rate should drop")
	assert.Greater(t, badQVal.AvgLatency, 0.2, "Latency should increase")

	t.Logf("Q-score degraded from %.3f to %.3f after failures", goodScore, badQVal.QScore)
}

// TestQRoutingPruning tests stale route pruning
func TestQRoutingPruning(t *testing.T) {
	qtable := routing.NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHDiVX8K8nDjgXxGD9W8F9ZGWbNj3eaBN5BK8HY6M3Mzn")

	// Add routes
	qtable.UpdateRoute(peer1, 50*time.Millisecond, true, 5000)
	qtable.UpdateRoute(peer2, 60*time.Millisecond, true, 4000)

	// Both should exist
	stats := qtable.Stats()
	assert.Equal(t, 2, stats["total_peers"], "Should have 2 peers")

	// Prune stale routes (max age = 0 should prune all)
	pruned := qtable.PruneStale(0)
	assert.Equal(t, 2, pruned, "Should prune both peers")

	// Both should be gone
	stats = qtable.Stats()
	assert.Equal(t, 0, stats["total_peers"], "Should have 0 peers after pruning")
}

// TestQRoutingStats tests statistics collection
func TestQRoutingStats(t *testing.T) {
	qtable := routing.NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHDiVX8K8nDjgXxGD9W8F9ZGWbNj3eaBN5BK8HY6M3Mzn")

	// Add diverse performance data
	qtable.UpdateRoute(peer1, 40*time.Millisecond, true, 10000)  // Fast, successful
	qtable.UpdateRoute(peer2, 150*time.Millisecond, false, 1000) // Slow, failed

	stats := qtable.Stats()
	assert.Equal(t, 2, stats["total_peers"], "Should track 2 peers")
	assert.Contains(t, stats, "avg_q_score", "Should include avg Q-score")
	assert.Contains(t, stats, "avg_latency", "Should include avg latency")
	assert.Contains(t, stats, "avg_success_rate", "Should include avg success rate")

	t.Logf("Q-routing stats: %+v", stats)
}

// TestQRoutingExploration tests epsilon-greedy exploration
func TestQRoutingExploration(t *testing.T) {
	qtable := routing.NewQTable()

	// Create 5 peers with varying performance
	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHDiVX8K8nDjgXxGD9W8F9ZGWbNj3eaBN5BK8HY6M3Mzn")
	peer3, _ := peer.Decode("12D3KooWKnDdG89FNZgMLGKfBnEm4GRUVKMgdv8kN9TnwF3nCY4K")
	peer4, _ := peer.Decode("12D3KooWPjceQrSwdWXPyLLeABRXmuqt69Rg3sBYUc1dL7ScrBGW")
	peer5, _ := peer.Decode("12D3KooWQYhTNQdmr3ArTeUHRYzFg94BKyTkoWBDYxBEqCfKYqVy")

	peers := []peer.ID{peer1, peer2, peer3, peer4, peer5}

	// Give different Q-scores
	qtable.UpdateRoute(peer1, 30*time.Millisecond, true, 10000) // Best
	qtable.UpdateRoute(peer2, 50*time.Millisecond, true, 8000)
	qtable.UpdateRoute(peer3, 80*time.Millisecond, true, 5000)
	qtable.UpdateRoute(peer4, 120*time.Millisecond, true, 3000)
	qtable.UpdateRoute(peer5, 200*time.Millisecond, true, 1000) // Worst

	// Run selection many times
	selections := make(map[peer.ID]int)
	for i := 0; i < 100; i++ {
		selected, ok := qtable.SelectBestPeer(peers)
		assert.True(t, ok, "Should always select a peer")
		selections[selected]++
	}

	// Best peer (peer1) should be selected most often
	assert.Greater(t, selections[peer1], 50, "Best peer should be selected majority of time")
	t.Logf("Selection distribution: peer1=%d, peer2=%d, peer3=%d, peer4=%d, peer5=%d",
		selections[peer1], selections[peer2], selections[peer3], selections[peer4], selections[peer5])
}

// BenchmarkQRoutingSelection benchmarks peer selection performance
func BenchmarkQRoutingSelection(b *testing.B) {
	qtable := routing.NewQTable()

	// Create 100 peers with random performance
	peers := make([]peer.ID, 100)
	for i := 0; i < 100; i++ {
		p, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
		peers[i] = p

		latency := time.Duration(10+i) * time.Millisecond
		qtable.UpdateRoute(p, latency, true, 5000)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qtable.SelectBestPeer(peers)
	}
}

// BenchmarkQRoutingUpdate benchmarks Q-value updates
func BenchmarkQRoutingUpdate(b *testing.B) {
	qtable := routing.NewQTable()
	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qtable.UpdateRoute(peer1, 50*time.Millisecond, true, 5000)
	}
}
