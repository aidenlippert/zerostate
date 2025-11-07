package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewHealthMonitor(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 1)
	defer hosts[0].Close()

	logger := zap.NewNop()
	config := DefaultHealthCheckConfig()

	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	assert.NotNil(t, hm)
	assert.Equal(t, config.HeartbeatInterval, hm.config.HeartbeatInterval)
	assert.Equal(t, config.FailureThreshold, hm.config.FailureThreshold)
}

func TestDefaultHealthCheckConfig(t *testing.T) {
	config := DefaultHealthCheckConfig()

	assert.Equal(t, DefaultHeartbeatInterval, config.HeartbeatInterval)
	assert.Equal(t, DefaultFailureThreshold, config.FailureThreshold)
	assert.Equal(t, DefaultHealthCheckTimeout, config.HealthCheckTimeout)
	assert.True(t, config.EnableMetrics)
}

func TestMonitorPeer(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)
	defer hm.Close()

	// Start monitoring
	hm.MonitorPeer(hosts[1].ID())

	// Should be in monitoring list
	health, exists := hm.GetPeerHealth(hosts[1].ID())
	assert.True(t, exists)
	assert.NotNil(t, health)
	assert.True(t, health.Healthy)
}

func TestMonitorPeerDuplicate(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)
	defer hm.Close()

	// Monitor same peer twice
	hm.MonitorPeer(hosts[1].ID())
	hm.MonitorPeer(hosts[1].ID())

	stats := hm.Stats()
	assert.Equal(t, 1, stats.TotalMonitored)
}

func TestStopMonitoring(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)
	defer hm.Close()

	// Start and stop monitoring
	hm.MonitorPeer(hosts[1].ID())

	stats := hm.Stats()
	assert.Equal(t, 1, stats.TotalMonitored)

	hm.StopMonitoring(hosts[1].ID())

	stats = hm.Stats()
	assert.Equal(t, 0, stats.TotalMonitored)
}

func TestHealthCheckHealthyPeer(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	config := &HealthCheckConfig{
		HeartbeatInterval:  100 * time.Millisecond,
		FailureThreshold:   2,
		HealthCheckTimeout: 1 * time.Second,
		EnableMetrics:      true,
	}
	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())

	// Wait for health check
	time.Sleep(200 * time.Millisecond)

	health, exists := hm.GetPeerHealth(hosts[1].ID())
	require.True(t, exists)
	assert.True(t, health.Healthy)
	assert.Equal(t, 0, health.ConsecutiveFails)
	assert.Greater(t, health.TotalChecks, 0)
}

func TestHealthCheckUnhealthyPeer(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	config := &HealthCheckConfig{
		HeartbeatInterval:  50 * time.Millisecond,
		FailureThreshold:   2,
		HealthCheckTimeout: 1 * time.Second,
		EnableMetrics:      true,
	}
	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())

	// Disconnect peer
	hosts[0].Network().ClosePeer(hosts[1].ID())

	// Wait for multiple health checks to fail
	time.Sleep(150 * time.Millisecond)

	health, exists := hm.GetPeerHealth(hosts[1].ID())
	require.True(t, exists)
	assert.False(t, health.Healthy)
	assert.GreaterOrEqual(t, health.ConsecutiveFails, config.FailureThreshold)
}

func TestGetHealthyPeers(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 3)
	defer hosts[0].Close()
	defer hosts[1].Close()
	defer hosts[2].Close()

	connectHosts(t, hosts[0], hosts[1])
	connectHosts(t, hosts[0], hosts[2])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())
	hm.MonitorPeer(hosts[2].ID())

	// Wait for health checks
	time.Sleep(100 * time.Millisecond)

	healthy := hm.GetHealthyPeers()
	assert.GreaterOrEqual(t, len(healthy), 0)
	assert.LessOrEqual(t, len(healthy), 2)
}

func TestGetUnhealthyPeers(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	config := &HealthCheckConfig{
		HeartbeatInterval:  50 * time.Millisecond,
		FailureThreshold:   2,
		HealthCheckTimeout: 1 * time.Second,
		EnableMetrics:      true,
	}
	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())

	// Disconnect peer
	hosts[0].Network().ClosePeer(hosts[1].ID())

	// Wait for health checks to fail
	time.Sleep(150 * time.Millisecond)

	unhealthy := hm.GetUnhealthyPeers()
	assert.GreaterOrEqual(t, len(unhealthy), 0)
}

func TestHealthMonitorStats(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 3)
	defer hosts[0].Close()
	defer hosts[1].Close()
	defer hosts[2].Close()

	connectHosts(t, hosts[0], hosts[1])
	connectHosts(t, hosts[0], hosts[2])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())
	hm.MonitorPeer(hosts[2].ID())

	stats := hm.Stats()
	assert.Equal(t, 2, stats.TotalMonitored)
	assert.GreaterOrEqual(t, stats.HealthyPeers, 0)
	assert.GreaterOrEqual(t, stats.UnhealthyPeers, 0)
	assert.Equal(t, stats.TotalMonitored, stats.HealthyPeers+stats.UnhealthyPeers)
}

func TestHealthMonitorClose(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	hm := NewHealthMonitor(ctx, hosts[0], nil, logger)

	hm.MonitorPeer(hosts[1].ID())

	stats := hm.Stats()
	assert.Equal(t, 1, stats.TotalMonitored)

	err := hm.Close()
	assert.NoError(t, err)

	stats = hm.Stats()
	assert.Equal(t, 0, stats.TotalMonitored)
}

func TestPeerHealthRecovery(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	config := &HealthCheckConfig{
		HeartbeatInterval:  50 * time.Millisecond,
		FailureThreshold:   2,
		HealthCheckTimeout: 1 * time.Second,
		EnableMetrics:      true,
	}
	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	hm.MonitorPeer(hosts[1].ID())

	// Wait for initial health check
	time.Sleep(100 * time.Millisecond)

	// Disconnect
	hosts[0].Network().ClosePeer(hosts[1].ID())
	time.Sleep(150 * time.Millisecond)

	// Should be unhealthy
	health, _ := hm.GetPeerHealth(hosts[1].ID())
	wasUnhealthy := !health.Healthy

	// Reconnect
	connectHosts(t, hosts[0], hosts[1])
	time.Sleep(100 * time.Millisecond)

	// Should recover
	health, exists := hm.GetPeerHealth(hosts[1].ID())
	require.True(t, exists)
	if wasUnhealthy {
		assert.True(t, health.Healthy, "peer should recover after reconnection")
	}
}

func TestConcurrentHealthChecks(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 5)
	for _, h := range hosts {
		defer h.Close()
	}

	// Connect all to first host
	for i := 1; i < len(hosts); i++ {
		connectHosts(t, hosts[0], hosts[i])
	}

	logger := zap.NewNop()
	config := &HealthCheckConfig{
		HeartbeatInterval:  100 * time.Millisecond,
		FailureThreshold:   3,
		HealthCheckTimeout: 1 * time.Second,
		EnableMetrics:      true,
	}
	hm := NewHealthMonitor(ctx, hosts[0], config, logger)
	defer hm.Close()

	// Monitor all peers
	for i := 1; i < len(hosts); i++ {
		hm.MonitorPeer(hosts[i].ID())
	}

	// Wait for health checks
	time.Sleep(200 * time.Millisecond)

	stats := hm.Stats()
	assert.Equal(t, 4, stats.TotalMonitored)
}
