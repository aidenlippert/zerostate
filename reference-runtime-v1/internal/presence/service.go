package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/zap"
)

// Service manages L3 Aether presence announcements
type Service struct {
	host   host.Host
	pubsub *pubsub.PubSub
	topic  *pubsub.Topic
	config *Config
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc
	doneCh chan struct{}
}

// Config contains presence service configuration
type Config struct {
	AgentDID          string
	AgentName         string
	Capabilities      []string
	GRPCAddress       string
	HeartbeatInterval time.Duration
	PresenceTopic     string // e.g., "ainur/v1/global/l3_aether/presence/did:ainur:agent:math-001"
	AgentCard         *agentcard.AgentCard
}

// PresenceMessage represents an agent presence announcement
type PresenceMessage struct {
	DID          string               `json:"did"`
	Name         string               `json:"name"`
	Capabilities []string             `json:"capabilities"`
	Addresses    []string             `json:"addresses"`     // gRPC addresses for ARI-v1
	P2PAddresses []string             `json:"p2p_addresses"` // libp2p multiaddrs
	Timestamp    int64                `json:"timestamp"`
	Status       string               `json:"status"` // "online", "offline", "busy"
	Metadata     map[string]string    `json:"metadata,omitempty"`
	AgentCard    *agentcard.AgentCard `json:"agent_card,omitempty"` // Full AgentCard-VC-v1
}

// NewService creates a new presence service
func NewService(ctx context.Context, h host.Host, config *Config, logger *zap.Logger) (*Service, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create GossipSub with relaxed settings for local testing
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSigning(false), // Disable signing for local testing
		pubsub.WithStrictSignatureVerification(false),
		pubsub.WithFloodPublish(true), // Use flood publish for better local delivery
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	// Join presence topic
	topic, err := ps.Join(config.PresenceTopic)
	if err != nil {
		return nil, fmt.Errorf("failed to join topic %s: %w", config.PresenceTopic, err)
	}

	svcCtx, cancel := context.WithCancel(ctx)

	s := &Service{
		host:   h,
		pubsub: ps,
		topic:  topic,
		config: config,
		logger: logger,
		ctx:    svcCtx,
		cancel: cancel,
		doneCh: make(chan struct{}),
	}

	logger.Info("presence service created",
		zap.String("topic", config.PresenceTopic),
		zap.Duration("heartbeat", config.HeartbeatInterval),
	)

	return s, nil
}

// Start begins publishing presence heartbeats
func (s *Service) Start() error {
	// Publish initial presence immediately
	if err := s.publishPresence("online"); err != nil {
		s.logger.Error("failed to publish initial presence", zap.Error(err))
		return err
	}

	// Start heartbeat loop
	go s.heartbeatLoop()

	s.logger.Info("presence service started",
		zap.String("did", s.config.AgentDID),
		zap.String("topic", s.config.PresenceTopic),
	)

	return nil
}

// Stop stops the presence service
func (s *Service) Stop() error {
	// Publish offline status
	if err := s.publishPresence("offline"); err != nil {
		s.logger.Error("failed to publish offline status", zap.Error(err))
	}

	s.cancel()
	close(s.doneCh)

	if err := s.topic.Close(); err != nil {
		s.logger.Error("failed to close topic", zap.Error(err))
		return err
	}

	s.logger.Info("presence service stopped")
	return nil
}

// heartbeatLoop publishes periodic presence updates
func (s *Service) heartbeatLoop() {
	ticker := time.NewTicker(s.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			if err := s.publishPresence("online"); err != nil {
				s.logger.Error("failed to publish heartbeat", zap.Error(err))
			}
		}
	}
}

// publishPresence publishes a presence announcement
func (s *Service) publishPresence(status string) error {
	// Get host multiaddrs
	p2pAddrs := make([]string, 0, len(s.host.Addrs()))
	for _, addr := range s.host.Addrs() {
		p2pAddrs = append(p2pAddrs, addr.String())
	}

	msg := &PresenceMessage{
		DID:          s.config.AgentDID,
		Name:         s.config.AgentName,
		Capabilities: s.config.Capabilities,
		Addresses:    []string{s.config.GRPCAddress}, // ARI-v1 gRPC endpoint
		P2PAddresses: p2pAddrs,
		Timestamp:    time.Now().Unix(),
		Status:       status,
		Metadata: map[string]string{
			"protocol": "ari-v1",
			"runtime":  "reference-runtime-v1",
			"version":  "1.0.0",
		},
		AgentCard: s.config.AgentCard, // Include full AgentCard
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal presence message: %w", err)
	}

	if err := s.topic.Publish(s.ctx, data); err != nil {
		return fmt.Errorf("failed to publish to topic: %w", err)
	}

	s.logger.Debug("published presence",
		zap.String("did", msg.DID),
		zap.String("status", status),
		zap.String("grpc_address", s.config.GRPCAddress),
		zap.Int("p2p_addresses", len(p2pAddrs)),
	)

	return nil
}
