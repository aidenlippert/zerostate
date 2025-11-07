package reputation

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	reputationScoreGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "reputation_score",
		Help: "Current reputation score for a peer",
	}, []string{"peer_id"})
	
	tasksExecutedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reputation_tasks_executed_total",
		Help: "Total number of tasks executed by peer",
	}, []string{"peer_id", "success"})
	
	trustEventsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "reputation_trust_events_total",
		Help: "Total number of trust events recorded",
	}, []string{"event_type"})
	
	blacklistedPeersGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "reputation_blacklisted_peers",
		Help: "Number of currently blacklisted peers",
	})
)

// ExecutionOutcome represents the result of a task execution
type ExecutionOutcome struct {
	TaskID     string
	ExecutorID peer.ID
	Success    bool
	Duration   time.Duration
	Cost       float64
	Timestamp  time.Time
	
	// Quality metrics
	ExitCode   int
	Error      string
	MemoryUsed uint64
}

// ReputationScore represents a peer's reputation
type ReputationScore struct {
	PeerID           peer.ID
	Score            float64   // 0.0 to 1.0
	TasksCompleted   int
	TasksFailed      int
	SuccessRate      float64
	AverageDuration  time.Duration
	TotalCost        float64
	LastUpdated      time.Time
	FirstSeen        time.Time
	
	// Trust metrics
	Blacklisted      bool
	BlacklistedUntil time.Time
	Violations       int
}

// ScoreConfig holds configuration for reputation scoring
type ScoreConfig struct {
	// Weights for different factors (must sum to 1.0)
	SuccessRateWeight  float64 // e.g., 0.5
	SpeedWeight        float64 // e.g., 0.2
	CostWeight         float64 // e.g., 0.2
	LongevityWeight    float64 // e.g., 0.1
	
	// Decay parameters
	DecayHalfLife      time.Duration // How long for score to decay by half
	DecayEnabled       bool
	
	// Thresholds
	MinTasksForScore   int     // Min tasks before score is considered valid
	BlacklistThreshold float64 // Score below which peer gets blacklisted
	BlacklistDuration  time.Duration
	
	// Performance baselines
	BaselineDuration   time.Duration // Expected task duration
	BaselineCost       float64       // Expected task cost
}

// DefaultScoreConfig returns default scoring configuration
func DefaultScoreConfig() *ScoreConfig {
	return &ScoreConfig{
		SuccessRateWeight:  0.50,
		SpeedWeight:        0.20,
		CostWeight:         0.20,
		LongevityWeight:    0.10,
		DecayHalfLife:      7 * 24 * time.Hour, // 1 week
		DecayEnabled:       true,
		MinTasksForScore:   5,
		BlacklistThreshold: 0.3,
		BlacklistDuration:  24 * time.Hour,
		BaselineDuration:   30 * time.Second,
		BaselineCost:       1.0,
	}
}

// ReputationManager manages peer reputation scores
type ReputationManager struct {
	config    *ScoreConfig
	scores    map[peer.ID]*ReputationScore
	outcomes  []ExecutionOutcome // Recent outcomes for analysis
	logger    *zap.Logger
	mu        sync.RWMutex
	
	// Blacklist
	blacklist map[peer.ID]time.Time
}

// NewReputationManager creates a new reputation manager
func NewReputationManager(config *ScoreConfig, logger *zap.Logger) *ReputationManager {
	if config == nil {
		config = DefaultScoreConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &ReputationManager{
		config:    config,
		scores:    make(map[peer.ID]*ReputationScore),
		outcomes:  make([]ExecutionOutcome, 0),
		blacklist: make(map[peer.ID]time.Time),
		logger:    logger,
	}
}

// RecordExecution records the outcome of a task execution and updates reputation
func (rm *ReputationManager) RecordExecution(ctx context.Context, outcome ExecutionOutcome) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	// Store outcome for historical analysis
	rm.outcomes = append(rm.outcomes, outcome)
	
	// Get or create reputation score
	score, exists := rm.scores[outcome.ExecutorID]
	if !exists {
		score = &ReputationScore{
			PeerID:    outcome.ExecutorID,
			Score:     0.5, // Start at neutral
			FirstSeen: time.Now(),
		}
		rm.scores[outcome.ExecutorID] = score
	}
	
	// Update task counters
	if outcome.Success {
		score.TasksCompleted++
		tasksExecutedTotal.WithLabelValues(outcome.ExecutorID.String(), "success").Inc()
	} else {
		score.TasksFailed++
		tasksExecutedTotal.WithLabelValues(outcome.ExecutorID.String(), "failure").Inc()
		score.Violations++
		trustEventsTotal.WithLabelValues("failure").Inc()
	}
	
	// Update success rate
	totalTasks := score.TasksCompleted + score.TasksFailed
	score.SuccessRate = float64(score.TasksCompleted) / float64(totalTasks)
	
	// Update average duration
	if score.TasksCompleted > 0 {
		prevTotal := score.AverageDuration * time.Duration(score.TasksCompleted-1)
		score.AverageDuration = (prevTotal + outcome.Duration) / time.Duration(score.TasksCompleted)
	} else {
		score.AverageDuration = outcome.Duration
	}
	
	// Update total cost
	score.TotalCost += outcome.Cost
	score.LastUpdated = time.Now()
	
	// Recalculate overall score
	newScore := rm.calculateScore(score)
	score.Score = newScore
	
	// Check for blacklisting
	if newScore < rm.config.BlacklistThreshold && totalTasks >= rm.config.MinTasksForScore {
		rm.blacklistPeer(outcome.ExecutorID, "low_reputation")
	}
	
	// Update metrics
	reputationScoreGauge.WithLabelValues(outcome.ExecutorID.String()).Set(newScore)
	
	rm.logger.Info("Reputation updated",
		zap.String("peer_id", outcome.ExecutorID.String()),
		zap.Float64("score", newScore),
		zap.Int("completed", score.TasksCompleted),
		zap.Int("failed", score.TasksFailed),
		zap.Float64("success_rate", score.SuccessRate),
	)
	
	return nil
}

// calculateScore computes the overall reputation score based on multiple factors
func (rm *ReputationManager) calculateScore(score *ReputationScore) float64 {
	totalTasks := score.TasksCompleted + score.TasksFailed
	
	// Not enough data yet
	if totalTasks < rm.config.MinTasksForScore {
		return 0.5 // Neutral score
	}
	
	// 1. Success Rate Component (0.0 to 1.0)
	successComponent := score.SuccessRate
	
	// 2. Speed Component (faster is better)
	// Compare to baseline: if faster than baseline, score approaches 1.0
	speedRatio := float64(rm.config.BaselineDuration) / float64(score.AverageDuration)
	speedComponent := 1.0 / (1.0 + math.Exp(-2*(speedRatio-1))) // Sigmoid
	
	// 3. Cost Component (cheaper is better)
	avgCost := score.TotalCost / float64(score.TasksCompleted)
	costRatio := rm.config.BaselineCost / avgCost
	costComponent := 1.0 / (1.0 + math.Exp(-2*(costRatio-1))) // Sigmoid
	
	// 4. Longevity Component (longer history is better)
	daysSinceFirstSeen := time.Since(score.FirstSeen).Hours() / 24
	longevityComponent := math.Tanh(daysSinceFirstSeen / 30.0) // Approaches 1.0 after ~30 days
	
	// Weighted combination
	overallScore := (
		rm.config.SuccessRateWeight*successComponent +
		rm.config.SpeedWeight*speedComponent +
		rm.config.CostWeight*costComponent +
		rm.config.LongevityWeight*longevityComponent)
	
	// Apply time decay if enabled
	if rm.config.DecayEnabled {
		timeSinceUpdate := time.Since(score.LastUpdated)
		decayFactor := math.Pow(0.5, float64(timeSinceUpdate)/float64(rm.config.DecayHalfLife))
		overallScore *= decayFactor
	}
	
	// Clamp to [0.0, 1.0]
	if overallScore < 0.0 {
		overallScore = 0.0
	}
	if overallScore > 1.0 {
		overallScore = 1.0
	}
	
	return overallScore
}

// GetScore retrieves the reputation score for a peer
func (rm *ReputationManager) GetScore(peerID peer.ID) (*ReputationScore, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	score, exists := rm.scores[peerID]
	if !exists {
		return nil, fmt.Errorf("no reputation data for peer %s", peerID)
	}
	
	// Create copy to avoid race conditions
	scoreCopy := *score
	return &scoreCopy, nil
}

// IsBlacklisted checks if a peer is currently blacklisted
func (rm *ReputationManager) IsBlacklisted(peerID peer.ID) bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	until, exists := rm.blacklist[peerID]
	if !exists {
		return false
	}
	
	// Check if blacklist has expired
	if time.Now().After(until) {
		return false
	}
	
	return true
}

// blacklistPeer adds a peer to the blacklist
func (rm *ReputationManager) blacklistPeer(peerID peer.ID, reason string) {
	until := time.Now().Add(rm.config.BlacklistDuration)
	rm.blacklist[peerID] = until
	
	score, exists := rm.scores[peerID]
	if exists {
		score.Blacklisted = true
		score.BlacklistedUntil = until
	}
	
	blacklistedPeersGauge.Inc()
	trustEventsTotal.WithLabelValues("blacklist").Inc()
	
	rm.logger.Warn("Peer blacklisted",
		zap.String("peer_id", peerID.String()),
		zap.String("reason", reason),
		zap.Time("until", until),
	)
}

// RemoveFromBlacklist manually removes a peer from the blacklist
func (rm *ReputationManager) RemoveFromBlacklist(peerID peer.ID) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	if _, exists := rm.blacklist[peerID]; exists {
		delete(rm.blacklist, peerID)
		blacklistedPeersGauge.Dec()
		
		if score, exists := rm.scores[peerID]; exists {
			score.Blacklisted = false
			score.BlacklistedUntil = time.Time{}
		}
		
		rm.logger.Info("Peer removed from blacklist",
			zap.String("peer_id", peerID.String()),
		)
	}
}

// GetTopPeers returns the top N peers by reputation score
func (rm *ReputationManager) GetTopPeers(n int, minTasks int) []ReputationScore {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	// Filter peers with minimum task count
	eligible := make([]ReputationScore, 0)
	for _, score := range rm.scores {
		totalTasks := score.TasksCompleted + score.TasksFailed
		if totalTasks >= minTasks && !score.Blacklisted {
			eligible = append(eligible, *score)
		}
	}
	
	// Sort by score (descending)
	for i := 0; i < len(eligible); i++ {
		for j := i + 1; j < len(eligible); j++ {
			if eligible[j].Score > eligible[i].Score {
				eligible[i], eligible[j] = eligible[j], eligible[i]
			}
		}
	}
	
	// Return top N
	if len(eligible) < n {
		return eligible
	}
	return eligible[:n]
}

// GetAllScores returns all reputation scores
func (rm *ReputationManager) GetAllScores() []ReputationScore {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	scores := make([]ReputationScore, 0, len(rm.scores))
	for _, score := range rm.scores {
		scores = append(scores, *score)
	}
	
	return scores
}

// Stats returns reputation manager statistics
func (rm *ReputationManager) Stats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	
	totalPeers := len(rm.scores)
	blacklisted := len(rm.blacklist)
	totalOutcomes := len(rm.outcomes)
	
	var avgScore float64
	for _, score := range rm.scores {
		avgScore += score.Score
	}
	if totalPeers > 0 {
		avgScore /= float64(totalPeers)
	}
	
	return map[string]interface{}{
		"total_peers":       totalPeers,
		"blacklisted_peers": blacklisted,
		"total_outcomes":    totalOutcomes,
		"average_score":     avgScore,
	}
}

// CleanupExpired removes expired blacklist entries
func (rm *ReputationManager) CleanupExpired() {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	
	now := time.Now()
	removed := 0
	
	for peerID, until := range rm.blacklist {
		if now.After(until) {
			delete(rm.blacklist, peerID)
			
			if score, exists := rm.scores[peerID]; exists {
				score.Blacklisted = false
				score.BlacklistedUntil = time.Time{}
			}
			
			removed++
		}
	}
	
	if removed > 0 {
		blacklistedPeersGauge.Sub(float64(removed))
		rm.logger.Info("Cleaned up expired blacklist entries",
			zap.Int("removed", removed),
		)
	}
}

// StartCleanupLoop starts a background cleanup loop
func (rm *ReputationManager) StartCleanupLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rm.CleanupExpired()
		}
	}
}
