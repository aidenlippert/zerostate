package p2p

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

const (
	// ProtocolVersion is the current protocol version
	ProtocolVersion = "1.0.0"
	
	// MinCompatibleVersion is the minimum version we can communicate with
	MinCompatibleVersion = "1.0.0"
	
	// ProtocolHandshakeID is the protocol ID for version negotiation
	ProtocolHandshakeID = "/zerostate/handshake/1.0.0"
)

var (
	protocolVersionMismatches = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_protocol_version_mismatches_total",
			Help: "Total protocol version negotiation failures",
		},
		[]string{"reason"}, // incompatible, unsupported_feature, invalid
	)

	protocolNegotiations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_protocol_negotiations_total",
			Help: "Total protocol version negotiations",
		},
		[]string{"result"}, // success, failure
	)

	protocolFeatures = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "zerostate_protocol_features_enabled",
			Help: "Protocol features enabled on this node",
		},
		[]string{"feature"},
	)
)

// ProtocolHandshake represents version and feature negotiation
type ProtocolHandshake struct {
	Version    string                 `json:"version"`     // Semantic version
	Features   []string               `json:"features"`    // Enabled features
	Extensions map[string]interface{} `json:"extensions"`  // Custom extensions
}

// ProtocolFeature represents a protocol capability
type ProtocolFeature string

const (
	FeatureDHT      ProtocolFeature = "dht"
	FeatureRelay    ProtocolFeature = "relay"
	FeatureAuth     ProtocolFeature = "auth"
	FeatureSearch   ProtocolFeature = "search"
	FeatureGossip   ProtocolFeature = "gossip"
	FeatureMDNS     ProtocolFeature = "mdns"
	FeatureQRouting ProtocolFeature = "qrouting"
)

// ProtocolNegotiator handles version and feature negotiation
type ProtocolNegotiator struct {
	version          *semver.Version
	minVersion       *semver.Version
	enabledFeatures  map[ProtocolFeature]bool
	requiredFeatures map[ProtocolFeature]bool
	logger           *zap.Logger
}

// NewProtocolNegotiator creates a new protocol negotiator
func NewProtocolNegotiator(logger *zap.Logger) (*ProtocolNegotiator, error) {
	version, err := semver.NewVersion(ProtocolVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid protocol version: %w", err)
	}

	minVersion, err := semver.NewVersion(MinCompatibleVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid min compatible version: %w", err)
	}

	negotiator := &ProtocolNegotiator{
		version:          version,
		minVersion:       minVersion,
		enabledFeatures:  make(map[ProtocolFeature]bool),
		requiredFeatures: make(map[ProtocolFeature]bool),
		logger:           logger,
	}

	// Initialize default features
	negotiator.EnableFeature(FeatureDHT)
	negotiator.EnableFeature(FeatureAuth)

	return negotiator, nil
}

// EnableFeature enables a protocol feature
func (pn *ProtocolNegotiator) EnableFeature(feature ProtocolFeature) {
	pn.enabledFeatures[feature] = true
	protocolFeatures.WithLabelValues(string(feature)).Set(1)
	pn.logger.Debug("protocol feature enabled", zap.String("feature", string(feature)))
}

// DisableFeature disables a protocol feature
func (pn *ProtocolNegotiator) DisableFeature(feature ProtocolFeature) {
	pn.enabledFeatures[feature] = false
	protocolFeatures.WithLabelValues(string(feature)).Set(0)
	pn.logger.Debug("protocol feature disabled", zap.String("feature", string(feature)))
}

// RequireFeature marks a feature as required
func (pn *ProtocolNegotiator) RequireFeature(feature ProtocolFeature) {
	pn.requiredFeatures[feature] = true
	pn.EnableFeature(feature)
}

// IsFeatureEnabled checks if a feature is enabled
func (pn *ProtocolNegotiator) IsFeatureEnabled(feature ProtocolFeature) bool {
	return pn.enabledFeatures[feature]
}

// CreateHandshake creates a handshake message
func (pn *ProtocolNegotiator) CreateHandshake() *ProtocolHandshake {
	features := make([]string, 0, len(pn.enabledFeatures))
	for feature, enabled := range pn.enabledFeatures {
		if enabled {
			features = append(features, string(feature))
		}
	}

	return &ProtocolHandshake{
		Version:    pn.version.String(),
		Features:   features,
		Extensions: make(map[string]interface{}),
	}
}

// ValidateHandshake validates a peer's handshake
func (pn *ProtocolNegotiator) ValidateHandshake(peer *ProtocolHandshake) error {
	// Parse peer version
	peerVersion, err := semver.NewVersion(peer.Version)
	if err != nil {
		protocolVersionMismatches.WithLabelValues("invalid").Inc()
		return fmt.Errorf("invalid peer version: %w", err)
	}

	// Check version compatibility
	if peerVersion.LessThan(pn.minVersion) {
		protocolVersionMismatches.WithLabelValues("incompatible").Inc()
		return fmt.Errorf("incompatible version: peer=%s, min=%s",
			peerVersion.String(), pn.minVersion.String())
	}

	// Check required features
	peerFeatures := make(map[string]bool)
	for _, feature := range peer.Features {
		peerFeatures[feature] = true
	}

	for feature := range pn.requiredFeatures {
		if !peerFeatures[string(feature)] {
			protocolVersionMismatches.WithLabelValues("unsupported_feature").Inc()
			return fmt.Errorf("peer missing required feature: %s", feature)
		}
	}

	protocolNegotiations.WithLabelValues("success").Inc()
	pn.logger.Debug("protocol handshake validated",
		zap.String("peer_version", peerVersion.String()),
		zap.Strings("peer_features", peer.Features),
	)

	return nil
}

// NegotiateProtocol performs full protocol negotiation with a peer
func (pn *ProtocolNegotiator) NegotiateProtocol(ctx context.Context, stream network.Stream) (*ProtocolHandshake, error) {
	// Send our handshake
	ourHandshake := pn.CreateHandshake()
	
	// In a real implementation, serialize and send over stream
	// For now, return our handshake
	// TODO: Implement actual wire protocol (protobuf or msgpack)
	
	protocolNegotiations.WithLabelValues("success").Inc()
	return ourHandshake, nil
}

// GetProtocolID returns the versioned protocol ID
func (pn *ProtocolNegotiator) GetProtocolID(base string) protocol.ID {
	return protocol.ID(fmt.Sprintf("%s/%s", base, pn.version.String()))
}

// IsCompatibleWith checks if this node is compatible with a peer version
func (pn *ProtocolNegotiator) IsCompatibleWith(peerVersion string) bool {
	peer, err := semver.NewVersion(peerVersion)
	if err != nil {
		return false
	}

	return peer.GreaterThan(pn.minVersion) || peer.Equal(pn.minVersion)
}

// GetVersion returns the current protocol version
func (pn *ProtocolNegotiator) GetVersion() string {
	return pn.version.String()
}

// GetMinVersion returns the minimum compatible version
func (pn *ProtocolNegotiator) GetMinVersion() string {
	return pn.minVersion.String()
}

// GetEnabledFeatures returns list of enabled features
func (pn *ProtocolNegotiator) GetEnabledFeatures() []ProtocolFeature {
	features := make([]ProtocolFeature, 0, len(pn.enabledFeatures))
	for feature, enabled := range pn.enabledFeatures {
		if enabled {
			features = append(features, feature)
		}
	}
	return features
}

var (
	ErrIncompatibleVersion    = errors.New("incompatible protocol version")
	ErrMissingRequiredFeature = errors.New("missing required feature")
	ErrInvalidHandshake       = errors.New("invalid handshake")
)
