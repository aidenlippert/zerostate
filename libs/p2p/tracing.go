package p2p

import (
	"context"
	"time"

	"github.com/aidenlippert/zerostate/libs/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// P2PTracer provides tracing utilities for P2P operations
type P2PTracer struct {
	helper *telemetry.TraceHelper
}

// NewP2PTracer creates a new P2P tracer
func NewP2PTracer() *P2PTracer {
	return &P2PTracer{
		helper: telemetry.NewTraceHelper("p2p"),
	}
}

// TraceConnection instruments a connection operation
func (t *P2PTracer) TraceConnection(ctx context.Context, peerID string, operation string) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "p2p.connection."+operation,
		telemetry.WithPeerID(peerID),
	)
}

// TraceMessage instruments a message send/receive operation
func (t *P2PTracer) TraceMessage(ctx context.Context, msgType, protocol string, size int) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "p2p.message."+msgType,
		telemetry.WithMessageType(msgType),
	)

	span.SetAttributes(
		attribute.String("protocol", protocol),
		attribute.Int("size_bytes", size),
	)

	return ctx, span
}

// TraceDHTOperation instruments a DHT operation
func (t *P2PTracer) TraceDHTOperation(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "p2p.dht."+operation)

	span.SetAttributes(
		attribute.String("dht.operation", operation),
		attribute.String("dht.key", key),
	)

	return ctx, span
}

// TraceGossip instruments a gossip protocol operation
func (t *P2PTracer) TraceGossip(ctx context.Context, action string, topicID string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "p2p.gossip."+action)

	span.SetAttributes(
		attribute.String("gossip.action", action),
		attribute.String("gossip.topic", topicID),
	)

	return ctx, span
}

// TraceRelay instruments a circuit relay operation
func (t *P2PTracer) TraceRelay(ctx context.Context, direction, srcPeer, dstPeer string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "p2p.relay")

	span.SetAttributes(
		attribute.String("relay.direction", direction),
		attribute.String("relay.src_peer", srcPeer),
		attribute.String("relay.dst_peer", dstPeer),
	)

	return ctx, span
}

// TraceHealthCheck instruments a health check operation
func (t *P2PTracer) TraceHealthCheck(ctx context.Context, peerID string) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "p2p.health_check",
		telemetry.WithPeerID(peerID),
	)
}

// Example: How to instrument a connection operation
func (n *Node) connectWithTracing(ctx context.Context, peerID string) error {
	tracer := NewP2PTracer()
	ctx, span := tracer.TraceConnection(ctx, peerID, "establish")
	defer span.End()

	start := time.Now()

	// Simulate connection logic
	// err := n.actualConnect(ctx, peerID)
	var err error = nil // placeholder

	// Record metrics
	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	return nil
}

// Example: How to instrument message sending with trace propagation
func (n *Node) sendMessageWithTracing(ctx context.Context, peerID, msgType string, payload []byte) error {
	tracer := NewP2PTracer()
	ctx, span := tracer.TraceMessage(ctx, msgType, "zerostate/v1", len(payload))
	defer span.End()

	// Inject trace context into message headers
	traceContext := telemetry.InjectTraceContext(ctx)

	// Create message with trace context
	// message := &Message{
	//     Type:         msgType,
	//     Payload:      payload,
	//     TraceContext: traceContext,
	// }

	// Send message
	// err := n.actualSend(ctx, peerID, message)
	var err error = nil // placeholder

	if err != nil {
		telemetry.RecordError(span, err)
		span.SetAttributes(attribute.String("error.type", "send_failed"))
		return err
	}

	span.SetStatus(codes.Ok, "message sent")
	return nil
}

// Example: How to instrument message receiving with trace propagation
func (n *Node) handleMessageWithTracing(ctx context.Context, peerID, msgType string, payload []byte, traceContext string) error {
	// Extract remote trace context
	ctx = telemetry.ExtractTraceContext(ctx, traceContext)

	tracer := NewP2PTracer()
	ctx, span := tracer.TraceMessage(ctx, msgType, "zerostate/v1", len(payload))
	defer span.End()

	telemetry.RecordPeerID(span, peerID)

	start := time.Now()

	// Process message
	// err := n.processMessage(ctx, payload)
	var err error = nil // placeholder

	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	return nil
}

// Example: How to instrument DHT operations
func (n *Node) dhtPutWithTracing(ctx context.Context, key string, value []byte) error {
	tracer := NewP2PTracer()
	ctx, span := tracer.TraceDHTOperation(ctx, "put", key)
	defer span.End()

	telemetry.RecordSize(span, int64(len(value)))

	start := time.Now()

	// Perform DHT PUT
	// err := n.dht.PutValue(ctx, key, value)
	var err error = nil // placeholder

	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	return nil
}

// Example: How to instrument gossip operations
func (n *Node) publishGossipWithTracing(ctx context.Context, topic string, data []byte) error {
	tracer := NewP2PTracer()
	ctx, span := tracer.TraceGossip(ctx, "publish", topic)
	defer span.End()

	telemetry.RecordSize(span, int64(len(data)))

	start := time.Now()

	// Inject trace context for propagation
	traceContext := telemetry.InjectTraceContext(ctx)

	// Publish with trace context
	// message := &GossipMessage{
	//     Topic:        topic,
	//     Data:         data,
	//     TraceContext: traceContext,
	// }
	// err := n.gossip.Publish(ctx, message)
	var err error = nil // placeholder

	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	span.SetAttributes(attribute.String("gossip.status", "propagated"))
	return nil
}
