package market

import (
	"context"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"go.uber.org/zap"
)

// Bidder is the runtime-side component that listens for CFPs
// and decides whether to bid. This is a scaffolding implementation
// that only logs incoming CFPs.
type Bidder struct {
	bus       MessageBus
	agentCard *agentcard.AgentCard
	logger    *zap.Logger
}

// NewBidder creates a new Bidder.
func NewBidder(bus MessageBus, card *agentcard.AgentCard, logger *zap.Logger) *Bidder {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Bidder{
		bus:       bus,
		agentCard: card,
		logger:    logger,
	}
}

// Start subscribes to CFP topics based on the agent's capabilities.
// For now this is a no-op stub that can be wired into the runtime
// main when the p2p message bus is available.
func (b *Bidder) Start(ctx context.Context) error {
	if b.agentCard == nil {
		b.logger.Warn("Bidder.Start called with nil agentCard; skipping subscription")
		return nil
	}

	b.logger.Info("Bidder scaffolding active - pending CFP topic subscription",
		zap.String("did", agentDID(b.agentCard)),
	)

	// TODO: derive CFP topics from capabilities and subscribe via p2p.MessageBus
	// Example topic pattern: ainur/v1/market/cfp/{capability}

	return nil
}
