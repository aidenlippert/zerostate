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

func TestNewTokenBucket(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	defer tb.Stop()

	assert.Equal(t, 100, tb.capacity)
	assert.Equal(t, 100, tb.tokens)
	assert.Equal(t, 10, tb.refillRate)
}

func TestTokenBucketTryTake(t *testing.T) {
	tb := NewTokenBucket(100, 10)
	defer tb.Stop()

	// Should succeed
	assert.True(t, tb.TryTake(50))
	assert.Equal(t, 50, tb.Available())

	// Should succeed
	assert.True(t, tb.TryTake(50))
	assert.Equal(t, 0, tb.Available())

	// Should fail (no tokens)
	assert.False(t, tb.TryTake(1))
}

func TestTokenBucketTake(t *testing.T) {
	tb := NewTokenBucket(100, 100) // Fast refill
	defer tb.Stop()

	ctx := context.Background()

	// Take all tokens
	err := tb.Take(ctx, 100)
	require.NoError(t, err)
	assert.Equal(t, 0, tb.Available())

	// Wait for refill
	time.Sleep(150 * time.Millisecond)

	// Should have refilled
	assert.Greater(t, tb.Available(), 0)
}

func TestTokenBucketTakeWithContext(t *testing.T) {
	tb := NewTokenBucket(10, 1) // Slow refill
	defer tb.Stop()

	// Take all tokens
	tb.TryTake(10)

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Should timeout (not enough tokens)
	err := tb.Take(ctx, 100)
	assert.Error(t, err)
	assert.Equal(t, context.DeadlineExceeded, err)
}

func TestTokenBucketRefill(t *testing.T) {
	tb := NewTokenBucket(100, 100) // 100 tokens/sec
	defer tb.Stop()

	// Take all tokens
	tb.TryTake(100)
	assert.Equal(t, 0, tb.Available())

	// Wait for refill (should refill ~10 tokens in 100ms)
	time.Sleep(150 * time.Millisecond)

	available := tb.Available()
	assert.Greater(t, available, 5)  // At least some refill
	assert.LessOrEqual(t, available, 100) // Not more than capacity
}

func TestNewSendWindow(t *testing.T) {
	sw := NewSendWindow(10)
	assert.Equal(t, 10, sw.WindowSize())
	assert.Equal(t, 0, sw.InFlight())
	assert.True(t, sw.CanSend())
}

func TestSendWindowSend(t *testing.T) {
	sw := NewSendWindow(3)

	// Send 3 messages (fill window)
	msgID1, err := sw.Send()
	require.NoError(t, err)
	assert.Equal(t, uint64(1), msgID1)
	assert.Equal(t, 1, sw.InFlight())

	msgID2, err := sw.Send()
	require.NoError(t, err)
	assert.Equal(t, uint64(2), msgID2)
	assert.Equal(t, 2, sw.InFlight())

	msgID3, err := sw.Send()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), msgID3)
	assert.Equal(t, 3, sw.InFlight())

	// Window full
	assert.False(t, sw.CanSend())
	_, err = sw.Send()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "window full")
}

func TestSendWindowAck(t *testing.T) {
	sw := NewSendWindow(2)

	// Fill window
	msgID1, _ := sw.Send()
	msgID2, _ := sw.Send()
	assert.Equal(t, 2, sw.InFlight())
	assert.False(t, sw.CanSend())

	// Ack first message
	sw.Ack(msgID1)
	assert.Equal(t, 1, sw.InFlight())
	assert.True(t, sw.CanSend())

	// Can send again
	_, err := sw.Send()
	require.NoError(t, err)
	assert.Equal(t, 2, sw.InFlight())

	// Ack second message
	sw.Ack(msgID2)
	assert.Equal(t, 1, sw.InFlight())
}

func TestSendWindowAdjust(t *testing.T) {
	sw := NewSendWindow(5)
	assert.Equal(t, 5, sw.WindowSize())

	// Increase window
	sw.AdjustWindow(10)
	assert.Equal(t, 10, sw.WindowSize())

	// Decrease window
	sw.AdjustWindow(3)
	assert.Equal(t, 3, sw.WindowSize())

	// Invalid adjustment (ignored)
	sw.AdjustWindow(0)
	assert.Equal(t, 3, sw.WindowSize())

	sw.AdjustWindow(-5)
	assert.Equal(t, 3, sw.WindowSize())
}

func TestNewFlowController(t *testing.T) {
	logger := zap.NewNop()
	fc := NewFlowController(nil, logger)
	defer fc.Close()

	assert.NotNil(t, fc)
	assert.NotNil(t, fc.globalLimiter)
	assert.NotNil(t, fc.config)
}

func TestFlowControllerAllowSend(t *testing.T) {
	config := &FlowControlConfig{
		GlobalRateLimit:  1024,
		PerPeerRateLimit: 512,
		BucketCapacity:   1024,
		WindowSize:       10,
		MaxTrackedPeers:  100,
	}

	logger := zap.NewNop()
	fc := NewFlowController(config, logger)
	defer fc.Close()

	ctx := context.Background()
	peerID := peer.ID("test-peer")

	// Should allow small send
	err := fc.AllowSend(ctx, peerID, 100)
	assert.NoError(t, err)

	// Should allow another small send
	err = fc.AllowSend(ctx, peerID, 100)
	assert.NoError(t, err)
}

func TestFlowControllerPeerLimit(t *testing.T) {
	config := &FlowControlConfig{
		GlobalRateLimit:  10000,
		PerPeerRateLimit: 500, // Very low peer limit
		BucketCapacity:   500,
		WindowSize:       10,
		MaxTrackedPeers:  100,
	}

	logger := zap.NewNop()
	fc := NewFlowController(config, logger)
	defer fc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	peerID := peer.ID("test-peer")

	// Take all peer tokens
	err := fc.AllowSend(context.Background(), peerID, 500)
	require.NoError(t, err)

	// Should fail (peer limit exceeded, context timeout)
	err = fc.AllowSend(ctx, peerID, 100)
	assert.Error(t, err)
}

func TestFlowControllerAcquireWindow(t *testing.T) {
	config := &FlowControlConfig{
		GlobalRateLimit:  10000,
		PerPeerRateLimit: 5000,
		BucketCapacity:   10000,
		WindowSize:       3,
		MaxTrackedPeers:  100,
	}

	logger := zap.NewNop()
	fc := NewFlowController(config, logger)
	defer fc.Close()

	peerID := peer.ID("test-peer")

	// Acquire 3 slots (fill window)
	msgID1, err := fc.AcquireWindow(peerID)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), msgID1)

	msgID2, err := fc.AcquireWindow(peerID)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), msgID2)

	msgID3, err := fc.AcquireWindow(peerID)
	require.NoError(t, err)
	assert.Equal(t, uint64(3), msgID3)

	// Window full
	_, err = fc.AcquireWindow(peerID)
	assert.Error(t, err)

	// Release one slot
	fc.ReleaseWindow(peerID, msgID1)

	// Can acquire again
	msgID4, err := fc.AcquireWindow(peerID)
	require.NoError(t, err)
	assert.Equal(t, uint64(4), msgID4)
}

func TestFlowControllerRemovePeer(t *testing.T) {
	logger := zap.NewNop()
	fc := NewFlowController(nil, logger)
	defer fc.Close()

	peerID := peer.ID("test-peer")

	// Create state for peer
	fc.AcquireWindow(peerID)
	fc.AllowSend(context.Background(), peerID, 100)

	// Verify state exists
	assert.NotNil(t, fc.GetPeerWindow(peerID))

	// Remove peer
	fc.RemovePeer(peerID)

	// State should be gone
	assert.Nil(t, fc.GetPeerWindow(peerID))
}

func TestFlowControllerMultiplePeers(t *testing.T) {
	config := DefaultFlowControlConfig()
	logger := zap.NewNop()
	fc := NewFlowController(config, logger)
	defer fc.Close()

	ctx := context.Background()
	peer1 := peer.ID("peer-1")
	peer2 := peer.ID("peer-2")

	// Both peers should be able to send independently
	err1 := fc.AllowSend(ctx, peer1, 1000)
	err2 := fc.AllowSend(ctx, peer2, 1000)

	assert.NoError(t, err1)
	assert.NoError(t, err2)

	// Both should have independent windows
	msgID1, err := fc.AcquireWindow(peer1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), msgID1)

	msgID2, err := fc.AcquireWindow(peer2)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), msgID2) // Independent counter
}
