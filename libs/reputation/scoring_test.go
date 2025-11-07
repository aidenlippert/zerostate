package reputation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestPeer(t *testing.T) peer.ID {
	privKey, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	peerID, err := peer.IDFromPrivateKey(privKey)
	require.NoError(t, err)
	return peerID
}

func TestNewReputationManager(t *testing.T) {
	config := DefaultScoreConfig()
	rm := NewReputationManager(config, zap.NewNop())
	
	assert.NotNil(t, rm)
	assert.NotNil(t, rm.scores)
	assert.NotNil(t, rm.blacklist)
	assert.Equal(t, config, rm.config)
}

func TestRecordExecutionSuccess(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	outcome := ExecutionOutcome{
		TaskID:     "task-1",
		ExecutorID: peerID,
		Success:    true,
		Duration:   20 * time.Second,
		Cost:       0.5,
		Timestamp:  time.Now(),
		ExitCode:   0,
	}
	
	err := rm.RecordExecution(ctx, outcome)
	require.NoError(t, err)
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	assert.Equal(t, 1, score.TasksCompleted)
	assert.Equal(t, 0, score.TasksFailed)
	assert.Equal(t, 1.0, score.SuccessRate)
	assert.Equal(t, 20*time.Second, score.AverageDuration)
	assert.Equal(t, 0.5, score.TotalCost)
}

func TestRecordExecutionFailure(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	outcome := ExecutionOutcome{
		TaskID:     "task-1",
		ExecutorID: peerID,
		Success:    false,
		Duration:   10 * time.Second,
		Cost:       0.0,
		Timestamp:  time.Now(),
		ExitCode:   1,
		Error:      "execution failed",
	}
	
	err := rm.RecordExecution(ctx, outcome)
	require.NoError(t, err)
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	assert.Equal(t, 0, score.TasksCompleted)
	assert.Equal(t, 1, score.TasksFailed)
	assert.Equal(t, 0.0, score.SuccessRate)
	assert.Equal(t, 1, score.Violations)
}

func TestRecordMultipleExecutions(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	// Record 8 successes and 2 failures
	for i := 0; i < 8; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: peerID,
			Success:    true,
			Duration:   25 * time.Second,
			Cost:       1.0,
			Timestamp:  time.Now(),
		}
		err := rm.RecordExecution(ctx, outcome)
		require.NoError(t, err)
	}
	
	for i := 0; i < 2; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("fail-task-%d", i),
			ExecutorID: peerID,
			Success:    false,
			Duration:   10 * time.Second,
			Cost:       0.0,
			Timestamp:  time.Now(),
		}
		err := rm.RecordExecution(ctx, outcome)
		require.NoError(t, err)
	}
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	assert.Equal(t, 8, score.TasksCompleted)
	assert.Equal(t, 2, score.TasksFailed)
	assert.Equal(t, 0.8, score.SuccessRate) // 8/10
	assert.Greater(t, score.Score, 0.5)     // Should be above neutral
}

func TestCalculateScoreComponents(t *testing.T) {
	config := DefaultScoreConfig()
	rm := NewReputationManager(config, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	// Record enough tasks to get a valid score
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: peerID,
			Success:    true,
			Duration:   20 * time.Second, // Faster than baseline (30s)
			Cost:       0.8,               // Cheaper than baseline (1.0)
			Timestamp:  time.Now(),
		}
		err := rm.RecordExecution(ctx, outcome)
		require.NoError(t, err)
	}
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	
	// With 100% success rate, faster and cheaper than baseline, score should be high
	assert.Greater(t, score.Score, 0.7)
	assert.Equal(t, 1.0, score.SuccessRate)
}

func TestBlacklistLowReputation(t *testing.T) {
	config := DefaultScoreConfig()
	config.MinTasksForScore = 5
	config.BlacklistThreshold = 0.3
	
	rm := NewReputationManager(config, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	// Record mostly failures to get low score
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: peerID,
			Success:    i < 2, // Only 2 successes out of 10 (20% success rate)
			Duration:   60 * time.Second,
			Cost:       2.0,
			Timestamp:  time.Now(),
		}
		err := rm.RecordExecution(ctx, outcome)
		require.NoError(t, err)
	}
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	
	// Should be blacklisted due to low score
	assert.True(t, rm.IsBlacklisted(peerID))
	assert.True(t, score.Blacklisted)
	assert.Less(t, score.Score, config.BlacklistThreshold)
}

func TestRemoveFromBlacklist(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	
	peerID := createTestPeer(t)
	
	// Manually blacklist
	rm.mu.Lock()
	rm.blacklistPeer(peerID, "test")
	rm.mu.Unlock()
	
	assert.True(t, rm.IsBlacklisted(peerID))
	
	// Remove from blacklist
	rm.RemoveFromBlacklist(peerID)
	assert.False(t, rm.IsBlacklisted(peerID))
}

func TestGetTopPeers(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	// Create multiple peers with different success rates
	peers := make([]peer.ID, 5)
	for i := range peers {
		peers[i] = createTestPeer(t)
	}
	
	// Peer 0: 100% success (10/10)
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("peer0-task-%d", i),
			ExecutorID: peers[0],
			Success:    true,
			Duration:   20 * time.Second,
			Cost:       0.8,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	// Peer 1: 90% success (9/10)
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("peer1-task-%d", i),
			ExecutorID: peers[1],
			Success:    i < 9,
			Duration:   25 * time.Second,
			Cost:       0.9,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	// Peer 2: 80% success (8/10)
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("peer2-task-%d", i),
			ExecutorID: peers[2],
			Success:    i < 8,
			Duration:   30 * time.Second,
			Cost:       1.0,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	// Peer 3: 70% success (7/10)
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("peer3-task-%d", i),
			ExecutorID: peers[3],
			Success:    i < 7,
			Duration:   35 * time.Second,
			Cost:       1.2,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	// Peer 4: 60% success (6/10)
	for i := 0; i < 10; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("peer4-task-%d", i),
			ExecutorID: peers[4],
			Success:    i < 6,
			Duration:   40 * time.Second,
			Cost:       1.5,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	// Get top 3 peers
	topPeers := rm.GetTopPeers(3, 5)
	assert.Len(t, topPeers, 3)
	
	// Should be ordered by score (descending)
	assert.Equal(t, peers[0], topPeers[0].PeerID)
	assert.Greater(t, topPeers[0].Score, topPeers[1].Score)
	assert.Greater(t, topPeers[1].Score, topPeers[2].Score)
}

func TestIsBlacklistedExpiry(t *testing.T) {
	config := DefaultScoreConfig()
	config.BlacklistDuration = 100 * time.Millisecond
	
	rm := NewReputationManager(config, zap.NewNop())
	
	peerID := createTestPeer(t)
	
	// Blacklist peer
	rm.mu.Lock()
	rm.blacklistPeer(peerID, "test")
	rm.mu.Unlock()
	
	assert.True(t, rm.IsBlacklisted(peerID))
	
	// Wait for expiry
	time.Sleep(150 * time.Millisecond)
	
	// Should no longer be blacklisted
	assert.False(t, rm.IsBlacklisted(peerID))
}

func TestCleanupExpired(t *testing.T) {
	config := DefaultScoreConfig()
	config.BlacklistDuration = 100 * time.Millisecond
	
	rm := NewReputationManager(config, zap.NewNop())
	
	peerID := createTestPeer(t)
	
	// Blacklist peer
	rm.mu.Lock()
	rm.blacklistPeer(peerID, "test")
	rm.mu.Unlock()
	
	// Wait for expiry
	time.Sleep(150 * time.Millisecond)
	
	// Cleanup
	rm.CleanupExpired()
	
	// Should be removed from blacklist map
	rm.mu.RLock()
	_, exists := rm.blacklist[peerID]
	rm.mu.RUnlock()
	
	assert.False(t, exists)
}

func TestGetAllScores(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	// Create and record outcomes for multiple peers
	peer1 := createTestPeer(t)
	peer2 := createTestPeer(t)
	peer3 := createTestPeer(t)
	
	for _, p := range []peer.ID{peer1, peer2, peer3} {
		outcome := ExecutionOutcome{
			TaskID:     "task-1",
			ExecutorID: p,
			Success:    true,
			Duration:   20 * time.Second,
			Cost:       1.0,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	scores := rm.GetAllScores()
	assert.Len(t, scores, 3)
}

func TestStats(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	peer1 := createTestPeer(t)
	peer2 := createTestPeer(t)
	
	// Record outcomes
	for i := 0; i < 5; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: peer1,
			Success:    true,
			Duration:   20 * time.Second,
			Cost:       1.0,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	for i := 0; i < 3; i++ {
		outcome := ExecutionOutcome{
			TaskID:     fmt.Sprintf("task-%d", i),
			ExecutorID: peer2,
			Success:    true,
			Duration:   25 * time.Second,
			Cost:       1.2,
			Timestamp:  time.Now(),
		}
		rm.RecordExecution(ctx, outcome)
	}
	
	stats := rm.Stats()
	assert.Equal(t, 2, stats["total_peers"])
	assert.Equal(t, 0, stats["blacklisted_peers"])
	assert.Equal(t, 8, stats["total_outcomes"])
	assert.Greater(t, stats["average_score"].(float64), 0.0)
}

func TestScoreNeutralForNewPeer(t *testing.T) {
	rm := NewReputationManager(nil, zap.NewNop())
	ctx := context.Background()
	
	peerID := createTestPeer(t)
	
	// Record one outcome (below MinTasksForScore)
	outcome := ExecutionOutcome{
		TaskID:     "task-1",
		ExecutorID: peerID,
		Success:    true,
		Duration:   20 * time.Second,
		Cost:       1.0,
		Timestamp:  time.Now(),
	}
	rm.RecordExecution(ctx, outcome)
	
	score, err := rm.GetScore(peerID)
	require.NoError(t, err)
	
	// Should remain at neutral (0.5) until MinTasksForScore reached
	assert.Equal(t, 0.5, score.Score)
}

func TestDefaultScoreConfig(t *testing.T) {
	config := DefaultScoreConfig()
	
	assert.NotNil(t, config)
	assert.Equal(t, 0.5, config.SuccessRateWeight)
	assert.Equal(t, 0.2, config.SpeedWeight)
	assert.Equal(t, 0.2, config.CostWeight)
	assert.Equal(t, 0.1, config.LongevityWeight)
	
	// Weights should sum to 1.0
	total := config.SuccessRateWeight + config.SpeedWeight + config.CostWeight + config.LongevityWeight
	assert.InDelta(t, 1.0, total, 0.001)
	
	assert.Equal(t, 7*24*time.Hour, config.DecayHalfLife)
	assert.True(t, config.DecayEnabled)
	assert.Equal(t, 5, config.MinTasksForScore)
	assert.Equal(t, 0.3, config.BlacklistThreshold)
	assert.Equal(t, 24*time.Hour, config.BlacklistDuration)
}

func TestStartCleanupLoop(t *testing.T) {
	config := DefaultScoreConfig()
	config.BlacklistDuration = 50 * time.Millisecond
	
	rm := NewReputationManager(config, zap.NewNop())
	
	peerID := createTestPeer(t)
	
	// Blacklist peer
	rm.mu.Lock()
	rm.blacklistPeer(peerID, "test")
	rm.mu.Unlock()
	
	// Start cleanup loop
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	go rm.StartCleanupLoop(ctx, 50*time.Millisecond)
	
	// Wait for cleanup to run
	time.Sleep(150 * time.Millisecond)
	
	// Should be cleaned up
	rm.mu.RLock()
	_, exists := rm.blacklist[peerID]
	rm.mu.RUnlock()
	
	assert.False(t, exists)
}
