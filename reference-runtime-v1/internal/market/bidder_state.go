package market

import (
	"sync"
	"time"
)

// BidderState tracks the runtime's current operational state
// This is critical for load-aware pricing and capacity management
type BidderState struct {
	mu sync.RWMutex

	// Capacity tracking
	ActiveTasks int     // Currently executing tasks
	MaxTasks    int     // Maximum concurrent capacity
	LoadFactor  float64 // 0.0 = idle, 1.0 = full capacity

	// Per-capability statistics
	CapabilityStats map[string]*CapabilityMetrics

	// Global metrics
	TotalBidsSubmitted  int
	TotalBidsAccepted   int
	TotalBidsRejected   int
	TotalTasksCompleted int
	TotalRevenue        float64

	// Last updated timestamp
	LastUpdated time.Time
}

// CapabilityMetrics tracks performance for a specific capability
// Used by pricing strategies to learn optimal behavior
type CapabilityMetrics struct {
	// Bidding statistics
	TotalBids    int
	AcceptedBids int
	RejectedBids int
	WinRate      float64 // AcceptedBids / TotalBids

	// Execution statistics
	TasksCompleted int
	TasksFailed    int
	SuccessRate    float64 // TasksCompleted / (TasksCompleted + TasksFailed)

	// Economic metrics
	TotalRevenue float64
	AvgProfit    float64 // Revenue - estimated costs
	LastBidPrice float64

	// Performance metrics
	AvgLatencyMs float64
	P95LatencyMs float64
	TimeoutRate  float64

	// Learning signals
	RecentOutcomes []TaskOutcome // Last N outcomes for RL training
	LastUpdated    time.Time
}

// TaskOutcome represents the result of a completed task
// Used as training signal for RL-based pricing strategies
type TaskOutcome struct {
	TaskID     string
	CFPId      string
	BidID      string
	Capability string
	BidPrice   float64
	ActualCost float64 // Estimated compute cost
	Profit     float64 // BidPrice - ActualCost
	LatencyMs  float64
	Success    bool
	Won        bool
	Rating     float64 // 0.0 - 1.0 from orchestrator
	Timestamp  time.Time
}

// NewBidderState creates a new state tracker
func NewBidderState(maxTasks int) *BidderState {
	return &BidderState{
		ActiveTasks:     0,
		MaxTasks:        maxTasks,
		LoadFactor:      0.0,
		CapabilityStats: make(map[string]*CapabilityMetrics),
		LastUpdated:     time.Now(),
	}
}

// GetLoadFactor returns current capacity utilization (0.0 - 1.0)
func (s *BidderState) GetLoadFactor() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.MaxTasks == 0 {
		return 0.0
	}
	return float64(s.ActiveTasks) / float64(s.MaxTasks)
}

// CanAcceptTask returns true if there's capacity for another task
func (s *BidderState) CanAcceptTask() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ActiveTasks < s.MaxTasks
}

// IncrementActiveTasks atomically increments active task count
func (s *BidderState) IncrementActiveTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ActiveTasks++
	s.LoadFactor = float64(s.ActiveTasks) / float64(s.MaxTasks)
	s.LastUpdated = time.Now()
}

// DecrementActiveTasks atomically decrements active task count
func (s *BidderState) DecrementActiveTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ActiveTasks > 0 {
		s.ActiveTasks--
	}
	s.LoadFactor = float64(s.ActiveTasks) / float64(s.MaxTasks)
	s.LastUpdated = time.Now()
}

// RecordBidSubmitted updates stats when a bid is sent
func (s *BidderState) RecordBidSubmitted(capability string, price float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalBidsSubmitted++

	if _, exists := s.CapabilityStats[capability]; !exists {
		s.CapabilityStats[capability] = &CapabilityMetrics{
			RecentOutcomes: make([]TaskOutcome, 0, 10),
		}
	}

	stats := s.CapabilityStats[capability]
	stats.TotalBids++
	stats.LastBidPrice = price
	stats.LastUpdated = time.Now()
	s.LastUpdated = time.Now()
}

// RecordBidAccepted updates stats when orchestrator accepts our bid
func (s *BidderState) RecordBidAccepted(capability string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalBidsAccepted++

	if stats, exists := s.CapabilityStats[capability]; exists {
		stats.AcceptedBids++
		stats.WinRate = float64(stats.AcceptedBids) / float64(stats.TotalBids)
		stats.LastUpdated = time.Now()
	}

	s.LastUpdated = time.Now()
}

// RecordBidRejected updates stats when orchestrator rejects our bid
func (s *BidderState) RecordBidRejected(capability string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.TotalBidsRejected++

	if stats, exists := s.CapabilityStats[capability]; exists {
		stats.RejectedBids++
		stats.WinRate = float64(stats.AcceptedBids) / float64(stats.TotalBids)
		stats.LastUpdated = time.Now()
	}

	s.LastUpdated = time.Now()
}

// RecordTaskOutcome updates stats when a task completes
// This is the key learning signal for RL-based strategies
func (s *BidderState) RecordTaskOutcome(outcome TaskOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if outcome.Success {
		s.TotalTasksCompleted++
		s.TotalRevenue += outcome.Profit
	}

	if stats, exists := s.CapabilityStats[outcome.Capability]; exists {
		if outcome.Success {
			stats.TasksCompleted++
			stats.TotalRevenue += outcome.Profit
		} else {
			stats.TasksFailed++
		}

		// Update success rate
		total := stats.TasksCompleted + stats.TasksFailed
		if total > 0 {
			stats.SuccessRate = float64(stats.TasksCompleted) / float64(total)
		}

		// Update average profit
		if stats.TasksCompleted > 0 {
			stats.AvgProfit = stats.TotalRevenue / float64(stats.TasksCompleted)
		}

		// Update latency statistics
		if outcome.Success {
			// Simple moving average (can be improved with exponential moving average)
			if stats.TasksCompleted == 1 {
				stats.AvgLatencyMs = outcome.LatencyMs
			} else {
				alpha := 0.1 // Smoothing factor
				stats.AvgLatencyMs = alpha*outcome.LatencyMs + (1-alpha)*stats.AvgLatencyMs
			}
		}

		// Store outcome for RL training (keep last 10)
		stats.RecentOutcomes = append(stats.RecentOutcomes, outcome)
		if len(stats.RecentOutcomes) > 10 {
			stats.RecentOutcomes = stats.RecentOutcomes[1:]
		}

		stats.LastUpdated = time.Now()
	}

	s.LastUpdated = time.Now()
}

// GetCapabilityStats returns a copy of metrics for a capability
func (s *BidderState) GetCapabilityStats(capability string) *CapabilityMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if stats, exists := s.CapabilityStats[capability]; exists {
		// Return a copy to avoid race conditions
		copy := *stats
		return &copy
	}

	return nil
}

// GetGlobalWinRate returns overall bid acceptance rate
func (s *BidderState) GetGlobalWinRate() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TotalBidsSubmitted == 0 {
		return 0.0
	}
	return float64(s.TotalBidsAccepted) / float64(s.TotalBidsSubmitted)
}

// GetAverageProfit returns average profit across all capabilities
func (s *BidderState) GetAverageProfit() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TotalTasksCompleted == 0 {
		return 0.0
	}
	return s.TotalRevenue / float64(s.TotalTasksCompleted)
}
