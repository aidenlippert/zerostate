package guild

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/aidenlippert/zerostate/libs/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// GuildTracer provides tracing utilities for guild operations
type GuildTracer struct {
	helper *telemetry.TraceHelper
}

// NewGuildTracer creates a new guild tracer
func NewGuildTracer() *GuildTracer {
	return &GuildTracer{
		helper: telemetry.NewTraceHelper("guild"),
	}
}

// TraceCreateGuild instruments guild creation
func (t *GuildTracer) TraceCreateGuild(ctx context.Context) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "guild.create")
}

// TraceJoinGuild instruments joining a guild
func (t *GuildTracer) TraceJoinGuild(ctx context.Context, guildID GuildID) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "guild.join",
		telemetry.WithGuildID(string(guildID)),
	)
}

// TraceLeaveGuild instruments leaving a guild
func (t *GuildTracer) TraceLeaveGuild(ctx context.Context, guildID GuildID, reason string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.leave",
		telemetry.WithGuildID(string(guildID)),
	)

	span.SetAttributes(
		attribute.String("leave.reason", reason),
	)

	return ctx, span
}

// TraceDissolveGuild instruments guild dissolution
func (t *GuildTracer) TraceDissolveGuild(ctx context.Context, guildID GuildID, reason string) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.dissolve",
		telemetry.WithGuildID(string(guildID)),
	)

	span.SetAttributes(
		attribute.String("dissolve.reason", reason),
	)

	return ctx, span
}

// TraceSendMessage instruments sending a guild message
func (t *GuildTracer) TraceSendMessage(ctx context.Context, guildID GuildID, msgType string, size int) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.send_message",
		telemetry.WithGuildID(string(guildID)),
		telemetry.WithMessageType(msgType),
	)

	telemetry.RecordSize(span, int64(size))

	return ctx, span
}

// TraceReceiveMessage instruments receiving a guild message
func (t *GuildTracer) TraceReceiveMessage(ctx context.Context, guildID GuildID, msgType string, fromPeer peer.ID) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.receive_message",
		telemetry.WithGuildID(string(guildID)),
		telemetry.WithMessageType(msgType),
	)

	telemetry.RecordPeerID(span, fromPeer.String())

	return ctx, span
}

// TraceHeartbeat instruments heartbeat operations
func (t *GuildTracer) TraceHeartbeat(ctx context.Context, guildID GuildID) (context.Context, trace.Span) {
	return t.helper.StartSpan(ctx, "guild.heartbeat",
		telemetry.WithGuildID(string(guildID)),
	)
}

// TraceKeyExchange instruments key exchange for encryption
func (t *GuildTracer) TraceKeyExchange(ctx context.Context, guildID GuildID, peerID peer.ID) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.key_exchange",
		telemetry.WithGuildID(string(guildID)),
	)

	telemetry.RecordPeerID(span, peerID.String())

	return ctx, span
}

// TraceMembershipUpdate instruments membership changes
func (t *GuildTracer) TraceMembershipUpdate(ctx context.Context, guildID GuildID, action string, memberCount int) (context.Context, trace.Span) {
	ctx, span := t.helper.StartSpan(ctx, "guild.membership."+action,
		telemetry.WithGuildID(string(guildID)),
	)

	span.SetAttributes(
		attribute.String("membership.action", action),
		attribute.Int("membership.count", memberCount),
	)

	return ctx, span
}

// Example: Instrumented CreateGuild with distributed tracing
func (gm *GuildManager) CreateGuildWithTracing(ctx context.Context, capabilities []string) (*Guild, error) {
	tracer := NewGuildTracer()
	ctx, span := tracer.TraceCreateGuild(ctx)
	defer span.End()

	start := time.Now()

	// Call original CreateGuild method
	guild, err := gm.CreateGuild(ctx, capabilities)

	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return nil, err
	}

	// Add guild-specific attributes
	telemetry.RecordGuildID(span, string(guild.ID))
	span.SetAttributes(
		attribute.String("guild.creator", guild.Creator.String()),
		attribute.Int("guild.max_members", guild.MaxMembers),
		attribute.Time("guild.expires_at", guild.ExpiresAt),
		attribute.Bool("guild.encryption_enabled", gm.config.EnableEncryption),
		attribute.StringSlice("guild.capabilities", capabilities),
	)

	return guild, nil
}

// Example: Instrumented JoinGuild with distributed tracing
func (gm *GuildManager) JoinGuildWithTracing(ctx context.Context, guildID GuildID, capabilities []string) error {
	tracer := NewGuildTracer()
	ctx, span := tracer.TraceJoinGuild(ctx, guildID)
	defer span.End()

	start := time.Now()

	// Call original JoinGuild method
	err := gm.JoinGuild(ctx, guildID, capabilities)

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)

		// Add error-specific attributes
		span.SetAttributes(
			attribute.String("error.type", errorType(err)),
		)

		return err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	// Add join-specific attributes
	span.SetAttributes(
		attribute.StringSlice("member.capabilities", capabilities),
		attribute.String("member.peer_id", gm.host.ID().String()),
	)

	// Get guild to add member count
	gm.mu.RLock()
	if guild, exists := gm.guilds[guildID]; exists {
		guild.mu.RLock()
		telemetry.RecordCount(span, len(guild.members))
		guild.mu.RUnlock()
	}
	gm.mu.RUnlock()

	return nil
}

// Example: Instrumented guild message with trace propagation
func (g *Guild) SendMessageWithTracing(ctx context.Context, msgType string, payload []byte) error {
	tracer := NewGuildTracer()
	ctx, span := tracer.TraceSendMessage(ctx, g.ID, msgType, len(payload))
	defer span.End()

	// Inject trace context for propagation to other guild members
	traceContext := telemetry.InjectTraceContext(ctx)

	start := time.Now()

	// Create message with trace context
	// message := &GuildMessage{
	//     Type:         msgType,
	//     Payload:      payload,
	//     TraceContext: traceContext,
	//     Timestamp:    time.Now(),
	// }

	// Send to all guild members
	// err := g.broadcast(ctx, message)
	var err error = nil // placeholder

	duration := time.Since(start)
	telemetry.RecordSuccess(span, duration.Milliseconds())

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	// Record broadcast statistics
	g.mu.RLock()
	recipientCount := len(g.members) - 1 // Exclude sender
	g.mu.RUnlock()

	span.SetAttributes(
		attribute.Int("message.recipients", recipientCount),
		attribute.String("message.type", msgType),
	)

	return nil
}

// Example: Instrumented guild message handling with trace extraction
func (g *Guild) HandleMessageWithTracing(ctx context.Context, msgType string, payload []byte, fromPeer peer.ID, traceContext string) error {
	// Extract remote trace context
	ctx = telemetry.ExtractTraceContext(ctx, traceContext)

	tracer := NewGuildTracer()
	ctx, span := tracer.TraceReceiveMessage(ctx, g.ID, msgType, fromPeer)
	defer span.End()

	start := time.Now()

	// Process message
	// err := g.processMessage(ctx, msgType, payload, fromPeer)
	var err error = nil // placeholder

	duration := time.Since(start)

	if err != nil {
		telemetry.RecordError(span, err)
		return err
	}

	telemetry.RecordSuccess(span, duration.Milliseconds())

	span.SetAttributes(
		attribute.String("message.from_peer", fromPeer.String()),
		attribute.Int("message.size_bytes", len(payload)),
	)

	return nil
}

// errorType maps guild errors to trace-friendly error types
func errorType(err error) string {
	switch err {
	case ErrGuildNotFound:
		return "guild_not_found"
	case ErrNotMember:
		return "not_member"
	case ErrGuildFull:
		return "guild_full"
	case ErrInvalidSignature:
		return "invalid_signature"
	case ErrGuildClosed:
		return "guild_closed"
	default:
		return "unknown"
	}
}
