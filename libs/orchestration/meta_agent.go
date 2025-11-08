package orchestration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"go.uber.org/zap"
)

var (
	ErrNoAgentsAvailable = errors.New("no agents available for task")
	ErrBudgetTooLow      = errors.New("budget too low for available agents")
	ErrAuctionFailed     = errors.New("auction failed to find suitable agent")
)

// MetaAgent is the intelligent orchestrator that selects the best agent for tasks
// using auction mechanisms, multi-criteria scoring, and geographic routing
type MetaAgent struct {
	db      *database.DB
	logger  *zap.Logger
	config  *MetaAgentConfig
}

// MetaAgentConfig configures the meta-agent behavior
type MetaAgentConfig struct {
	// Scoring weights (must sum to 1.0)
	PriceWeight      float64 // Weight for price in scoring (0.3)
	QualityWeight    float64 // Weight for quality/rating (0.3)
	SpeedWeight      float64 // Weight for avg execution speed (0.2)
	ReputationWeight float64 // Weight for reputation (0.2)

	// Agent selection
	MinAgentsForAuction int     // Minimum agents to consider (default: 3)
	MaxAgentsForAuction int     // Maximum agents to consider (default: 10)
	MinAgentRating      float64 // Minimum agent rating (default: 3.0)

	// Failover
	EnableFailover    bool // Enable automatic failover to backup agents
	MaxFailoverAgents int  // Maximum failover attempts (default: 3)

	// Geographic routing
	EnableGeoRouting bool // Enable geographic routing for latency optimization
}

// DefaultMetaAgentConfig returns default configuration
func DefaultMetaAgentConfig() *MetaAgentConfig {
	return &MetaAgentConfig{
		PriceWeight:         0.3,
		QualityWeight:       0.3,
		SpeedWeight:         0.2,
		ReputationWeight:    0.2,
		MinAgentsForAuction: 3,
		MaxAgentsForAuction: 10,
		MinAgentRating:      3.0,
		EnableFailover:      true,
		MaxFailoverAgents:   3,
		EnableGeoRouting:    false, // Disabled until geographic metadata is available
	}
}

// NewMetaAgent creates a new meta-agent orchestrator
func NewMetaAgent(db *database.DB, config *MetaAgentConfig, logger *zap.Logger) *MetaAgent {
	if config == nil {
		config = DefaultMetaAgentConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// Validate weights sum to 1.0
	totalWeight := config.PriceWeight + config.QualityWeight + config.SpeedWeight + config.ReputationWeight
	if math.Abs(totalWeight-1.0) > 0.01 {
		logger.Warn("scoring weights do not sum to 1.0, normalizing",
			zap.Float64("total", totalWeight),
		)
		// Normalize weights
		config.PriceWeight /= totalWeight
		config.QualityWeight /= totalWeight
		config.SpeedWeight /= totalWeight
		config.ReputationWeight /= totalWeight
	}

	return &MetaAgent{
		db:     db,
		logger: logger,
		config: config,
	}
}

// AgentBid represents an agent's bid for a task
type AgentBid struct {
	Agent          *database.Agent
	BidPrice       float64   // Agent's bid price
	Score          float64   // Multi-criteria score (0.0-1.0)
	EstimatedTime  int64     // Estimated execution time (ms)
	CapabilityMatch float64  // How well agent capabilities match task (0.0-1.0)
	SubmittedAt    time.Time
}

// AgentScore represents detailed scoring breakdown for an agent
type AgentScore struct {
	AgentID         string
	PriceScore      float64 // Normalized price score (lower is better, inverted)
	QualityScore    float64 // Rating-based quality score
	SpeedScore      float64 // Execution speed score (tasks/hour)
	ReputationScore float64 // Tasks completed score
	TotalScore      float64 // Weighted total score
	Rank            int     // Final ranking (1 = best)
}

// SelectAgent selects the best agent for a task using auction mechanism
func (m *MetaAgent) SelectAgent(ctx context.Context, task *Task) (*database.Agent, error) {
	startTime := time.Now()

	m.logger.Info("meta-agent selecting agent for task",
		zap.String("task_id", task.ID),
		zap.Strings("capabilities", task.Capabilities),
		zap.Float64("budget", task.Budget),
	)

	// Step 1: Find agents matching required capabilities
	agents, err := m.findEligibleAgents(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("failed to find eligible agents: %w", err)
	}

	if len(agents) == 0 {
		return nil, ErrNoAgentsAvailable
	}

	m.logger.Info("found eligible agents",
		zap.Int("count", len(agents)),
		zap.Duration("search_time", time.Since(startTime)),
	)

	// Step 2: Run auction to get bids from agents
	bids, err := m.runAuction(ctx, task, agents)
	if err != nil {
		return nil, fmt.Errorf("auction failed: %w", err)
	}

	if len(bids) == 0 {
		return nil, ErrAuctionFailed
	}

	m.logger.Info("auction completed",
		zap.Int("bids_received", len(bids)),
		zap.Duration("auction_time", time.Since(startTime)),
	)

	// Step 3: Score and rank agents
	scoredBids := m.scoreAgents(task, bids)

	// Step 4: Select best agent
	bestBid := scoredBids[0]

	m.logger.Info("agent selected via meta-agent",
		zap.String("task_id", task.ID),
		zap.String("agent_id", bestBid.Agent.ID),
		zap.String("agent_name", bestBid.Agent.Name),
		zap.Float64("bid_price", bestBid.BidPrice),
		zap.Float64("score", bestBid.Score),
		zap.Float64("capability_match", bestBid.CapabilityMatch),
		zap.Duration("total_time", time.Since(startTime)),
	)

	return bestBid.Agent, nil
}

// findEligibleAgents finds agents that meet task requirements
func (m *MetaAgent) findEligibleAgents(ctx context.Context, task *Task) ([]*database.Agent, error) {
	// For now, search all agents and filter by capabilities
	// TODO: Optimize with capability indexing and database queries

	// Build search query from capabilities
	query := ""
	if len(task.Capabilities) > 0 {
		query = task.Capabilities[0] // Use first capability for search
	}

	// Search agents by capabilities
	agents, err := m.db.SearchAgents(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search agents: %w", err)
	}

	// Filter agents by status and rating
	eligible := make([]*database.Agent, 0)
	for _, agent := range agents {
		// Must be active
		if agent.Status != "active" {
			continue
		}

		// Must meet minimum rating
		if agent.Rating < m.config.MinAgentRating {
			continue
		}

		// Check if agent has required capabilities
		if m.hasRequiredCapabilities(agent, task.Capabilities) {
			eligible = append(eligible, agent)
		}
	}

	// Limit to max agents for auction
	if len(eligible) > m.config.MaxAgentsForAuction {
		eligible = eligible[:m.config.MaxAgentsForAuction]
	}

	return eligible, nil
}

// hasRequiredCapabilities checks if agent has all required capabilities
func (m *MetaAgent) hasRequiredCapabilities(agent *database.Agent, required []string) bool {
	// Parse agent capabilities (stored as JSON array string)
	var agentCaps []string
	if err := json.Unmarshal([]byte(agent.Capabilities), &agentCaps); err != nil {
		m.logger.Warn("failed to parse agent capabilities",
			zap.String("agent_id", agent.ID),
			zap.Error(err),
		)
		return false
	}

	// Convert to map for fast lookup
	capMap := make(map[string]bool)
	for _, cap := range agentCaps {
		capMap[cap] = true
	}

	// Check if all required capabilities are present
	for _, req := range required {
		if !capMap[req] {
			return false
		}
	}

	return true
}

// runAuction runs an auction to get bids from eligible agents
func (m *MetaAgent) runAuction(ctx context.Context, task *Task, agents []*database.Agent) ([]*AgentBid, error) {
	bids := make([]*AgentBid, 0, len(agents))

	for _, agent := range agents {
		// Calculate agent's bid price (currently using agent's base price)
		// TODO: Implement dynamic pricing based on demand, capacity, etc.
		bidPrice := agent.Price

		// Skip if bid exceeds budget
		if bidPrice > task.Budget {
			continue
		}

		// Calculate capability match score
		capMatch := m.calculateCapabilityMatch(agent, task.Capabilities)

		// Estimate execution time based on historical data
		estimatedTime := m.estimateExecutionTime(agent, task)

		bid := &AgentBid{
			Agent:           agent,
			BidPrice:        bidPrice,
			EstimatedTime:   estimatedTime,
			CapabilityMatch: capMatch,
			SubmittedAt:     time.Now(),
		}

		bids = append(bids, bid)
	}

	return bids, nil
}

// calculateCapabilityMatch calculates how well agent capabilities match task requirements
func (m *MetaAgent) calculateCapabilityMatch(agent *database.Agent, required []string) float64 {
	if len(required) == 0 {
		return 1.0
	}

	// Parse agent capabilities
	var agentCaps []string
	if err := json.Unmarshal([]byte(agent.Capabilities), &agentCaps); err != nil {
		return 0.0
	}

	// Count matching capabilities
	capMap := make(map[string]bool)
	for _, cap := range agentCaps {
		capMap[cap] = true
	}

	matches := 0
	for _, req := range required {
		if capMap[req] {
			matches++
		}
	}

	return float64(matches) / float64(len(required))
}

// estimateExecutionTime estimates task execution time based on agent's historical performance
func (m *MetaAgent) estimateExecutionTime(agent *database.Agent, task *Task) int64 {
	// Base estimation on tasks completed and average performance
	// For now, use a simple heuristic: faster agents have completed more tasks

	if agent.TasksCompleted == 0 {
		// New agent, estimate 5 seconds
		return 5000
	}

	// Assume faster agents complete more tasks
	// Rough heuristic: 1000ms + (10000ms / sqrt(tasks_completed))
	baseTime := 1000.0
	variableTime := 10000.0 / math.Sqrt(float64(agent.TasksCompleted))

	return int64(baseTime + variableTime)
}

// scoreAgents scores and ranks agent bids using multi-criteria algorithm
func (m *MetaAgent) scoreAgents(task *Task, bids []*AgentBid) []*AgentBid {
	if len(bids) == 0 {
		return bids
	}

	// Find min/max values for normalization
	minPrice, maxPrice := math.MaxFloat64, 0.0
	minTime, maxTime := int64(math.MaxInt64), int64(0)
	maxRating, maxTasks := 0.0, int64(0)

	for _, bid := range bids {
		if bid.BidPrice < minPrice {
			minPrice = bid.BidPrice
		}
		if bid.BidPrice > maxPrice {
			maxPrice = bid.BidPrice
		}
		if bid.EstimatedTime < minTime {
			minTime = bid.EstimatedTime
		}
		if bid.EstimatedTime > maxTime {
			maxTime = bid.EstimatedTime
		}
		if bid.Agent.Rating > maxRating {
			maxRating = bid.Agent.Rating
		}
		if bid.Agent.TasksCompleted > maxTasks {
			maxTasks = bid.Agent.TasksCompleted
		}
	}

	// Score each bid
	for _, bid := range bids {
		// Price score (lower is better, so invert)
		priceScore := 0.0
		if maxPrice > minPrice {
			priceScore = 1.0 - ((bid.BidPrice - minPrice) / (maxPrice - minPrice))
		} else {
			priceScore = 1.0
		}

		// Quality score (rating 0-5, normalize to 0-1)
		qualityScore := bid.Agent.Rating / 5.0

		// Speed score (lower time is better, so invert)
		speedScore := 0.0
		if maxTime > minTime {
			speedScore = 1.0 - (float64(bid.EstimatedTime-minTime) / float64(maxTime-minTime))
		} else {
			speedScore = 1.0
		}

		// Reputation score (more tasks completed is better)
		reputationScore := 0.0
		if maxTasks > 0 {
			reputationScore = float64(bid.Agent.TasksCompleted) / float64(maxTasks)
		}

		// Calculate weighted total score
		bid.Score = (priceScore * m.config.PriceWeight) +
			(qualityScore * m.config.QualityWeight) +
			(speedScore * m.config.SpeedWeight) +
			(reputationScore * m.config.ReputationWeight)

		// Boost score based on capability match
		bid.Score *= bid.CapabilityMatch
	}

	// Sort bids by score (highest first)
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Score > bids[j].Score
	})

	return bids
}

// GetFailoverAgent returns a backup agent if primary fails
func (m *MetaAgent) GetFailoverAgent(ctx context.Context, task *Task, failedAgentID string) (*database.Agent, error) {
	if !m.config.EnableFailover {
		return nil, errors.New("failover disabled")
	}

	m.logger.Info("getting failover agent",
		zap.String("task_id", task.ID),
		zap.String("failed_agent_id", failedAgentID),
	)

	// Find agents excluding the failed one
	agents, err := m.findEligibleAgents(ctx, task)
	if err != nil {
		return nil, err
	}

	// Filter out failed agent
	filtered := make([]*database.Agent, 0)
	for _, agent := range agents {
		if agent.ID != failedAgentID {
			filtered = append(filtered, agent)
		}
	}

	if len(filtered) == 0 {
		return nil, ErrNoAgentsAvailable
	}

	// Run auction with remaining agents
	bids, err := m.runAuction(ctx, task, filtered)
	if err != nil || len(bids) == 0 {
		return nil, ErrAuctionFailed
	}

	// Score and select best alternative
	scoredBids := m.scoreAgents(task, bids)

	m.logger.Info("failover agent selected",
		zap.String("agent_id", scoredBids[0].Agent.ID),
		zap.String("agent_name", scoredBids[0].Agent.Name),
	)

	return scoredBids[0].Agent, nil
}
