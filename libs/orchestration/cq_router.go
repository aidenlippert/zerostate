package orchestration

import (
	"container/ring"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CQRouter implements Confidence-based Q-Routing for intelligent agent discovery
// Based on 2024 research: "Confidence-Based Q-Routing" - 2x faster convergence vs standard Q-routing
//
// Performance targets:
// - Convergence: <4000 time steps (vs 8000 for Q-routing)
// - Latency reduction: 40% vs baseline GossipSub
// - Network load: 10x reduction in broadcast messages
type CQRouter struct {
	// Q-table: (capability, peer) → expected delivery time (ms)
	qTable map[CapabilityPeerKey]float64
	qMutex sync.RWMutex

	// Confidence: (capability, peer) → confidence in Q-value estimate [0.0-1.0]
	confidence map[CapabilityPeerKey]float64
	confMutex  sync.RWMutex

	// Temporal difference history for Predictive Q-Routing (PQ-Routing)
	tdHistory *ring.Ring // Size: 100 samples

	// Learning parameters
	baseLearningRate float64 // α₀ = 0.1 (typical)
	discountFactor   float64 // γ = 0.9 (future reward discount)
	confidenceGrowth float64 // Rate at which confidence increases (0.1)

	// Routing state
	peerCapabilities map[string][]string // peer DID → capabilities
	capabilityPeers  map[string][]string // capability → peer DIDs
	stateMutex       sync.RWMutex

	logger *zap.Logger
}

// CapabilityPeerKey is composite key for Q-table and confidence maps
type CapabilityPeerKey struct {
	Capability string
	PeerDID    string
}

// RouteOutcome tracks result of routing decision for learning
type RouteOutcome struct {
	Capability string
	PeerDID    string
	Latency    time.Duration // Actual delivery time
	Success    bool
	Timestamp  time.Time
}

// NewCQRouter creates Confidence-based Q-Routing coordinator
func NewCQRouter(logger *zap.Logger) *CQRouter {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &CQRouter{
		qTable:           make(map[CapabilityPeerKey]float64),
		confidence:       make(map[CapabilityPeerKey]float64),
		tdHistory:        ring.New(100), // 100 recent TD samples
		baseLearningRate: 0.1,
		discountFactor:   0.9,
		confidenceGrowth: 0.1,
		peerCapabilities: make(map[string][]string),
		capabilityPeers:  make(map[string][]string),
		logger:           logger,
	}
}

// RegisterPeer adds peer with capabilities to routing table
func (r *CQRouter) RegisterPeer(peerDID string, capabilities []string) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()

	// Store peer capabilities
	r.peerCapabilities[peerDID] = capabilities

	// Build reverse index: capability → peers
	for _, cap := range capabilities {
		if !contains(r.capabilityPeers[cap], peerDID) {
			r.capabilityPeers[cap] = append(r.capabilityPeers[cap], peerDID)
		}

		// Initialize Q-value and confidence for new peer
		key := CapabilityPeerKey{cap, peerDID}

		r.qMutex.Lock()
		if _, exists := r.qTable[key]; !exists {
			// Initialize with optimistic value (low latency estimate)
			r.qTable[key] = 100.0 // 100ms initial estimate
		}
		r.qMutex.Unlock()

		r.confMutex.Lock()
		if _, exists := r.confidence[key]; !exists {
			// Start with low confidence (high exploration)
			r.confidence[key] = 0.1
		}
		r.confMutex.Unlock()
	}

	r.logger.Info("registered peer with CQ-Router",
		zap.String("peer", peerDID),
		zap.Strings("capabilities", capabilities),
	)
}

// UnregisterPeer removes peer from routing table
func (r *CQRouter) UnregisterPeer(peerDID string) {
	r.stateMutex.Lock()
	defer r.stateMutex.Unlock()

	// Remove from capability index
	caps := r.peerCapabilities[peerDID]
	for _, cap := range caps {
		r.capabilityPeers[cap] = removeString(r.capabilityPeers[cap], peerDID)
	}

	// Remove peer capabilities
	delete(r.peerCapabilities, peerDID)

	r.logger.Info("unregistered peer from CQ-Router",
		zap.String("peer", peerDID),
	)
}

// RouteCFP selects best peer for capability using CQ-Routing algorithm
func (r *CQRouter) RouteCFP(capability string) (string, float64, error) {
	// Find all peers with this capability
	r.stateMutex.RLock()
	candidates := r.capabilityPeers[capability]
	r.stateMutex.RUnlock()

	if len(candidates) == 0 {
		return "", 0, fmt.Errorf("no peers available for capability: %s", capability)
	}

	// Select peer with lowest expected latency (Q-value)
	bestPeer := ""
	bestQ := math.MaxFloat64

	r.qMutex.RLock()
	for _, peer := range candidates {
		key := CapabilityPeerKey{capability, peer}
		q := r.qTable[key]

		if q < bestQ {
			bestQ = q
			bestPeer = peer
		}
	}
	r.qMutex.RUnlock()

	if bestPeer == "" {
		return "", 0, fmt.Errorf("failed to select peer for capability: %s", capability)
	}

	r.logger.Debug("routed CFP via CQ-Routing",
		zap.String("capability", capability),
		zap.String("peer", bestPeer),
		zap.Float64("expected_latency_ms", bestQ),
		zap.Int("candidates", len(candidates)),
	)

	return bestPeer, bestQ, nil
}

// Learn updates Q-values and confidence based on routing outcome
// This is the core CQ-Routing algorithm with confidence-based learning rate
func (r *CQRouter) Learn(outcome RouteOutcome) {
	key := CapabilityPeerKey{outcome.Capability, outcome.PeerDID}

	// Get current Q-value and confidence
	r.qMutex.RLock()
	oldQ := r.qTable[key]
	r.qMutex.RUnlock()

	r.confMutex.RLock()
	conf := r.confidence[key]
	r.confMutex.RUnlock()

	// Calculate reward (negative latency = lower is better)
	reward := -float64(outcome.Latency.Milliseconds())
	if !outcome.Success {
		// Penalty for failure (simulated infinite latency)
		reward = -10000.0
	}

	// Find best Q-value for next state (min latency among peers)
	nextBestQ := r.getMinQValue(outcome.Capability)

	// Temporal difference: TD = R + γ·Q(s',a') - Q(s,a)
	td := reward + r.discountFactor*nextBestQ - oldQ

	// Confidence-based learning rate: α(t) = α₀ / (1 + confidence)
	// High confidence → low learning rate (stable)
	// Low confidence → high learning rate (explore)
	learningRate := r.baseLearningRate / (1.0 + conf)

	// Q-learning update: Q(s,a) ← Q(s,a) + α·TD
	newQ := oldQ + learningRate*td

	// Update Q-value
	r.qMutex.Lock()
	r.qTable[key] = newQ
	r.qMutex.Unlock()

	// Update confidence: confidence increases with experience
	// conf(t+1) = conf(t) + β·(1 - conf(t))
	// Asymptotically approaches 1.0
	newConf := conf + r.confidenceGrowth*(1.0-conf)

	r.confMutex.Lock()
	r.confidence[key] = newConf
	r.confMutex.Unlock()

	// Store TD for Predictive Q-Routing analysis
	r.tdHistory.Value = td
	r.tdHistory = r.tdHistory.Next()

	r.logger.Debug("CQ-Routing learning update",
		zap.String("capability", outcome.Capability),
		zap.String("peer", outcome.PeerDID),
		zap.Float64("old_q", oldQ),
		zap.Float64("new_q", newQ),
		zap.Float64("td", td),
		zap.Float64("learning_rate", learningRate),
		zap.Float64("confidence", newConf),
		zap.Bool("success", outcome.Success),
		zap.Duration("latency", outcome.Latency),
	)
}

// getMinQValue finds minimum Q-value (fastest route) for capability
func (r *CQRouter) getMinQValue(capability string) float64 {
	r.stateMutex.RLock()
	peers := r.capabilityPeers[capability]
	r.stateMutex.RUnlock()

	if len(peers) == 0 {
		return 0.0
	}

	minQ := math.MaxFloat64

	r.qMutex.RLock()
	defer r.qMutex.RUnlock()

	for _, peer := range peers {
		key := CapabilityPeerKey{capability, peer}
		if q, exists := r.qTable[key]; exists && q < minQ {
			minQ = q
		}
	}

	return minQ
}

// GetRoutingStats returns current Q-table statistics for monitoring
func (r *CQRouter) GetRoutingStats() map[string]interface{} {
	r.qMutex.RLock()
	defer r.qMutex.RUnlock()

	r.confMutex.RLock()
	defer r.confMutex.RUnlock()

	r.stateMutex.RLock()
	defer r.stateMutex.RUnlock()

	// Calculate average Q-value and confidence
	totalQ := 0.0
	totalConf := 0.0
	count := len(r.qTable)

	for key := range r.qTable {
		totalQ += r.qTable[key]
		totalConf += r.confidence[key]
	}

	avgQ := 0.0
	avgConf := 0.0
	if count > 0 {
		avgQ = totalQ / float64(count)
		avgConf = totalConf / float64(count)
	}

	// Calculate TD variance for convergence detection
	tdVariance := r.calculateTDVariance()

	return map[string]interface{}{
		"total_routes":            count,
		"total_peers":             len(r.peerCapabilities),
		"total_capabilities":      len(r.capabilityPeers),
		"avg_expected_latency_ms": avgQ,
		"avg_confidence":          avgConf,
		"td_variance":             tdVariance,
		"converged":               tdVariance < 10.0, // Convergence threshold
	}
}

// calculateTDVariance computes variance of recent temporal differences
// Low variance indicates convergence (stable Q-values)
func (r *CQRouter) calculateTDVariance() float64 {
	samples := make([]float64, 0, 100)

	r.tdHistory.Do(func(v interface{}) {
		if td, ok := v.(float64); ok {
			samples = append(samples, td)
		}
	})

	if len(samples) < 10 {
		return math.MaxFloat64 // Not enough data
	}

	// Calculate mean
	sum := 0.0
	for _, td := range samples {
		sum += td
	}
	mean := sum / float64(len(samples))

	// Calculate variance
	variance := 0.0
	for _, td := range samples {
		diff := td - mean
		variance += diff * diff
	}
	variance /= float64(len(samples))

	return variance
}

// Utility functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func removeString(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
