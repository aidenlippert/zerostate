package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogConfig holds logging configuration
type LogConfig struct {
	// Level is the minimum log level (debug, info, warn, error)
	Level string
	// Format is the log format (json, console)
	Format string
	// OutputPaths is the list of output paths (stdout, stderr, file paths)
	OutputPaths []string
	// ErrorOutputPaths is the list of error output paths
	ErrorOutputPaths []string
	// EnableCaller adds caller information (file:line)
	EnableCaller bool
	// EnableStacktrace adds stack traces for errors
	EnableStacktrace bool
	// ServiceName for structured field
	ServiceName string
	// ServiceVersion for structured field
	ServiceVersion string
	// Environment (dev, staging, prod)
	Environment string
}

// DefaultLogConfig returns default logging configuration
func DefaultLogConfig(serviceName string) *LogConfig {
	return &LogConfig{
		Level:            "info",
		Format:           "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EnableCaller:     true,
		EnableStacktrace: true,
		ServiceName:      serviceName,
		ServiceVersion:   "0.1.0",
		Environment:      "development",
	}
}

// NewLogger creates a new structured logger with service context
func NewLogger(cfg *LogConfig) (*zap.Logger, error) {
	if cfg == nil {
		cfg = DefaultLogConfig("zerostate")
	}

	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Configure encoder
	var encoderConfig zapcore.EncoderConfig
	if cfg.Format == "console" {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	} else {
		encoderConfig = zap.NewProductionEncoderConfig()
		encoderConfig.TimeKey = "timestamp"
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	}

	// Build config
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       cfg.Environment == "development",
		DisableCaller:     !cfg.EnableCaller,
		DisableStacktrace: !cfg.EnableStacktrace,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         cfg.Format,
		EncoderConfig:    encoderConfig,
		OutputPaths:      cfg.OutputPaths,
		ErrorOutputPaths: cfg.ErrorOutputPaths,
		InitialFields: map[string]interface{}{
			"service":     cfg.ServiceName,
			"version":     cfg.ServiceVersion,
			"environment": cfg.Environment,
		},
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger, nil
}

// WithTraceContext adds trace context fields to logger
// This correlates logs with distributed traces in Jaeger
func WithTraceContext(ctx context.Context, logger *zap.Logger) *zap.Logger {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return logger
	}

	spanCtx := span.SpanContext()
	return logger.With(
		zap.String("trace_id", spanCtx.TraceID().String()),
		zap.String("span_id", spanCtx.SpanID().String()),
		zap.Bool("trace_sampled", spanCtx.IsSampled()),
	)
}

// StructuredLogger provides context-aware structured logging
type StructuredLogger struct {
	base   *zap.Logger
	fields []zap.Field
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger(base *zap.Logger) *StructuredLogger {
	return &StructuredLogger{
		base:   base,
		fields: []zap.Field{},
	}
}

// WithContext returns a logger with trace context fields
func (l *StructuredLogger) WithContext(ctx context.Context) *zap.Logger {
	return WithTraceContext(ctx, l.base.With(l.fields...))
}

// WithFields returns a new logger with additional fields
func (l *StructuredLogger) WithFields(fields ...zap.Field) *StructuredLogger {
	return &StructuredLogger{
		base:   l.base,
		fields: append(l.fields, fields...),
	}
}

// WithPeerID adds peer ID field
func (l *StructuredLogger) WithPeerID(peerID string) *StructuredLogger {
	return l.WithFields(zap.String("peer_id", peerID))
}

// WithGuildID adds guild ID field
func (l *StructuredLogger) WithGuildID(guildID string) *StructuredLogger {
	return l.WithFields(zap.String("guild_id", guildID))
}

// WithTaskID adds task ID field
func (l *StructuredLogger) WithTaskID(taskID string) *StructuredLogger {
	return l.WithFields(zap.String("task_id", taskID))
}

// WithChannelID adds channel ID field
func (l *StructuredLogger) WithChannelID(channelID string) *StructuredLogger {
	return l.WithFields(zap.String("channel_id", channelID))
}

// WithError adds error field
func (l *StructuredLogger) WithError(err error) *StructuredLogger {
	return l.WithFields(zap.Error(err))
}

// WithDuration adds duration field
func (l *StructuredLogger) WithDuration(key string, duration int64) *StructuredLogger {
	return l.WithFields(zap.Int64(key, duration))
}

// Common logging helpers for structured fields
var (
	// PeerID creates a peer_id field
	PeerID = func(id string) zap.Field { return zap.String("peer_id", id) }

	// GuildID creates a guild_id field
	GuildID = func(id string) zap.Field { return zap.String("guild_id", id) }

	// TaskID creates a task_id field
	TaskID = func(id string) zap.Field { return zap.String("task_id", id) }

	// ChannelID creates a channel_id field
	ChannelID = func(id string) zap.Field { return zap.String("channel_id", id) }

	// MessageType creates a message_type field
	MessageType = func(msgType string) zap.Field { return zap.String("message_type", msgType) }

	// Operation creates an operation field
	Operation = func(op string) zap.Field { return zap.String("operation", op) }

	// Status creates a status field
	Status = func(status string) zap.Field { return zap.String("status", status) }

	// DurationMS creates a duration_ms field
	DurationMS = func(ms int64) zap.Field { return zap.Int64("duration_ms", ms) }

	// SizeBytes creates a size_bytes field
	SizeBytes = func(bytes int64) zap.Field { return zap.Int64("size_bytes", bytes) }

	// Count creates a count field
	Count = func(count int) zap.Field { return zap.Int("count", count) }
)

// Example usage patterns

// LogWithTrace logs with trace context automatically included
func LogWithTrace(ctx context.Context, logger *zap.Logger, level zapcore.Level, msg string, fields ...zap.Field) {
	logger = WithTraceContext(ctx, logger)

	switch level {
	case zapcore.DebugLevel:
		logger.Debug(msg, fields...)
	case zapcore.InfoLevel:
		logger.Info(msg, fields...)
	case zapcore.WarnLevel:
		logger.Warn(msg, fields...)
	case zapcore.ErrorLevel:
		logger.Error(msg, fields...)
	}
}

// DebugCtx logs debug message with trace context
func DebugCtx(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithTraceContext(ctx, logger).Debug(msg, fields...)
}

// InfoCtx logs info message with trace context
func InfoCtx(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithTraceContext(ctx, logger).Info(msg, fields...)
}

// WarnCtx logs warning message with trace context
func WarnCtx(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithTraceContext(ctx, logger).Warn(msg, fields...)
}

// ErrorCtx logs error message with trace context
func ErrorCtx(ctx context.Context, logger *zap.Logger, msg string, fields ...zap.Field) {
	WithTraceContext(ctx, logger).Error(msg, fields...)
}
