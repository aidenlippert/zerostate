package p2p

import (
	"context"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
)

const (
	// ContentExchangeProtocol is the protocol ID for content exchange
	ContentExchangeProtocol = protocol.ID("/zerostate/content-exchange/1.0.0")
	
	// MaxContentSize is the maximum size for content exchange (10MB)
	MaxContentSize = 10 * 1024 * 1024
)

// ContentProvider implements content exchange protocol
type ContentProvider struct {
	host   host.Host
	logger *zap.Logger
}

// StartContentProvider initializes the content exchange protocol handler
func (n *Node) StartContentProvider(ctx context.Context) error {
	provider := &ContentProvider{
		host:   n.host,
		logger: n.logger,
	}

	// Set stream handler for content requests
	n.host.SetStreamHandler(ContentExchangeProtocol, provider.handleContentRequest)

	n.logger.Info("content provider started",
		zap.String("protocol", string(ContentExchangeProtocol)),
	)

	return nil
}

// handleContentRequest handles incoming content requests
func (cp *ContentProvider) handleContentRequest(s network.Stream) {
	defer s.Close()

	ctx := context.Background() // Use background context for content operations
	remotePeer := s.Conn().RemotePeer()
	cp.logger.Debug("received content request",
		zap.String("peer", remotePeer.String()),
	)

	// Read CID from stream (up to 100 bytes for CID)
	cidBytes := make([]byte, 100)
	n, err := s.Read(cidBytes)
	if err != nil {
		cp.logger.Warn("failed to read CID from stream",
			zap.Error(err),
		)
		return
	}

	cidStr := string(cidBytes[:n])
	cp.logger.Debug("content requested",
		zap.String("cid", cidStr),
		zap.String("peer", remotePeer.String()),
	)

	// Fetch from content store
	content, err := getContent(ctx, cidStr)
	if err != nil {
		// Send error response (0-byte indicates not found)
		cp.logger.Debug("content not found",
			zap.String("cid", cidStr),
		)
		return
	}

	// Send content size (4 bytes, big-endian)
	size := uint32(len(content))
	sizeBytes := []byte{
		byte(size >> 24),
		byte(size >> 16),
		byte(size >> 8),
		byte(size),
	}

	if _, err := s.Write(sizeBytes); err != nil {
		cp.logger.Warn("failed to write content size",
			zap.Error(err),
		)
		return
	}

	// Send content
	if _, err := s.Write(content); err != nil {
		cp.logger.Warn("failed to write content",
			zap.Error(err),
		)
		return
	}

	cp.logger.Info("content sent",
		zap.String("cid", cidStr),
		zap.String("peer", remotePeer.String()),
		zap.Int("size_bytes", len(content)),
	)
}

// FetchContent retrieves content from a peer using the content exchange protocol
func (n *Node) FetchContent(ctx context.Context, peerID peer.ID, cidStr string) ([]byte, error) {
	n.logger.Debug("fetching content from peer",
		zap.String("cid", cidStr),
		zap.String("peer", peerID.String()),
	)

	// Open stream to peer
	stream, err := n.host.NewStream(ctx, peerID, ContentExchangeProtocol)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream: %w", err)
	}
	defer stream.Close()

	// Send CID
	if _, err := stream.Write([]byte(cidStr)); err != nil {
		return nil, fmt.Errorf("failed to send CID: %w", err)
	}

	// Read content size (4 bytes)
	sizeBytes := make([]byte, 4)
	if _, err := io.ReadFull(stream, sizeBytes); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("content not found on peer")
		}
		return nil, fmt.Errorf("failed to read content size: %w", err)
	}

	size := uint32(sizeBytes[0])<<24 | uint32(sizeBytes[1])<<16 |
		uint32(sizeBytes[2])<<8 | uint32(sizeBytes[3])

	if size > MaxContentSize {
		return nil, fmt.Errorf("content too large: %d bytes", size)
	}

	// Read content
	content := make([]byte, size)
	if _, err := io.ReadFull(stream, content); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	n.logger.Info("content fetched",
		zap.String("cid", cidStr),
		zap.String("peer", peerID.String()),
		zap.Int("size_bytes", len(content)),
	)

	// Store locally for caching
	if err := putContent(ctx, cidStr, content); err != nil {
		n.logger.Warn("failed to cache content locally", zap.Error(err))
	}

	return content, nil
}
