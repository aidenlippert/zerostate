package health

import (
	"context"
	"fmt"
	"time"
)

// P2PChecker checks P2P network health
type P2PChecker struct {
	getPeerCount  func() int
	minPeers      int
	getHealthRate func() float64  // Health check success rate
}

// NewP2PChecker creates a P2P health checker
func NewP2PChecker(getPeerCount func() int, minPeers int, getHealthRate func() float64) *P2PChecker {
	return &P2PChecker{
		getPeerCount:  getPeerCount,
		minPeers:      minPeers,
		getHealthRate: getHealthRate,
	}
}

func (c *P2PChecker) Name() string {
	return "p2p"
}

func (c *P2PChecker) Check(ctx context.Context) CheckResult {
	peerCount := c.getPeerCount()
	healthRate := c.getHealthRate()

	// No peers = unhealthy
	if peerCount == 0 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "no peer connections",
			Metadata: map[string]interface{}{
				"peer_count":  peerCount,
				"health_rate": healthRate,
			},
		}
	}

	// Low health check rate = degraded
	if healthRate < 0.7 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("low health check rate: %.2f%%", healthRate*100),
			Metadata: map[string]interface{}{
				"peer_count":  peerCount,
				"health_rate": healthRate,
			},
		}
	}

	// Below minimum peers = degraded
	if peerCount < c.minPeers {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("peer count %d below minimum %d", peerCount, c.minPeers),
			Metadata: map[string]interface{}{
				"peer_count":  peerCount,
				"min_peers":   c.minPeers,
				"health_rate": healthRate,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("%d peers connected, health rate %.2f%%", peerCount, healthRate*100),
		Metadata: map[string]interface{}{
			"peer_count":  peerCount,
			"health_rate": healthRate,
		},
	}
}

// ExecutionChecker checks WASM execution health
type ExecutionChecker struct {
	getActiveExecutions func() int
	getSuccessRate      func() float64
	getAvgDuration      func() time.Duration
	maxDuration         time.Duration
}

// NewExecutionChecker creates an execution health checker
func NewExecutionChecker(
	getActiveExecutions func() int,
	getSuccessRate func() float64,
	getAvgDuration func() time.Duration,
	maxDuration time.Duration,
) *ExecutionChecker {
	return &ExecutionChecker{
		getActiveExecutions: getActiveExecutions,
		getSuccessRate:      getSuccessRate,
		getAvgDuration:      getAvgDuration,
		maxDuration:         maxDuration,
	}
}

func (c *ExecutionChecker) Name() string {
	return "execution"
}

func (c *ExecutionChecker) Check(ctx context.Context) CheckResult {
	activeExecutions := c.getActiveExecutions()
	successRate := c.getSuccessRate()
	avgDuration := c.getAvgDuration()

	// Low success rate = unhealthy
	if successRate < 0.5 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: fmt.Sprintf("low success rate: %.2f%%", successRate*100),
			Metadata: map[string]interface{}{
				"active_executions": activeExecutions,
				"success_rate":      successRate,
				"avg_duration_ms":   avgDuration.Milliseconds(),
			},
		}
	}

	// High average duration = degraded
	if avgDuration > c.maxDuration {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("high avg duration: %s (max: %s)", avgDuration, c.maxDuration),
			Metadata: map[string]interface{}{
				"active_executions": activeExecutions,
				"success_rate":      successRate,
				"avg_duration_ms":   avgDuration.Milliseconds(),
				"max_duration_ms":   c.maxDuration.Milliseconds(),
			},
		}
	}

	// Medium success rate = degraded
	if successRate < 0.9 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("degraded success rate: %.2f%%", successRate*100),
			Metadata: map[string]interface{}{
				"active_executions": activeExecutions,
				"success_rate":      successRate,
				"avg_duration_ms":   avgDuration.Milliseconds(),
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("%d active, %.2f%% success, avg %s", activeExecutions, successRate*100, avgDuration),
		Metadata: map[string]interface{}{
			"active_executions": activeExecutions,
			"success_rate":      successRate,
			"avg_duration_ms":   avgDuration.Milliseconds(),
		},
	}
}

// PaymentChecker checks payment channel health
type PaymentChecker struct {
	getActiveChannels func() int
	getSuccessRate    func() float64
	getTotalLocked    func() float64
}

// NewPaymentChecker creates a payment health checker
func NewPaymentChecker(
	getActiveChannels func() int,
	getSuccessRate func() float64,
	getTotalLocked func() float64,
) *PaymentChecker {
	return &PaymentChecker{
		getActiveChannels: getActiveChannels,
		getSuccessRate:    getSuccessRate,
		getTotalLocked:    getTotalLocked,
	}
}

func (c *PaymentChecker) Name() string {
	return "payment"
}

func (c *PaymentChecker) Check(ctx context.Context) CheckResult {
	activeChannels := c.getActiveChannels()
	successRate := c.getSuccessRate()
	totalLocked := c.getTotalLocked()

	// Very low success rate = unhealthy
	if successRate < 0.8 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: fmt.Sprintf("low payment success rate: %.2f%%", successRate*100),
			Metadata: map[string]interface{}{
				"active_channels": activeChannels,
				"success_rate":    successRate,
				"total_locked":    totalLocked,
			},
		}
	}

	// Low success rate = degraded
	if successRate < 0.95 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("degraded payment success rate: %.2f%%", successRate*100),
			Metadata: map[string]interface{}{
				"active_channels": activeChannels,
				"success_rate":    successRate,
				"total_locked":    totalLocked,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("%d channels, %.2f%% success, %.2f locked", activeChannels, successRate*100, totalLocked),
		Metadata: map[string]interface{}{
			"active_channels": activeChannels,
			"success_rate":    successRate,
			"total_locked":    totalLocked,
		},
	}
}

// GuildChecker checks guild formation health
type GuildChecker struct {
	getActiveGuilds func() int
	getAvgMembers   func() float64
}

// NewGuildChecker creates a guild health checker
func NewGuildChecker(getActiveGuilds func() int, getAvgMembers func() float64) *GuildChecker {
	return &GuildChecker{
		getActiveGuilds: getActiveGuilds,
		getAvgMembers:   getAvgMembers,
	}
}

func (c *GuildChecker) Name() string {
	return "guild"
}

func (c *GuildChecker) Check(ctx context.Context) CheckResult {
	activeGuilds := c.getActiveGuilds()
	avgMembers := c.getAvgMembers()

	// No guilds = healthy (may be idle)
	if activeGuilds == 0 {
		return CheckResult{
			Status:  StatusHealthy,
			Message: "no active guilds (idle)",
			Metadata: map[string]interface{}{
				"active_guilds": activeGuilds,
				"avg_members":   avgMembers,
			},
		}
	}

	// Very low average members = degraded
	if avgMembers < 2.0 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("low average guild members: %.1f", avgMembers),
			Metadata: map[string]interface{}{
				"active_guilds": activeGuilds,
				"avg_members":   avgMembers,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("%d active guilds, avg %.1f members", activeGuilds, avgMembers),
		Metadata: map[string]interface{}{
			"active_guilds": activeGuilds,
			"avg_members":   avgMembers,
		},
	}
}

// DHTChecker checks DHT (Kademlia) health
type DHTChecker struct {
	getRoutingTableSize func() int
	getDHTSuccessRate   func() float64
}

// NewDHTChecker creates a DHT health checker
func NewDHTChecker(getRoutingTableSize func() int, getDHTSuccessRate func() float64) *DHTChecker {
	return &DHTChecker{
		getRoutingTableSize: getRoutingTableSize,
		getDHTSuccessRate:   getDHTSuccessRate,
	}
}

func (c *DHTChecker) Name() string {
	return "dht"
}

func (c *DHTChecker) Check(ctx context.Context) CheckResult {
	routingTableSize := c.getRoutingTableSize()
	successRate := c.getDHTSuccessRate()

	// Empty routing table = unhealthy
	if routingTableSize == 0 {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: "empty DHT routing table",
			Metadata: map[string]interface{}{
				"routing_table_size": routingTableSize,
				"success_rate":       successRate,
			},
		}
	}

	// Low success rate = degraded
	if successRate < 0.8 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("low DHT success rate: %.2f%%", successRate*100),
			Metadata: map[string]interface{}{
				"routing_table_size": routingTableSize,
				"success_rate":       successRate,
			},
		}
	}

	// Small routing table = degraded
	if routingTableSize < 10 {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("small DHT routing table: %d peers", routingTableSize),
			Metadata: map[string]interface{}{
				"routing_table_size": routingTableSize,
				"success_rate":       successRate,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("%d peers in routing table, %.2f%% success", routingTableSize, successRate*100),
		Metadata: map[string]interface{}{
			"routing_table_size": routingTableSize,
			"success_rate":       successRate,
		},
	}
}

// StorageChecker checks storage health
type StorageChecker struct {
	getDiskUsagePercent func() float64
	warnThreshold       float64
	criticalThreshold   float64
}

// NewStorageChecker creates a storage health checker
func NewStorageChecker(getDiskUsagePercent func() float64, warnThreshold, criticalThreshold float64) *StorageChecker {
	return &StorageChecker{
		getDiskUsagePercent: getDiskUsagePercent,
		warnThreshold:       warnThreshold,
		criticalThreshold:   criticalThreshold,
	}
}

func (c *StorageChecker) Name() string {
	return "storage"
}

func (c *StorageChecker) Check(ctx context.Context) CheckResult {
	usage := c.getDiskUsagePercent()

	if usage >= c.criticalThreshold {
		return CheckResult{
			Status:  StatusUnhealthy,
			Message: fmt.Sprintf("critical disk usage: %.1f%%", usage),
			Metadata: map[string]interface{}{
				"disk_usage_percent": usage,
				"critical_threshold": c.criticalThreshold,
			},
		}
	}

	if usage >= c.warnThreshold {
		return CheckResult{
			Status:  StatusDegraded,
			Message: fmt.Sprintf("high disk usage: %.1f%%", usage),
			Metadata: map[string]interface{}{
				"disk_usage_percent": usage,
				"warn_threshold":     c.warnThreshold,
			},
		}
	}

	return CheckResult{
		Status:  StatusHealthy,
		Message: fmt.Sprintf("disk usage: %.1f%%", usage),
		Metadata: map[string]interface{}{
			"disk_usage_percent": usage,
		},
	}
}
