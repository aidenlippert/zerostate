// Package telemetry provides OpenTelemetry tracing and metrics.
package telemetry

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// Config holds telemetry configuration
type Config struct {
	ServiceName     string
	ServiceVersion  string
	OTLPEndpoint    string // e.g., "otel-collector:4318" (HTTP endpoint, no http://)
	JaegerUI        string // e.g., "http://localhost:16686" for reference
	Enabled         bool
	SamplingRate    float64 // 0.0 to 1.0 (1.0 = sample everything)
	Logger          *zap.Logger
}

// TracerProvider wraps the OpenTelemetry tracer provider
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	logger   *zap.Logger
}

// InitTracer initializes OpenTelemetry with OTLP HTTP exporter
func InitTracer(cfg *Config) (*TracerProvider, error) {
	if !cfg.Enabled {
		cfg.Logger.Info("telemetry disabled")
		return &TracerProvider{logger: cfg.Logger}, nil
	}

	// Default sampling rate
	if cfg.SamplingRate == 0 {
		cfg.SamplingRate = 1.0 // Sample everything by default
	}

	// Create OTLP HTTP exporter
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		otlptracehttp.WithInsecure(), // Use insecure for local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler based on sampling rate
	var sampler sdktrace.Sampler
	if cfg.SamplingRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SamplingRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SamplingRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
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
		zap.String("version", cfg.ServiceVersion),
		zap.String("otlp_endpoint", cfg.OTLPEndpoint),
		zap.Float64("sampling_rate", cfg.SamplingRate),
	)

	if cfg.JaegerUI != "" {
		cfg.Logger.Info("traces viewable at Jaeger UI", zap.String("url", cfg.JaegerUI))
	}

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

// ForceFlush forces a flush of pending traces
func (t *TracerProvider) ForceFlush(ctx context.Context) error {
	if t.provider == nil {
		return nil
	}

	if err := t.provider.ForceFlush(ctx); err != nil {
		return fmt.Errorf("failed to flush traces: %w", err)
	}

	return nil
}
