package p2p

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewRequestDeduplicator(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := DefaultRequestDeduplicatorConfig()

	rd := NewRequestDeduplicator(ctx, config, logger)
	defer rd.Close()

	assert.NotNil(t, rd)
	assert.Equal(t, config.CacheTTL, rd.config.CacheTTL)
	assert.Equal(t, config.MaxCacheSize, rd.config.MaxCacheSize)
}

func TestDefaultRequestDeduplicatorConfig(t *testing.T) {
	config := DefaultRequestDeduplicatorConfig()

	assert.Equal(t, DefaultCacheTTL, config.CacheTTL)
	assert.Equal(t, DefaultDedupCleanupInterval, config.CleanupInterval)
	assert.Equal(t, DefaultMaxCacheSize, config.MaxCacheSize)
	assert.True(t, config.EnableMetrics)
}

func TestRequestDeduplicatorDo(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	callCount := 0
	fn := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	result, err := rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, "result", result)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	result, err = rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, "result", result)
	assert.Equal(t, 1, callCount, "function should not be called again")
}

func TestRequestDeduplicatorConcurrentRequests(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	var callCount int32
	fn := func() (interface{}, error) {
		atomic.AddInt32(&callCount, 1)
		time.Sleep(100 * time.Millisecond)
		return "result", nil
	}

	// Launch 10 concurrent requests
	results := make(chan interface{}, 10)
	for i := 0; i < 10; i++ {
		go func() {
			result, _ := rd.Do(ctx, "test-key", fn)
			results <- result
		}()
	}

	// Collect all results
	for i := 0; i < 10; i++ {
		result := <-results
		assert.Equal(t, "result", result)
	}

	// Function should only be called once
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))
}

func TestRequestDeduplicatorError(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	testErr := errors.New("test error")
	fn := func() (interface{}, error) {
		return nil, testErr
	}

	result, err := rd.Do(ctx, "test-key", fn)
	assert.Error(t, err)
	assert.Equal(t, testErr, err)
	assert.Nil(t, result)

	// Error should not be cached
	stats := rd.Stats()
	assert.Equal(t, 0, stats.CachedEntries)
}

func TestRequestDeduplicatorInvalidate(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	callCount := 0
	fn := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	// Cache entry
	_, err := rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Invalidate
	rd.Invalidate("test-key")

	// Should execute again
	_, err = rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRequestDeduplicatorInvalidatePattern(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	// Cache multiple entries
	_, _ = rd.Do(ctx, "prefix:key1", fn)
	_, _ = rd.Do(ctx, "prefix:key2", fn)
	_, _ = rd.Do(ctx, "other:key3", fn)

	stats := rd.Stats()
	assert.Equal(t, 3, stats.CachedEntries)

	// Invalidate pattern
	removed := rd.InvalidatePattern("prefix:")
	assert.Equal(t, 2, removed)

	stats = rd.Stats()
	assert.Equal(t, 1, stats.CachedEntries)
}

func TestRequestDeduplicatorCacheTTL(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := &RequestDeduplicatorConfig{
		CacheTTL:        100 * time.Millisecond,
		CleanupInterval: 50 * time.Millisecond,
		MaxCacheSize:    100,
		EnableMetrics:   true,
	}
	rd := NewRequestDeduplicator(ctx, config, logger)
	defer rd.Close()

	callCount := 0
	fn := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	// Cache entry
	_, err := rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Should use cache
	_, err = rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Wait for TTL expiry
	time.Sleep(200 * time.Millisecond)

	// Should execute again
	_, err = rd.Do(ctx, "test-key", fn)
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestRequestDeduplicatorMaxCacheSize(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := &RequestDeduplicatorConfig{
		CacheTTL:        1 * time.Hour,
		CleanupInterval: 1 * time.Minute,
		MaxCacheSize:    5,
		EnableMetrics:   true,
	}
	rd := NewRequestDeduplicator(ctx, config, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	// Cache more than max
	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		_, err := rd.Do(ctx, key, fn)
		require.NoError(t, err)
	}

	stats := rd.Stats()
	assert.LessOrEqual(t, stats.CachedEntries, 5)
}

func TestRequestDeduplicatorStats(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	// Cache some entries
	_, _ = rd.Do(ctx, "key1", fn)
	_, _ = rd.Do(ctx, "key2", fn)

	stats := rd.Stats()
	assert.Equal(t, 2, stats.CachedEntries)
	assert.Equal(t, 0, stats.InflightRequests)
}

func TestRequestDeduplicatorClose(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)

	fn := func() (interface{}, error) {
		return "result", nil
	}

	_, _ = rd.Do(ctx, "key1", fn)

	stats := rd.Stats()
	assert.Equal(t, 1, stats.CachedEntries)

	err := rd.Close()
	assert.NoError(t, err)

	stats = rd.Stats()
	assert.Equal(t, 0, stats.CachedEntries)
}

func TestRequestDeduplicatorCacheHits(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	// First call
	_, _ = rd.Do(ctx, "test-key", fn)

	// Multiple cache hits
	for i := 0; i < 5; i++ {
		_, _ = rd.Do(ctx, "test-key", fn)
	}

	stats := rd.Stats()
	assert.Greater(t, stats.TotalCacheHits, 0)
}

func BenchmarkRequestDeduplicatorDo(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := string(rune('a' + (i % 26)))
		rd.Do(ctx, key, fn)
	}
}

func BenchmarkRequestDeduplicatorConcurrent(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	rd := NewRequestDeduplicator(ctx, nil, logger)
	defer rd.Close()

	fn := func() (interface{}, error) {
		return "result", nil
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune('a' + (i % 26)))
			rd.Do(ctx, key, fn)
			i++
		}
	})
}
