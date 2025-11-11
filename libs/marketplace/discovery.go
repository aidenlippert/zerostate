package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
)

var (
	// ErrAgentNotFound indicates no agent was found matching criteria
	ErrAgentNotFound = errors.New("no agent found matching criteria")

	// ErrNoCapableAgents indicates no agents have the required capabilities
	ErrNoCapableAgents = errors.New("no capable agents available")

	// ErrAgentUnavailable indicates agent is registered but unavailable
	ErrAgentUnavailable = errors.New("agent is unavailable")
)

// AgentStatus represents an agent's availability status
type AgentStatus string

const (
	AgentStatusOnline      AgentStatus = "online"
	AgentStatusBusy        AgentStatus = "busy"
	AgentStatusOffline     AgentStatus = "offline"
	AgentStatusMaintenance AgentStatus = "maintenance"
)

// AgentRecord represents a registered agent with discovery metadata
type AgentRecord struct {
	AgentCard *identity.AgentCard `json:"agent_card"`
	Status    AgentStatus         `json:"status"`

	// Health tracking
	LastSeen            time.Time     `json:"last_seen"`
	LastHealthCheck     time.Time     `json:"last_health_check"`
	ConsecutiveFailures int           `json:"consecutive_failures"`
	AverageResponseTime time.Duration `json:"average_response_time"`

	// Availability tracking
	CurrentLoad     int     `json:"current_load"`     // Number of active tasks
	MaxCapacity     int     `json:"max_capacity"`     // Maximum concurrent tasks
	UtilizationRate float64 `json:"utilization_rate"` // 0.0 to 1.0

	// Geographic/network metadata
	Region    string        `json:"region,omitempty"`
	NetworkID string        `json:"network_id,omitempty"`
	Latency   time.Duration `json:"latency,omitempty"`

	// Reputation integration
	ReputationScore float64 `json:"reputation_score"`
	QualityScore    float64 `json:"quality_score"`
}

// DiscoveryQuery represents a query for finding agents
type DiscoveryQuery struct {
	// Required capabilities
	Capabilities []string `json:"capabilities"`

	// Filtering criteria
	MinReputation   float64       `json:"min_reputation,omitempty"`
	MinQuality      float64       `json:"min_quality,omitempty"`
	MaxResponseTime time.Duration `json:"max_response_time,omitempty"`
	MaxUtilization  float64       `json:"max_utilization,omitempty"`

	// Geographic/network preferences
	PreferredRegions []string      `json:"preferred_regions,omitempty"`
	MaxLatency       time.Duration `json:"max_latency,omitempty"`

	// Result limits
	Limit int `json:"limit,omitempty"` // Default: 10
}

// DiscoveryResult represents a discovered agent with match quality
type DiscoveryResult struct {
	Record     *AgentRecord `json:"record"`
	MatchScore float64      `json:"match_score"` // 0.0 to 1.0
	Distance   float64      `json:"distance"`    // For similarity/proximity
}

// CapabilityIndex provides fast capability-based lookups
type CapabilityIndex struct {
	mu sync.RWMutex

	// Inverted index: capability -> set of agent DIDs
	capabilityToAgents map[string]map[string]bool

	// Agent registry
	agents map[string]*AgentRecord
}

// DiscoveryService manages agent discovery and health tracking
type DiscoveryService struct {
	mu    sync.RWMutex
	index *CapabilityIndex

	// P2P integration for distributed discovery
	messageBus p2p.MessageBus

	// Reputation integration
	reputationService *reputation.ReputationService

	// Health checking
	healthCheckInterval    time.Duration
	healthCheckTimeout     time.Duration
	maxConsecutiveFailures int

	// Configuration
	defaultLimit          int
	defaultMaxUtilization float64

	// Metrics
	metricsAgentsRegistered    prometheus.Gauge
	metricsAgentsOnline        prometheus.Gauge
	metricsDiscoveryQueries    prometheus.Counter
	metricsDiscoveryLatency    prometheus.Histogram
	metricsHealthChecks        prometheus.Counter
	metricsHealthCheckFailures prometheus.Counter

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewCapabilityIndex creates a new capability index
func NewCapabilityIndex() *CapabilityIndex {
	return &CapabilityIndex{
		capabilityToAgents: make(map[string]map[string]bool),
		agents:             make(map[string]*AgentRecord),
	}
}

// AddAgent adds an agent to the index
func (idx *CapabilityIndex) AddAgent(record *AgentRecord) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	agentDID := record.AgentCard.DID

	// Add to agent registry
	idx.agents[agentDID] = record

	// Update inverted index
	for _, capability := range record.AgentCard.Capabilities {
		if idx.capabilityToAgents[capability] == nil {
			idx.capabilityToAgents[capability] = make(map[string]bool)
		}
		idx.capabilityToAgents[capability][agentDID] = true
	}
}

// RemoveAgent removes an agent from the index
func (idx *CapabilityIndex) RemoveAgent(agentDID string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	record, exists := idx.agents[agentDID]
	if !exists {
		return
	}

	// Remove from inverted index
	for _, capability := range record.AgentCard.Capabilities {
		delete(idx.capabilityToAgents[capability], agentDID)
		if len(idx.capabilityToAgents[capability]) == 0 {
			delete(idx.capabilityToAgents, capability)
		}
	}

	// Remove from agent registry
	delete(idx.agents, agentDID)
}

// FindByCapabilities finds agents with all required capabilities
func (idx *CapabilityIndex) FindByCapabilities(capabilities []string) []*AgentRecord {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if len(capabilities) == 0 {
		// Return all agents
		results := make([]*AgentRecord, 0, len(idx.agents))
		for _, record := range idx.agents {
			results = append(results, record)
		}
		return results
	}

	// Find intersection of agents with all capabilities
	var candidateSet map[string]bool
	for i, capability := range capabilities {
		agentSet, exists := idx.capabilityToAgents[capability]
		if !exists {
			// No agents have this capability
			return nil
		}

		if i == 0 {
			// First capability: start with this set
			candidateSet = make(map[string]bool)
			for agentDID := range agentSet {
				candidateSet[agentDID] = true
			}
		} else {
			// Subsequent capabilities: intersect with current candidates
			for agentDID := range candidateSet {
				if !agentSet[agentDID] {
					delete(candidateSet, agentDID)
				}
			}
		}

		if len(candidateSet) == 0 {
			// No agents have all capabilities so far
			return nil
		}
	}

	// Convert to agent records
	results := make([]*AgentRecord, 0, len(candidateSet))
	for agentDID := range candidateSet {
		if record, exists := idx.agents[agentDID]; exists {
			results = append(results, record)
		}
	}

	return results
}

// GetAgent retrieves an agent record by DID
func (idx *CapabilityIndex) GetAgent(agentDID string) (*AgentRecord, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	record, exists := idx.agents[agentDID]
	return record, exists
}

// UpdateAgent updates an existing agent record
func (idx *CapabilityIndex) UpdateAgent(record *AgentRecord) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	agentDID := record.AgentCard.DID
	oldRecord, exists := idx.agents[agentDID]
	if !exists {
		return ErrAgentNotFound
	}

	// Check if capabilities changed
	capabilitiesChanged := false
	if len(oldRecord.AgentCard.Capabilities) != len(record.AgentCard.Capabilities) {
		capabilitiesChanged = true
	} else {
		oldCaps := make(map[string]bool)
		for _, cap := range oldRecord.AgentCard.Capabilities {
			oldCaps[cap] = true
		}
		for _, cap := range record.AgentCard.Capabilities {
			if !oldCaps[cap] {
				capabilitiesChanged = true
				break
			}
		}
	}

	// If capabilities changed, rebuild index for this agent
	if capabilitiesChanged {
		// Remove old capability mappings
		for _, capability := range oldRecord.AgentCard.Capabilities {
			delete(idx.capabilityToAgents[capability], agentDID)
			if len(idx.capabilityToAgents[capability]) == 0 {
				delete(idx.capabilityToAgents, capability)
			}
		}

		// Add new capability mappings
		for _, capability := range record.AgentCard.Capabilities {
			if idx.capabilityToAgents[capability] == nil {
				idx.capabilityToAgents[capability] = make(map[string]bool)
			}
			idx.capabilityToAgents[capability][agentDID] = true
		}
	}

	// Update agent record
	idx.agents[agentDID] = record
	return nil
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(
	messageBus p2p.MessageBus,
	reputationService *reputation.ReputationService,
) *DiscoveryService {
	ctx, cancel := context.WithCancel(context.Background())

	ds := &DiscoveryService{
		index:                  NewCapabilityIndex(),
		messageBus:             messageBus,
		reputationService:      reputationService,
		healthCheckInterval:    30 * time.Second,
		healthCheckTimeout:     5 * time.Second,
		maxConsecutiveFailures: 3,
		defaultLimit:           10,
		defaultMaxUtilization:  0.8,
		ctx:                    ctx,
		cancel:                 cancel,

		// Metrics
		metricsAgentsRegistered: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "zerostate_agents_registered",
			Help: "Total number of registered agents",
		}),
		metricsAgentsOnline: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "zerostate_agents_online",
			Help: "Number of online agents",
		}),
		metricsDiscoveryQueries: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_discovery_queries_total",
			Help: "Total number of discovery queries",
		}),
		metricsDiscoveryLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_discovery_latency_seconds",
			Help:    "Discovery query latency",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),
		metricsHealthChecks: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_agent_health_checks_total",
			Help: "Total number of agent health checks",
		}),
		metricsHealthCheckFailures: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_agent_health_check_failures_total",
			Help: "Total number of failed health checks",
		}),
	}

	// Start background health checking
	ds.wg.Add(1)
	go ds.healthCheckLoop()

	return ds
}

// RegisterAgent registers a new agent with the discovery service
func (ds *DiscoveryService) RegisterAgent(ctx context.Context, agentCard *identity.AgentCard) error {
	// Get reputation score
	var reputationScore float64
	if ds.reputationService != nil {
		score, err := ds.reputationService.GetScore(ctx, agentCard.DID)
		if err == nil {
			reputationScore = score.OverallScore
		}
	}

	record := &AgentRecord{
		AgentCard:           agentCard,
		Status:              AgentStatusOnline,
		LastSeen:            time.Now(),
		LastHealthCheck:     time.Now(),
		ConsecutiveFailures: 0,
		CurrentLoad:         0,
		MaxCapacity:         10, // Default capacity
		UtilizationRate:     0.0,
		ReputationScore:     reputationScore,
		QualityScore:        80.0, // Default quality score
	}

	ds.index.AddAgent(record)
	ds.metricsAgentsRegistered.Inc()
	ds.metricsAgentsOnline.Inc()

	return nil
}

// UnregisterAgent removes an agent from the discovery service
func (ds *DiscoveryService) UnregisterAgent(ctx context.Context, agentDID string) error {
	record, exists := ds.index.GetAgent(agentDID)
	if !exists {
		return ErrAgentNotFound
	}

	ds.index.RemoveAgent(agentDID)
	ds.metricsAgentsRegistered.Dec()

	if record.Status == AgentStatusOnline {
		ds.metricsAgentsOnline.Dec()
	}

	return nil
}

// UpdateAgentStatus updates an agent's status
func (ds *DiscoveryService) UpdateAgentStatus(ctx context.Context, agentDID string, status AgentStatus) error {
	record, exists := ds.index.GetAgent(agentDID)
	if !exists {
		return ErrAgentNotFound
	}

	oldStatus := record.Status
	record.Status = status
	record.LastSeen = time.Now()

	// Update metrics
	if oldStatus != AgentStatusOnline && status == AgentStatusOnline {
		ds.metricsAgentsOnline.Inc()
	} else if oldStatus == AgentStatusOnline && status != AgentStatusOnline {
		ds.metricsAgentsOnline.Dec()
	}

	return ds.index.UpdateAgent(record)
}

// UpdateAgentLoad updates an agent's current load
func (ds *DiscoveryService) UpdateAgentLoad(ctx context.Context, agentDID string, currentLoad int) error {
	record, exists := ds.index.GetAgent(agentDID)
	if !exists {
		return ErrAgentNotFound
	}

	record.CurrentLoad = currentLoad
	record.UtilizationRate = float64(currentLoad) / float64(record.MaxCapacity)
	record.LastSeen = time.Now()

	return ds.index.UpdateAgent(record)
}

// DiscoverAgents finds agents matching the query
func (ds *DiscoveryService) DiscoverAgents(ctx context.Context, query *DiscoveryQuery) ([]*DiscoveryResult, error) {
	startTime := time.Now()
	defer func() {
		ds.metricsDiscoveryQueries.Inc()
		ds.metricsDiscoveryLatency.Observe(time.Since(startTime).Seconds())
	}()

	// Find agents with required capabilities
	candidates := ds.index.FindByCapabilities(query.Capabilities)
	if len(candidates) == 0 {
		return nil, ErrNoCapableAgents
	}

	// Filter and score candidates
	results := make([]*DiscoveryResult, 0)
	for _, record := range candidates {
		// Apply filters
		if !ds.matchesFilters(record, query) {
			continue
		}

		// Calculate match score
		matchScore := ds.calculateMatchScore(record, query)

		results = append(results, &DiscoveryResult{
			Record:     record,
			MatchScore: matchScore,
		})
	}

	if len(results) == 0 {
		return nil, ErrNoCapableAgents
	}

	// Sort by match score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].MatchScore > results[j].MatchScore
	})

	// Apply limit
	limit := query.Limit
	if limit <= 0 {
		limit = ds.defaultLimit
	}
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// matchesFilters checks if agent record matches query filters
func (ds *DiscoveryService) matchesFilters(record *AgentRecord, query *DiscoveryQuery) bool {
	// Status check: only online or busy agents
	if record.Status != AgentStatusOnline && record.Status != AgentStatusBusy {
		return false
	}

	// Reputation filter
	if query.MinReputation > 0 && record.ReputationScore < query.MinReputation {
		return false
	}

	// Quality filter
	if query.MinQuality > 0 && record.QualityScore < query.MinQuality {
		return false
	}

	// Response time filter
	if query.MaxResponseTime > 0 && record.AverageResponseTime > query.MaxResponseTime {
		return false
	}

	// Utilization filter
	maxUtil := query.MaxUtilization
	if maxUtil <= 0 {
		maxUtil = ds.defaultMaxUtilization
	}
	if record.UtilizationRate > maxUtil {
		return false
	}

	// Latency filter
	if query.MaxLatency > 0 && record.Latency > query.MaxLatency {
		return false
	}

	// Region preference (soft filter via scoring, not hard filter)
	return true
}

// calculateMatchScore computes a match score for an agent
func (ds *DiscoveryService) calculateMatchScore(record *AgentRecord, query *DiscoveryQuery) float64 {
	const (
		weightReputation   = 0.30 // 30% - Reputation
		weightQuality      = 0.25 // 25% - Quality
		weightAvailability = 0.20 // 20% - Availability
		weightResponseTime = 0.15 // 15% - Response time
		weightRegion       = 0.10 // 10% - Region preference
	)

	// Reputation score (0 to 1)
	reputationScore := record.ReputationScore / 100.0

	// Quality score (0 to 1)
	qualityScore := record.QualityScore / 100.0

	// Availability score (lower utilization is better)
	availabilityScore := 1.0 - record.UtilizationRate

	// Response time score (lower is better, normalized)
	responseTimeScore := 1.0
	if record.AverageResponseTime > 0 && query.MaxResponseTime > 0 {
		responseTimeScore = 1.0 - float64(record.AverageResponseTime)/float64(query.MaxResponseTime)
		if responseTimeScore < 0 {
			responseTimeScore = 0
		}
	}

	// Region score (1.0 if in preferred regions, 0.5 otherwise)
	regionScore := 0.5
	if len(query.PreferredRegions) > 0 {
		for _, region := range query.PreferredRegions {
			if record.Region == region {
				regionScore = 1.0
				break
			}
		}
	} else {
		regionScore = 1.0 // No preference
	}

	return (weightReputation * reputationScore) +
		(weightQuality * qualityScore) +
		(weightAvailability * availabilityScore) +
		(weightResponseTime * responseTimeScore) +
		(weightRegion * regionScore)
}

// healthCheckLoop performs periodic health checks on all agents
func (ds *DiscoveryService) healthCheckLoop() {
	defer ds.wg.Done()

	ticker := time.NewTicker(ds.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ds.ctx.Done():
			return
		case <-ticker.C:
			ds.performHealthChecks()
		}
	}
}

// performHealthChecks checks health of all registered agents
func (ds *DiscoveryService) performHealthChecks() {
	ds.mu.RLock()
	agents := make([]*AgentRecord, 0, len(ds.index.agents))
	for _, record := range ds.index.agents {
		agents = append(agents, record)
	}
	ds.mu.RUnlock()

	for _, record := range agents {
		ds.checkAgentHealth(record)
	}
}

// checkAgentHealth performs health check on a single agent
func (ds *DiscoveryService) checkAgentHealth(record *AgentRecord) {
	ds.metricsHealthChecks.Inc()

	ctx, cancel := context.WithTimeout(ds.ctx, ds.healthCheckTimeout)
	defer cancel()

	// Send health check request via P2P
	req := &p2p.TaskRequest{
		TaskID: fmt.Sprintf("health-check-%d", time.Now().Unix()),
		Type:   "health-check",
	}

	startTime := time.Now()
	_, err := ds.messageBus.SendRequest(ctx, record.AgentCard.DID, req, ds.healthCheckTimeout)
	responseTime := time.Since(startTime)

	record.LastHealthCheck = time.Now()

	if err != nil {
		// Health check failed
		ds.metricsHealthCheckFailures.Inc()
		record.ConsecutiveFailures++

		if record.ConsecutiveFailures >= ds.maxConsecutiveFailures {
			// Mark as offline
			if record.Status == AgentStatusOnline {
				record.Status = AgentStatusOffline
				ds.metricsAgentsOnline.Dec()
			}
		}
	} else {
		// Health check succeeded
		record.ConsecutiveFailures = 0
		record.LastSeen = time.Now()

		// Update average response time (exponential moving average)
		if record.AverageResponseTime == 0 {
			record.AverageResponseTime = responseTime
		} else {
			alpha := 0.3 // Smoothing factor
			record.AverageResponseTime = time.Duration(
				alpha*float64(responseTime) + (1-alpha)*float64(record.AverageResponseTime),
			)
		}

		// Update status if was offline
		if record.Status == AgentStatusOffline {
			record.Status = AgentStatusOnline
			ds.metricsAgentsOnline.Inc()
		}
	}

	ds.index.UpdateAgent(record)
}

// Close shuts down the discovery service
func (ds *DiscoveryService) Close() error {
	ds.cancel()
	ds.wg.Wait()
	return nil
}

// GetAgentCount returns the number of registered agents by status
func (ds *DiscoveryService) GetAgentCount() map[AgentStatus]int {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	counts := make(map[AgentStatus]int)
	for _, record := range ds.index.agents {
		counts[record.Status]++
	}
	return counts
}
