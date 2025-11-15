package market

import (
	"context"
	"math"
	"math/rand"

	"go.uber.org/zap"
)

// PricingStrategy defines the interface for pluggable bidding strategies
// This enables MARL experimentation without changing the Bidder or AACL protocol
type PricingStrategy interface {
	// ShouldBid decides whether to bid on a CFP given current state
	ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool

	// CalculatePrice determines the bid price for a CFP
	CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64

	// OnBidResult is called when orchestrator accepts/rejects the bid
	// Used by learning strategies to update their models
	OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState)

	// OnTaskOutcome is called when a task completes
	// Primary learning signal for RL-based strategies
	OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState)

	// Name returns the strategy name for logging
	Name() string
}

// CFPMessage wraps the parsed CFP for easier access
type CFPMessage struct {
	CFPID           string
	OrchestratorDID string
	Capability      string
	Budget          float64
	Deadline        string
	Requirements    map[string]interface{}
}

// ============================================================================
// Strategy 1: StaticFloorPricing (Current behavior - baseline)
// ============================================================================

// StaticFloorPricing always bids at a fixed floor price
// This is the simplest strategy and serves as a baseline
type StaticFloorPricing struct {
	FloorPrice float64
	logger     *zap.Logger
}

func NewStaticFloorPricing(floorPrice float64, logger *zap.Logger) *StaticFloorPricing {
	return &StaticFloorPricing{
		FloorPrice: floorPrice,
		logger:     logger,
	}
}

func (s *StaticFloorPricing) Name() string {
	return "StaticFloorPricing"
}

func (s *StaticFloorPricing) ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool {
	// Bid if budget meets floor AND we have capacity
	return cfp.Budget >= s.FloorPrice && state.CanAcceptTask()
}

func (s *StaticFloorPricing) CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64 {
	// Always bid at floor price
	return s.FloorPrice
}

func (s *StaticFloorPricing) OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState) {
	// Static strategy doesn't learn, but we log for debugging
	if accepted {
		s.logger.Debug("bid accepted (static strategy)",
			zap.String("cfp_id", cfp.CFPID),
			zap.Float64("price", bidPrice),
		)
	}
}

func (s *StaticFloorPricing) OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState) {
	// Static strategy doesn't learn
}

// ============================================================================
// Strategy 2: LoadAwarePricing (Simple adaptive pricing)
// ============================================================================

// LoadAwarePricing adjusts price based on current capacity utilization
// Price increases as load increases to prevent overcommitment
type LoadAwarePricing struct {
	BasePrice     float64 // Price when idle (load = 0.0)
	MaxMultiplier float64 // Max price multiplier (at load = 1.0)
	LoadExponent  float64 // Controls pricing curve steepness
	logger        *zap.Logger
}

func NewLoadAwarePricing(basePrice, maxMultiplier, loadExponent float64, logger *zap.Logger) *LoadAwarePricing {
	return &LoadAwarePricing{
		BasePrice:     basePrice,
		MaxMultiplier: maxMultiplier,
		LoadExponent:  loadExponent,
		logger:        logger,
	}
}

func (s *LoadAwarePricing) Name() string {
	return "LoadAwarePricing"
}

func (s *LoadAwarePricing) ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool {
	// Calculate what our price would be
	price := s.CalculatePrice(ctx, cfp, state)

	// Bid if budget covers our price AND we have capacity
	return cfp.Budget >= price && state.CanAcceptTask()
}

func (s *LoadAwarePricing) CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64 {
	// Price formula: BasePrice * (1 + LoadFactor^LoadExponent * (MaxMultiplier - 1))
	// Example with base=100, max=3, exp=2:
	//   Load=0.0 → Price=100
	//   Load=0.5 → Price=150
	//   Load=0.8 → Price=228
	//   Load=1.0 → Price=300

	loadFactor := state.GetLoadFactor()
	multiplier := 1.0 + math.Pow(loadFactor, s.LoadExponent)*(s.MaxMultiplier-1.0)
	price := s.BasePrice * multiplier

	return price
}

func (s *LoadAwarePricing) OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState) {
	load := state.GetLoadFactor()
	s.logger.Debug("bid result (load-aware strategy)",
		zap.String("cfp_id", cfp.CFPID),
		zap.Bool("accepted", accepted),
		zap.Float64("price", bidPrice),
		zap.Float64("load", load),
	)
}

func (s *LoadAwarePricing) OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState) {
	// Simple strategy doesn't learn from outcomes yet
	// Future: could adjust BasePrice based on profitability
}

// ============================================================================
// Strategy 3: CompetitivePricing (Market-aware bidding)
// ============================================================================

// CompetitivePricing learns optimal prices by observing win/loss patterns
// Uses epsilon-greedy exploration with exponential moving average
type CompetitivePricing struct {
	BasePrice     float64
	CurrentPrice  float64
	WinRateTarget float64 // Target win rate (e.g., 0.3 = win 30% of bids)
	LearningRate  float64 // How fast to adjust prices (0.01 - 0.1)
	Epsilon       float64 // Exploration rate (0.1 = explore 10% of time)
	PriceStdDev   float64 // Standard deviation for exploration

	logger *zap.Logger
	rng    *rand.Rand
}

func NewCompetitivePricing(basePrice, winRateTarget, learningRate, epsilon float64, logger *zap.Logger) *CompetitivePricing {
	return &CompetitivePricing{
		BasePrice:     basePrice,
		CurrentPrice:  basePrice,
		WinRateTarget: winRateTarget,
		LearningRate:  learningRate,
		Epsilon:       epsilon,
		PriceStdDev:   basePrice * 0.1, // 10% of base price
		logger:        logger,
		rng:           rand.New(rand.NewSource(rand.Int63())),
	}
}

func (s *CompetitivePricing) Name() string {
	return "CompetitivePricing"
}

func (s *CompetitivePricing) ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool {
	price := s.CalculatePrice(ctx, cfp, state)
	return cfp.Budget >= price && state.CanAcceptTask()
}

func (s *CompetitivePricing) CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64 {
	// Epsilon-greedy: explore with probability epsilon
	if s.rng.Float64() < s.Epsilon {
		// Exploration: sample from Gaussian around current price
		noise := s.rng.NormFloat64() * s.PriceStdDev
		explorationPrice := s.CurrentPrice + noise

		// Clamp to reasonable bounds
		minPrice := s.BasePrice * 0.5
		maxPrice := s.BasePrice * 3.0
		explorationPrice = math.Max(minPrice, math.Min(maxPrice, explorationPrice))

		return explorationPrice
	}

	// Exploitation: use learned price
	return s.CurrentPrice
}

func (s *CompetitivePricing) OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState) {
	// Get capability-specific win rate
	stats := state.GetCapabilityStats(cfp.Capability)
	if stats == nil {
		return
	}

	currentWinRate := stats.WinRate

	// Adjust price based on win rate vs target
	// If winning too often (> target), prices are too low → increase
	// If winning too rarely (< target), prices are too high → decrease

	var priceAdjustment float64
	if currentWinRate > s.WinRateTarget {
		// Winning too often - increase price
		priceAdjustment = s.LearningRate * (currentWinRate - s.WinRateTarget) * s.BasePrice
	} else {
		// Winning too rarely - decrease price
		priceAdjustment = -s.LearningRate * (s.WinRateTarget - currentWinRate) * s.BasePrice
	}

	s.CurrentPrice += priceAdjustment

	// Clamp to reasonable bounds
	minPrice := s.BasePrice * 0.5
	maxPrice := s.BasePrice * 3.0
	s.CurrentPrice = math.Max(minPrice, math.Min(maxPrice, s.CurrentPrice))

	s.logger.Debug("adjusted competitive price",
		zap.String("capability", cfp.Capability),
		zap.Float64("win_rate", currentWinRate),
		zap.Float64("target_win_rate", s.WinRateTarget),
		zap.Float64("adjustment", priceAdjustment),
		zap.Float64("new_price", s.CurrentPrice),
	)
}

func (s *CompetitivePricing) OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState) {
	// Future: incorporate profitability into pricing decisions
	// If profit margins are too low, increase base price
	// If profit margins are high and win rate is low, decrease price
}

// ============================================================================
// Strategy 4: HybridPricing (Combines load-awareness and competition)
// ============================================================================

// HybridPricing uses competitive learning for base price, then applies load multiplier
// This gives best of both worlds: market-competitive + capacity-aware
type HybridPricing struct {
	competitiveBase *CompetitivePricing
	loadMultiplier  *LoadAwarePricing
	logger          *zap.Logger
}

func NewHybridPricing(basePrice, winRateTarget, learningRate, epsilon, maxLoadMultiplier float64, logger *zap.Logger) *HybridPricing {
	return &HybridPricing{
		competitiveBase: NewCompetitivePricing(basePrice, winRateTarget, learningRate, epsilon, logger),
		loadMultiplier:  NewLoadAwarePricing(1.0, maxLoadMultiplier, 2.0, logger),
		logger:          logger,
	}
}

func (s *HybridPricing) Name() string {
	return "HybridPricing"
}

func (s *HybridPricing) ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool {
	price := s.CalculatePrice(ctx, cfp, state)
	return cfp.Budget >= price && state.CanAcceptTask()
}

func (s *HybridPricing) CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64 {
	// Get market-competitive base price
	basePrice := s.competitiveBase.CalculatePrice(ctx, cfp, state)

	// Apply load multiplier
	loadFactor := state.GetLoadFactor()
	loadMultiplier := 1.0 + math.Pow(loadFactor, 2.0)*(s.loadMultiplier.MaxMultiplier-1.0)

	finalPrice := basePrice * loadMultiplier

	return finalPrice
}

func (s *HybridPricing) OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState) {
	// Let competitive component learn from result
	s.competitiveBase.OnBidResult(ctx, accepted, cfp, bidPrice, state)
}

func (s *HybridPricing) OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState) {
	s.competitiveBase.OnTaskOutcome(ctx, outcome, state)
}

// ============================================================================
// Future: RLPricing (Full reinforcement learning - placeholder)
// ============================================================================

// RLPricing is a placeholder for future deep RL-based pricing strategies
// Could use Q-learning, Actor-Critic, or Policy Gradient methods
type RLPricing struct {
	// model *ReinforcementLearningModel  // To be implemented
	logger *zap.Logger
}

func NewRLPricing(logger *zap.Logger) *RLPricing {
	return &RLPricing{
		logger: logger,
	}
}

func (s *RLPricing) Name() string {
	return "RLPricing (not implemented)"
}

func (s *RLPricing) ShouldBid(ctx context.Context, cfp *CFPMessage, state *BidderState) bool {
	// TODO: Implement using RL model
	s.logger.Warn("RLPricing.ShouldBid called but not implemented")
	return false
}

func (s *RLPricing) CalculatePrice(ctx context.Context, cfp *CFPMessage, state *BidderState) float64 {
	// TODO: Implement using RL model
	s.logger.Warn("RLPricing.CalculatePrice called but not implemented")
	return 0.0
}

func (s *RLPricing) OnBidResult(ctx context.Context, accepted bool, cfp *CFPMessage, bidPrice float64, state *BidderState) {
	// TODO: Update RL model
}

func (s *RLPricing) OnTaskOutcome(ctx context.Context, outcome TaskOutcome, state *BidderState) {
	// TODO: Primary learning signal for RL
	// This is where you'd update Q-values, policy gradients, etc.
}
