package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPriorityString(t *testing.T) {
	assert.Equal(t, "low", PriorityLow.String())
	assert.Equal(t, "normal", PriorityNormal.String())
	assert.Equal(t, "high", PriorityHigh.String())
	assert.Equal(t, "unknown", Priority(99).String())
}

func TestDefaultBandwidthQoSConfig(t *testing.T) {
	config := DefaultBandwidthQoSConfig()

	assert.Equal(t, int64(DefaultBandwidthLimit), config.BandwidthLimit)
	assert.Equal(t, int64(DefaultBurstSize), config.BurstSize)
	assert.Equal(t, DefaultRefillInterval, config.RefillInterval)
	assert.Equal(t, DefaultMaxQueueSize, config.MaxQueueSize)
	assert.True(t, config.EnableMetrics)
}

func TestNewPriorityQueue(t *testing.T) {
	logger := zap.NewNop()
	pq := NewPriorityQueue(100, logger)
	defer pq.Close()

	assert.NotNil(t, pq)
	assert.Equal(t, 100, pq.maxSize)

	depth := pq.Depth()
	assert.Equal(t, 0, depth[PriorityHigh])
	assert.Equal(t, 0, depth[PriorityNormal])
	assert.Equal(t, 0, depth[PriorityLow])
}

func TestPriorityQueueEnqueueDequeue(t *testing.T) {
	logger := zap.NewNop()
	pq := NewPriorityQueue(10, logger)
	defer pq.Close()

	// Enqueue messages with different priorities
	msg1 := &QueuedMessage{
		Data:      []byte("high"),
		Priority:  PriorityHigh,
		Timestamp: time.Now(),
		ResultCh:  make(chan error, 1),
	}
	msg2 := &QueuedMessage{
		Data:      []byte("normal"),
		Priority:  PriorityNormal,
		Timestamp: time.Now(),
		ResultCh:  make(chan error, 1),
	}
	msg3 := &QueuedMessage{
		Data:      []byte("low"),
		Priority:  PriorityLow,
		Timestamp: time.Now(),
		ResultCh:  make(chan error, 1),
	}

	err := pq.Enqueue(msg3) // Low
	require.NoError(t, err)
	err = pq.Enqueue(msg2) // Normal
	require.NoError(t, err)
	err = pq.Enqueue(msg1) // High
	require.NoError(t, err)

	// Should dequeue in priority order: High > Normal > Low
	dequeued, err := pq.Dequeue()
	require.NoError(t, err)
	assert.Equal(t, "high", string(dequeued.Data))

	dequeued, err = pq.Dequeue()
	require.NoError(t, err)
	assert.Equal(t, "normal", string(dequeued.Data))

	dequeued, err = pq.Dequeue()
	require.NoError(t, err)
	assert.Equal(t, "low", string(dequeued.Data))
}

func TestPriorityQueueFull(t *testing.T) {
	logger := zap.NewNop()
	pq := NewPriorityQueue(2, logger)
	defer pq.Close()

	// Fill queue
	msg := &QueuedMessage{
		Data:     []byte("test"),
		Priority: PriorityNormal,
		ResultCh: make(chan error, 1),
	}

	err := pq.Enqueue(msg)
	require.NoError(t, err)
	err = pq.Enqueue(msg)
	require.NoError(t, err)

	// Should reject when full
	err = pq.Enqueue(msg)
	assert.Equal(t, ErrQueueFull, err)
}

func TestPriorityQueueClose(t *testing.T) {
	logger := zap.NewNop()
	pq := NewPriorityQueue(10, logger)

	msg := &QueuedMessage{
		Data:     []byte("test"),
		Priority: PriorityNormal,
		ResultCh: make(chan error, 1),
	}

	err := pq.Enqueue(msg)
	require.NoError(t, err)

	err = pq.Close()
	require.NoError(t, err)

	// Should error after close
	err = pq.Enqueue(msg)
	assert.Equal(t, ErrClosed, err)
}

func TestNewBandwidthQoS(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := DefaultBandwidthQoSConfig()

	bq := NewBandwidthQoS(ctx, config, logger)
	defer bq.Close()

	assert.NotNil(t, bq)
	assert.Equal(t, config.BandwidthLimit, bq.config.BandwidthLimit)
}

func TestRecordBandwidth(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	// Record upload
	bq.RecordUpload(peerID, 1000)

	peer := bq.GetPeerBandwidth(peerID)
	require.NotNil(t, peer)
	assert.Equal(t, int64(1000), peer.uploadBytes)

	// Record download
	bq.RecordDownload(peerID, 500)

	peer = bq.GetPeerBandwidth(peerID)
	require.NotNil(t, peer)
	assert.Equal(t, int64(500), peer.downloadBytes)
}

func TestCheckBandwidth(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := &BandwidthQoSConfig{
		BandwidthLimit: 1000,
		BurstSize:      5000,
		RefillInterval: 100 * time.Millisecond,
		MaxQueueSize:   100,
		EnableMetrics:  true,
	}
	bq := NewBandwidthQoS(ctx, config, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	// Record initial upload to create peer bandwidth tracking
	bq.RecordUpload(peerID, 0)

	// First request should succeed (within burst)
	allowed := bq.CheckBandwidth(peerID, 3000)
	assert.True(t, allowed)

	// Second request should succeed
	allowed = bq.CheckBandwidth(peerID, 2000)
	assert.True(t, allowed)

	// Third request should fail (exceeds remaining burst)
	allowed = bq.CheckBandwidth(peerID, 1000)
	assert.False(t, allowed)
}

func TestTokenRefill(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	config := &BandwidthQoSConfig{
		BandwidthLimit: 10000, // 10KB/s
		BurstSize:      5000,
		RefillInterval: 100 * time.Millisecond,
		MaxQueueSize:   100,
		EnableMetrics:  true,
	}
	bq := NewBandwidthQoS(ctx, config, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	// Record to create peer
	bq.RecordUpload(peerID, 0)

	// Use all tokens
	allowed := bq.CheckBandwidth(peerID, 5000)
	assert.True(t, allowed)

	// Should be throttled
	allowed = bq.CheckBandwidth(peerID, 1000)
	assert.False(t, allowed)

	// Wait for refill (100ms at 10KB/s = 1KB refill)
	time.Sleep(150 * time.Millisecond)

	// Should have tokens now
	allowed = bq.CheckBandwidth(peerID, 1000)
	assert.True(t, allowed)
}

func TestBandwidthRateCalculation(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	// Record some data
	bq.RecordUpload(peerID, 1000)
	bq.RecordDownload(peerID, 500)

	// Wait for rate update (runs every 1 second)
	time.Sleep(1200 * time.Millisecond)

	peer := bq.GetPeerBandwidth(peerID)
	require.NotNil(t, peer)

	// Rates should be calculated
	assert.Greater(t, peer.uploadRate, 0.0)
	assert.Greater(t, peer.downloadRate, 0.0)
}

func TestBandwidthQoSStats(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)
	defer bq.Close()

	peerID1 := peer.ID("peer1")
	peerID2 := peer.ID("peer2")

	bq.RecordUpload(peerID1, 100)
	bq.RecordUpload(peerID2, 200)

	stats := bq.Stats()
	assert.Equal(t, 2, stats.TrackedPeers)
}

func TestBandwidthQoSClose(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)

	peerID := peer.ID("test-peer")
	bq.RecordUpload(peerID, 100)

	stats := bq.Stats()
	assert.Equal(t, 1, stats.TrackedPeers)

	err := bq.Close()
	assert.NoError(t, err)

	stats = bq.Stats()
	assert.Equal(t, 0, stats.TrackedPeers)
}

func BenchmarkRecordBandwidth(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			bq.RecordUpload(peerID, 1024)
		} else {
			bq.RecordDownload(peerID, 1024)
		}
	}
}

func BenchmarkCheckBandwidth(b *testing.B) {
	ctx := context.Background()
	logger := zap.NewNop()
	bq := NewBandwidthQoS(ctx, nil, logger)
	defer bq.Close()

	peerID := peer.ID("test-peer")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bq.CheckBandwidth(peerID, 100)
	}
}

func BenchmarkPriorityQueue(b *testing.B) {
	logger := zap.NewNop()
	pq := NewPriorityQueue(1000, logger)
	defer pq.Close()

	msg := &QueuedMessage{
		Data:     []byte("test"),
		Priority: PriorityNormal,
		ResultCh: make(chan error, 1),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pq.Enqueue(msg)
		pq.Dequeue()
	}
}
