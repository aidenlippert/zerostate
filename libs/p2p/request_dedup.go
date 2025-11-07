// Package p2p provides request deduplication and caching
package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Prometheus metrics
var (
	deduplicatedRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "deduplicated_requests_total",
			Help: "Total deduplicated requests",
		},
		[]string{"key"},
	)

	cacheHitRate = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cache_hit_rate",
			Help: "Cache hit rate per key type",
		},
		[]string{"key_type"},
	)

	cacheEntriesGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cache_entries",
			Help: "Number of cached entries",
		},
	)

	cacheEvictionsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cache_evictions_total",
			Help: "Total cache evictions due to TTL expiry",
		},
	)

	inflightRequestsGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "inflight_requests",
			Help: "Number of in-flight requests",
		},
	)
)

const (
	// DefaultCacheTTL is the default cache entry TTL
	DefaultCacheTTL = 5 * time.Minute
	// DefaultDedupCleanupInterval is the default cache cleanup interval
	DefaultDedupCleanupInterval = 1 * time.Minute
	// DefaultMaxCacheSize is the default maximum cache size
	DefaultMaxCacheSize = 1000
)

// RequestDeduplicatorConfig holds deduplicator configuration
type RequestDeduplicatorConfig struct {
	// CacheTTL is how long to cache results
	CacheTTL time.Duration
	// CleanupInterval is how often to clean expired entries
	CleanupInterval time.Duration
	// MaxCacheSize is maximum number of cached entries
	MaxCacheSize int
	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool
}

// DefaultRequestDeduplicatorConfig returns default configuration
func DefaultRequestDeduplicatorConfig() *RequestDeduplicatorConfig {
	return &RequestDeduplicatorConfig{
		CacheTTL:        DefaultCacheTTL,
		CleanupInterval: DefaultDedupCleanupInterval,
		MaxCacheSize:    DefaultMaxCacheSize,
		EnableMetrics:   true,
	}
}

// CacheEntry represents a cached result
type CacheEntry struct {
	Value     interface{}
	ExpiresAt time.Time
	Hits      int
	mu        sync.RWMutex
}

// inflightRequest tracks an in-flight request
type inflightRequest struct {
	result     *RequestResult
	resultChan chan RequestResult
	waiting    int
	mu         sync.Mutex
}

// RequestResult holds the result of a request
type RequestResult struct {
	Data  interface{}
	Error error
}

// RequestDeduplicator deduplicates requests and caches results
type RequestDeduplicator struct {
	cache    map[string]*CacheEntry
	inflight map[string]*inflightRequest
	config   *RequestDeduplicatorConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewRequestDeduplicator creates a new request deduplicator
func NewRequestDeduplicator(ctx context.Context, config *RequestDeduplicatorConfig, logger *zap.Logger) *RequestDeduplicator {
	if config == nil {
		config = DefaultRequestDeduplicatorConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	dedupCtx, cancel := context.WithCancel(ctx)

	rd := &RequestDeduplicator{
		cache:    make(map[string]*CacheEntry),
		inflight: make(map[string]*inflightRequest),
		config:   config,
		logger:   logger,
		ctx:      dedupCtx,
		cancel:   cancel,
	}

	// Start cleanup goroutine
	go rd.cleanupLoop()

	logger.Info("request deduplicator created",
		zap.Duration("cache_ttl", config.CacheTTL),
		zap.Int("max_cache_size", config.MaxCacheSize),
	)

	return rd
}

// Do executes a function with deduplication
// Multiple concurrent calls with the same key will only execute once
func (rd *RequestDeduplicator) Do(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	// Check cache first
	if entry := rd.getCached(key); entry != nil {
		rd.logger.Debug("cache hit",
			zap.String("key", key),
			zap.Int("hits", entry.Hits),
		)
		return entry.Value, nil
	}

	rd.mu.Lock()

	// Check if request is in-flight
	if req, exists := rd.inflight[key]; exists {
		req.mu.Lock()
		req.waiting++
		waitCount := req.waiting
		req.mu.Unlock()
		rd.mu.Unlock()

		deduplicatedRequestsTotal.WithLabelValues(key).Inc()
		inflightRequestsGauge.Set(float64(len(rd.inflight)))

		rd.logger.Debug("joining in-flight request",
			zap.String("key", key),
			zap.Int("waiting", waitCount),
		)

		// Wait for result - read from closed channel works for broadcast
		result := <-req.resultChan
		
		// Result was stored before channel close
		req.mu.Lock()
		storedResult := req.result
		req.mu.Unlock()
		
		if storedResult != nil {
			return storedResult.Data, storedResult.Error
		}
		return result.Data, result.Error
	}

	// Create new in-flight request
	req := &inflightRequest{
		resultChan: make(chan RequestResult),
		waiting:    0,
		result:     nil,
	}
	rd.inflight[key] = req
	inflightRequestsGauge.Set(float64(len(rd.inflight)))
	rd.mu.Unlock()

	// Execute function
	data, err := fn()

	// Cache result if successful
	if err == nil {
		rd.setCached(key, data)
	}

	// Store and broadcast result to all waiters
	result := RequestResult{Data: data, Error: err}
	
	req.mu.Lock()
	req.result = &result
	waitCount := req.waiting
	req.mu.Unlock()
	
	// Broadcast by storing result and closing channel
	// All readers will get zero value but can check req.result
	close(req.resultChan)

	// Clean up in-flight
	rd.mu.Lock()
	delete(rd.inflight, key)
	inflightRequestsGauge.Set(float64(len(rd.inflight)))
	rd.mu.Unlock()

	rd.logger.Debug("request completed",
		zap.String("key", key),
		zap.Int("waiters", waitCount),
		zap.Bool("cached", err == nil),
	)

	return data, err
}

// getCached retrieves a cached entry if valid
func (rd *RequestDeduplicator) getCached(key string) *CacheEntry {
	rd.mu.RLock()
	entry, exists := rd.cache[key]
	rd.mu.RUnlock()

	if !exists {
		return nil
	}

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	if time.Now().After(entry.ExpiresAt) {
		// Expired
		rd.mu.Lock()
		delete(rd.cache, key)
		cacheEntriesGauge.Set(float64(len(rd.cache)))
		rd.mu.Unlock()
		return nil
	}

	entry.mu.RUnlock()
	entry.mu.Lock()
	entry.Hits++
	entry.mu.Unlock()
	entry.mu.RLock()

	return entry
}

// setCached stores a value in the cache
func (rd *RequestDeduplicator) setCached(key string, value interface{}) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	// Evict oldest if at max size
	if len(rd.cache) >= rd.config.MaxCacheSize {
		rd.evictOldest()
	}

	entry := &CacheEntry{
		Value:     value,
		ExpiresAt: time.Now().Add(rd.config.CacheTTL),
		Hits:      0,
	}

	rd.cache[key] = entry
	cacheEntriesGauge.Set(float64(len(rd.cache)))
}

// evictOldest removes the oldest cache entry
func (rd *RequestDeduplicator) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range rd.cache {
		entry.mu.RLock()
		if oldestKey == "" || entry.ExpiresAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.ExpiresAt
		}
		entry.mu.RUnlock()
	}

	if oldestKey != "" {
		delete(rd.cache, oldestKey)
		cacheEvictionsTotal.Inc()
	}
}

// Invalidate removes a cache entry
func (rd *RequestDeduplicator) Invalidate(key string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	delete(rd.cache, key)
	cacheEntriesGauge.Set(float64(len(rd.cache)))

	rd.logger.Debug("cache entry invalidated", zap.String("key", key))
}

// InvalidatePattern removes all cache entries matching a pattern
func (rd *RequestDeduplicator) InvalidatePattern(pattern string) int {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	removed := 0
	for key := range rd.cache {
		// Simple prefix matching
		if len(key) >= len(pattern) && key[:len(pattern)] == pattern {
			delete(rd.cache, key)
			removed++
		}
	}

	cacheEntriesGauge.Set(float64(len(rd.cache)))

	if removed > 0 {
		rd.logger.Debug("cache entries invalidated by pattern",
			zap.String("pattern", pattern),
			zap.Int("removed", removed),
		)
	}

	return removed
}

// cleanupLoop periodically removes expired entries
func (rd *RequestDeduplicator) cleanupLoop() {
	ticker := time.NewTicker(rd.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rd.cleanup()
		case <-rd.ctx.Done():
			return
		}
	}
}

// cleanup removes expired cache entries
func (rd *RequestDeduplicator) cleanup() {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, entry := range rd.cache {
		entry.mu.RLock()
		expired := now.After(entry.ExpiresAt)
		entry.mu.RUnlock()

		if expired {
			delete(rd.cache, key)
			cacheEvictionsTotal.Inc()
			removed++
		}
	}

	if removed > 0 {
		cacheEntriesGauge.Set(float64(len(rd.cache)))
		rd.logger.Debug("cleaned up expired cache entries",
			zap.Int("removed", removed),
			zap.Int("remaining", len(rd.cache)),
		)
	}
}

// Stats returns deduplicator statistics
func (rd *RequestDeduplicator) Stats() DeduplicationStats {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	totalHits := 0
	for _, entry := range rd.cache {
		entry.mu.RLock()
		totalHits += entry.Hits
		entry.mu.RUnlock()
	}

	return DeduplicationStats{
		CachedEntries:    len(rd.cache),
		InflightRequests: len(rd.inflight),
		TotalCacheHits:   totalHits,
	}
}

// DeduplicationStats represents deduplication statistics
type DeduplicationStats struct {
	CachedEntries    int
	InflightRequests int
	TotalCacheHits   int
}

// Close stops the deduplicator
func (rd *RequestDeduplicator) Close() error {
	rd.cancel()

	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.cache = nil
	rd.inflight = nil
	cacheEntriesGauge.Set(0)
	inflightRequestsGauge.Set(0)

	rd.logger.Info("request deduplicator closed")
	return nil
}
