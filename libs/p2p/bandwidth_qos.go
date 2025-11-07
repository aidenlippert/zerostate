// Package p2p provides bandwidth accounting and QoS management
package p2p

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	// ErrQueueFull is returned when a priority queue is full
	ErrQueueFull = errors.New("priority queue is full")
	// ErrClosed is returned when operating on a closed queue
	ErrClosed = errors.New("priority queue is closed")
)

// Prometheus metrics
var (
	bandwidthBytesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bandwidth_bytes_total",
			Help: "Total bandwidth usage in bytes",
		},
		[]string{"peer_id", "direction"},
	)

	qosDropsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "qos_drops_total",
			Help: "Total message drops due to QoS",
		},
		[]string{"priority", "reason"},
	)

	qosQueueDepth = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "qos_queue_depth",
			Help: "Current QoS queue depth per priority",
		},
		[]string{"priority"},
	)

	bandwidthThrottleEvents = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "bandwidth_throttle_events_total",
			Help: "Total bandwidth throttle events",
		},
		[]string{"peer_id"},
	)

	peerBandwidthRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "peer_bandwidth_rate_bytes_per_sec",
			Help: "Current bandwidth rate per peer in bytes/sec",
		},
		[]string{"peer_id", "direction"},
	)
)

const (
	// DefaultBandwidthLimit is the default per-peer bandwidth limit (1 MB/s)
	DefaultBandwidthLimit = 1024 * 1024
	// DefaultBurstSize is the default token bucket burst size (10 MB)
	DefaultBurstSize = 10 * 1024 * 1024
	// DefaultRefillInterval is the default token refill interval
	DefaultRefillInterval = 100 * time.Millisecond
	// DefaultMaxQueueSize is the default maximum QoS queue size
	DefaultMaxQueueSize = 1000
)

// Priority represents message priority levels
type Priority int

const (
	// PriorityLow is low priority (background tasks)
	PriorityLow Priority = iota
	// PriorityNormal is normal priority (regular operations)
	PriorityNormal
	// PriorityHigh is high priority (critical messages)
	PriorityHigh
)

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	default:
		return "unknown"
	}
}

// BandwidthQoSConfig holds bandwidth and QoS configuration
type BandwidthQoSConfig struct {
	// BandwidthLimit is the per-peer bandwidth limit in bytes/sec
	BandwidthLimit int64
	// BurstSize is the token bucket burst size
	BurstSize int64
	// RefillInterval is how often to refill tokens
	RefillInterval time.Duration
	// MaxQueueSize is the maximum queue size per priority
	MaxQueueSize int
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
}

// DefaultBandwidthQoSConfig returns default configuration
func DefaultBandwidthQoSConfig() *BandwidthQoSConfig {
	return &BandwidthQoSConfig{
		BandwidthLimit: DefaultBandwidthLimit,
		BurstSize:      DefaultBurstSize,
		RefillInterval: DefaultRefillInterval,
		MaxQueueSize:   DefaultMaxQueueSize,
		EnableMetrics:  true,
	}
}

// PeerBandwidth tracks bandwidth usage for a peer
type PeerBandwidth struct {
	peerID         peer.ID
	tokens         int64
	lastRefill     time.Time
	uploadBytes    int64
	downloadBytes  int64
	uploadRate     float64
	downloadRate   float64
	lastRateUpdate time.Time
	mu             sync.Mutex
}

// QueuedMessage represents a queued message with priority
type QueuedMessage struct {
	Data      []byte
	Priority  Priority
	Timestamp time.Time
	ResultCh  chan error
}

// PriorityQueue manages message queues by priority
type PriorityQueue struct {
	queues    [3]chan *QueuedMessage // High, Normal, Low
	mu        sync.RWMutex
	maxSize   int
	closed    bool
	logger    *zap.Logger
}

// NewPriorityQueue creates a new priority queue
func NewPriorityQueue(maxSize int, logger *zap.Logger) *PriorityQueue {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &PriorityQueue{
		queues: [3]chan *QueuedMessage{
			make(chan *QueuedMessage, maxSize), // High
			make(chan *QueuedMessage, maxSize), // Normal
			make(chan *QueuedMessage, maxSize), // Low
		},
		maxSize: maxSize,
		logger:  logger,
	}
}

// Enqueue adds a message to the appropriate priority queue
func (pq *PriorityQueue) Enqueue(msg *QueuedMessage) error {
	pq.mu.RLock()
	if pq.closed {
		pq.mu.RUnlock()
		return ErrClosed
	}
	pq.mu.RUnlock()

	queueIdx := int(msg.Priority)
	select {
	case pq.queues[queueIdx] <- msg:
		qosQueueDepth.WithLabelValues(msg.Priority.String()).Inc()
		return nil
	default:
		qosDropsTotal.WithLabelValues(msg.Priority.String(), "queue_full").Inc()
		return ErrQueueFull
	}
}

// Dequeue returns the next message with priority ordering (High > Normal > Low)
func (pq *PriorityQueue) Dequeue() (*QueuedMessage, error) {
	pq.mu.RLock()
	closed := pq.closed
	pq.mu.RUnlock()

	if closed {
		return nil, ErrClosed
	}

	// Try high priority first
	select {
	case msg := <-pq.queues[PriorityHigh]:
		qosQueueDepth.WithLabelValues(PriorityHigh.String()).Dec()
		return msg, nil
	default:
	}

	// Then normal priority
	select {
	case msg := <-pq.queues[PriorityNormal]:
		qosQueueDepth.WithLabelValues(PriorityNormal.String()).Dec()
		return msg, nil
	default:
	}

	// Finally low priority (blocking)
	msg := <-pq.queues[PriorityLow]
	qosQueueDepth.WithLabelValues(PriorityLow.String()).Dec()
	return msg, nil
}

// Close closes all queues
func (pq *PriorityQueue) Close() error {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if pq.closed {
		return nil
	}

	pq.closed = true
	for i := range pq.queues {
		close(pq.queues[i])
	}

	return nil
}

// Depth returns the current queue depth per priority
func (pq *PriorityQueue) Depth() map[Priority]int {
	return map[Priority]int{
		PriorityHigh:   len(pq.queues[PriorityHigh]),
		PriorityNormal: len(pq.queues[PriorityNormal]),
		PriorityLow:    len(pq.queues[PriorityLow]),
	}
}

// BandwidthQoS manages bandwidth accounting and QoS
type BandwidthQoS struct {
	peers    map[peer.ID]*PeerBandwidth
	queue    *PriorityQueue
	config   *BandwidthQoSConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewBandwidthQoS creates a new bandwidth and QoS manager
func NewBandwidthQoS(ctx context.Context, config *BandwidthQoSConfig, logger *zap.Logger) *BandwidthQoS {
	if config == nil {
		config = DefaultBandwidthQoSConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	bwCtx, cancel := context.WithCancel(ctx)

	bq := &BandwidthQoS{
		peers:  make(map[peer.ID]*PeerBandwidth),
		queue:  NewPriorityQueue(config.MaxQueueSize, logger),
		config: config,
		logger: logger,
		ctx:    bwCtx,
		cancel: cancel,
	}

	// Start token refill loop
	bq.wg.Add(1)
	go bq.refillLoop()

	// Start rate calculation loop
	bq.wg.Add(1)
	go bq.rateUpdateLoop()

	logger.Info("bandwidth QoS manager started",
		zap.Int64("bandwidth_limit_bps", config.BandwidthLimit),
		zap.Int64("burst_size", config.BurstSize),
		zap.Duration("refill_interval", config.RefillInterval),
		zap.Int("max_queue_size", config.MaxQueueSize),
	)

	return bq
}

// RecordUpload records upload bandwidth for a peer
func (bq *BandwidthQoS) RecordUpload(peerID peer.ID, bytes int64) {
	bq.mu.Lock()
	defer bq.mu.Unlock()

	peer, exists := bq.peers[peerID]
	if !exists {
		peer = &PeerBandwidth{
			peerID:         peerID,
			tokens:         bq.config.BurstSize,
			lastRefill:     time.Now(),
			lastRateUpdate: time.Now(),
		}
		bq.peers[peerID] = peer
	}

	peer.mu.Lock()
	peer.uploadBytes += bytes
	peer.mu.Unlock()

	bandwidthBytesTotal.WithLabelValues(peerID.String(), "upload").Add(float64(bytes))
}

// RecordDownload records download bandwidth for a peer
func (bq *BandwidthQoS) RecordDownload(peerID peer.ID, bytes int64) {
	bq.mu.Lock()
	defer bq.mu.Unlock()

	peer, exists := bq.peers[peerID]
	if !exists {
		peer = &PeerBandwidth{
			peerID:         peerID,
			tokens:         bq.config.BurstSize,
			lastRefill:     time.Now(),
			lastRateUpdate: time.Now(),
		}
		bq.peers[peerID] = peer
	}

	peer.mu.Lock()
	peer.downloadBytes += bytes
	peer.mu.Unlock()

	bandwidthBytesTotal.WithLabelValues(peerID.String(), "download").Add(float64(bytes))
}

// CheckBandwidth checks if a peer has available bandwidth
func (bq *BandwidthQoS) CheckBandwidth(peerID peer.ID, bytes int64) bool {
	bq.mu.RLock()
	peer, exists := bq.peers[peerID]
	bq.mu.RUnlock()

	if !exists {
		return true // Allow if not tracked
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()

	if peer.tokens >= bytes {
		peer.tokens -= bytes
		return true
	}

	bandwidthThrottleEvents.WithLabelValues(peerID.String()).Inc()
	return false
}

// refillLoop periodically refills token buckets
func (bq *BandwidthQoS) refillLoop() {
	defer bq.wg.Done()

	ticker := time.NewTicker(bq.config.RefillInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bq.ctx.Done():
			return
		case <-ticker.C:
			bq.refillTokens()
		}
	}
}

// refillTokens refills token buckets for all peers
func (bq *BandwidthQoS) refillTokens() {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	refillAmount := bq.config.BandwidthLimit * int64(bq.config.RefillInterval) / int64(time.Second)

	for _, peer := range bq.peers {
		peer.mu.Lock()
		peer.tokens += refillAmount
		if peer.tokens > bq.config.BurstSize {
			peer.tokens = bq.config.BurstSize
		}
		peer.mu.Unlock()
	}
}

// rateUpdateLoop periodically updates bandwidth rates
func (bq *BandwidthQoS) rateUpdateLoop() {
	defer bq.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-bq.ctx.Done():
			return
		case <-ticker.C:
			bq.updateRates()
		}
	}
}

// updateRates calculates current bandwidth rates
func (bq *BandwidthQoS) updateRates() {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	now := time.Now()

	for _, peer := range bq.peers {
		peer.mu.Lock()

		elapsed := now.Sub(peer.lastRateUpdate).Seconds()
		if elapsed > 0 {
			peer.uploadRate = float64(peer.uploadBytes) / elapsed
			peer.downloadRate = float64(peer.downloadBytes) / elapsed

			peerBandwidthRate.WithLabelValues(peer.peerID.String(), "upload").Set(peer.uploadRate)
			peerBandwidthRate.WithLabelValues(peer.peerID.String(), "download").Set(peer.downloadRate)

			peer.uploadBytes = 0
			peer.downloadBytes = 0
			peer.lastRateUpdate = now
		}

		peer.mu.Unlock()
	}
}

// GetPeerBandwidth returns bandwidth stats for a peer
func (bq *BandwidthQoS) GetPeerBandwidth(peerID peer.ID) *PeerBandwidth {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	peer, exists := bq.peers[peerID]
	if !exists {
		return nil
	}

	// Return a copy
	peer.mu.Lock()
	defer peer.mu.Unlock()

	return &PeerBandwidth{
		peerID:         peer.peerID,
		tokens:         peer.tokens,
		lastRefill:     peer.lastRefill,
		uploadBytes:    peer.uploadBytes,
		downloadBytes:  peer.downloadBytes,
		uploadRate:     peer.uploadRate,
		downloadRate:   peer.downloadRate,
		lastRateUpdate: peer.lastRateUpdate,
	}
}

// Queue returns the priority queue
func (bq *BandwidthQoS) Queue() *PriorityQueue {
	return bq.queue
}

// Stats returns bandwidth and QoS statistics
type BandwidthQoSStats struct {
	TrackedPeers int
	QueueDepth   map[Priority]int
}

// Stats returns current statistics
func (bq *BandwidthQoS) Stats() BandwidthQoSStats {
	bq.mu.RLock()
	defer bq.mu.RUnlock()

	return BandwidthQoSStats{
		TrackedPeers: len(bq.peers),
		QueueDepth:   bq.queue.Depth(),
	}
}

// Close stops the bandwidth QoS manager
func (bq *BandwidthQoS) Close() error {
	bq.logger.Info("closing bandwidth QoS manager")

	bq.cancel()
	bq.wg.Wait()

	if err := bq.queue.Close(); err != nil {
		return err
	}

	bq.mu.Lock()
	bq.peers = make(map[peer.ID]*PeerBandwidth)
	bq.mu.Unlock()

	return nil
}
