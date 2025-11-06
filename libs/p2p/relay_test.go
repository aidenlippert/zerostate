package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRelayHostCreation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := DefaultRelayConfig()
	cfg.Logger = zap.NewNop()

	h, err := NewRelayHost(ctx, []string{"/ip4/127.0.0.1/udp/0/quic-v1"}, cfg)
	require.NoError(t, err)
	defer h.Close()

	assert.NotNil(t, h)
	assert.NotEmpty(t, h.ID())
	assert.NotEmpty(t, h.Addrs())
}

func TestRelayConfiguration(t *testing.T) {
	cfg := DefaultRelayConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, 256, cfg.Resources.MaxReservations)
	assert.Equal(t, 64, cfg.Resources.MaxCircuits)
	assert.Equal(t, 2, cfg.Resources.MaxReservationsPerPeer)
	assert.Equal(t, 4, cfg.Resources.MaxReservationsPerIP)
	assert.Equal(t, time.Hour, cfg.Resources.ReservationTTL)
}

func TestRelayWithCustomLimits(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg := DefaultRelayConfig()
	cfg.Logger = zap.NewNop()
	
	// Customize limits
	cfg.Resources.MaxReservations = 100
	cfg.Resources.MaxCircuits = 32
	cfg.Resources.ReservationTTL = 30 * time.Minute

	h, err := NewRelayHost(ctx, []string{"/ip4/127.0.0.1/udp/0/quic-v1"}, cfg)
	require.NoError(t, err)
	defer h.Close()

	assert.NotNil(t, h)
}
