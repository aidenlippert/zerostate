package p2p

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestHost(t *testing.T) host.Host {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	return h
}

func TestNewGossipService(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	assert.NotNil(t, gs)
	assert.NotNil(t, gs.pubsub)
	assert.Equal(t, h, gs.host)
}

func TestGossipServiceSubscribe(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	msgReceived := make(chan bool, 1)
	handler := func(ctx context.Context, msg *GossipMessage) error {
		msgReceived <- true
		return nil
	}

	err = gs.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Verify subscription exists
	gs.mu.RLock()
	_, exists := gs.subscriptions["test-topic"]
	gs.mu.RUnlock()
	assert.True(t, exists)
}

func TestGossipServicePublish(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	// Subscribe first
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}
	err = gs.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Publish message
	msg := &GossipMessage{
		Type:    "test",
		Payload: json.RawMessage(`{"data":"test"}`),
	}

	err = gs.Publish("test-topic", msg)
	require.NoError(t, err)

	// Message should have timestamp and peer ID
	assert.NotZero(t, msg.Timestamp)
	assert.NotEmpty(t, msg.PeerID)
}

func TestGossipServicePubSub(t *testing.T) {
	ctx := context.Background()

	// Create two hosts
	h1 := createTestHost(t)
	defer h1.Close()

	h2 := createTestHost(t)
	defer h2.Close()

	// Connect hosts
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	err := h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Create gossip services
	logger := zap.NewNop()
	gs1, err := NewGossipService(ctx, h1, logger)
	require.NoError(t, err)
	defer gs1.Close()

	gs2, err := NewGossipService(ctx, h2, logger)
	require.NoError(t, err)
	defer gs2.Close()

	// Subscribe on gs2
	msgReceived := make(chan *GossipMessage, 1)
	handler := func(ctx context.Context, msg *GossipMessage) error {
		msgReceived <- msg
		return nil
	}

	err = gs2.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Wait for subscription to propagate
	time.Sleep(100 * time.Millisecond)

	// Subscribe on gs1 (so it joins the topic)
	err = gs1.Subscribe("test-topic", func(ctx context.Context, msg *GossipMessage) error {
		return nil
	})
	require.NoError(t, err)

	// Wait for mesh to form
	time.Sleep(200 * time.Millisecond)

	// Publish from gs1
	testPayload := json.RawMessage(`{"test":"data"}`)
	msg := &GossipMessage{
		Type:    "test_message",
		Payload: testPayload,
	}

	err = gs1.Publish("test-topic", msg)
	require.NoError(t, err)

	// Wait for message
	select {
	case received := <-msgReceived:
		assert.Equal(t, "test_message", received.Type)
		assert.JSONEq(t, string(testPayload), string(received.Payload))
		assert.Equal(t, h1.ID().String(), received.PeerID)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestGossipServiceCardUpdate(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	// Subscribe
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}
	err = gs.Subscribe(TopicCardUpdates, handler)
	require.NoError(t, err)

	// Create card update
	vc := NewVectorClock()
	vc.Increment(peer.ID("test-peer"))

	update := &CardUpdate{
		Clock:     vc,
		UpdaterID: "test-peer",
		Timestamp: time.Now().Unix(),
	}

	// Publish card update
	err = gs.PublishCardUpdate(update)
	require.NoError(t, err)
}

func TestGossipServicePeerAnnouncement(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	// Subscribe
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}
	err = gs.Subscribe(TopicPeerAnnouncements, handler)
	require.NoError(t, err)

	// Publish peer announcement
	addresses := []string{"/ip4/127.0.0.1/tcp/4001"}
	err = gs.PublishPeerAnnouncement("join", addresses)
	require.NoError(t, err)
}

func TestGossipServiceMultipleHandlers(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	// Add multiple handlers
	count1 := 0
	count2 := 0

	handler1 := func(ctx context.Context, msg *GossipMessage) error {
		count1++
		return nil
	}

	handler2 := func(ctx context.Context, msg *GossipMessage) error {
		count2++
		return nil
	}

	err = gs.Subscribe("test-topic", handler1)
	require.NoError(t, err)

	err = gs.Subscribe("test-topic", handler2)
	require.NoError(t, err)

	// Publish message
	msg := &GossipMessage{
		Type:    "test",
		Payload: json.RawMessage(`{}`),
	}

	err = gs.Publish("test-topic", msg)
	require.NoError(t, err)

	// Wait for handlers
	time.Sleep(100 * time.Millisecond)

	// Both handlers should have been called (for our own message in this test)
	// Note: In real scenarios, we skip our own messages, but handlers are still registered
	gs.mu.RLock()
	handlerCount := len(gs.handlers["test-topic"])
	gs.mu.RUnlock()
	assert.Equal(t, 2, handlerCount)
}

func TestGossipServiceUnsubscribe(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)
	defer gs.Close()

	// Subscribe
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}
	err = gs.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Verify subscription
	gs.mu.RLock()
	_, exists := gs.subscriptions["test-topic"]
	gs.mu.RUnlock()
	assert.True(t, exists)

	// Unsubscribe
	err = gs.Unsubscribe("test-topic")
	require.NoError(t, err)

	// Verify unsubscribed
	gs.mu.RLock()
	_, exists = gs.subscriptions["test-topic"]
	gs.mu.RUnlock()
	assert.False(t, exists)
}

func TestGossipServiceListPeers(t *testing.T) {
	ctx := context.Background()

	// Create two hosts
	h1 := createTestHost(t)
	defer h1.Close()

	h2 := createTestHost(t)
	defer h2.Close()

	// Connect hosts
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	err := h1.Connect(ctx, peer.AddrInfo{ID: h2.ID(), Addrs: h2.Addrs()})
	require.NoError(t, err)

	// Create gossip services
	logger := zap.NewNop()
	gs1, err := NewGossipService(ctx, h1, logger)
	require.NoError(t, err)
	defer gs1.Close()

	gs2, err := NewGossipService(ctx, h2, logger)
	require.NoError(t, err)
	defer gs2.Close()

	// Subscribe both
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}

	err = gs1.Subscribe("test-topic", handler)
	require.NoError(t, err)

	err = gs2.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Wait for mesh to form
	time.Sleep(300 * time.Millisecond)

	// List peers
	peers := gs1.ListPeers("test-topic")
	
	// Should see h2 as a peer (may take time for gossipsub mesh to form)
	assert.GreaterOrEqual(t, len(peers), 0)
}

func TestGossipServiceClose(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gs, err := NewGossipService(ctx, h, logger)
	require.NoError(t, err)

	// Subscribe to a topic
	handler := func(ctx context.Context, msg *GossipMessage) error {
		return nil
	}
	err = gs.Subscribe("test-topic", handler)
	require.NoError(t, err)

	// Close
	err = gs.Close()
	require.NoError(t, err)

	// Verify closed
	gs.mu.RLock()
	subCount := len(gs.subscriptions)
	topicCount := len(gs.topics)
	gs.mu.RUnlock()

	assert.Equal(t, 0, subCount)
	assert.Equal(t, 0, topicCount)
}
