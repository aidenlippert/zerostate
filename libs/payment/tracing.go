package payment

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/aidenlippert/zerostate/libs/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PaymentTracer provides tracing utilities for payment operations
type PaymentTracer struct {
	helper *telemetry.TraceHelper
}

// NewPaymentTracer creates a new payment tracer
func NewPaymentTracer() *PaymentTracer {
	return &PaymentTracer{
		helper: telemetry.NewTraceHelper("payment"),
	}
}

// TraceOpenChannel instruments payment channel opening
func (t *PaymentTracer) TraceOpenChannel(ctx context.Context, partyA, partyB peer.ID) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.channel.open")

	span.SetAttributes(
		attribute.String("channel.party_a", partyA.String()),
		attribute.String("channel.party_b", partyB.String()),
	)

	return ctx, span
}

// TraceCloseChannel instruments payment channel closing
func (t *PaymentTracer) TraceCloseChannel(ctx context.Context, channelID string, reason string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.channel.close",
		telemetry.WithChannelID(channelID),
	)

	span.SetAttributes(
		attribute.String("channel.close_reason", reason),
	)

	return ctx, span
}

// TracePayment instruments a payment transaction
func (t *PaymentTracer) TracePayment(ctx context.Context, channelID string, from, to peer.ID, amount float64) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.transaction",
		telemetry.WithChannelID(channelID),
	)

	span.SetAttributes(
		attribute.String("payment.from", from.String()),
		attribute.String("payment.to", to.String()),
		attribute.Float64("payment.amount", amount),
	)

	return ctx, span
}

// TraceChannelUpdate instruments channel state update
func (t *PaymentTracer) TraceChannelUpdate(ctx context.Context, channelID string, sequenceNum uint64) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.channel.update",
		telemetry.WithChannelID(channelID),
	)

	span.SetAttributes(
		attribute.Int64("channel.sequence_num", int64(sequenceNum)),
	)

	return ctx, span
}

// TraceSettlement instruments channel settlement (on-chain finalization)
func (t *PaymentTracer) TraceSettlement(ctx context.Context, channelID string) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "payment.channel.settle",
		telemetry.WithChannelID(channelID),
	)
}

// TraceDispute instruments dispute resolution
func (t *PaymentTracer) TraceDispute(ctx context.Context, channelID string, disputeType string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.dispute",
		telemetry.WithChannelID(channelID),
	)

	span.SetAttributes(
		attribute.String("dispute.type", disputeType),
	)

	return ctx, span
}

// TraceSignature instruments cryptographic signature operation
func (t *PaymentTracer) TraceSignature(ctx context.Context, operation string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "payment.signature."+operation)

	return ctx, span
}

// TraceVerification instruments signature verification
func (t *PaymentTracer) TraceVerification(ctx context.Context, channelID string) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "payment.verify",
		telemetry.WithChannelID(channelID),
	)
}

// Example: Instrumented channel opening with distributed tracing
func (cm *ChannelManager) OpenChannelWithTracing(ctx context.Context, otherPeer peer.ID, depositA, depositB float64, expiry time.Duration) (*PaymentChannel, error) {
	tracer := NewPaymentTracer()
	ctx, span := tracer.TraceOpenChannel(ctx, cm.localPeer, otherPeer)
	defer span.End()

	start := time.Now()

	// Call original OpenChannel method
	channel, err := cm.OpenChannel(ctx, otherPeer, depositA, depositB, expiry)

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(
			attribute.String("error.type", "channel_open_failed"),
		)
		return nil, err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	// Add channel details
	telemetry.RecordChannelID(span, channel.ChannelID)
	span.SetAttributes(
		attribute.String("channel.id", channel.ChannelID),
		attribute.String("channel.state", string(channel.State)),
		attribute.Float64("channel.deposit_a", depositA),
		attribute.Float64("channel.deposit_b", depositB),
		attribute.Time("channel.expires_at", channel.ExpiresAt),
		attribute.Float64("channel.total_locked", depositA+depositB),
	)

	return channel, nil
}

// Example: Instrumented payment with distributed tracing
func (cm *ChannelManager) SendPaymentWithTracing(ctx context.Context, channelID string, to peer.ID, amount float64, memo string) (*Payment, error) {
	tracer := NewPaymentTracer()

	// Get channel to determine from peer
	cm.mu.RLock()
	channel, exists := cm.channels[channelID]
	cm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	ctx, span := tracer.TracePayment(ctx, channelID, cm.localPeer, to, amount)
	defer span.End()

	start := time.Now()

	// Call original send payment method (hypothetical)
	// payment, err := cm.SendPayment(ctx, channelID, to, amount, memo)

	// Placeholder implementation
	payment := &Payment{
		PaymentID:   generatePaymentID(),
		ChannelID:   channelID,
		From:        cm.localPeer,
		To:          to,
		Amount:      amount,
		SequenceNum: channel.SequenceNum + 1,
		Timestamp:   time.Now(),
		Memo:        memo,
	}

	// Sign payment
	// signature, err := signPayment(cm.privKey, payment)
	var err error = nil // placeholder

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(
			attribute.String("error.type", "payment_failed"),
		)
		return nil, err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	span.SetAttributes(
		attribute.String("payment.id", payment.PaymentID),
		attribute.String("payment.memo", memo),
		attribute.Int64("payment.sequence", int64(payment.SequenceNum)),
		attribute.Float64("payment.amount", amount),
		attribute.String("channel.state", string(channel.State)),
	)

	// Add balance changes
	span.SetAttributes(
		attribute.Float64("balance.before_a", channel.BalanceA),
		attribute.Float64("balance.before_b", channel.BalanceB),
	)

	return payment, nil
}

// Example: Instrumented channel settlement with distributed tracing
func (cm *ChannelManager) SettleChannelWithTracing(ctx context.Context, channelID string) error {
	tracer := NewPaymentTracer()
	ctx, span := tracer.TraceSettlement(ctx, channelID)
	defer span.End()

	start := time.Now()

	// Get channel info
	cm.mu.RLock()
	channel, exists := cm.channels[channelID]
	if exists {
		span.SetAttributes(
			attribute.Float64("settlement.balance_a", channel.BalanceA),
			attribute.Float64("settlement.balance_b", channel.BalanceB),
			attribute.Int64("settlement.sequence_num", int64(channel.SequenceNum)),
			attribute.String("settlement.party_a", channel.PartyA.String()),
			attribute.String("settlement.party_b", channel.PartyB.String()),
		)
	}
	cm.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("channel not found: %s", channelID)
		telemetry.RecordError(span, err)
		return err
	}

	// Perform settlement (placeholder)
	// err := cm.settleOnChain(ctx, channel)
	var err error = nil // placeholder

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(
			attribute.String("error.type", "settlement_failed"),
		)
		return err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	span.SetAttributes(
		attribute.String("settlement.status", "success"),
		attribute.Float64("settlement.total_value", channel.BalanceA+channel.BalanceB),
	)

	return nil
}

// Example: Instrumented dispute resolution with distributed tracing
func (cm *ChannelManager) RaiseDisputeWithTracing(ctx context.Context, channelID string, disputeType string, evidence []byte) error {
	tracer := NewPaymentTracer()
	ctx, span := tracer.TraceDispute(ctx, channelID, disputeType)
	defer span.End()

	start := time.Now()

	// Get channel info
	cm.mu.RLock()
	channel, exists := cm.channels[channelID]
	if exists {
		span.SetAttributes(
			attribute.String("dispute.channel_state", string(channel.State)),
			attribute.Int64("dispute.sequence_num", int64(channel.SequenceNum)),
			attribute.Int("dispute.evidence_size", len(evidence)),
		)
	}
	cm.mu.RUnlock()

	if !exists {
		err := fmt.Errorf("channel not found: %s", channelID)
		telemetry.RecordError(span, err)
		return err
	}

	// Raise dispute (placeholder)
	// err := cm.initiateDispute(ctx, channel, disputeType, evidence)
	var err error = nil // placeholder

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(
			attribute.String("error.type", "dispute_failed"),
		)
		return err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	span.SetAttributes(
		attribute.String("dispute.status", "initiated"),
		attribute.String("dispute.type", disputeType),
	)

	return nil
}

// Example: End-to-end traced payment flow
func ExecutePaymentFlowWithTracing(ctx context.Context, cm *ChannelManager, taskID, guildID string, executorPeer peer.ID, taskCost float64, traceContext string) error {
	// Extract remote trace context from guild coordinator
	ctx = telemetry.ExtractTraceContext(ctx, traceContext)

	tracer := NewPaymentTracer()

	// Top-level payment flow span
	ctx, flowSpan := tracer.helper.StartSpan(ctx, "payment.flow")
	defer flowSpan.End()

	flowSpan.SetAttributes(
		attribute.String("task.id", taskID),
		attribute.String("guild.id", guildID),
		attribute.String("executor.peer", executorPeer.String()),
		attribute.Float64("task.cost", taskCost),
	)

	// Phase 1: Open channel if not exists
	channelID := generateChannelID(cm.localPeer, executorPeer)

	cm.mu.RLock()
	_, exists := cm.channels[channelID]
	cm.mu.RUnlock()

	if !exists {
		channel, err := cm.OpenChannelWithTracing(ctx, executorPeer, 10.0, 5.0, 24*time.Hour)
		if err != nil {
			telemetry.RecordError(flowSpan, err)
			return err
		}
		channelID = channel.ChannelID
	}

	// Phase 2: Send payment for task
	payment, err := cm.SendPaymentWithTracing(ctx, channelID, executorPeer, taskCost, "task:"+taskID)
	if err != nil {
		telemetry.RecordError(flowSpan, err)
		return err
	}

	// Phase 3: Verify payment
	ctx, verifySpan := tracer.TraceVerification(ctx, channelID)
	// err = verifyPaymentSignature(payment)
	err = nil // placeholder
	if err != nil {
		telemetry.RecordError(verifySpan, err)
		verifySpan.End()
		telemetry.RecordError(flowSpan, err)
		return err
	}
	telemetry.RecordSuccess(verifySpan, 0)
	verifySpan.End()

	// Mark flow as successful
	telemetry.RecordSuccess(flowSpan, 0)
	flowSpan.SetAttributes(
		attribute.String("payment.id", payment.PaymentID),
		attribute.String("channel.id", channelID),
		attribute.String("status", "success"),
	)

	return nil
}

// Helper functions (placeholders)
func generatePaymentID() string {
	return fmt.Sprintf("pay-%d", time.Now().UnixNano())
}

func generateChannelID(a, b peer.ID) string {
	return fmt.Sprintf("chan-%s-%s", a.String()[:8], b.String()[:8])
}
