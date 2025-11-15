package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
)

const (
	// QRoutingProtocolID is the protocol ID for Q-routed direct messages
	QRoutingProtocolID = "/zerostate/qrouting/1.0.0"
)

// QRoutingMessenger sends messages using Q-learning for peer selection
type QRoutingMessenger struct {
	node   *Node
	logger *zap.Logger
}

// NewQRoutingMessenger creates a new Q-routing messenger
func NewQRoutingMessenger(node *Node, logger *zap.Logger) *QRoutingMessenger {
	if logger == nil {
		logger = zap.NewNop()
	}

	qrm := &QRoutingMessenger{
		node:   node,
		logger: logger,
	}

	// Register stream handler for incoming Q-routed messages
	node.Host().SetStreamHandler(protocol.ID(QRoutingProtocolID), qrm.handleStream)

	logger.Info("Q-routing messenger initialized")
	return qrm
}

// SendDirect sends a message directly to the best peer using Q-routing
// This is the key integration: use Q-table to select best peer, then track performance
func (qrm *QRoutingMessenger) SendDirect(ctx context.Context, candidates []peer.ID, payload []byte) error {
	if len(candidates) == 0 {
		return fmt.Errorf("no candidate peers provided")
	}

	startTime := time.Now()

	// Step 1: Use Q-routing to select best peer
	bestPeer, err := qrm.node.SelectBestPeer(ctx, candidates)
	if err != nil {
		return fmt.Errorf("failed to select best peer: %w", err)
	}

	qrm.logger.Debug("Q-routing selected peer",
		zap.String("peer_id", bestPeer.String()),
		zap.Int("payload_bytes", len(payload)),
	)

	// Step 2: Open stream to selected peer
	stream, err := qrm.node.Host().NewStream(ctx, bestPeer, protocol.ID(QRoutingProtocolID))
	if err != nil {
		// Failed to connect - update Q-table with negative reward
		latency := time.Since(startTime)
		qrm.node.UpdateRouteMetrics(bestPeer, latency, false, 0)
		return fmt.Errorf("failed to open stream to %s: %w", bestPeer, err)
	}
	defer stream.Close()

	// Step 3: Send payload
	n, err := stream.Write(payload)
	if err != nil {
		// Write failed - update Q-table with negative reward
		latency := time.Since(startTime)
		qrm.node.UpdateRouteMetrics(bestPeer, latency, false, int64(n))
		return fmt.Errorf("failed to write to stream: %w", err)
	}

	// Step 4: Success - update Q-table with positive reward
	latency := time.Since(startTime)
	qrm.node.UpdateRouteMetrics(bestPeer, latency, true, int64(n))

	qrm.logger.Info("Q-routed message sent successfully",
		zap.String("peer_id", bestPeer.String()),
		zap.Duration("latency", latency),
		zap.Int("bytes_sent", n),
	)

	return nil
}

// SendToTopPeers sends a message to the N best-performing peers
func (qrm *QRoutingMessenger) SendToTopPeers(ctx context.Context, n int, payload []byte) (int, error) {
	topPeers := qrm.node.GetTopPeers(n)
	if len(topPeers) == 0 {
		return 0, fmt.Errorf("no top peers available")
	}

	qrm.logger.Info("sending to top peers",
		zap.Int("peer_count", len(topPeers)),
		zap.Int("payload_bytes", len(payload)),
	)

	successCount := 0
	for _, peerID := range topPeers {
		if err := qrm.SendDirect(ctx, []peer.ID{peerID}, payload); err != nil {
			qrm.logger.Warn("failed to send to peer",
				zap.String("peer_id", peerID.String()),
				zap.Error(err),
			)
			continue
		}
		successCount++
	}

	return successCount, nil
}

// SendAgentMessage sends an agent message using Q-routing
func (qrm *QRoutingMessenger) SendAgentMessage(ctx context.Context, candidates []peer.ID, msg *AgentMessage) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal agent message: %w", err)
	}

	return qrm.SendDirect(ctx, candidates, payload)
}

// handleStream handles incoming Q-routed messages
func (qrm *QRoutingMessenger) handleStream(stream network.Stream) {
	defer stream.Close()

	startTime := time.Now()
	remotePeer := stream.Conn().RemotePeer()

	qrm.logger.Debug("received Q-routed stream",
		zap.String("peer_id", remotePeer.String()),
	)

	// Read message
	buf := make([]byte, 1024*1024) // 1MB max
	n, err := stream.Read(buf)
	if err != nil {
		qrm.logger.Error("failed to read from stream",
			zap.String("peer_id", remotePeer.String()),
			zap.Error(err),
		)
		// Update Q-table with failure
		latency := time.Since(startTime)
		qrm.node.UpdateRouteMetrics(remotePeer, latency, false, int64(n))
		return
	}

	// Success - update Q-table with positive reward
	latency := time.Since(startTime)
	qrm.node.UpdateRouteMetrics(remotePeer, latency, true, int64(n))

	qrm.logger.Info("Q-routed message received",
		zap.String("peer_id", remotePeer.String()),
		zap.Duration("latency", latency),
		zap.Int("bytes_received", n),
	)

	// TODO: Process message (add handler callback)
}

// GetRoutingStats returns Q-routing statistics
func (qrm *QRoutingMessenger) GetRoutingStats() map[string]interface{} {
	return qrm.node.GetRoutingStats()
}

// PruneStaleRoutes removes stale Q-table entries
func (qrm *QRoutingMessenger) PruneStaleRoutes(maxAge time.Duration) int {
	return qrm.node.PruneStaleRoutes(maxAge)
}
