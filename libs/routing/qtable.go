// Package routing implements Q-learning based routing for zerostate.
package routing

import (
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Metrics for Q-routing
	qValueGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_routing_q_value",
			Help: "Current Q-value for peer routes",
		},
		[]string{"peer_id", "metric_type"},
	)

	routingDecisionsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_routing_decisions_total",
			Help: "Total number of routing decisions made",
		},
		[]string{"strategy", "status"},
	)

	routeLatencyHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_route_latency_seconds",
			Help:    "Latency of routes to peers",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"peer_id"},
	)
)

// QValue represents the quality metrics for a route to a peer
type QValue struct {
	AvgLatency    float64   // Average latency in seconds
	SuccessRate   float64   // Success rate (0.0 - 1.0)
	Bandwidth     float64   // Estimated bandwidth in bytes/sec
	LastUpdate    time.Time // Last time this Q-value was updated
	SampleCount   int       // Number of samples used for averaging
	RewardSum     float64   // Sum of rewards for Q-learning
	QScore        float64   // Computed Q-score for routing decisions
}

// QTable maintains routing quality information for peers
type QTable struct {
	mu      sync.RWMutex
	entries map[peer.ID]*QValue
	
	// Q-learning parameters
	alpha       float64 // Learning rate (0.0 - 1.0)
	gamma       float64 // Discount factor (0.0 - 1.0)
	epsilon     float64 // Exploration rate (0.0 - 1.0)
	
	// Weights for composite Q-score
	latencyWeight   float64
	successWeight   float64
	bandwidthWeight float64
}

// NewQTable creates a new Q-routing table
func NewQTable() *QTable {
	return &QTable{
		entries:         make(map[peer.ID]*QValue),
		alpha:           0.3,  // Learning rate
		gamma:           0.9,  // Discount factor
		epsilon:         0.1,  // 10% exploration
		latencyWeight:   0.4,  // 40% weight on latency
		successWeight:   0.4,  // 40% weight on success rate
		bandwidthWeight: 0.2,  // 20% weight on bandwidth
	}
}

// UpdateRoute records a routing observation and updates Q-value
func (qt *QTable) UpdateRoute(peerID peer.ID, latency time.Duration, success bool, bytesTransferred int64) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	qval, exists := qt.entries[peerID]
	if !exists {
		qval = &QValue{
			AvgLatency:  latency.Seconds(),
			SuccessRate: 0.0,
			Bandwidth:   0.0,
			LastUpdate:  time.Now(),
			SampleCount: 0,
			RewardSum:   0.0,
			QScore:      0.0,
		}
		qt.entries[peerID] = qval
	}

	// Calculate reward based on performance
	reward := qt.calculateReward(latency, success, bytesTransferred)
	
	// Q-learning update: Q(s,a) = Q(s,a) + α[R + γ*maxQ(s',a') - Q(s,a)]
	// For simplicity, we use the reward directly as we don't have next-state info yet
	qval.QScore = qval.QScore + qt.alpha*(reward - qval.QScore)
	qval.RewardSum += reward
	qval.SampleCount++
	
	// Update metrics with exponential moving average
	alpha := qt.alpha
	qval.AvgLatency = (1-alpha)*qval.AvgLatency + alpha*latency.Seconds()
	
	successValue := 0.0
	if success {
		successValue = 1.0
	}
	qval.SuccessRate = (1-alpha)*qval.SuccessRate + alpha*successValue
	
	if latency.Seconds() > 0 {
		bandwidth := float64(bytesTransferred) / latency.Seconds()
		qval.Bandwidth = (1-alpha)*qval.Bandwidth + alpha*bandwidth
	}
	
	qval.LastUpdate = time.Now()

	// Update Prometheus metrics
	qValueGauge.WithLabelValues(peerID.String(), "latency").Set(qval.AvgLatency)
	qValueGauge.WithLabelValues(peerID.String(), "success_rate").Set(qval.SuccessRate)
	qValueGauge.WithLabelValues(peerID.String(), "q_score").Set(qval.QScore)
	routeLatencyHistogram.WithLabelValues(peerID.String()).Observe(latency.Seconds())

	status := "success"
	if !success {
		status = "failure"
	}
	routingDecisionsTotal.WithLabelValues("q_learning", status).Inc()
}

// calculateReward computes the reward for a routing decision
func (qt *QTable) calculateReward(latency time.Duration, success bool, bytesTransferred int64) float64 {
	if !success {
		return -1.0 // Negative reward for failures
	}
	
	// Reward components (normalized to 0-1 range)
	latencyScore := 1.0 / (1.0 + latency.Seconds()) // Lower latency = higher score
	bandwidthScore := 0.5 // Neutral if no data transferred
	if latency.Seconds() > 0 && bytesTransferred > 0 {
		bandwidth := float64(bytesTransferred) / latency.Seconds()
		bandwidthScore = 1.0 / (1.0 + 1000000.0/bandwidth) // Normalize around 1MB/s
	}
	
	// Weighted combination
	reward := qt.latencyWeight*latencyScore + 
	          qt.successWeight*1.0 + // Success is always 1.0 here
	          qt.bandwidthWeight*bandwidthScore
	
	return reward
}

// SelectBestPeer returns the peer with the highest Q-score
func (qt *QTable) SelectBestPeer(candidates []peer.ID) (peer.ID, bool) {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	if len(candidates) == 0 {
		return "", false
	}

	// Epsilon-greedy exploration: with probability epsilon, choose random peer
	// This is simplified - in production, use proper random selection
	
	var bestPeer peer.ID
	bestScore := -1.0
	
	for _, peerID := range candidates {
		qval, exists := qt.entries[peerID]
		if !exists {
			// Unknown peer - give it a chance (exploration)
			qval = &QValue{QScore: 0.5} // Neutral score
		}
		
		// Composite score: Q-score weighted by recency
		age := time.Since(qval.LastUpdate).Seconds()
		recencyFactor := 1.0 / (1.0 + age/3600.0) // Decay over 1 hour
		score := qval.QScore * (0.8 + 0.2*recencyFactor)
		
		if score > bestScore {
			bestScore = score
			bestPeer = peerID
		}
	}

	routingDecisionsTotal.WithLabelValues("best_peer", "selected").Inc()
	return bestPeer, true
}

// GetQValue returns the Q-value for a peer
func (qt *QTable) GetQValue(peerID peer.ID) (*QValue, bool) {
	qt.mu.RLock()
	defer qt.mu.RUnlock()
	
	qval, exists := qt.entries[peerID]
	if !exists {
		return nil, false
	}
	
	// Return a copy to avoid race conditions
	copy := *qval
	return &copy, true
}

// GetTopPeers returns the N peers with highest Q-scores
func (qt *QTable) GetTopPeers(n int) []peer.ID {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	type peerScore struct {
		id    peer.ID
		score float64
	}

	scores := make([]peerScore, 0, len(qt.entries))
	for id, qval := range qt.entries {
		scores = append(scores, peerScore{id: id, score: qval.QScore})
	}

	// Simple bubble sort for top N (good enough for small N)
	for i := 0; i < len(scores) && i < n; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	result := make([]peer.ID, 0, n)
	for i := 0; i < n && i < len(scores); i++ {
		result = append(result, scores[i].id)
	}

	return result
}

// PruneStale removes entries that haven't been updated recently
func (qt *QTable) PruneStale(maxAge time.Duration) int {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	pruned := 0
	now := time.Now()
	
	for peerID, qval := range qt.entries {
		if now.Sub(qval.LastUpdate) > maxAge {
			delete(qt.entries, peerID)
			pruned++
		}
	}

	return pruned
}

// Stats returns statistics about the Q-table
func (qt *QTable) Stats() map[string]interface{} {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	totalScore := 0.0
	totalLatency := 0.0
	totalSuccess := 0.0
	count := len(qt.entries)

	for _, qval := range qt.entries {
		totalScore += qval.QScore
		totalLatency += qval.AvgLatency
		totalSuccess += qval.SuccessRate
	}

	stats := map[string]interface{}{
		"total_peers": count,
	}

	if count > 0 {
		stats["avg_q_score"] = totalScore / float64(count)
		stats["avg_latency"] = totalLatency / float64(count)
		stats["avg_success_rate"] = totalSuccess / float64(count)
	}

	return stats
}
