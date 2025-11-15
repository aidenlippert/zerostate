package market

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RLBidder implements Multi-Agent Reinforcement Learning for intelligent pricing
// Based on Deep Q-Network research (2024) - SDN routing and pricing optimization
//
// Key features:
// - Q-learning with epsilon-greedy exploration
// - State encoding: capacity, win rate, revenue, market demand
// - Action space: discrete price levels (10%, 20%, ..., 100% of budget)
// - Reward function: profit - cost + completion bonus - failure penalty
type RLBidder struct {
	// Agent identity
	agentID      string
	capabilities []string

	// Current state
	state    *RLBidderState
	stateMux sync.RWMutex

	// Q-table: (state, action) → expected reward
	qTable    map[StateActionKey]float64
	qTableMux sync.RWMutex

	// Learning parameters
	epsilon      float64 // Exploration rate (start high, decay over time)
	learningRate float64 // α = 0.1 (Q-learning step size)
	discount     float64 // γ = 0.9 (future reward discount)
	epsilonDecay float64 // 0.995 per episode

	// Pricing strategy
	minBidRatio float64 // 0.3 (bid at least 30% of budget)
	maxBidRatio float64 // 0.9 (bid at most 90% of budget)
	numActions  int     // 7 discrete price levels

	// Experience replay buffer (for better learning)
	replayBuffer []Experience
	bufferSize   int
	replayMux    sync.Mutex

	logger *zap.Logger
}

// BidderState represents agent's current situation for RL decision-making
type RLBidderState struct {
	// Capacity utilization [0.0-1.0]
	Capacity float64

	// Historical win rate [0.0-1.0]
	AvgWinRate float64

	// Recent revenue (last 10 tasks) in AINU
	RecentRevenue float64

	// Market demand indicator [0.0-1.0]
	// High demand → can bid higher
	// Low demand → must bid lower to compete
	MarketDemand float64

	// Number of active competitors for this capability
	NumCompetitors int

	// Agent's reputation score [0.0-1.0]
	Reputation float64
}

// StateActionKey is composite key for Q-table
type StateActionKey struct {
	State  string // Discretized state (5 bins per dimension)
	Action int    // Price action index [0, numActions-1]
}

// Experience represents single RL episode for replay learning
type Experience struct {
	State     *RLBidderState
	Action    int
	Reward    float64
	NextState *RLBidderState
	Done      bool
	Timestamp time.Time
}

// NewRLBidder creates reinforcement learning bidder
func NewRLBidder(agentID string, capabilities []string, logger *zap.Logger) *RLBidder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &RLBidder{
		agentID:      agentID,
		capabilities: capabilities,
		state: &RLBidderState{
			Capacity:       0.0,
			AvgWinRate:     0.0,
			RecentRevenue:  0.0,
			MarketDemand:   0.5, // Start neutral
			NumCompetitors: 5,   // Assume moderate competition
			Reputation:     0.5, // Start neutral
		},
		qTable:       make(map[StateActionKey]float64),
		epsilon:      0.5, // Start with 50% exploration
		learningRate: 0.1,
		discount:     0.9,
		epsilonDecay: 0.995,
		minBidRatio:  0.3,
		maxBidRatio:  0.9,
		numActions:   7, // 7 discrete price levels
		replayBuffer: make([]Experience, 0, 1000),
		bufferSize:   1000,
		logger:       logger,
	}
}

// SelectBidPrice uses epsilon-greedy Q-learning to choose bid price
func (b *RLBidder) SelectBidPrice(ctx context.Context, cfp *CFP) (float64, error) {
	// Update market state
	b.updateMarketState(cfp)

	// Get current state
	b.stateMux.RLock()
	state := b.state
	b.stateMux.RUnlock()

	// Encode state for Q-table lookup
	stateKey := b.encodeState(state)

	// Epsilon-greedy action selection
	var action int
	if rand.Float64() < b.epsilon {
		// Explore: random action
		action = rand.Intn(b.numActions)

		b.logger.Debug("RL bidder exploring",
			zap.String("agent", b.agentID),
			zap.String("cfp", cfp.ID),
			zap.Int("action", action),
			zap.Float64("epsilon", b.epsilon),
		)
	} else {
		// Exploit: best known action
		action = b.findBestAction(stateKey)

		b.logger.Debug("RL bidder exploiting",
			zap.String("agent", b.agentID),
			zap.String("cfp", cfp.ID),
			zap.Int("action", action),
			zap.Float64("q_value", b.getQValue(stateKey, action)),
		)
	}

	// Convert action to actual bid price
	bidPrice := b.actionToPrice(action, cfp.Budget)

	// Store experience for learning later
	b.storePreDecision(state, action, cfp)

	return bidPrice, nil
}

// Learn updates Q-table based on task outcome
// This is the core Q-learning algorithm: Q(s,a) ← Q(s,a) + α·[R + γ·max_a' Q(s',a') - Q(s,a)]
func (b *RLBidder) Learn(outcome TaskOutcome) {
	// Calculate reward
	reward := b.calculateReward(outcome)

	// Get stored experience
	b.replayMux.Lock()
	if len(b.replayBuffer) == 0 {
		b.replayMux.Unlock()
		b.logger.Warn("no experience to learn from", zap.String("agent", b.agentID))
		return
	}
	exp := b.replayBuffer[len(b.replayBuffer)-1]
	b.replayMux.Unlock()

	// Update experience with outcome
	exp.Reward = reward
	exp.NextState = b.state
	exp.Done = true

	// Q-learning update
	stateKey := b.encodeState(exp.State)
	nextStateKey := b.encodeState(exp.NextState)

	b.qTableMux.Lock()

	// Get current Q-value
	oldQ := b.qTable[StateActionKey{stateKey, exp.Action}]

	// Find max Q-value for next state
	maxNextQ := b.maxQValueUnsafe(nextStateKey)

	// Temporal difference: TD = R + γ·max_a' Q(s',a') - Q(s,a)
	td := reward + b.discount*maxNextQ - oldQ

	// Update: Q(s,a) ← Q(s,a) + α·TD
	newQ := oldQ + b.learningRate*td
	b.qTable[StateActionKey{stateKey, exp.Action}] = newQ

	b.qTableMux.Unlock()

	// Decay exploration rate
	b.epsilon = math.Max(0.01, b.epsilon*b.epsilonDecay)

	// Update agent state
	b.updateAgentState(outcome)

	b.logger.Info("RL bidder learned from outcome",
		zap.String("agent", b.agentID),
		zap.Bool("won", outcome.Won),
		zap.Bool("completed", outcome.Success),
		zap.Float64("reward", reward),
		zap.Float64("old_q", oldQ),
		zap.Float64("new_q", newQ),
		zap.Float64("td", td),
		zap.Float64("epsilon", b.epsilon),
	)
}

// calculateReward computes reward signal for RL update
func (b *RLBidder) calculateReward(outcome TaskOutcome) float64 {
	reward := 0.0

	if outcome.Won {
		// Base reward: profit (revenue - cost)
		reward = outcome.Profit

		if outcome.Success {
			// Completion bonus (10 AINU equivalent)
			reward += 10.0
		} else {
			// Failure penalty (-50 AINU)
			reward -= 50.0
		}
	} else {
		// Lost auction: small negative reward (opportunity cost)
		reward = -1.0
	}

	return reward
}

// updateMarketState updates market demand based on CFP characteristics
func (b *RLBidder) updateMarketState(cfp *CFP) {
	// Simple heuristic: high budget + urgent deadline = high demand
	demandScore := 0.5

	// Budget factor: higher budget indicates more demand
	if cfp.Budget > 100.0 {
		demandScore += 0.2
	}

	// Deadline factor: tight deadline indicates urgency
	timeToDeadline := time.Until(time.Unix(cfp.Deadline, 0))
	if timeToDeadline < 1*time.Hour {
		demandScore += 0.2
	}

	// Complexity factor: complex tasks have less competition
	if len(cfp.Capabilities) > 2 {
		demandScore -= 0.1
	}

	// Clamp to [0.0, 1.0]
	demandScore = math.Max(0.0, math.Min(1.0, demandScore))

	b.stateMux.Lock()
	b.state.MarketDemand = demandScore
	b.stateMux.Unlock()
}

// updateAgentState updates agent's internal state after task outcome
func (b *RLBidder) updateAgentState(outcome TaskOutcome) {
	b.stateMux.Lock()
	defer b.stateMux.Unlock()

	// Update win rate (exponential moving average)
	alpha := 0.1
	if outcome.Won {
		b.state.AvgWinRate = alpha*1.0 + (1-alpha)*b.state.AvgWinRate
	} else {
		b.state.AvgWinRate = alpha*0.0 + (1-alpha)*b.state.AvgWinRate
	}

	// Update recent revenue (moving window sum)
	if outcome.Won && outcome.Success {
		b.state.RecentRevenue = alpha*outcome.Profit + (1-alpha)*b.state.RecentRevenue
	}

	// Update capacity (for now, assume each task uses 10% capacity)
	// In real implementation, this would be based on actual task execution
	if outcome.Won && outcome.Success {
		b.state.Capacity = math.Max(0.0, b.state.Capacity-0.1)
	}
}

// encodeState discretizes continuous state into string key for Q-table
// Uses 5 bins per dimension for reasonable granularity
func (b *RLBidder) encodeState(state *RLBidderState) string {
	// Discretize each dimension into 5 bins: [0.0, 0.2, 0.4, 0.6, 0.8, 1.0]
	capacityBin := int(state.Capacity * 5)
	winRateBin := int(state.AvgWinRate * 5)
	demandBin := int(state.MarketDemand * 5)
	reputationBin := int(state.Reputation * 5)

	// Clamp to valid range
	capacityBin = clamp(capacityBin, 0, 4)
	winRateBin = clamp(winRateBin, 0, 4)
	demandBin = clamp(demandBin, 0, 4)
	reputationBin = clamp(reputationBin, 0, 4)

	return fmt.Sprintf("c%dw%dd%dr%d", capacityBin, winRateBin, demandBin, reputationBin)
}

// findBestAction selects action with highest Q-value for given state
func (b *RLBidder) findBestAction(stateKey string) int {
	b.qTableMux.RLock()
	defer b.qTableMux.RUnlock()

	bestAction := 0
	bestQ := math.Inf(-1)

	for action := 0; action < b.numActions; action++ {
		q := b.qTable[StateActionKey{stateKey, action}]
		if q > bestQ {
			bestQ = q
			bestAction = action
		}
	}

	return bestAction
}

// maxQValueUnsafe finds maximum Q-value for state (caller must hold lock)
func (b *RLBidder) maxQValueUnsafe(stateKey string) float64 {
	maxQ := 0.0

	for action := 0; action < b.numActions; action++ {
		q := b.qTable[StateActionKey{stateKey, action}]
		if q > maxQ {
			maxQ = q
		}
	}

	return maxQ
}

// getQValue retrieves Q-value for (state, action) pair
func (b *RLBidder) getQValue(stateKey string, action int) float64 {
	b.qTableMux.RLock()
	defer b.qTableMux.RUnlock()
	return b.qTable[StateActionKey{stateKey, action}]
}

// actionToPrice converts discrete action to actual bid price
// Actions: [0, numActions-1] map to [minBidRatio, maxBidRatio] of budget
func (b *RLBidder) actionToPrice(action int, budget float64) float64 {
	// Linear interpolation between min and max bid ratio
	ratio := b.minBidRatio + float64(action)*((b.maxBidRatio-b.minBidRatio)/float64(b.numActions-1))

	return budget * ratio
}

// storePreDecision saves state and action before outcome is known
func (b *RLBidder) storePreDecision(state *RLBidderState, action int, cfp *CFP) {
	exp := Experience{
		State:     state,
		Action:    action,
		Reward:    0, // Will be filled in Learn()
		NextState: nil,
		Done:      false,
		Timestamp: time.Now(),
	}

	b.replayMux.Lock()
	defer b.replayMux.Unlock()

	// Add to buffer
	if len(b.replayBuffer) >= b.bufferSize {
		// Remove oldest experience
		b.replayBuffer = b.replayBuffer[1:]
	}
	b.replayBuffer = append(b.replayBuffer, exp)
}

// GetStats returns current RL bidder statistics
func (b *RLBidder) GetStats() map[string]interface{} {
	b.stateMux.RLock()
	state := *b.state
	b.stateMux.RUnlock()

	b.qTableMux.RLock()
	qTableSize := len(b.qTable)
	b.qTableMux.RUnlock()

	b.replayMux.Lock()
	bufferSize := len(b.replayBuffer)
	b.replayMux.Unlock()

	return map[string]interface{}{
		"agent_id":         b.agentID,
		"capacity":         state.Capacity,
		"win_rate":         state.AvgWinRate,
		"recent_revenue":   state.RecentRevenue,
		"market_demand":    state.MarketDemand,
		"reputation":       state.Reputation,
		"epsilon":          b.epsilon,
		"q_table_size":     qTableSize,
		"experience_count": bufferSize,
	}
}

// Utility functions
func clamp(x, min, max int) int {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
