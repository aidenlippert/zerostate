// Package guild implements ephemeral private guilds for collaborative task execution
package guild

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"golang.org/x/crypto/curve25519"
)

const (
	// ProtocolIDPrefix is the protocol prefix for guild communication
	ProtocolIDPrefix = "/zerostate/guild/1.0.0"
	// DefaultMembershipTTL is the default time before guild membership expires
	DefaultMembershipTTL = 1 * time.Hour
	// DefaultMaxMembers is the default maximum guild size
	DefaultMaxMembers = 50
	// DefaultHeartbeatInterval is how often members send heartbeats
	DefaultHeartbeatInterval = 30 * time.Second
)

var (
	// Metrics
	guildCreationsTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "guild_creations_total",
			Help: "Total number of guilds created",
		},
	)

	guildMembersGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "guild_members",
			Help: "Current number of members per guild",
		},
		[]string{"guild_id"},
	)

	guildLifetimeSeconds = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "guild_lifetime_seconds",
			Help:    "Guild lifetime from creation to dissolution",
			Buckets: prometheus.ExponentialBuckets(60, 2, 10), // 1min to ~17hrs
		},
		[]string{"reason"},
	)

	guildJoinLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "guild_join_latency_seconds",
			Help:    "Latency to join a guild",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
		},
	)

	guildMessagesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "guild_messages_total",
			Help: "Total guild messages sent",
		},
		[]string{"guild_id", "type"},
	)
)

var (
	// ErrGuildNotFound is returned when a guild doesn't exist
	ErrGuildNotFound = errors.New("guild not found")
	// ErrNotMember is returned when peer is not a guild member
	ErrNotMember = errors.New("not a guild member")
	// ErrGuildFull is returned when guild has reached max members
	ErrGuildFull = errors.New("guild is full")
	// ErrInvalidSignature is returned when signature verification fails
	ErrInvalidSignature = errors.New("invalid signature")
	// ErrGuildClosed is returned when operating on closed guild
	ErrGuildClosed = errors.New("guild is closed")
)

// GuildID uniquely identifies a guild
type GuildID string

// Role represents a member's role in the guild
type Role string

const (
	// RoleCreator is the guild creator (admin privileges)
	RoleCreator Role = "creator"
	// RoleMember is a regular guild member
	RoleMember Role = "member"
	// RoleExecutor can execute tasks
	RoleExecutor Role = "executor"
	// RoleObserver can only observe (read-only)
	RoleObserver Role = "observer"
)

// Member represents a guild member
type Member struct {
	PeerID       peer.ID
	Role         Role
	JoinedAt     time.Time
	LastSeen     time.Time
	PublicKey    []byte // X25519 public key for encrypted messaging
	Capabilities []string
}

// Guild represents an ephemeral private group for task execution
type Guild struct {
	ID           GuildID
	Creator      peer.ID
	CreatedAt    time.Time
	ExpiresAt    time.Time
	MaxMembers   int
	Topic        string // PubSub topic for guild messages
	
	members      map[peer.ID]*Member
	sharedSecret []byte // Derived shared secret for encryption
	privateKey   []byte // X25519 private key
	publicKey    []byte // X25519 public key
	
	mu           sync.RWMutex
	closed       bool
	logger       *zap.Logger
}

// GuildConfig holds guild configuration
type GuildConfig struct {
	MaxMembers       int
	MembershipTTL    time.Duration
	HeartbeatInterval time.Duration
	EnableEncryption  bool
	RequiredCapabilities []string
}

// DefaultGuildConfig returns default guild configuration
func DefaultGuildConfig() *GuildConfig {
	return &GuildConfig{
		MaxMembers:       DefaultMaxMembers,
		MembershipTTL:    DefaultMembershipTTL,
		HeartbeatInterval: DefaultHeartbeatInterval,
		EnableEncryption:  true,
		RequiredCapabilities: []string{},
	}
}

// GuildManager manages multiple guilds
type GuildManager struct {
	host      host.Host
	guilds    map[GuildID]*Guild
	config    *GuildConfig
	logger    *zap.Logger
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewGuildManager creates a new guild manager
func NewGuildManager(ctx context.Context, h host.Host, config *GuildConfig, logger *zap.Logger) *GuildManager {
	if config == nil {
		config = DefaultGuildConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	gmCtx, cancel := context.WithCancel(ctx)

	gm := &GuildManager{
		host:   h,
		guilds: make(map[GuildID]*Guild),
		config: config,
		logger: logger,
		ctx:    gmCtx,
		cancel: cancel,
	}

	// Start background cleanup
	gm.wg.Add(1)
	go gm.cleanupLoop()

	logger.Info("guild manager started",
		zap.Int("max_members", config.MaxMembers),
		zap.Duration("membership_ttl", config.MembershipTTL),
		zap.Bool("encryption", config.EnableEncryption),
	)

	return gm
}

// CreateGuild creates a new ephemeral guild
func (gm *GuildManager) CreateGuild(ctx context.Context, capabilities []string) (*Guild, error) {
	start := time.Now()

	// Generate guild ID
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return nil, fmt.Errorf("failed to generate guild ID: %w", err)
	}
	guildID := GuildID(fmt.Sprintf("guild-%x", idBytes))

	// Generate X25519 keypair for encryption
	var privateKey, publicKey []byte
	if gm.config.EnableEncryption {
		privateKey = make([]byte, curve25519.ScalarSize)
		if _, err := rand.Read(privateKey); err != nil {
			return nil, fmt.Errorf("failed to generate private key: %w", err)
		}
		publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
		if err != nil {
			return nil, fmt.Errorf("failed to generate public key: %w", err)
		}
		privateKey = privateKey
		publicKey = publicKey
	}

	now := time.Now()
	guild := &Guild{
		ID:         guildID,
		Creator:    gm.host.ID(),
		CreatedAt:  now,
		ExpiresAt:  now.Add(gm.config.MembershipTTL),
		MaxMembers: gm.config.MaxMembers,
		Topic:      string(guildID),
		members:    make(map[peer.ID]*Member),
		privateKey: privateKey,
		publicKey:  publicKey,
		logger:     gm.logger.With(zap.String("guild_id", string(guildID))),
	}

	// Add creator as first member
	guild.members[gm.host.ID()] = &Member{
		PeerID:       gm.host.ID(),
		Role:         RoleCreator,
		JoinedAt:     now,
		LastSeen:     now,
		PublicKey:    publicKey,
		Capabilities: capabilities,
	}

	gm.mu.Lock()
	gm.guilds[guildID] = guild
	gm.mu.Unlock()

	guildCreationsTotal.Inc()
	guildMembersGauge.WithLabelValues(string(guildID)).Set(1)

	guild.logger.Info("guild created",
		zap.String("creator", gm.host.ID().String()),
		zap.Time("expires_at", guild.ExpiresAt),
		zap.Int("max_members", guild.MaxMembers),
		zap.Duration("creation_latency", time.Since(start)),
	)

	return guild, nil
}

// JoinGuild allows a peer to join an existing guild
func (gm *GuildManager) JoinGuild(ctx context.Context, guildID GuildID, capabilities []string) error {
	start := time.Now()

	gm.mu.RLock()
	guild, exists := gm.guilds[guildID]
	gm.mu.RUnlock()

	if !exists {
		return ErrGuildNotFound
	}

	guild.mu.Lock()
	defer guild.mu.Unlock()

	if guild.closed {
		return ErrGuildClosed
	}

	// Check if already a member
	if _, exists := guild.members[gm.host.ID()]; exists {
		return nil // Already joined
	}

	// Check capacity
	if len(guild.members) >= guild.MaxMembers {
		return ErrGuildFull
	}

	// Generate member's public key
	var publicKey []byte
	if gm.config.EnableEncryption {
		privateKey := make([]byte, curve25519.ScalarSize)
		if _, err := rand.Read(privateKey); err != nil {
			return fmt.Errorf("failed to generate member key: %w", err)
		}
		var err error
		publicKey, err = curve25519.X25519(privateKey, curve25519.Basepoint)
		if err != nil {
			return fmt.Errorf("failed to generate public key: %w", err)
		}
	}

	now := time.Now()
	member := &Member{
		PeerID:       gm.host.ID(),
		Role:         RoleMember,
		JoinedAt:     now,
		LastSeen:     now,
		PublicKey:    publicKey,
		Capabilities: capabilities,
	}

	guild.members[gm.host.ID()] = member
	guildMembersGauge.WithLabelValues(string(guildID)).Set(float64(len(guild.members)))

	guildJoinLatency.Observe(time.Since(start).Seconds())

	guild.logger.Info("member joined",
		zap.String("peer_id", gm.host.ID().String()),
		zap.Int("total_members", len(guild.members)),
		zap.Duration("join_latency", time.Since(start)),
	)

	return nil
}

// LeaveGuild removes a peer from a guild
func (gm *GuildManager) LeaveGuild(ctx context.Context, guildID GuildID) error {
	gm.mu.RLock()
	guild, exists := gm.guilds[guildID]
	gm.mu.RUnlock()

	if !exists {
		return ErrGuildNotFound
	}

	guild.mu.Lock()
	defer guild.mu.Unlock()

	if _, exists := guild.members[gm.host.ID()]; !exists {
		return ErrNotMember
	}

	delete(guild.members, gm.host.ID())
	guildMembersGauge.WithLabelValues(string(guildID)).Set(float64(len(guild.members)))

	guild.logger.Info("member left",
		zap.String("peer_id", gm.host.ID().String()),
		zap.Int("remaining_members", len(guild.members)),
	)

	// Auto-dissolve if no members left
	if len(guild.members) == 0 {
		guild.closed = true
		gm.mu.Lock()
		delete(gm.guilds, guildID)
		gm.mu.Unlock()

		lifetime := time.Since(guild.CreatedAt).Seconds()
		guildLifetimeSeconds.WithLabelValues("empty").Observe(lifetime)

		guild.logger.Info("guild auto-dissolved (no members)")
	}

	return nil
}

// DissolveGuild forcibly dissolves a guild (creator only)
func (gm *GuildManager) DissolveGuild(ctx context.Context, guildID GuildID) error {
	gm.mu.Lock()
	guild, exists := gm.guilds[guildID]
	if !exists {
		gm.mu.Unlock()
		return ErrGuildNotFound
	}
	delete(gm.guilds, guildID)
	gm.mu.Unlock()

	guild.mu.Lock()
	defer guild.mu.Unlock()

	// Check if caller is creator
	if guild.Creator != gm.host.ID() {
		return fmt.Errorf("only creator can dissolve guild")
	}

	guild.closed = true

	// Clear members and keys
	guild.members = make(map[peer.ID]*Member)
	if guild.privateKey != nil {
		for i := range guild.privateKey {
			guild.privateKey[i] = 0
		}
	}
	if guild.sharedSecret != nil {
		for i := range guild.sharedSecret {
			guild.sharedSecret[i] = 0
		}
	}

	guildMembersGauge.WithLabelValues(string(guildID)).Set(0)
	lifetime := time.Since(guild.CreatedAt).Seconds()
	guildLifetimeSeconds.WithLabelValues("dissolved").Observe(lifetime)

	guild.logger.Info("guild dissolved",
		zap.Duration("lifetime", time.Since(guild.CreatedAt)),
	)

	return nil
}

// GetGuild retrieves a guild by ID
func (gm *GuildManager) GetGuild(guildID GuildID) (*Guild, error) {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	guild, exists := gm.guilds[guildID]
	if !exists {
		return nil, ErrGuildNotFound
	}

	return guild, nil
}

// ListGuilds returns all active guilds
func (gm *GuildManager) ListGuilds() []*Guild {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	guilds := make([]*Guild, 0, len(gm.guilds))
	for _, guild := range gm.guilds {
		guilds = append(guilds, guild)
	}

	return guilds
}

// GetMembers returns all members of a guild
func (g *Guild) GetMembers() []*Member {
	g.mu.RLock()
	defer g.mu.RUnlock()

	members := make([]*Member, 0, len(g.members))
	for _, member := range g.members {
		members = append(members, member)
	}

	return members
}

// IsMember checks if a peer is a guild member
func (g *Guild) IsMember(peerID peer.ID) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, exists := g.members[peerID]
	return exists
}

// GetMember retrieves a specific member
func (g *Guild) GetMember(peerID peer.ID) (*Member, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	member, exists := g.members[peerID]
	if !exists {
		return nil, ErrNotMember
	}

	return member, nil
}

// UpdateHeartbeat updates member's last seen time
func (g *Guild) UpdateHeartbeat(peerID peer.ID) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	member, exists := g.members[peerID]
	if !exists {
		return ErrNotMember
	}

	member.LastSeen = time.Now()
	return nil
}

// cleanupLoop removes expired guilds
func (gm *GuildManager) cleanupLoop() {
	defer gm.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-gm.ctx.Done():
			return
		case <-ticker.C:
			gm.cleanup()
		}
	}
}

// cleanup removes expired guilds and inactive members
func (gm *GuildManager) cleanup() {
	now := time.Now()

	gm.mu.Lock()
	defer gm.mu.Unlock()

	for guildID, guild := range gm.guilds {
		guild.mu.Lock()

		// Remove expired guilds
		if now.After(guild.ExpiresAt) {
			guild.closed = true
			delete(gm.guilds, guildID)
			
			lifetime := time.Since(guild.CreatedAt).Seconds()
			guildLifetimeSeconds.WithLabelValues("expired").Observe(lifetime)
			
			guild.logger.Info("guild expired and removed")
			guild.mu.Unlock()
			continue
		}

		// Remove inactive members (no heartbeat for 2x heartbeat interval)
		timeout := gm.config.HeartbeatInterval * 2
		for peerID, member := range guild.members {
			if now.Sub(member.LastSeen) > timeout {
				delete(guild.members, peerID)
				guild.logger.Info("removed inactive member",
					zap.String("peer_id", peerID.String()),
					zap.Duration("inactive_for", now.Sub(member.LastSeen)),
				)
			}
		}

		guildMembersGauge.WithLabelValues(string(guildID)).Set(float64(len(guild.members)))

		guild.mu.Unlock()
	}
}

// Stats returns guild manager statistics
type GuildStats struct {
	TotalGuilds   int
	TotalMembers  int
	AverageSize   float64
	OldestGuild   time.Duration
}

// Stats returns current guild statistics
func (gm *GuildManager) Stats() GuildStats {
	gm.mu.RLock()
	defer gm.mu.RUnlock()

	stats := GuildStats{
		TotalGuilds: len(gm.guilds),
	}

	if stats.TotalGuilds == 0 {
		return stats
	}

	now := time.Now()
	var oldestAge time.Duration

	for _, guild := range gm.guilds {
		guild.mu.RLock()
		memberCount := len(guild.members)
		age := now.Sub(guild.CreatedAt)
		guild.mu.RUnlock()

		stats.TotalMembers += memberCount
		if age > oldestAge {
			oldestAge = age
		}
	}

	stats.AverageSize = float64(stats.TotalMembers) / float64(stats.TotalGuilds)
	stats.OldestGuild = oldestAge

	return stats
}

// Close stops the guild manager and dissolves all guilds
func (gm *GuildManager) Close() error {
	gm.logger.Info("closing guild manager")

	gm.cancel()
	gm.wg.Wait()

	gm.mu.Lock()
	defer gm.mu.Unlock()

	// Dissolve all guilds
	for guildID, guild := range gm.guilds {
		guild.mu.Lock()
		guild.closed = true
		
		// Clear sensitive data
		if guild.privateKey != nil {
			for i := range guild.privateKey {
				guild.privateKey[i] = 0
			}
		}
		if guild.sharedSecret != nil {
			for i := range guild.sharedSecret {
				guild.sharedSecret[i] = 0
			}
		}
		
		guild.mu.Unlock()
		delete(gm.guilds, guildID)
	}

	gm.logger.Info("guild manager closed")
	return nil
}
