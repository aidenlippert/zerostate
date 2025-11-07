// Package p2p provides connection pooling for libp2p streams
package p2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	pooledConnectionsGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "pooled_connections_gauge",
			Help: "Number of pooled connections per peer",
		},
		[]string{"peer_id"},
	)

	streamReuseTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stream_reuse_total",
			Help: "Total stream reuse events",
		},
		[]string{"peer_id", "protocol"},
	)

	streamCreationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "stream_creation_total",
			Help: "Total new stream creations",
		},
		[]string{"peer_id", "protocol", "result"}, // success, failure
	)

	idleConnectionsEvicted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "idle_connections_evicted_total",
			Help: "Total idle connections evicted",
		},
		[]string{"peer_id"},
	)

	poolAcquireLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pool_acquire_latency_seconds",
			Help:    "Time to acquire stream from pool",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"peer_id", "source"}, // pool, new
	)

	poolSizeGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "connection_pool_size",
			Help: "Total number of pooled connections",
		},
	)
)

const (
	// DefaultMaxIdleTime is the default idle timeout for connections
	DefaultMaxIdleTime = 5 * time.Minute
	// DefaultMaxStreamsPerConn is the default max streams per connection
	DefaultMaxStreamsPerConn = 10
	// DefaultCleanupInterval is the default cleanup interval
	DefaultCleanupInterval = 1 * time.Minute
)

// ConnectionPoolConfig holds connection pool configuration
type ConnectionPoolConfig struct {
	// MaxIdleTime is how long to keep idle connections
	MaxIdleTime time.Duration
	// MaxStreamsPerConn is max concurrent streams per connection
	MaxStreamsPerConn int
	// CleanupInterval is how often to clean up idle connections
	CleanupInterval time.Duration
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
}

// DefaultConnectionPoolConfig returns default configuration
func DefaultConnectionPoolConfig() *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		MaxIdleTime:       DefaultMaxIdleTime,
		MaxStreamsPerConn: DefaultMaxStreamsPerConn,
		CleanupInterval:   DefaultCleanupInterval,
		EnableMetrics:     true,
	}
}

// PooledConnection represents a pooled connection with streams
type PooledConnection struct {
	conn       network.Conn
	streams    []network.Stream
	lastUsed   time.Time
	inUse      int32
	maxStreams int
	mu         sync.RWMutex
}

// NewPooledConnection creates a new pooled connection
func NewPooledConnection(conn network.Conn, maxStreams int) *PooledConnection {
	return &PooledConnection{
		conn:       conn,
		streams:    make([]network.Stream, 0),
		lastUsed:   time.Now(),
		maxStreams: maxStreams,
	}
}

// AcquireStream gets an available stream or creates a new one
func (pc *PooledConnection) AcquireStream(ctx context.Context, pid protocol.ID) (network.Stream, bool, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Try to reuse an existing idle stream
	for i, stream := range pc.streams {
		if stream != nil {
			// Remove from pool
			pc.streams = append(pc.streams[:i], pc.streams[i+1:]...)
			pc.lastUsed = time.Now()
			return stream, true, nil // true = reused
		}
	}

	// Create new stream
	stream, err := pc.conn.NewStream(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create stream: %w", err)
	}

	pc.lastUsed = time.Now()
	return stream, false, nil // false = new stream
}

// ReleaseStream returns a stream to the pool
func (pc *PooledConnection) ReleaseStream(stream network.Stream) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Only pool if under limit
	if len(pc.streams) < pc.maxStreams {
		pc.streams = append(pc.streams, stream)
		pc.lastUsed = time.Now()
	} else {
		// Close excess stream
		_ = stream.Close()
	}
}

// IsIdle checks if connection has been idle
func (pc *PooledConnection) IsIdle(maxIdleTime time.Duration) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return time.Since(pc.lastUsed) > maxIdleTime
}

// Close closes all streams and the connection
func (pc *PooledConnection) Close() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	// Close all pooled streams
	for _, stream := range pc.streams {
		if stream != nil {
			_ = stream.Close()
		}
	}
	pc.streams = nil

	// Close connection
	return pc.conn.Close()
}

// StreamCount returns the number of pooled streams
func (pc *PooledConnection) StreamCount() int {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return len(pc.streams)
}

// ConnectionPool manages a pool of connections
type ConnectionPool struct {
	host   host.Host
	conns  map[peer.ID]*PooledConnection
	config *ConnectionPoolConfig
	logger *zap.Logger
	mu     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
}

// NewConnectionPool creates a new connection pool
func NewConnectionPool(ctx context.Context, h host.Host, config *ConnectionPoolConfig, logger *zap.Logger) *ConnectionPool {
	if config == nil {
		config = DefaultConnectionPoolConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	poolCtx, cancel := context.WithCancel(ctx)

	cp := &ConnectionPool{
		host:   h,
		conns:  make(map[peer.ID]*PooledConnection),
		config: config,
		logger: logger,
		ctx:    poolCtx,
		cancel: cancel,
	}

	// Start cleanup goroutine
	go cp.cleanupLoop()

	logger.Info("connection pool created",
		zap.Duration("max_idle_time", config.MaxIdleTime),
		zap.Int("max_streams_per_conn", config.MaxStreamsPerConn),
	)

	return cp
}

// GetStream acquires a stream to a peer
func (cp *ConnectionPool) GetStream(ctx context.Context, peerID peer.ID, pid protocol.ID) (network.Stream, error) {
	start := time.Now()

	cp.mu.Lock()
	pooledConn, exists := cp.conns[peerID]
	cp.mu.Unlock()

	var stream network.Stream
	var reused bool
	var err error

	if exists {
		// Try to get stream from existing connection
		stream, reused, err = pooledConn.AcquireStream(ctx, pid)
		if err == nil {
			if reused {
				streamReuseTotal.WithLabelValues(peerID.String(), string(pid)).Inc()
				poolAcquireLatency.WithLabelValues(peerID.String(), "pool").Observe(time.Since(start).Seconds())
			} else {
				streamCreationTotal.WithLabelValues(peerID.String(), string(pid), "success").Inc()
				poolAcquireLatency.WithLabelValues(peerID.String(), "new").Observe(time.Since(start).Seconds())
			}
			return stream, nil
		}
		// If stream creation failed, fall through to create new connection
	}

	// Create new connection
	stream, err = cp.createNewStream(ctx, peerID, pid)
	if err != nil {
		streamCreationTotal.WithLabelValues(peerID.String(), string(pid), "failure").Inc()
		return nil, err
	}

	streamCreationTotal.WithLabelValues(peerID.String(), string(pid), "success").Inc()
	poolAcquireLatency.WithLabelValues(peerID.String(), "new").Observe(time.Since(start).Seconds())

	cp.logger.Debug("created new stream",
		zap.String("peer_id", peerID.String()),
		zap.String("protocol", string(pid)),
		zap.Float64("latency_seconds", time.Since(start).Seconds()),
	)

	return stream, nil
}

// createNewStream creates a new stream and adds connection to pool
func (cp *ConnectionPool) createNewStream(ctx context.Context, peerID peer.ID, pid protocol.ID) (network.Stream, error) {
	// Get or create connection
	conn := cp.host.Network().ConnsToPeer(peerID)
	if len(conn) == 0 {
		// No existing connection, need to dial
		if err := cp.host.Connect(ctx, peer.AddrInfo{ID: peerID}); err != nil {
			return nil, fmt.Errorf("failed to connect to peer: %w", err)
		}
		conn = cp.host.Network().ConnsToPeer(peerID)
		if len(conn) == 0 {
			return nil, fmt.Errorf("no connection to peer after dial")
		}
	}

	// Create stream
	stream, err := cp.host.NewStream(ctx, peerID, pid)
	if err != nil {
		return nil, fmt.Errorf("failed to create stream: %w", err)
	}

	// Add to pool
	cp.mu.Lock()
	if _, exists := cp.conns[peerID]; !exists {
		cp.conns[peerID] = NewPooledConnection(conn[0], cp.config.MaxStreamsPerConn)
		pooledConnectionsGauge.WithLabelValues(peerID.String()).Set(1)
		poolSizeGauge.Set(float64(len(cp.conns)))
	}
	cp.mu.Unlock()

	return stream, nil
}

// ReleaseStream returns a stream to the pool for reuse
func (cp *ConnectionPool) ReleaseStream(peerID peer.ID, stream network.Stream) {
	cp.mu.RLock()
	pooledConn, exists := cp.conns[peerID]
	cp.mu.RUnlock()

	if exists {
		pooledConn.ReleaseStream(stream)
	} else {
		// No pool entry, just close
		_ = stream.Close()
	}
}

// RemovePeer removes a peer from the pool
func (cp *ConnectionPool) RemovePeer(peerID peer.ID) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if pooledConn, exists := cp.conns[peerID]; exists {
		_ = pooledConn.Close()
		delete(cp.conns, peerID)
		pooledConnectionsGauge.WithLabelValues(peerID.String()).Set(0)
		poolSizeGauge.Set(float64(len(cp.conns)))
	}
}

// cleanupLoop periodically removes idle connections
func (cp *ConnectionPool) cleanupLoop() {
	ticker := time.NewTicker(cp.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			cp.cleanup()
		case <-cp.ctx.Done():
			return
		}
	}
}

// cleanup removes idle connections
func (cp *ConnectionPool) cleanup() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	toRemove := make([]peer.ID, 0)

	for peerID, pooledConn := range cp.conns {
		if pooledConn.IsIdle(cp.config.MaxIdleTime) {
			toRemove = append(toRemove, peerID)
		}
	}

	for _, peerID := range toRemove {
		if pooledConn, exists := cp.conns[peerID]; exists {
			_ = pooledConn.Close()
			delete(cp.conns, peerID)
			pooledConnectionsGauge.WithLabelValues(peerID.String()).Set(0)
			idleConnectionsEvicted.WithLabelValues(peerID.String()).Inc()
		}
	}

	if len(toRemove) > 0 {
		poolSizeGauge.Set(float64(len(cp.conns)))
		cp.logger.Debug("cleaned up idle connections",
			zap.Int("removed", len(toRemove)),
			zap.Int("remaining", len(cp.conns)),
		)
	}
}

// Stats returns pool statistics
func (cp *ConnectionPool) Stats() PoolStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()

	totalStreams := 0
	for _, pooledConn := range cp.conns {
		totalStreams += pooledConn.StreamCount()
	}

	return PoolStats{
		TotalConnections: len(cp.conns),
		TotalStreams:     totalStreams,
	}
}

// PoolStats represents pool statistics
type PoolStats struct {
	TotalConnections int
	TotalStreams     int
}

// Close closes the connection pool
func (cp *ConnectionPool) Close() error {
	cp.cancel()

	cp.mu.Lock()
	defer cp.mu.Unlock()

	for peerID, pooledConn := range cp.conns {
		_ = pooledConn.Close()
		pooledConnectionsGauge.WithLabelValues(peerID.String()).Set(0)
	}
	cp.conns = nil
	poolSizeGauge.Set(0)

	cp.logger.Info("connection pool closed")
	return nil
}
