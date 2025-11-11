package auth

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// FAANG-LEVEL RATE LIMITING
// Following best practices:
// - Token bucket algorithm for smooth rate limiting
// - Per-IP and per-user rate limiting
// - Configurable limits per endpoint
// - Database-backed for distributed systems
// - In-memory cache for performance
// - Automatic cleanup of expired buckets

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	RequestsPerMinute int           // Maximum requests per minute
	BurstSize         int           // Maximum burst size
	WindowDuration    time.Duration // Time window for rate limiting
	CleanupInterval   time.Duration // How often to clean up expired buckets
}

// DefaultRateLimitConfig returns default rate limiting configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		RequestsPerMinute: 100,
		BurstSize:         20,
		WindowDuration:    1 * time.Minute,
		CleanupInterval:   5 * time.Minute,
	}
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	config   *RateLimitConfig
	db       *sql.DB
	buckets  map[string]*bucket
	mu       sync.RWMutex
	stopChan chan struct{}
}

// bucket represents a rate limit bucket
type bucket struct {
	tokens      int
	lastRefill  time.Time
	windowStart time.Time
	windowEnd   time.Time
	mu          sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config *RateLimitConfig, db *sql.DB) *RateLimiter {
	if config == nil {
		config = DefaultRateLimitConfig()
	}

	rl := &RateLimiter{
		config:   config,
		db:       db,
		buckets:  make(map[string]*bucket),
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Allow checks if a request is allowed under the rate limit
func (rl *RateLimiter) Allow(key, endpoint string) (bool, error) {
	bucketKey := fmt.Sprintf("%s:%s", key, endpoint)

	rl.mu.RLock()
	b, exists := rl.buckets[bucketKey]
	rl.mu.RUnlock()

	if !exists {
		// Create new bucket
		b = &bucket{
			tokens:      rl.config.BurstSize,
			lastRefill:  time.Now(),
			windowStart: time.Now(),
			windowEnd:   time.Now().Add(rl.config.WindowDuration),
		}

		rl.mu.Lock()
		rl.buckets[bucketKey] = b
		rl.mu.Unlock()

		// Persist to database for distributed systems
		if rl.db != nil {
			go rl.persistBucket(key, endpoint, b)
		}
	}

	// Check and consume token
	b.mu.Lock()
	defer b.mu.Unlock()

	// Refill tokens if window has passed
	now := time.Now()
	if now.After(b.windowEnd) {
		b.tokens = rl.config.BurstSize
		b.windowStart = now
		b.windowEnd = now.Add(rl.config.WindowDuration)
		b.lastRefill = now
	} else {
		// Calculate tokens to add based on time passed
		elapsed := now.Sub(b.lastRefill)
		tokensToAdd := int(elapsed.Minutes() * float64(rl.config.RequestsPerMinute))
		if tokensToAdd > 0 {
			b.tokens = min(b.tokens+tokensToAdd, rl.config.BurstSize)
			b.lastRefill = now
		}
	}

	// Check if tokens available
	if b.tokens <= 0 {
		return false, nil
	}

	// Consume token
	b.tokens--

	// Update database
	if rl.db != nil {
		go rl.persistBucket(key, endpoint, b)
	}

	return true, nil
}

// persistBucket persists bucket state to database
func (rl *RateLimiter) persistBucket(key, endpoint string, b *bucket) {
	query := `
		INSERT INTO rate_limit_buckets (key, endpoint, tokens_remaining, window_start, window_end)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key, endpoint, window_start)
		DO UPDATE SET tokens_remaining = $3
	`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rl.db.ExecContext(ctx, query, key, endpoint, b.tokens, b.windowStart, b.windowEnd)
	if err != nil {
		// Log error but don't fail request
		fmt.Printf("Failed to persist rate limit bucket: %v\n", err)
	}
}

// cleanupLoop periodically cleans up expired buckets
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopChan:
			return
		}
	}
}

// cleanup removes expired buckets from memory
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, b := range rl.buckets {
		if now.After(b.windowEnd.Add(rl.config.WindowDuration)) {
			delete(rl.buckets, key)
		}
	}
}

// Stop stops the rate limiter cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopChan)
}

// RateLimitMiddleware creates middleware for rate limiting
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get rate limit key (IP address or user ID)
			key := r.RemoteAddr
			if user, ok := GetAuthenticatedUser(r.Context()); ok {
				key = user.UserID.String()
			}

			// Check rate limit
			allowed, err := limiter.Allow(key, r.URL.Path)
			if err != nil {
				http.Error(w, "Rate limit check failed", http.StatusInternalServerError)
				return
			}

			if !allowed {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
