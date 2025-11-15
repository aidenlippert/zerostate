package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/metrics"
	"github.com/libp2p/go-libp2p"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// RuntimeRegistry maintains a registry of discovered ARI-v1 runtimes
type RuntimeRegistry struct {
	host     host.Host
	pubsub   *pubsub.PubSub
	runtimes map[string]*RuntimeInfo // DID -> RuntimeInfo
	mu       sync.RWMutex
	logger   *zap.Logger
	ctx      context.Context
	cancel   context.CancelFunc
	metrics  *metrics.PrometheusMetrics

	lastStatusLabels     map[string]struct{}
	lastCapabilityLabels map[string]struct{}
	presenceTopic        string
	lastUpdated          time.Time
}

// RuntimeInfo represents a discovered runtime
type RuntimeInfo struct {
	DID          string            `json:"did"`
	Name         string            `json:"name"`
	Capabilities []string          `json:"capabilities"`
	GRPCAddress  string            `json:"grpc_address"`  // Primary ARI-v1 endpoint
	P2PAddresses []string          `json:"p2p_addresses"` // libp2p multiaddrs
	LastSeen     time.Time         `json:"last_seen"`
	Status       string            `json:"status"` // "online", "offline", "busy"
	Metadata     map[string]string `json:"metadata"`
}

// PresenceMessage represents a runtime presence announcement (matches reference-runtime-v1)
type PresenceMessage struct {
	DID          string            `json:"did"`
	Name         string            `json:"name"`
	Capabilities []string          `json:"capabilities"`
	Addresses    []string          `json:"addresses"`     // gRPC addresses for ARI-v1
	P2PAddresses []string          `json:"p2p_addresses"` // libp2p multiaddrs
	Timestamp    int64             `json:"timestamp"`
	Status       string            `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// RuntimeRegistryStats represents aggregate registry statistics
type RuntimeRegistryStats struct {
	Total            int            `json:"total"`
	StatusCounts     map[string]int `json:"status_counts"`
	CapabilityCounts map[string]int `json:"capability_counts"`
	PresenceTopic    string         `json:"presence_topic"`
	LastUpdated      time.Time      `json:"last_updated"`
}

// GetBootstrapAddrs returns bootstrap node addresses from environment
func GetBootstrapAddrs() []string {
	bootstrap := os.Getenv("P2P_BOOTSTRAP")
	if bootstrap == "" {
		return nil
	}
	return strings.Split(bootstrap, ",")
}

// ParseMultiaddr parses a multiaddress string
func ParseMultiaddr(addr string) (multiaddr.Multiaddr, error) {
	return multiaddr.NewMultiaddr(addr)
}

// NewRuntimeRegistry creates a new runtime registry
func NewRuntimeRegistry(ctx context.Context, logger *zap.Logger, promMetrics *metrics.PrometheusMetrics) (*RuntimeRegistry, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create libp2p host
	h, err := libp2p.New(
		libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	// Connect to bootstrap nodes if provided
	bootstrapAddrs := GetBootstrapAddrs()
	if len(bootstrapAddrs) > 0 {
		logger.Info("connecting to bootstrap nodes", zap.Int("count", len(bootstrapAddrs)))
		for _, addrStr := range bootstrapAddrs {
			addr, err := ParseMultiaddr(addrStr)
			if err != nil {
				logger.Warn("failed to parse bootstrap address", zap.String("addr", addrStr), zap.Error(err))
				continue
			}

			peerInfo, err := peer.AddrInfoFromP2pAddr(addr)
			if err != nil {
				logger.Warn("failed to get peer info from address", zap.String("addr", addrStr), zap.Error(err))
				continue
			}

			if err := h.Connect(ctx, *peerInfo); err != nil {
				logger.Warn("failed to connect to bootstrap node", zap.String("peer_id", peerInfo.ID.String()), zap.Error(err))
			} else {
				logger.Info("connected to bootstrap node", zap.String("peer_id", peerInfo.ID.String()))
			}
		}
	}

	// Create GossipSub with relaxed settings for local testing
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithMessageSigning(false), // Disable signing for local testing
		pubsub.WithStrictSignatureVerification(false),
		pubsub.WithFloodPublish(true), // Use flood publish for better local delivery
		pubsub.WithPeerExchange(true),
	)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("failed to create gossipsub: %w", err)
	}

	regCtx, cancel := context.WithCancel(ctx)

	r := &RuntimeRegistry{
		host:                 h,
		pubsub:               ps,
		runtimes:             make(map[string]*RuntimeInfo),
		logger:               logger,
		ctx:                  regCtx,
		cancel:               cancel,
		metrics:              promMetrics,
		lastStatusLabels:     make(map[string]struct{}),
		lastCapabilityLabels: make(map[string]struct{}),
		presenceTopic:        "unknown",
	}

	logger.Info("runtime registry created",
		zap.String("peer_id", h.ID().String()),
	)

	return r, nil
}

// SubscribeToPresence subscribes to runtime presence announcements
func (r *RuntimeRegistry) SubscribeToPresence(topicPattern string) error {
	// For now, subscribe to global presence topic
	// TODO: Support wildcards/patterns for multiple topics
	topic, err := r.pubsub.Join(topicPattern)
	if err != nil {
		return fmt.Errorf("failed to join topic %s: %w", topicPattern, err)
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return fmt.Errorf("failed to subscribe to topic: %w", err)
	}

	r.logger.Info("subscribed to presence topic",
		zap.String("topic", topicPattern),
	)

	r.mu.Lock()
	r.presenceTopic = topicPattern
	r.mu.Unlock()

	// Start message handler
	go r.handlePresenceMessages(sub)

	// Start cleanup goroutine to remove stale runtimes
	go r.cleanupStaleRuntimes()

	return nil
}

// handlePresenceMessages processes incoming presence messages
func (r *RuntimeRegistry) handlePresenceMessages(sub *pubsub.Subscription) {
	for {
		msg, err := sub.Next(r.ctx)
		if err != nil {
			if r.ctx.Err() != nil {
				return // Context cancelled
			}
			r.logger.Error("failed to get next message", zap.Error(err))
			continue
		}

		var presence PresenceMessage
		if err := json.Unmarshal(msg.Data, &presence); err != nil {
			r.logger.Error("failed to unmarshal presence message", zap.Error(err))
			continue
		}

		r.updateRuntime(&presence)
	}
}

// updateRuntime updates or adds a runtime to the registry
func (r *RuntimeRegistry) updateRuntime(presence *PresenceMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Extract primary gRPC address
	grpcAddr := ""
	if len(presence.Addresses) > 0 {
		grpcAddr = presence.Addresses[0]
	}

	info := &RuntimeInfo{
		DID:          presence.DID,
		Name:         presence.Name,
		Capabilities: presence.Capabilities,
		GRPCAddress:  grpcAddr,
		P2PAddresses: presence.P2PAddresses,
		LastSeen:     time.Now(),
		Status:       presence.Status,
		Metadata:     presence.Metadata,
	}

	if presence.Status == "offline" {
		if _, ok := r.runtimes[presence.DID]; ok {
			delete(r.runtimes, presence.DID)
			r.logger.Info("runtime went offline",
				zap.String("did", presence.DID),
				zap.String("name", presence.Name),
			)
			r.recordEventLocked("removed")
			r.updateMetricsLocked()
		}
		return
	}

	_, existed := r.runtimes[presence.DID]
	r.runtimes[presence.DID] = info
	r.lastUpdated = time.Now()

	if existed {
		r.logger.Info("runtime updated",
			zap.String("did", presence.DID),
			zap.String("name", presence.Name),
			zap.String("grpc_address", grpcAddr),
			zap.Strings("capabilities", presence.Capabilities),
		)
		r.recordEventLocked("updated")
	} else {
		r.logger.Info("runtime discovered",
			zap.String("did", presence.DID),
			zap.String("name", presence.Name),
			zap.String("grpc_address", grpcAddr),
			zap.Strings("capabilities", presence.Capabilities),
		)
		r.recordEventLocked("discovered")
	}

	r.updateMetricsLocked()
}

// cleanupStaleRuntimes removes runtimes that haven't sent heartbeats
func (r *RuntimeRegistry) cleanupStaleRuntimes() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			now := time.Now()
			for did, info := range r.runtimes {
				// Remove runtimes that haven't been seen in 2 minutes
				if now.Sub(info.LastSeen) > 2*time.Minute {
					delete(r.runtimes, did)
					r.logger.Warn("runtime timed out",
						zap.String("did", did),
						zap.String("name", info.Name),
						zap.Duration("since_last_seen", now.Sub(info.LastSeen)),
					)
					r.recordEventLocked("timed_out")
				}
			}
			r.lastUpdated = now
			r.updateMetricsLocked()
			r.mu.Unlock()
		}
	}
}

// GetRuntimeByCapabilities returns runtimes that have the required capabilities
func (r *RuntimeRegistry) GetRuntimeByCapabilities(capabilities []string) []*RuntimeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matches []*RuntimeInfo

	for _, runtime := range r.runtimes {
		if runtime.Status != "online" {
			continue
		}

		// Check if runtime has all required capabilities
		hasAll := true
		for _, required := range capabilities {
			found := false
			for _, cap := range runtime.Capabilities {
				if cap == required {
					found = true
					break
				}
			}
			if !found {
				hasAll = false
				break
			}
		}

		if hasAll {
			matches = append(matches, runtime)
		}
	}

	return matches
}

// GetAllRuntimes returns all registered runtimes
func (r *RuntimeRegistry) GetAllRuntimes() []*RuntimeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runtimes := make([]*RuntimeInfo, 0, len(r.runtimes))
	for _, info := range r.runtimes {
		clone := *info
		runtimes = append(runtimes, &clone)
	}

	return runtimes
}

// GetRuntime returns runtime information for a specific DID
func (r *RuntimeRegistry) GetRuntime(did string) *RuntimeInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.runtimes[did]
	if !ok {
		return nil
	}
	clone := *info
	return &clone
}

// GetStats returns aggregate runtime registry stats
func (r *RuntimeRegistry) GetStats() *RuntimeRegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statusCounts := make(map[string]int)
	capabilityCounts := make(map[string]int)

	for _, info := range r.runtimes {
		status := info.Status
		if status == "" {
			status = "unknown"
		}
		statusCounts[status]++

		for _, capability := range info.Capabilities {
			capabilityCounts[capability]++
		}
	}

	return &RuntimeRegistryStats{
		Total:            len(r.runtimes),
		StatusCounts:     statusCounts,
		CapabilityCounts: capabilityCounts,
		PresenceTopic:    r.presenceTopic,
		LastUpdated:      r.lastUpdated,
	}
}

// Close stops the registry and cleans up resources
func (r *RuntimeRegistry) Close() error {
	r.cancel()
	return r.host.Close()
}

func (r *RuntimeRegistry) recordEventLocked(event string) {
	if r.metrics != nil {
		r.metrics.RecordRuntimeEvent(event)
	}
}

func (r *RuntimeRegistry) updateMetricsLocked() {
	if r.metrics == nil {
		return
	}

	r.metrics.UpdateRuntimeCount(len(r.runtimes))

	statusCounts := make(map[string]int)
	capabilityCounts := make(map[string]int)

	for _, info := range r.runtimes {
		status := info.Status
		if status == "" {
			status = "unknown"
		}
		statusCounts[status]++

		for _, capability := range info.Capabilities {
			capabilityCounts[capability]++
		}
	}

	defaultStatuses := []string{"online", "offline", "busy"}
	for _, status := range defaultStatuses {
		if _, ok := statusCounts[status]; !ok {
			statusCounts[status] = 0
		}
	}

	for status, count := range statusCounts {
		r.metrics.UpdateRuntimeStatus(status, count)
	}

	for capability, count := range capabilityCounts {
		r.metrics.UpdateRuntimeCapability(capability, count)
	}

	for status := range r.lastStatusLabels {
		if _, ok := statusCounts[status]; !ok {
			r.metrics.UpdateRuntimeStatus(status, 0)
			delete(r.lastStatusLabels, status)
		}
	}
	for status := range statusCounts {
		r.lastStatusLabels[status] = struct{}{}
	}

	for capability := range r.lastCapabilityLabels {
		if _, ok := capabilityCounts[capability]; !ok {
			r.metrics.UpdateRuntimeCapability(capability, 0)
			delete(r.lastCapabilityLabels, capability)
		}
	}
	for capability := range capabilityCounts {
		r.lastCapabilityLabels[capability] = struct{}{}
	}
}
