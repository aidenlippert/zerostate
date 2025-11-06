package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	relayReservations = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zerostate_relay_reservations_total",
			Help: "Number of active relay reservations",
		},
	)

	relayConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zerostate_relay_connections_total",
			Help: "Number of active relay connections",
		},
	)

	relayBytesTransferred = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_relay_bytes_transferred_total",
			Help: "Total bytes transferred through relay",
		},
		[]string{"direction"}, // inbound, outbound
	)

	relayConnectionsAccepted = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "zerostate_relay_connections_accepted_total",
			Help: "Total relay connections accepted",
		},
	)

	relayConnectionsRejected = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_relay_connections_rejected_total",
			Help: "Total relay connections rejected",
		},
		[]string{"reason"}, // resource_limit, ip_limit, peer_limit
	)
)

// RelayConfig holds configuration for circuit relay v2
type RelayConfig struct {
	// Enable the relay service
	Enabled bool

	// Resources configures relay resource limits
	Resources relayv2.Resources

	// Logger for relay operations
	Logger *zap.Logger
}

// DefaultRelayConfig returns sensible defaults for a public relay
func DefaultRelayConfig() *RelayConfig {
	return &RelayConfig{
		Enabled: true,
		Resources: relayv2.Resources{
			// Maximum number of reservations
			Limit: &relayv2.RelayLimit{
				Duration: 2 * time.Hour,
				Data:     1 << 20, // 1 MB per reservation
			},

			// Maximum number of reservations from a single peer
			ReservationTTL: 1 * time.Hour,

			// Maximum number of active reservations
			MaxReservations: 256,

			// Maximum number of reservations per peer
			MaxReservationsPerPeer: 2,

			// Maximum number of reservations per IP
			MaxReservationsPerIP: 4,

			// Maximum number of circuits
			MaxCircuits: 64,

			// Buffer size for relay connections
			BufferSize: 2048,
		},
		Logger: zap.NewNop(),
	}
}

// EnableRelay configures a libp2p host to act as a circuit relay v2
func EnableRelay(cfg *RelayConfig) libp2p.Option {
	if !cfg.Enabled {
		return func(_ *libp2p.Config) error { return nil }
	}

	return libp2p.ChainOptions(
		libp2p.EnableRelayService(relayv2.WithResources(cfg.Resources)),
		libp2p.EnableHolePunching(),
	)
}

// NewRelayHost creates a libp2p host configured as a circuit relay
func NewRelayHost(ctx context.Context, listenAddrs []string, cfg *RelayConfig) (host.Host, error) {
	if cfg == nil {
		cfg = DefaultRelayConfig()
	}

	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	// Create host with relay enabled
	h, err := libp2p.New(
		libp2p.ListenAddrStrings(listenAddrs...),
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
		libp2p.EnableNATService(),
		EnableRelay(cfg),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create relay host: %w", err)
	}

	cfg.Logger.Info("relay host created",
		zap.String("peer_id", h.ID().String()),
		zap.Int("max_reservations", cfg.Resources.MaxReservations),
		zap.Int("max_circuits", cfg.Resources.MaxCircuits),
		zap.Duration("reservation_ttl", cfg.Resources.ReservationTTL),
	)

	// Start metrics collection
	go collectRelayMetrics(ctx, h, cfg.Logger)

	return h, nil
}

// collectRelayMetrics periodically updates relay metrics
func collectRelayMetrics(ctx context.Context, h host.Host, logger *zap.Logger) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get relay stats
			conns := h.Network().Conns()
			relayConns := 0
			for _, conn := range conns {
				// Check if connection is relayed
				if conn.Stat().Direction == 0 { // Relayed connections
					relayConns++
				}
			}

			relayConnections.Set(float64(relayConns))

			logger.Debug("relay metrics updated",
				zap.Int("total_connections", len(conns)),
				zap.Int("relay_connections", relayConns),
			)
		}
	}
}

// EnableRelayClient configures a node to use circuit relay as a client
func EnableRelayClient() libp2p.Option {
	return libp2p.ChainOptions(
		libp2p.EnableAutoRelayWithStaticRelays([]peer.AddrInfo{}),
		libp2p.EnableHolePunching(),
	)
}
