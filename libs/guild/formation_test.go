package guild

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func createTestHost(t *testing.T) host.Host {
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
	)
	require.NoError(t, err)
	return h
}

func TestNewGuildManager(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	config := DefaultGuildConfig()

	gm := NewGuildManager(ctx, h, config, logger)
	defer gm.Close()

	assert.NotNil(t, gm)
	assert.Equal(t, config.MaxMembers, gm.config.MaxMembers)
	assert.Equal(t, config.MembershipTTL, gm.config.MembershipTTL)
}

func TestCreateGuild(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	capabilities := []string{"compute", "storage"}
	guild, err := gm.CreateGuild(ctx, capabilities)
	require.NoError(t, err)
	require.NotNil(t, guild)

	assert.NotEmpty(t, guild.ID)
	assert.Equal(t, h.ID(), guild.Creator)
	assert.Equal(t, DefaultMaxMembers, guild.MaxMembers)
	assert.False(t, guild.CreatedAt.IsZero())
	assert.True(t, guild.ExpiresAt.After(guild.CreatedAt))

	// Creator should be first member
	members := guild.GetMembers()
	assert.Len(t, members, 1)
	assert.Equal(t, h.ID(), members[0].PeerID)
	assert.Equal(t, RoleCreator, members[0].Role)
	assert.Equal(t, capabilities, members[0].Capabilities)
}

func TestJoinGuild(t *testing.T) {
	ctx := context.Background()
	
	// Create two hosts
	host1 := createTestHost(t)
	defer host1.Close()
	host2 := createTestHost(t)
	defer host2.Close()

	logger := zap.NewNop()

	// Host1 creates guild
	gm1 := NewGuildManager(ctx, host1, nil, logger)
	defer gm1.Close()

	guild, err := gm1.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	// Host2 joins guild
	gm2 := NewGuildManager(ctx, host2, nil, logger)
	defer gm2.Close()

	// First, add guild to host2's manager (simulating discovery)
	gm2.guilds[guild.ID] = guild

	err = gm2.JoinGuild(ctx, guild.ID, []string{"storage"})
	require.NoError(t, err)

	// Verify membership
	members := guild.GetMembers()
	assert.Len(t, members, 2)

	// Check both members exist
	assert.True(t, guild.IsMember(host1.ID()))
	assert.True(t, guild.IsMember(host2.ID()))

	// Check roles
	member1, err := guild.GetMember(host1.ID())
	require.NoError(t, err)
	assert.Equal(t, RoleCreator, member1.Role)

	member2, err := guild.GetMember(host2.ID())
	require.NoError(t, err)
	assert.Equal(t, RoleMember, member2.Role)
}

func TestJoinGuildFull(t *testing.T) {
	ctx := context.Background()
	host1 := createTestHost(t)
	defer host1.Close()

	logger := zap.NewNop()
	config := DefaultGuildConfig()
	config.MaxMembers = 2 // Only allow 2 members

	gm1 := NewGuildManager(ctx, host1, config, logger)
	defer gm1.Close()

	guild, err := gm1.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	// Host2 joins (should succeed)
	host2 := createTestHost(t)
	defer host2.Close()
	gm2 := NewGuildManager(ctx, host2, config, logger)
	defer gm2.Close()
	gm2.guilds[guild.ID] = guild

	err = gm2.JoinGuild(ctx, guild.ID, []string{"storage"})
	require.NoError(t, err)
	assert.Len(t, guild.GetMembers(), 2)

	// Host3 tries to join (should fail - guild full)
	host3 := createTestHost(t)
	defer host3.Close()
	gm3 := NewGuildManager(ctx, host3, config, logger)
	defer gm3.Close()
	gm3.guilds[guild.ID] = guild

	err = gm3.JoinGuild(ctx, guild.ID, []string{"compute"})
	assert.Equal(t, ErrGuildFull, err)
}

func TestLeaveGuild(t *testing.T) {
	ctx := context.Background()
	host1 := createTestHost(t)
	defer host1.Close()
	host2 := createTestHost(t)
	defer host2.Close()

	logger := zap.NewNop()
	gm1 := NewGuildManager(ctx, host1, nil, logger)
	defer gm1.Close()

	guild, err := gm1.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	gm2 := NewGuildManager(ctx, host2, nil, logger)
	defer gm2.Close()
	gm2.guilds[guild.ID] = guild

	err = gm2.JoinGuild(ctx, guild.ID, []string{"storage"})
	require.NoError(t, err)
	assert.Len(t, guild.GetMembers(), 2)

	// Host2 leaves
	err = gm2.LeaveGuild(ctx, guild.ID)
	require.NoError(t, err)

	assert.Len(t, guild.GetMembers(), 1)
	assert.True(t, guild.IsMember(host1.ID()))
	assert.False(t, guild.IsMember(host2.ID()))
}

func TestLeaveGuildAutoDissolve(t *testing.T) {
	ctx := context.Background()
	host1 := createTestHost(t)
	defer host1.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, host1, nil, logger)
	defer gm.Close()

	guild, err := gm.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)
	guildID := guild.ID

	// Creator leaves - should auto-dissolve
	err = gm.LeaveGuild(ctx, guildID)
	require.NoError(t, err)

	// Guild should be gone
	_, err = gm.GetGuild(guildID)
	assert.Equal(t, ErrGuildNotFound, err)
}

func TestDissolveGuild(t *testing.T) {
	ctx := context.Background()
	host1 := createTestHost(t)
	defer host1.Close()
	host2 := createTestHost(t)
	defer host2.Close()

	logger := zap.NewNop()
	gm1 := NewGuildManager(ctx, host1, nil, logger)
	defer gm1.Close()

	guild, err := gm1.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)
	guildID := guild.ID

	gm2 := NewGuildManager(ctx, host2, nil, logger)
	defer gm2.Close()
	gm2.guilds[guild.ID] = guild

	err = gm2.JoinGuild(ctx, guild.ID, []string{"storage"})
	require.NoError(t, err)
	assert.Len(t, guild.GetMembers(), 2)

	// Creator dissolves guild
	err = gm1.DissolveGuild(ctx, guildID)
	require.NoError(t, err)

	assert.True(t, guild.closed)
	assert.Len(t, guild.GetMembers(), 0)

	// Guild should be removed from manager
	_, err = gm1.GetGuild(guildID)
	assert.Equal(t, ErrGuildNotFound, err)
}

func TestDissolveGuildNonCreator(t *testing.T) {
	ctx := context.Background()
	host1 := createTestHost(t)
	defer host1.Close()
	host2 := createTestHost(t)
	defer host2.Close()

	logger := zap.NewNop()
	gm1 := NewGuildManager(ctx, host1, nil, logger)
	defer gm1.Close()

	guild, err := gm1.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	gm2 := NewGuildManager(ctx, host2, nil, logger)
	defer gm2.Close()
	gm2.guilds[guild.ID] = guild

	err = gm2.JoinGuild(ctx, guild.ID, []string{"storage"})
	require.NoError(t, err)

	// Non-creator tries to dissolve - should fail
	err = gm2.DissolveGuild(ctx, guild.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "only creator")
}

func TestGetGuild(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	guild, err := gm.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	retrieved, err := gm.GetGuild(guild.ID)
	require.NoError(t, err)
	assert.Equal(t, guild.ID, retrieved.ID)

	// Non-existent guild
	_, err = gm.GetGuild(GuildID("nonexistent"))
	assert.Equal(t, ErrGuildNotFound, err)
}

func TestListGuilds(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	// Create multiple guilds
	guild1, err := gm.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	guild2, err := gm.CreateGuild(ctx, []string{"storage"})
	require.NoError(t, err)

	guilds := gm.ListGuilds()
	assert.Len(t, guilds, 2)

	ids := []GuildID{guilds[0].ID, guilds[1].ID}
	assert.Contains(t, ids, guild1.ID)
	assert.Contains(t, ids, guild2.ID)
}

func TestUpdateHeartbeat(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	guild, err := gm.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)

	member, err := guild.GetMember(h.ID())
	require.NoError(t, err)
	originalTime := member.LastSeen

	time.Sleep(10 * time.Millisecond)

	err = guild.UpdateHeartbeat(h.ID())
	require.NoError(t, err)

	member, err = guild.GetMember(h.ID())
	require.NoError(t, err)
	assert.True(t, member.LastSeen.After(originalTime))
}

func TestGuildExpiration(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	config := DefaultGuildConfig()
	config.MembershipTTL = 100 * time.Millisecond // Short TTL for testing

	gm := NewGuildManager(ctx, h, config, logger)
	defer gm.Close()

	guild, err := gm.CreateGuild(ctx, []string{"compute"})
	require.NoError(t, err)
	guildID := guild.ID

	// Wait for expiration + cleanup
	time.Sleep(200 * time.Millisecond)
	gm.cleanup()

	// Guild should be removed
	_, err = gm.GetGuild(guildID)
	assert.Equal(t, ErrGuildNotFound, err)
}

func TestStats(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	// Empty stats
	stats := gm.Stats()
	assert.Equal(t, 0, stats.TotalGuilds)
	assert.Equal(t, 0, stats.TotalMembers)

	// Create guilds
	guild1, _ := gm.CreateGuild(ctx, []string{"compute"})
	_, _ = gm.CreateGuild(ctx, []string{"storage"})

	// Add member to guild1
	host2 := createTestHost(t)
	defer host2.Close()
	gm2 := NewGuildManager(ctx, host2, nil, logger)
	defer gm2.Close()
	gm2.guilds[guild1.ID] = guild1
	gm2.JoinGuild(ctx, guild1.ID, []string{"network"})

	stats = gm.Stats()
	assert.Equal(t, 2, stats.TotalGuilds)
	assert.Equal(t, 3, stats.TotalMembers) // guild1 has 2, guild2 has 1
	assert.Equal(t, 1.5, stats.AverageSize)
	assert.Greater(t, stats.OldestGuild, time.Duration(0))
}

func TestGuildClose(t *testing.T) {
	ctx := context.Background()
	h := createTestHost(t)
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)

	// Create some guilds
	gm.CreateGuild(ctx, []string{"compute"})
	gm.CreateGuild(ctx, []string{"storage"})

	err := gm.Close()
	require.NoError(t, err)

	// All guilds should be dissolved
	stats := gm.Stats()
	assert.Equal(t, 0, stats.TotalGuilds)
}

func TestDefaultGuildConfig(t *testing.T) {
	config := DefaultGuildConfig()

	assert.Equal(t, DefaultMaxMembers, config.MaxMembers)
	assert.Equal(t, DefaultMembershipTTL, config.MembershipTTL)
	assert.Equal(t, DefaultHeartbeatInterval, config.HeartbeatInterval)
	assert.True(t, config.EnableEncryption)
	assert.Empty(t, config.RequiredCapabilities)
}

func BenchmarkCreateGuild(b *testing.B) {
	ctx := context.Background()
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		b.Fatal(err)
	}
	defer h.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, h, nil, logger)
	defer gm.Close()

	capabilities := []string{"compute", "storage"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gm.CreateGuild(ctx, capabilities)
	}
}

func BenchmarkJoinGuild(b *testing.B) {
	ctx := context.Background()
	host1, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	if err != nil {
		b.Fatal(err)
	}
	defer host1.Close()

	logger := zap.NewNop()
	gm := NewGuildManager(ctx, host1, nil, logger)
	defer gm.Close()

	guild, _ := gm.CreateGuild(ctx, []string{"compute"})

	hosts := make([]host.Host, b.N)
	managers := make([]*GuildManager, b.N)
	for i := 0; i < b.N; i++ {
		hosts[i], err = libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		if err != nil {
			b.Fatal(err)
		}
		defer hosts[i].Close()
		managers[i] = NewGuildManager(ctx, hosts[i], nil, logger)
		defer managers[i].Close()
		managers[i].guilds[guild.ID] = guild
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		managers[i].JoinGuild(ctx, guild.ID, []string{"storage"})
	}
}
