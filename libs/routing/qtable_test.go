package routing

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestQTableBasics(t *testing.T) {
	qt := NewQTable()

	// Create test peer IDs
	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHBzpKqavMA5X5qAi4vkpNLqHNqXQqpXzU8QnHSPCz3wF")

	// Test initial update
	qt.UpdateRoute(peer1, 10*time.Millisecond, true, 1024)
	
	qval, exists := qt.GetQValue(peer1)
	if !exists {
		t.Fatal("Expected Q-value to exist after update")
	}

	if qval.SampleCount != 1 {
		t.Errorf("Expected sample count 1, got %d", qval.SampleCount)
	}

	// Success rate uses exponential moving average with alpha=0.3
	// First update: (1-0.3)*0 + 0.3*1.0 = 0.3
	if qval.SuccessRate < 0.29 || qval.SuccessRate > 0.31 {
		t.Errorf("Expected success rate ~0.3, got %f", qval.SuccessRate)
	}

	// Test multiple updates
	qt.UpdateRoute(peer1, 20*time.Millisecond, true, 2048)
	qt.UpdateRoute(peer1, 15*time.Millisecond, false, 0)

	qval, _ = qt.GetQValue(peer1)
	if qval.SampleCount != 3 {
		t.Errorf("Expected sample count 3, got %d", qval.SampleCount)
	}

	// Test peer selection
	qt.UpdateRoute(peer2, 5*time.Millisecond, true, 4096)
	
	bestPeer, found := qt.SelectBestPeer([]peer.ID{peer1, peer2})
	if !found {
		t.Fatal("Expected to find best peer")
	}

	// peer2 should be better (lower latency, higher success)
	if bestPeer != peer2 {
		t.Errorf("Expected peer2 to be best, got %s", bestPeer)
	}
}

func TestQTableTopPeers(t *testing.T) {
	qt := NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHBzpKqavMA5X5qAi4vkpNLqHNqXQqpXzU8QnHSPCz3wF")
	peer3, _ := peer.Decode("12D3KooWMn1TSCsei6ExHs6Kd69odyEA2MqFbRNeCWScQHbPAiYt")

	// Create different quality peers
	qt.UpdateRoute(peer1, 50*time.Millisecond, true, 1000)  // Medium
	qt.UpdateRoute(peer2, 10*time.Millisecond, true, 5000)  // Best
	qt.UpdateRoute(peer3, 100*time.Millisecond, false, 100) // Worst

	topPeers := qt.GetTopPeers(2)
	if len(topPeers) != 2 {
		t.Errorf("Expected 2 top peers, got %d", len(topPeers))
	}

	// peer2 should be first
	if topPeers[0] != peer2 {
		t.Errorf("Expected peer2 as top peer, got %s", topPeers[0])
	}
}

func TestQTablePruning(t *testing.T) {
	qt := NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	
	qt.UpdateRoute(peer1, 10*time.Millisecond, true, 1024)
	
	// Manually set old timestamp
	qt.mu.Lock()
	qt.entries[peer1].LastUpdate = time.Now().Add(-2 * time.Hour)
	qt.mu.Unlock()

	pruned := qt.PruneStale(1 * time.Hour)
	if pruned != 1 {
		t.Errorf("Expected 1 pruned entry, got %d", pruned)
	}

	_, exists := qt.GetQValue(peer1)
	if exists {
		t.Error("Expected peer to be pruned")
	}
}

func TestQTableStats(t *testing.T) {
	qt := NewQTable()

	peer1, _ := peer.Decode("12D3KooWEyoppNCUx8Yx66oV9fJnriXwCcXwDDUA2kj6vnc6iDEp")
	peer2, _ := peer.Decode("12D3KooWHBzpKqavMA5X5qAi4vkpNLqHNqXQqpXzU8QnHSPCz3wF")

	qt.UpdateRoute(peer1, 10*time.Millisecond, true, 1024)
	qt.UpdateRoute(peer2, 20*time.Millisecond, true, 2048)

	stats := qt.Stats()
	
	totalPeers, ok := stats["total_peers"].(int)
	if !ok || totalPeers != 2 {
		t.Errorf("Expected 2 total peers, got %v", stats["total_peers"])
	}

	if stats["avg_success_rate"] == nil {
		t.Error("Expected avg_success_rate in stats")
	}
}
