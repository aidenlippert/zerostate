package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Common attribute keys for consistent tracing across ZeroState
const (
	AttrPeerID       = "peer.id"
	AttrGuildID      = "guild.id"
	AttrTaskID       = "task.id"
	AttrChannelID    = "channel.id"
	AttrMessageType  = "message.type"
	AttrStatus       = "status"
	AttrErrorMsg     = "error.message"
	AttrDuration     = "duration.ms"
	AttrSize         = "size.bytes"
	AttrCount        = "count"
)

// TraceHelper provides convenient tracing utilities
type TraceHelper struct {
	tracer trace.Tracer
}

// NewTraceHelper creates a new trace helper for a component
func NewTraceHelper(componentName string) *TraceHelper {
	return &TraceHelper{
		tracer: otel.Tracer("zerostate/" + componentName),
	}
}

// StartSpan starts a new span with common setup
func (h *TraceHelper) StartSpan(ctx context.Context, operationName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return h.tracer.Start(ctx, operationName, opts...)
}

// RecordSuccess marks a span as successful and adds duration
func RecordSuccess(span trace.Span, durationMS int64) {
	span.SetStatus(codes.Ok, "success")
	span.SetAttributes(attribute.Int64(AttrDuration, durationMS))
}

// RecordError marks a span as failed and records the error
func RecordError(span trace.Span, err error) {
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String(AttrErrorMsg, err.Error()))
	span.RecordError(err)
}

// RecordPeerID adds peer ID to span
func RecordPeerID(span trace.Span, peerID string) {
	span.SetAttributes(attribute.String(AttrPeerID, peerID))
}

// RecordGuildID adds guild ID to span
func RecordGuildID(span trace.Span, guildID string) {
	span.SetAttributes(attribute.String(AttrGuildID, guildID))
}

// RecordTaskID adds task ID to span
func RecordTaskID(span trace.Span, taskID string) {
	span.SetAttributes(attribute.String(AttrTaskID, taskID))
}

// RecordChannelID adds channel ID to span
func RecordChannelID(span trace.Span, channelID string) {
	span.SetAttributes(attribute.String(AttrChannelID, channelID))
}

// RecordMessageType adds message type to span
func RecordMessageType(span trace.Span, msgType string) {
	span.SetAttributes(attribute.String(AttrMessageType, msgType))
}

// RecordSize adds size attribute to span
func RecordSize(span trace.Span, sizeBytes int64) {
	span.SetAttributes(attribute.Int64(AttrSize, sizeBytes))
}

// RecordCount adds count attribute to span
func RecordCount(span trace.Span, count int) {
	span.SetAttributes(attribute.Int(AttrCount, count))
}

// WithPeerID returns a SpanStartOption that adds peer ID
func WithPeerID(peerID string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(AttrPeerID, peerID))
}

// WithGuildID returns a SpanStartOption that adds guild ID
func WithGuildID(guildID string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(AttrGuildID, guildID))
}

// WithTaskID returns a SpanStartOption that adds task ID
func WithTaskID(taskID string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(AttrTaskID, taskID))
}

// WithChannelID returns a SpanStartOption that adds channel ID
func WithChannelID(channelID string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(AttrChannelID, channelID))
}

// WithMessageType returns a SpanStartOption that adds message type
func WithMessageType(msgType string) trace.SpanStartOption {
	return trace.WithAttributes(attribute.String(AttrMessageType, msgType))
}
