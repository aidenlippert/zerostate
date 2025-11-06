// Package telemetry provides OpenTelemetry tracing and metrics.
package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
)

// Config holds telemetry configuration
type Config struct {
	ServiceName    string
	ServiceVersion string
	JaegerEndpoint string // e.g., "http://jaeger:14268/api/traces"
	Enabled        bool
	Logger         *zap.Logger
}

// TracerProvider wraps the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	logger   *zap.Logger
}

// InitTracer initializes OpenTelemetry with Jaeger exporter
func InitTracer(cfg *Config) (*TracerProvider, error) {
	if !cfg.Enabled {
		cfg.Logger.Info("telemetry disabled")
		return &TracerProvider{logger: cfg.Logger}, nil
	}

	// Create Jaeger exporter
	exp, err := jaeger.New(
		jaeger.WithCollectorEndpoint(
			jaeger.WithEndpoint(cfg.JaegerEndpoint),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jaeger exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator for distributed tracing
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	cfg.Logger.Info("telemetry initialized",
		zap.String("service", cfg.ServiceName),
		zap.String("jaeger", cfg.JaegerEndpoint),
	)

	return &TracerProvider{
		provider: tp,
		logger:   cfg.Logger,
	}, nil
}

// Shutdown flushes any pending traces
func (t *TracerProvider) Shutdown(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	if err := t.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown tracer provider: %w", err)
	}

	t.logger.Info("telemetry shutdown complete")
	return nil
}
