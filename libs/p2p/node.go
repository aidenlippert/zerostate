// Package p2p provides libp2p-based peer-to-peer networking for zerostate.
package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/zerostate/libs/routing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	// ProtocolPrefix is the libp2p protocol prefix for zerostate
	ProtocolPrefix = "/zerostate"
	// DHTProtocolID is the Kademlia DHT protocol ID
	DHTProtocolID = ProtocolPrefix + "/kad/1.0.0"
	// DefaultK is the Kademlia bucket size
	DefaultK = 20
	// DefaultAlpha is the Kademlia concurrency parameter
	DefaultAlpha = 3
)

var tracer = otel.Tracer("zerostate/p2p")

// Config holds configuration for the P2P node
type Config struct {
	// ListenAddrs is the list of multiaddrs to listen on
	ListenAddrs []string
	// BootstrapPeers is the list of bootstrap peer multiaddrs
	BootstrapPeers []string
	// EnableDHT enables Kademlia DHT
	EnableDHT bool
	// DHTMode sets DHT mode (server vs client)
	DHTMode dht.ModeOpt
	// EnableMDNS enables mDNS peer discovery for LAN
	EnableMDNS bool
	// Logger is the structured logger
	Logger *zap.Logger
}

// Node represents a zerostate P2P node
type Node struct {
	host            host.Host
	dht             *dht.IpfsDHT
	qtable          *routing.QTable
	protocol        *ProtocolNegotiator
	flowCtrl        *FlowController
	gossip          *GossipService
	providerRefresh *ProviderRefresher
	connPool        *ConnectionPool
	config          *Config
	logger          *zap.Logger
	tracer          trace.Tracer
}

// NewNode creates and initializes a new P2P node
func NewNode(ctx context.Context, cfg *Config) (*Node, error) {
	ctx, span := tracer.Start(ctx, "NewNode")
	defer span.End()

	if cfg.Logger == nil {
		cfg.Logger = zap.NewNop()
	}

	// Parse listen addresses
	listenAddrs := make([]multiaddr.Multiaddr, 0, len(cfg.ListenAddrs))
	for _, addr := range cfg.ListenAddrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid listen address %s: %w", addr, err)
		}
		listenAddrs = append(listenAddrs, ma)
	}

	// Create libp2p host with QUIC transport
	h, err := libp2p.New(
		libp2p.ListenAddrs(listenAddrs...),
		libp2p.DefaultSecurity,
		libp2p.DefaultMuxers,
		libp2p.EnableNATService(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Initialize protocol negotiator
	protocol, err := NewProtocolNegotiator(cfg.Logger)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create protocol negotiator: %w", err)
	}

	// Initialize flow controller
	flowCtrl := NewFlowController(DefaultFlowControlConfig(), cfg.Logger)

	// Initialize gossip service
	gossip, err := NewGossipService(ctx, h, cfg.Logger)
	if err != nil {
		h.Close()
		flowCtrl.Close()
		return nil, fmt.Errorf("failed to create gossip service: %w", err)
	}

	// Initialize connection pool
	connPool := NewConnectionPool(ctx, h, nil, cfg.Logger)

	node := &Node{
		host:     h,
		config:   cfg,
		logger:   cfg.Logger,
		tracer:   tracer,
		qtable:   routing.NewQTable(),
		protocol: protocol,
		flowCtrl: flowCtrl,
		gossip:   gossip,
		connPool: connPool,
	}

	cfg.Logger.Info("libp2p host created",
		zap.String("peer_id", h.ID().String()),
		zap.Strings("addrs", multiaddrsToStrings(h.Addrs())),
	)

	// Initialize DHT if enabled
	if cfg.EnableDHT {
		if err := node.initDHT(ctx); err != nil {
			h.Close()
			return nil, fmt.Errorf("failed to initialize DHT: %w", err)
		}
	}

	// Start mDNS discovery if enabled
	if cfg.EnableMDNS {
		if err := node.StartMDNS(ctx); err != nil {
			cfg.Logger.Warn("failed to start mDNS", zap.Error(err))
		}
	}

	// Start content provider protocol
	if err := node.StartContentProvider(ctx); err != nil {
		cfg.Logger.Warn("failed to start content provider", zap.Error(err))
	}

	return node, nil
}

// initDHT initializes the Kademlia DHT
func (n *Node) initDHT(ctx context.Context) error {
	ctx, span := n.tracer.Start(ctx, "initDHT")
	defer span.End()

	dhtOpts := []dht.Option{
		dht.Mode(n.config.DHTMode),
		dht.ProtocolPrefix(ProtocolPrefix),
		dht.BucketSize(DefaultK),
		dht.Concurrency(DefaultAlpha),
	}

	kdht, err := dht.New(ctx, n.host, dhtOpts...)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}

	n.dht = kdht

	// Initialize provider refresher
	n.providerRefresh = NewProviderRefresher(ctx, kdht, nil, n.logger)

	n.logger.Info("DHT initialized",
		zap.String("protocol", DHTProtocolID),
		zap.Int("k", DefaultK),
		zap.Int("alpha", DefaultAlpha),
	)

	return nil
}

// Bootstrap connects to bootstrap peers and bootstraps the DHT
func (n *Node) Bootstrap(ctx context.Context) error {
	ctx, span := n.tracer.Start(ctx, "Bootstrap")
	defer span.End()

	if n.dht == nil {
		return fmt.Errorf("DHT not enabled")
	}

	// Parse bootstrap peers
	bootstrapPeers := make([]peer.AddrInfo, 0, len(n.config.BootstrapPeers))
	for _, peerAddr := range n.config.BootstrapPeers {
		ma, err := multiaddr.NewMultiaddr(peerAddr)
		if err != nil {
			n.logger.Warn("invalid bootstrap peer address", zap.String("addr", peerAddr), zap.Error(err))
			continue
		}
		ai, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			n.logger.Warn("failed to parse bootstrap peer", zap.String("addr", peerAddr), zap.Error(err))
			continue
		}
		bootstrapPeers = append(bootstrapPeers, *ai)
	}

	if len(bootstrapPeers) == 0 {
		return fmt.Errorf("no valid bootstrap peers")
	}

	n.logger.Info("bootstrapping DHT", zap.Int("peer_count", len(bootstrapPeers)))

	// Connect to bootstrap peers
	for _, ai := range bootstrapPeers {
		if err := n.host.Connect(ctx, ai); err != nil {
			n.logger.Warn("failed to connect to bootstrap peer",
				zap.String("peer_id", ai.ID.String()),
				zap.Error(err),
			)
		} else {
			n.logger.Info("connected to bootstrap peer", zap.String("peer_id", ai.ID.String()))
		}
	}

	// Bootstrap the DHT
	if err := n.dht.Bootstrap(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap DHT: %w", err)
	}

	n.logger.Info("DHT bootstrap complete")
	return nil
}

// ID returns the peer ID of this node
func (n *Node) ID() peer.ID {
	return n.host.ID()
}

// Addrs returns the listen addresses of this node
func (n *Node) Addrs() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

// DHT returns the DHT instance (for provider records, etc.)
func (n *Node) DHT() *dht.IpfsDHT {
	return n.dht
}

// Host returns the underlying libp2p host
func (n *Node) Host() host.Host {
	return n.host
}

// Close stops the P2P node
func (n *Node) Close() error {
	n.logger.Info("shutting down node")
	if n.connPool != nil {
		if err := n.connPool.Close(); err != nil {
			n.logger.Error("error closing connection pool", zap.Error(err))
		}
	}
	if n.providerRefresh != nil {
		if err := n.providerRefresh.Close(); err != nil {
			n.logger.Error("error closing provider refresh", zap.Error(err))
		}
	}
	if n.gossip != nil {
		if err := n.gossip.Close(); err != nil {
			n.logger.Error("error closing gossip service", zap.Error(err))
		}
	}
	if n.flowCtrl != nil {
		n.flowCtrl.Close()
	}
	if n.dht != nil {
		if err := n.dht.Close(); err != nil {
			n.logger.Error("error closing DHT", zap.Error(err))
		}
	}
	return n.host.Close()
}

// WaitForPeers waits until at least minPeers are connected
func (n *Node) WaitForPeers(ctx context.Context, minPeers int, timeout time.Duration) error {
	ctx, span := n.tracer.Start(ctx, "WaitForPeers")
	defer span.End()

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			peers := n.host.Network().Peers()
			if len(peers) >= minPeers {
				n.logger.Info("peer threshold reached", zap.Int("peer_count", len(peers)))
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for peers (have %d, need %d)", len(peers), minPeers)
			}
		}
	}
}

// Protocol returns the protocol negotiator
func (n *Node) Protocol() *ProtocolNegotiator {
	return n.protocol
}

// FlowControl returns the flow controller
func (n *Node) FlowControl() *FlowController {
	return n.flowCtrl
}

// Gossip returns the gossip service
func (n *Node) Gossip() *GossipService {
	return n.gossip
}

// ProviderRefresh returns the provider refresher
func (n *Node) ProviderRefresh() *ProviderRefresher {
	return n.providerRefresh
}

// ConnectionPool returns the connection pool
func (n *Node) ConnectionPool() *ConnectionPool {
	return n.connPool
}

// multiaddrsToStrings converts multiaddrs to strings
func multiaddrsToStrings(addrs []multiaddr.Multiaddr) []string {
	strs := make([]string, len(addrs))
	for i, addr := range addrs {
		strs[i] = addr.String()
	}
	return strs
}
