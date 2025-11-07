// Package p2p provides peer health monitoring and failure detection
package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	peerFailuresTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "peer_failures_total",
			Help: "Total peer failures detected",
		},
		[]string{"peer_id", "reason"},
	)

	heartbeatMissesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "heartbeat_misses_total",
			Help: "Total heartbeat misses",
		},
		[]string{"peer_id"},
	)

	peerHealthGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "peer_health_status",
			Help: "Peer health status (1=healthy, 0=unhealthy)",
		},
		[]string{"peer_id"},
	)

	healthCheckLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "health_check_latency_seconds",
			Help:    "Health check latency",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"peer_id"},
	)

	activePeerMonitors = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_peer_monitors",
			Help: "Number of actively monitored peers",
		},
	)
)

const (
	// DefaultHeartbeatInterval is the default heartbeat interval
	DefaultHeartbeatInterval = 30 * time.Second
	// DefaultFailureThreshold is the default consecutive failures before marking dead
	DefaultFailureThreshold = 3
	// DefaultHealthCheckTimeout is the default health check timeout
	DefaultHealthCheckTimeout = 5 * time.Second
)

// HealthCheckConfig holds health check configuration
type HealthCheckConfig struct {
	// HeartbeatInterval is how often to check peer health
	HeartbeatInterval time.Duration
	// FailureThreshold is consecutive failures before marking dead
	FailureThreshold int
	// HealthCheckTimeout is timeout for health checks
	HealthCheckTimeout time.Duration
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
}

// DefaultHealthCheckConfig returns default configuration
func DefaultHealthCheckConfig() *HealthCheckConfig {
	return &HealthCheckConfig{
		HeartbeatInterval:  DefaultHeartbeatInterval,
		FailureThreshold:   DefaultFailureThreshold,
		HealthCheckTimeout: DefaultHealthCheckTimeout,
		EnableMetrics:      true,
	}
}

// PeerHealth represents peer health status
type PeerHealth struct {
	PeerID           peer.ID
	Healthy          bool
	ConsecutiveFails int
	LastCheck        time.Time
	LastSuccess      time.Time
	AverageLatency   time.Duration
	TotalChecks      int
	mu               sync.RWMutex
}

// HealthMonitor monitors peer health
type HealthMonitor struct {
	host    host.Host
	peers   map[peer.ID]*PeerHealth
	config  *HealthCheckConfig
	logger  *zap.Logger
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewHealthMonitor creates a new health monitor
func NewHealthMonitor(ctx context.Context, h host.Host, config *HealthCheckConfig, logger *zap.Logger) *HealthMonitor {
	if config == nil {
		config = DefaultHealthCheckConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	monitorCtx, cancel := context.WithCancel(ctx)

	hm := &HealthMonitor{
		host:   h,
		peers:  make(map[peer.ID]*PeerHealth),
		config: config,
		logger: logger,
		ctx:    monitorCtx,
		cancel: cancel,
	}

	logger.Info("health monitor created",
		zap.Duration("heartbeat_interval", config.HeartbeatInterval),
		zap.Int("failure_threshold", config.FailureThreshold),
	)

	return hm
}

// MonitorPeer starts monitoring a peer's health
func (hm *HealthMonitor) MonitorPeer(peerID peer.ID) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.peers[peerID]; exists {
		return // Already monitoring
	}

	peerHealth := &PeerHealth{
		PeerID:      peerID,
		Healthy:     true,
		LastCheck:   time.Now(),
		LastSuccess: time.Now(),
	}

	hm.peers[peerID] = peerHealth
	activePeerMonitors.Set(float64(len(hm.peers)))
	peerHealthGauge.WithLabelValues(peerID.String()).Set(1)

	// Start monitoring goroutine
	hm.wg.Add(1)
	go hm.monitorLoop(peerID)

	hm.logger.Debug("started monitoring peer",
		zap.String("peer_id", peerID.String()),
	)
}

// StopMonitoring stops monitoring a peer
func (hm *HealthMonitor) StopMonitoring(peerID peer.ID) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.peers[peerID]; exists {
		delete(hm.peers, peerID)
		activePeerMonitors.Set(float64(len(hm.peers)))
		peerHealthGauge.WithLabelValues(peerID.String()).Set(0)

		hm.logger.Debug("stopped monitoring peer",
			zap.String("peer_id", peerID.String()),
		)
	}
}

// monitorLoop runs health checks for a peer
func (hm *HealthMonitor) monitorLoop(peerID peer.ID) {
	defer hm.wg.Done()

	ticker := time.NewTicker(hm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hm.checkPeerHealth(peerID)
		case <-hm.ctx.Done():
			return
		}

		// Stop if peer removed
		hm.mu.RLock()
		_, exists := hm.peers[peerID]
		hm.mu.RUnlock()
		if !exists {
			return
		}
	}
}

// checkPeerHealth performs a health check on a peer
func (hm *HealthMonitor) checkPeerHealth(peerID peer.ID) {
	start := time.Now()

	// Get peer health
	hm.mu.RLock()
	peerHealth, exists := hm.peers[peerID]
	hm.mu.RUnlock()

	if !exists {
		return
	}

	// Perform ping check
	ctx, cancel := context.WithTimeout(hm.ctx, hm.config.HealthCheckTimeout)
	defer cancel()

	healthy := hm.pingPeer(ctx, peerID)
	latency := time.Since(start)

	// Update health status
	peerHealth.mu.Lock()
	peerHealth.LastCheck = time.Now()
	peerHealth.TotalChecks++

	if healthy {
		peerHealth.ConsecutiveFails = 0
		peerHealth.LastSuccess = time.Now()
		peerHealth.Healthy = true
		
		// Update average latency
		if peerHealth.AverageLatency == 0 {
			peerHealth.AverageLatency = latency
		} else {
			peerHealth.AverageLatency = (peerHealth.AverageLatency + latency) / 2
		}

		peerHealthGauge.WithLabelValues(peerID.String()).Set(1)
		healthCheckLatency.WithLabelValues(peerID.String()).Observe(latency.Seconds())
	} else {
		peerHealth.ConsecutiveFails++
		heartbeatMissesTotal.WithLabelValues(peerID.String()).Inc()

		if peerHealth.ConsecutiveFails >= hm.config.FailureThreshold {
			if peerHealth.Healthy {
				peerHealth.Healthy = false
				peerHealthGauge.WithLabelValues(peerID.String()).Set(0)
				peerFailuresTotal.WithLabelValues(peerID.String(), "heartbeat_timeout").Inc()

				hm.logger.Warn("peer marked unhealthy",
					zap.String("peer_id", peerID.String()),
					zap.Int("consecutive_fails", peerHealth.ConsecutiveFails),
				)
			}
		}
	}
	peerHealth.mu.Unlock()

	hm.logger.Debug("health check completed",
		zap.String("peer_id", peerID.String()),
		zap.Bool("healthy", healthy),
		zap.Duration("latency", latency),
	)
}

// pingPeer performs a simple connectivity check
func (hm *HealthMonitor) pingPeer(ctx context.Context, peerID peer.ID) bool {
	// Check if we have a connection
	conns := hm.host.Network().ConnsToPeer(peerID)
	if len(conns) == 0 {
		return false
	}

	// Check connection status
	for _, conn := range conns {
		if conn.Stat().Direction == network.DirOutbound || conn.Stat().Direction == network.DirInbound {
			// Connection exists and is active
			return true
		}
	}

	return false
}

// GetPeerHealth returns health status for a peer
func (hm *HealthMonitor) GetPeerHealth(peerID peer.ID) (*PeerHealth, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	peerHealth, exists := hm.peers[peerID]
	if !exists {
		return nil, false
	}

	// Return a copy
	peerHealth.mu.RLock()
	defer peerHealth.mu.RUnlock()

	return &PeerHealth{
		PeerID:           peerHealth.PeerID,
		Healthy:          peerHealth.Healthy,
		ConsecutiveFails: peerHealth.ConsecutiveFails,
		LastCheck:        peerHealth.LastCheck,
		LastSuccess:      peerHealth.LastSuccess,
		AverageLatency:   peerHealth.AverageLatency,
		TotalChecks:      peerHealth.TotalChecks,
	}, true
}

// GetHealthyPeers returns all healthy peers
func (hm *HealthMonitor) GetHealthyPeers() []peer.ID {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	healthy := make([]peer.ID, 0)
	for peerID, peerHealth := range hm.peers {
		peerHealth.mu.RLock()
		if peerHealth.Healthy {
			healthy = append(healthy, peerID)
		}
		peerHealth.mu.RUnlock()
	}

	return healthy
}

// GetUnhealthyPeers returns all unhealthy peers
func (hm *HealthMonitor) GetUnhealthyPeers() []peer.ID {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	unhealthy := make([]peer.ID, 0)
	for peerID, peerHealth := range hm.peers {
		peerHealth.mu.RLock()
		if !peerHealth.Healthy {
			unhealthy = append(unhealthy, peerID)
		}
		peerHealth.mu.RUnlock()
	}

	return unhealthy
}

// Stats returns health monitor statistics
func (hm *HealthMonitor) Stats() HealthStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	healthy := 0
	unhealthy := 0

	for _, peerHealth := range hm.peers {
		peerHealth.mu.RLock()
		if peerHealth.Healthy {
			healthy++
		} else {
			unhealthy++
		}
		peerHealth.mu.RUnlock()
	}

	return HealthStats{
		TotalMonitored: len(hm.peers),
		HealthyPeers:   healthy,
		UnhealthyPeers: unhealthy,
	}
}

// HealthStats represents health statistics
type HealthStats struct {
	TotalMonitored int
	HealthyPeers   int
	UnhealthyPeers int
}

// Close stops the health monitor
func (hm *HealthMonitor) Close() error {
	hm.cancel()
	hm.wg.Wait()

	hm.mu.Lock()
	defer hm.mu.Unlock()

	for peerID := range hm.peers {
		peerHealthGauge.WithLabelValues(peerID.String()).Set(0)
	}
	hm.peers = nil
	activePeerMonitors.Set(0)

	hm.logger.Info("health monitor closed")
	return nil
}
