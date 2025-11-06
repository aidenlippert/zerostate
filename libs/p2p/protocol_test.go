package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewProtocolNegotiator(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)

	require.NoError(t, err)
	assert.NotNil(t, negotiator)
	assert.Equal(t, "1.0.0", negotiator.GetVersion())
	assert.Equal(t, "1.0.0", negotiator.GetMinVersion())
	
	// Default features should be enabled
	assert.True(t, negotiator.IsFeatureEnabled(FeatureDHT))
	assert.True(t, negotiator.IsFeatureEnabled(FeatureAuth))
}

func TestFeatureManagement(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	// Enable a feature
	negotiator.EnableFeature(FeatureRelay)
	assert.True(t, negotiator.IsFeatureEnabled(FeatureRelay))

	// Disable a feature
	negotiator.DisableFeature(FeatureRelay)
	assert.False(t, negotiator.IsFeatureEnabled(FeatureRelay))

	// Require a feature
	negotiator.RequireFeature(FeatureSearch)
	assert.True(t, negotiator.IsFeatureEnabled(FeatureSearch))
	assert.True(t, negotiator.requiredFeatures[FeatureSearch])
}

func TestCreateHandshake(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	negotiator.EnableFeature(FeatureRelay)
	negotiator.EnableFeature(FeatureSearch)

	handshake := negotiator.CreateHandshake()

	assert.Equal(t, "1.0.0", handshake.Version)
	assert.Contains(t, handshake.Features, "dht")
	assert.Contains(t, handshake.Features, "auth")
	assert.Contains(t, handshake.Features, "relay")
	assert.Contains(t, handshake.Features, "search")
	assert.NotNil(t, handshake.Extensions)
}

func TestValidateHandshake_Compatible(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	peerHandshake := &ProtocolHandshake{
		Version:    "1.0.0",
		Features:   []string{"dht", "auth"},
		Extensions: make(map[string]interface{}),
	}

	err = negotiator.ValidateHandshake(peerHandshake)
	assert.NoError(t, err)
}

func TestValidateHandshake_CompatibleNewerVersion(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	peerHandshake := &ProtocolHandshake{
		Version:    "1.2.0",
		Features:   []string{"dht", "auth", "newfeature"},
		Extensions: make(map[string]interface{}),
	}

	err = negotiator.ValidateHandshake(peerHandshake)
	assert.NoError(t, err)
}

func TestValidateHandshake_IncompatibleVersion(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	peerHandshake := &ProtocolHandshake{
		Version:    "0.9.0",
		Features:   []string{"dht", "auth"},
		Extensions: make(map[string]interface{}),
	}

	err = negotiator.ValidateHandshake(peerHandshake)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible version")
}

func TestValidateHandshake_MissingRequiredFeature(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	negotiator.RequireFeature(FeatureSearch)

	peerHandshake := &ProtocolHandshake{
		Version:    "1.0.0",
		Features:   []string{"dht", "auth"}, // Missing "search"
		Extensions: make(map[string]interface{}),
	}

	err = negotiator.ValidateHandshake(peerHandshake)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing required feature")
}

func TestValidateHandshake_InvalidVersion(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	peerHandshake := &ProtocolHandshake{
		Version:    "invalid",
		Features:   []string{"dht"},
		Extensions: make(map[string]interface{}),
	}

	err = negotiator.ValidateHandshake(peerHandshake)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid peer version")
}

func TestIsCompatibleWith(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	tests := []struct {
		name     string
		version  string
		expected bool
	}{
		{"exact match", "1.0.0", true},
		{"newer patch", "1.0.1", true},
		{"newer minor", "1.1.0", true},
		{"newer major", "2.0.0", true},
		{"older patch", "0.9.9", false},
		{"older minor", "0.9.0", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := negotiator.IsCompatibleWith(tt.version)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetProtocolID(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	protocolID := negotiator.GetProtocolID("/zerostate/test")
	assert.Equal(t, "/zerostate/test/1.0.0", string(protocolID))
}

func TestGetEnabledFeatures(t *testing.T) {
	logger := zap.NewNop()
	negotiator, err := NewProtocolNegotiator(logger)
	require.NoError(t, err)

	negotiator.EnableFeature(FeatureRelay)
	negotiator.EnableFeature(FeatureSearch)

	features := negotiator.GetEnabledFeatures()

	assert.Contains(t, features, FeatureDHT)
	assert.Contains(t, features, FeatureAuth)
	assert.Contains(t, features, FeatureRelay)
	assert.Contains(t, features, FeatureSearch)
}
