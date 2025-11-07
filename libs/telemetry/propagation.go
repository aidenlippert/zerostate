package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const (
	// TraceContextHeader is the header key for trace context in P2P messages
	TraceContextHeader = "zs-trace-context"
)

// InjectTraceContext injects OpenTelemetry trace context into a message header
// Returns the serialized trace context as a base64-encoded string
func InjectTraceContext(ctx context.Context) string {
	propagator := otel.GetTextMapPropagator()

	carrier := propagation.MapCarrier{}
	propagator.Inject(ctx, carrier)

	// Serialize the carrier to a single string
	// In production, this would be a proper serialization format
	// For now, we'll use a simple format: traceparent value
	if traceparent, ok := carrier["traceparent"]; ok {
		return traceparent
	}

	return ""
}

// ExtractTraceContext extracts OpenTelemetry trace context from a message header
// Returns a new context with the extracted span context
func ExtractTraceContext(ctx context.Context, traceContext string) context.Context {
	if traceContext == "" {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()

	// Reconstruct the carrier from the serialized string
	carrier := propagation.MapCarrier{
		"traceparent": traceContext,
	}

	return propagator.Extract(ctx, carrier)
}

// InjectTraceContextMap injects trace context into a map (for JSON/Protobuf messages)
func InjectTraceContextMap(ctx context.Context, headers map[string]string) {
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier(headers)
	propagator.Inject(ctx, carrier)
}

// ExtractTraceContextMap extracts trace context from a map (for JSON/Protobuf messages)
func ExtractTraceContextMap(ctx context.Context, headers map[string]string) context.Context {
	if len(headers) == 0 {
		return ctx
	}

	propagator := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier(headers)
	return propagator.Extract(ctx, carrier)
}

// InjectTraceContextBytes injects trace context into a byte slice for wire protocol
func InjectTraceContextBytes(ctx context.Context) []byte {
	traceContext := InjectTraceContext(ctx)
	if traceContext == "" {
		return nil
	}
	return []byte(traceContext)
}

// ExtractTraceContextBytes extracts trace context from a byte slice
func ExtractTraceContextBytes(ctx context.Context, data []byte) context.Context {
	if len(data) == 0 {
		return ctx
	}
	traceContext := string(data)
	return ExtractTraceContext(ctx, traceContext)
}

// SerializeCarrier serializes a trace context carrier to base64
func SerializeCarrier(carrier propagation.MapCarrier) string {
	// Simple serialization: just take traceparent
	if traceparent, ok := carrier["traceparent"]; ok {
		return traceparent
	}
	return ""
}

// DeserializeCarrier deserializes a base64 trace context to a carrier
func DeserializeCarrier(encoded string) propagation.MapCarrier {
	if encoded == "" {
		return propagation.MapCarrier{}
	}

	return propagation.MapCarrier{
		"traceparent": encoded,
	}
}

// WithRemoteSpan creates a child span from a remote parent context
// This is useful when receiving a message from another node
func WithRemoteSpan(ctx context.Context, tracer, operationName string, traceContext string) (context.Context, func()) {
	// Extract remote context
	ctx = ExtractTraceContext(ctx, traceContext)

	// Start a new span as a child of the remote span
	tr := otel.Tracer(tracer)
	ctx, span := tr.Start(ctx, operationName)

	return ctx, func() { span.End() }
}
