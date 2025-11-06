package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	flowControlThrottles = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_flow_control_throttles_total",
			Help: "Total number of times flow control throttled a send",
		},
		[]string{"peer_id", "reason"}, // token_wait, window_full, peer_limit
	)

	flowControlWindowSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_flow_control_window_size",
			Help: "Current send window size for each peer",
		},
		[]string{"peer_id"},
	)

	flowControlTokensAvailable = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_flow_control_tokens_available",
			Help: "Available tokens in the rate limiter bucket",
		},
		[]string{"peer_id"},
	)

	flowControlBytesThrottled = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_flow_control_bytes_throttled_total",
			Help: "Total bytes delayed by flow control",
		},
		[]string{"peer_id"},
	)
)

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	mu           sync.Mutex
	capacity     int           // Maximum tokens
	tokens       int           // Current tokens
	refillRate   int           // Tokens per second
	lastRefill   time.Time     // Last refill time
	refillTicker *time.Ticker  // Periodic refill
	stopChan     chan struct{} // Stop signal
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	tb := &TokenBucket{
		capacity:     capacity,
		tokens:       capacity,
		refillRate:   refillRate,
		lastRefill:   time.Now(),
		refillTicker: time.NewTicker(100 * time.Millisecond),
		stopChan:     make(chan struct{}),
	}

	// Start refill goroutine
	go tb.refillLoop()

	return tb
}

// Take attempts to take n tokens, blocking until available or context cancelled
func (tb *TokenBucket) Take(ctx context.Context, n int) error {
	for {
		tb.mu.Lock()
		if tb.tokens >= n {
			tb.tokens -= n
			tb.mu.Unlock()
			return nil
		}
		tb.mu.Unlock()

		// Wait for refill or context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
			// Try again after short delay
		}
	}
}

// TryTake attempts to take n tokens without blocking
func (tb *TokenBucket) TryTake(n int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// Available returns the number of available tokens
func (tb *TokenBucket) Available() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.tokens
}

// refillLoop periodically refills tokens
func (tb *TokenBucket) refillLoop() {
	for {
		select {
		case <-tb.refillTicker.C:
			tb.refill()
		case <-tb.stopChan:
			return
		}
	}
}

// refill adds tokens based on elapsed time
func (tb *TokenBucket) refill() {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tokensToAdd := int(elapsed * float64(tb.refillRate))

	if tokensToAdd > 0 {
		tb.tokens += tokensToAdd
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		tb.lastRefill = now
	}
}

// Stop stops the token bucket
func (tb *TokenBucket) Stop() {
	close(tb.stopChan)
	tb.refillTicker.Stop()
}

// SendWindow implements a sliding window flow control
type SendWindow struct {
	mu          sync.RWMutex
	windowSize  int       // Maximum in-flight messages
	inFlight    int       // Current in-flight count
	pendingAcks map[uint64]time.Time // Message ID -> send time
	nextID      uint64
}

// NewSendWindow creates a new send window
func NewSendWindow(windowSize int) *SendWindow {
	return &SendWindow{
		windowSize:  windowSize,
		inFlight:    0,
		pendingAcks: make(map[uint64]time.Time),
		nextID:      1,
	}
}

// CanSend checks if the window has space
func (sw *SendWindow) CanSend() bool {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.inFlight < sw.windowSize
}

// Send marks a message as sent, returns message ID
func (sw *SendWindow) Send() (uint64, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if sw.inFlight >= sw.windowSize {
		return 0, fmt.Errorf("send window full: %d/%d", sw.inFlight, sw.windowSize)
	}

	msgID := sw.nextID
	sw.nextID++
	sw.inFlight++
	sw.pendingAcks[msgID] = time.Now()

	return msgID, nil
}

// Ack acknowledges a message, freeing window space
func (sw *SendWindow) Ack(msgID uint64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if _, exists := sw.pendingAcks[msgID]; exists {
		delete(sw.pendingAcks, msgID)
		sw.inFlight--
	}
}

// InFlight returns the current in-flight count
func (sw *SendWindow) InFlight() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.inFlight
}

// WindowSize returns the maximum window size
func (sw *SendWindow) WindowSize() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.windowSize
}

// AdjustWindow dynamically adjusts the window size
func (sw *SendWindow) AdjustWindow(newSize int) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	
	if newSize > 0 {
		sw.windowSize = newSize
	}
}

// FlowController manages per-peer flow control
type FlowController struct {
	mu            sync.RWMutex
	peerLimiters  map[peer.ID]*TokenBucket
	peerWindows   map[peer.ID]*SendWindow
	globalLimiter *TokenBucket
	config        *FlowControlConfig
	logger        *zap.Logger
}

// FlowControlConfig holds flow control configuration
type FlowControlConfig struct {
	// Global rate limit (bytes per second)
	GlobalRateLimit int
	// Per-peer rate limit (bytes per second)
	PerPeerRateLimit int
	// Token bucket capacity (bytes)
	BucketCapacity int
	// Send window size (messages)
	WindowSize int
	// Maximum peers to track
	MaxTrackedPeers int
}

// DefaultFlowControlConfig returns default configuration
func DefaultFlowControlConfig() *FlowControlConfig {
	return &FlowControlConfig{
		GlobalRateLimit:  10 * 1024 * 1024, // 10 MB/s
		PerPeerRateLimit: 1 * 1024 * 1024,  // 1 MB/s per peer
		BucketCapacity:   5 * 1024 * 1024,  // 5 MB bucket
		WindowSize:       256,              // 256 messages in flight
		MaxTrackedPeers:  1000,             // Track up to 1000 peers
	}
}

// NewFlowController creates a new flow controller
func NewFlowController(config *FlowControlConfig, logger *zap.Logger) *FlowController {
	if config == nil {
		config = DefaultFlowControlConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &FlowController{
		peerLimiters:  make(map[peer.ID]*TokenBucket),
		peerWindows:   make(map[peer.ID]*SendWindow),
		globalLimiter: NewTokenBucket(config.BucketCapacity, config.GlobalRateLimit),
		config:        config,
		logger:        logger,
	}
}

// AllowSend checks if sending n bytes to peer is allowed
func (fc *FlowController) AllowSend(ctx context.Context, peerID peer.ID, nBytes int) error {
	// Check global rate limit
	if err := fc.globalLimiter.Take(ctx, nBytes); err != nil {
		flowControlThrottles.WithLabelValues(peerID.String(), "global_limit").Inc()
		flowControlBytesThrottled.WithLabelValues(peerID.String()).Add(float64(nBytes))
		return fmt.Errorf("global rate limit exceeded: %w", err)
	}

	// Get or create per-peer limiter
	fc.mu.Lock()
	limiter, exists := fc.peerLimiters[peerID]
	if !exists {
		limiter = NewTokenBucket(fc.config.BucketCapacity, fc.config.PerPeerRateLimit)
		fc.peerLimiters[peerID] = limiter
	}
	fc.mu.Unlock()

	// Check per-peer rate limit
	if err := limiter.Take(ctx, nBytes); err != nil {
		flowControlThrottles.WithLabelValues(peerID.String(), "peer_limit").Inc()
		flowControlBytesThrottled.WithLabelValues(peerID.String()).Add(float64(nBytes))
		return fmt.Errorf("peer rate limit exceeded: %w", err)
	}

	// Update metrics
	flowControlTokensAvailable.WithLabelValues(peerID.String()).Set(float64(limiter.Available()))

	return nil
}

// AcquireWindow acquires a send window slot for a peer
func (fc *FlowController) AcquireWindow(peerID peer.ID) (uint64, error) {
	fc.mu.Lock()
	window, exists := fc.peerWindows[peerID]
	if !exists {
		window = NewSendWindow(fc.config.WindowSize)
		fc.peerWindows[peerID] = window
	}
	fc.mu.Unlock()

	msgID, err := window.Send()
	if err != nil {
		flowControlThrottles.WithLabelValues(peerID.String(), "window_full").Inc()
		return 0, err
	}

	// Update metrics
	flowControlWindowSize.WithLabelValues(peerID.String()).Set(float64(window.InFlight()))

	return msgID, nil
}

// ReleaseWindow releases a send window slot
func (fc *FlowController) ReleaseWindow(peerID peer.ID, msgID uint64) {
	fc.mu.RLock()
	window, exists := fc.peerWindows[peerID]
	fc.mu.RUnlock()

	if exists {
		window.Ack(msgID)
		flowControlWindowSize.WithLabelValues(peerID.String()).Set(float64(window.InFlight()))
	}
}

// GetPeerWindow returns the send window for a peer
func (fc *FlowController) GetPeerWindow(peerID peer.ID) *SendWindow {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	return fc.peerWindows[peerID]
}

// RemovePeer removes flow control state for a peer
func (fc *FlowController) RemovePeer(peerID peer.ID) {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	if limiter, exists := fc.peerLimiters[peerID]; exists {
		limiter.Stop()
		delete(fc.peerLimiters, peerID)
	}
	delete(fc.peerWindows, peerID)

	fc.logger.Debug("removed flow control state for peer", zap.String("peer_id", peerID.String()))
}

// Close stops all token buckets
func (fc *FlowController) Close() {
	fc.mu.Lock()
	defer fc.mu.Unlock()

	fc.globalLimiter.Stop()
	for _, limiter := range fc.peerLimiters {
		limiter.Stop()
	}
}
