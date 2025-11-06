package p2p

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multihash"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// PublishAgentCard publishes an agent card to the DHT as a provider record
func (n *Node) PublishAgentCard(ctx context.Context, cardJSON []byte) (string, error) {
	ctx, span := n.tracer.Start(ctx, "PublishAgentCard")
	defer span.End()

	timer := prometheus.NewTimer(dhtLookupDuration.WithLabelValues("publish"))
	defer timer.ObserveDuration()

	if n.dht == nil {
		dhtLookupsTotal.WithLabelValues("publish", "error").Inc()
		return "", fmt.Errorf("DHT not enabled")
	}

	// Compute content hash
	hash, err := multihash.Sum(cardJSON, multihash.SHA2_256, -1)
	if err != nil {
		dhtLookupsTotal.WithLabelValues("publish", "error").Inc()
		return "", fmt.Errorf("failed to hash card: %w", err)
	}

	c := cid.NewCidV1(cid.Raw, hash)
	cidStr := c.String()

	span.SetAttributes(attribute.String("cid", cidStr))

	// Provide the content in the DHT
	if err := n.dht.Provide(ctx, c, true); err != nil {
		n.logger.Error("failed to provide agent card",
			zap.String("cid", cidStr),
			zap.Error(err),
		)
		dhtLookupsTotal.WithLabelValues("publish", "error").Inc()
		return "", fmt.Errorf("failed to provide card: %w", err)
	}

	// Store content in content store
	if err := putContent(ctx, cidStr, cardJSON); err != nil {
		n.logger.Warn("failed to store content locally", zap.Error(err))
	}

	agentCardPublishTotal.Inc()
	dhtLookupsTotal.WithLabelValues("publish", "success").Inc()

	n.logger.Info("agent card published to DHT",
		zap.String("cid", cidStr),
		zap.Int("size_bytes", len(cardJSON)),
	)

	return cidStr, nil
}

// ResolveAgentCard resolves an agent card from the DHT by CID
func (n *Node) ResolveAgentCard(ctx context.Context, cidStr string) ([]byte, error) {
	ctx, span := n.tracer.Start(ctx, "ResolveAgentCard")
	defer span.End()

	timer := prometheus.NewTimer(dhtLookupDuration.WithLabelValues("resolve"))
	defer timer.ObserveDuration()

	if n.dht == nil {
		dhtLookupsTotal.WithLabelValues("resolve", "error").Inc()
		return nil, fmt.Errorf("DHT not enabled")
	}

	c, err := cid.Decode(cidStr)
	if err != nil {
		dhtLookupsTotal.WithLabelValues("resolve", "error").Inc()
		return nil, fmt.Errorf("invalid CID: %w", err)
	}

	span.SetAttributes(attribute.String("cid", cidStr))

	// Check local store first
	content, err := getContent(ctx, cidStr)
	if err == nil {
		dhtLookupsTotal.WithLabelValues("resolve", "success").Inc()
		n.logger.Info("resolved agent card from local cache",
			zap.String("cid", cidStr),
			zap.Int("size_bytes", len(content)),
		)
		return content, nil
	}

	// Find providers for this CID
	n.logger.Debug("searching for providers",
		zap.String("cid", cidStr),
	)
	
	// Collect multiple providers for Q-routing
	providersCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	providersChan := n.dht.FindProvidersAsync(providersCtx, c, 10)
	var candidates []peer.AddrInfo
	
	for providerPeer := range providersChan {
		candidates = append(candidates, providerPeer)
		if len(candidates) >= 5 { // Limit to 5 candidates for Q-routing
			break
		}
	}
	
	if len(candidates) == 0 {
		dhtLookupsTotal.WithLabelValues("resolve", "not_found").Inc()
		return nil, fmt.Errorf("no providers found for CID %s", cidStr)
	}
	
	n.logger.Info("found providers for agent card",
		zap.String("cid", cidStr),
		zap.Int("provider_count", len(candidates)),
	)
	
	// Use Q-routing to select best peer if we have multiple candidates
	var selectedPeer peer.ID
	if len(candidates) > 1 {
		candidateIDs := make([]peer.ID, len(candidates))
		for i, p := range candidates {
			candidateIDs[i] = p.ID
		}
		
		bestPeer, ok := n.qtable.SelectBestPeer(candidateIDs)
		if ok {
			selectedPeer = bestPeer
			n.logger.Debug("selected peer using Q-routing",
				zap.String("peer_id", selectedPeer.String()),
			)
		} else {
			selectedPeer = candidates[0].ID
		}
	} else {
		selectedPeer = candidates[0].ID
	}

	// Fetch content from selected provider using content exchange protocol
	startTime := time.Now()
	content, err = n.FetchContent(ctx, selectedPeer, cidStr)
	latency := time.Since(startTime)
	
	// Update Q-table with the result
	success := err == nil
	bytesTransferred := int64(0)
	if success {
		bytesTransferred = int64(len(content))
	}
	n.qtable.UpdateRoute(selectedPeer, latency, success, bytesTransferred)
	
	if err != nil {
		dhtLookupsTotal.WithLabelValues("resolve", "error").Inc()
		n.logger.Warn("failed to fetch content from selected peer",
			zap.String("peer_id", selectedPeer.String()),
			zap.Duration("latency", latency),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to fetch content from provider: %w", err)
	}

	dhtLookupsTotal.WithLabelValues("resolve", "success").Inc()

	n.logger.Info("resolved agent card content from peer",
		zap.String("cid", cidStr),
		zap.String("provider", selectedPeer.String()),
		zap.Int("size_bytes", len(content)),
		zap.Duration("latency", latency),
	)

	return content, nil
}
