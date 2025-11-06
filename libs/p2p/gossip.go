package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	gossipMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_gossip_messages_published_total",
			Help: "Total messages published to gossipsub topics",
		},
		[]string{"topic"},
	)

	gossipMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_gossip_messages_received_total",
			Help: "Total messages received from gossipsub topics",
		},
		[]string{"topic"},
	)

	gossipPeerCount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_gossip_peer_count",
			Help: "Number of peers subscribed to each topic",
		},
		[]string{"topic"},
	)

	gossipMessageLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_gossip_message_latency_seconds",
			Help:    "Latency from publish to receive for gossip messages",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"topic"},
	)

	gossipValidationFailures = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_gossip_validation_failures_total",
			Help: "Total message validation failures",
		},
		[]string{"topic", "reason"},
	)
)

const (
	// TopicCardUpdates is the topic for agent card updates
	TopicCardUpdates = "/zerostate/cards/1.0.0"
	// TopicPeerAnnouncements is the topic for peer announcements
	TopicPeerAnnouncements = "/zerostate/peers/1.0.0"
	// TopicContentAnnouncements is the topic for content announcements
	TopicContentAnnouncements = "/zerostate/content/1.0.0"
)

// GossipMessage represents a message on the gossip network
type GossipMessage struct {
	Type      string          `json:"type"`       // Message type
	Payload   json.RawMessage `json:"payload"`    // Message payload
	Timestamp int64           `json:"timestamp"`  // Unix timestamp
	PeerID    string          `json:"peer_id"`    // Sender peer ID
	Signature []byte          `json:"signature"`  // Ed25519 signature
}

// CardUpdateMessage represents an agent card update
type CardUpdateMessage struct {
	Update *CardUpdate `json:"update"` // Card update with vector clock
}

// PeerAnnouncementMessage represents a peer joining/leaving
type PeerAnnouncementMessage struct {
	PeerID    string   `json:"peer_id"`
	Addresses []string `json:"addresses"`
	Action    string   `json:"action"` // "join" or "leave"
}

// MessageHandler is called when a message is received
type MessageHandler func(ctx context.Context, msg *GossipMessage) error

// GossipService manages GossipSub pub/sub
type GossipService struct {
	mu           sync.RWMutex
	host         host.Host
	pubsub       *pubsub.PubSub
	topics       map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription
	handlers     map[string][]MessageHandler
	logger       *zap.Logger
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewGossipService creates a new gossip service
func NewGossipService(ctx context.Context, h host.Host, logger *zap.Logger) (*GossipService, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create GossipSub with good defaults
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSigning(true),
		pubsub.WithStrictSignatureVerification(true),
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(false), // Use gossip, not flood
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	svcCtx, cancel := context.WithCancel(ctx)

	gs := &GossipService{
		host:          h,
		pubsub:        ps,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
		handlers:      make(map[string][]MessageHandler),
		logger:        logger,
		ctx:           svcCtx,
		cancel:        cancel,
	}

	logger.Info("gossip service created")
	return gs, nil
}

// Subscribe subscribes to a topic with a message handler
func (gs *GossipService) Subscribe(topicName string, handler MessageHandler) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Get or join topic
	topic, err := gs.getOrJoinTopic(topicName)
	if err != nil {
		return fmt.Errorf("failed to join topic: %w", err)
	}

	// Subscribe if not already subscribed
	if _, exists := gs.subscriptions[topicName]; !exists {
		sub, err := topic.Subscribe()
		if err != nil {
			return fmt.Errorf("failed to subscribe to topic: %w", err)
		}
		gs.subscriptions[topicName] = sub

		// Start message handler goroutine
		go gs.handleMessages(topicName, sub)
	}

	// Add handler
	gs.handlers[topicName] = append(gs.handlers[topicName], handler)

	gs.logger.Info("subscribed to topic",
		zap.String("topic", topicName),
		zap.Int("handler_count", len(gs.handlers[topicName])),
	)

	return nil
}

// Publish publishes a message to a topic
func (gs *GossipService) Publish(topicName string, msg *GossipMessage) error {
	gs.mu.RLock()
	topic, exists := gs.topics[topicName]
	gs.mu.RUnlock()

	if !exists {
		gs.mu.Lock()
		var err error
		topic, err = gs.getOrJoinTopic(topicName)
		gs.mu.Unlock()
		if err != nil {
			return fmt.Errorf("failed to join topic: %w", err)
		}
	}

	// Set metadata
	msg.Timestamp = time.Now().Unix()
	msg.PeerID = gs.host.ID().String()

	// Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish
	if err := topic.Publish(gs.ctx, data); err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	gossipMessagesPublished.WithLabelValues(topicName).Inc()
	gs.logger.Debug("published message",
		zap.String("topic", topicName),
		zap.String("type", msg.Type),
	)

	return nil
}

// PublishCardUpdate publishes a card update
func (gs *GossipService) PublishCardUpdate(update *CardUpdate) error {
	payload, err := json.Marshal(&CardUpdateMessage{Update: update})
	if err != nil {
		return fmt.Errorf("failed to marshal card update: %w", err)
	}

	msg := &GossipMessage{
		Type:    "card_update",
		Payload: payload,
	}

	return gs.Publish(TopicCardUpdates, msg)
}

// PublishPeerAnnouncement publishes a peer announcement
func (gs *GossipService) PublishPeerAnnouncement(action string, addresses []string) error {
	payload, err := json.Marshal(&PeerAnnouncementMessage{
		PeerID:    gs.host.ID().String(),
		Addresses: addresses,
		Action:    action,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal peer announcement: %w", err)
	}

	msg := &GossipMessage{
		Type:    "peer_announcement",
		Payload: payload,
	}

	return gs.Publish(TopicPeerAnnouncements, msg)
}

// getOrJoinTopic gets an existing topic or joins a new one
func (gs *GossipService) getOrJoinTopic(topicName string) (*pubsub.Topic, error) {
	if topic, exists := gs.topics[topicName]; exists {
		return topic, nil
	}

	topic, err := gs.pubsub.Join(topicName)
	if err != nil {
		return nil, err
	}

	gs.topics[topicName] = topic
	gs.logger.Info("joined topic", zap.String("topic", topicName))

	return topic, nil
}

// handleMessages processes incoming messages for a topic
func (gs *GossipService) handleMessages(topicName string, sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(gs.ctx)
		if err != nil {
			if gs.ctx.Err() != nil {
				// Context cancelled, shutting down
				return
			}
			gs.logger.Error("error reading message",
				zap.String("topic", topicName),
				zap.Error(err),
			)
			continue
		}

		// Skip our own messages
		if msg.ReceivedFrom == gs.host.ID() {
			continue
		}

		// Parse message
		var gossipMsg GossipMessage
		if err := json.Unmarshal(msg.Data, &gossipMsg); err != nil {
			gs.logger.Error("failed to unmarshal message",
				zap.String("topic", topicName),
				zap.Error(err),
			)
			gossipValidationFailures.WithLabelValues(topicName, "unmarshal_error").Inc()
			continue
		}

		// Calculate latency
		if gossipMsg.Timestamp > 0 {
			latency := time.Since(time.Unix(gossipMsg.Timestamp, 0)).Seconds()
			gossipMessageLatency.WithLabelValues(topicName).Observe(latency)
		}

		gossipMessagesReceived.WithLabelValues(topicName).Inc()

		// Call handlers
		gs.mu.RLock()
		handlers := gs.handlers[topicName]
		gs.mu.RUnlock()

		for _, handler := range handlers {
			if err := handler(gs.ctx, &gossipMsg); err != nil {
				gs.logger.Error("handler error",
					zap.String("topic", topicName),
					zap.String("type", gossipMsg.Type),
					zap.Error(err),
				)
			}
		}

		// Update peer count
		peers := gs.pubsub.ListPeers(topicName)
		gossipPeerCount.WithLabelValues(topicName).Set(float64(len(peers)))
	}
}

// ListPeers returns peers subscribed to a topic
func (gs *GossipService) ListPeers(topicName string) []peer.ID {
	return gs.pubsub.ListPeers(topicName)
}

// Unsubscribe unsubscribes from a topic
func (gs *GossipService) Unsubscribe(topicName string) error {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	if sub, exists := gs.subscriptions[topicName]; exists {
		sub.Cancel()
		delete(gs.subscriptions, topicName)
	}

	if topic, exists := gs.topics[topicName]; exists {
		if err := topic.Close(); err != nil {
			return fmt.Errorf("failed to close topic: %w", err)
		}
		delete(gs.topics, topicName)
	}

	delete(gs.handlers, topicName)

	gs.logger.Info("unsubscribed from topic", zap.String("topic", topicName))
	return nil
}

// Close stops the gossip service
func (gs *GossipService) Close() error {
	gs.cancel()

	gs.mu.Lock()
	defer gs.mu.Unlock()

	// Close all subscriptions
	for name, sub := range gs.subscriptions {
		sub.Cancel()
		gs.logger.Debug("cancelled subscription", zap.String("topic", name))
	}
	gs.subscriptions = make(map[string]*pubsub.Subscription)

	// Close all topics
	for name, topic := range gs.topics {
		if err := topic.Close(); err != nil {
			gs.logger.Error("error closing topic",
				zap.String("topic", name),
				zap.Error(err),
			)
		}
	}
	gs.topics = make(map[string]*pubsub.Topic)
	gs.handlers = make(map[string][]MessageHandler)

	gs.logger.Info("gossip service closed")
	return nil
}
