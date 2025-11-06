package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
)

// discoveryNotifee implements mdns.Notifee to handle peer discovery
type discoveryNotifee struct {
	h      host.Host
	logger *zap.Logger
}

// HandlePeerFound connects to newly discovered peers
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	n.logger.Info("discovered peer via mDNS",
		zap.String("peer_id", pi.ID.String()),
		zap.Int("addrs", len(pi.Addrs)),
	)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Connect to the discovered peer
	if err := n.h.Connect(ctx, pi); err != nil {
		n.logger.Warn("failed to connect to discovered peer",
			zap.String("peer_id", pi.ID.String()),
			zap.Error(err),
		)
		return
	}
	
	n.logger.Info("connected to mDNS peer",
		zap.String("peer_id", pi.ID.String()),
	)
}

// StartMDNS starts mDNS peer discovery for LAN peers
func (n *Node) StartMDNS(ctx context.Context) error {
	notifee := &discoveryNotifee{
		h:      n.host,
		logger: n.logger,
	}
	
	// Create mDNS service with zerostate service tag
	service := mdns.NewMdnsService(n.host, "zerostate", notifee)
	
	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start mDNS: %w", err)
	}
	
	n.logger.Info("mDNS discovery started", zap.String("service", "zerostate"))
	
	return nil
}
