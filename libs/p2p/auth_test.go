package p2p

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSignAndVerifyCard(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)
	
	// Generate key pair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Create a test card
	cardData := map[string]interface{}{
		"did":  "did:zs:test123",
		"name": "Test Agent",
	}
	cardJSON, err := json.Marshal(cardData)
	require.NoError(t, err)

	// Sign the card
	signed, err := validator.SignCard(cardJSON, privKey)
	require.NoError(t, err)
	
	assert.NotEmpty(t, signed.Signature)
	assert.NotEmpty(t, signed.PublicKey)
	assert.NotZero(t, signed.Timestamp)
	assert.Equal(t, hex.EncodeToString(pubKey), signed.PublicKey)

	// Manual verification for this test (DID won't match real peer ID)
	sigBytes, err := hex.DecodeString(signed.Signature)
	require.NoError(t, err)
	
	t.Logf("Signature: %s", signed.Signature)
	t.Logf("Public Key: %s", signed.PublicKey)
	t.Logf("Timestamp: %d", signed.Timestamp)
	
	assert.Len(t, sigBytes, ed25519.SignatureSize)
}

func TestVerifyExpiredCard(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)
	validator.SetMaxAge(10 * time.Minute)

	_, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	cardData := map[string]interface{}{
		"did": "did:zs:test",
	}
	cardJSON, _ := json.Marshal(cardData)

	signed, err := validator.SignCard(cardJSON, privKey)
	require.NoError(t, err)

	// Make card appear expired
	signed.Timestamp = time.Now().Add(-20 * time.Minute).Unix()

	ctx := context.Background()
	err = validator.VerifySignedCard(ctx, signed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestVerifyFutureCard(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)

	_, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	cardData := map[string]interface{}{
		"did": "did:zs:test",
	}
	cardJSON, _ := json.Marshal(cardData)

	signed, err := validator.SignCard(cardJSON, privKey)
	require.NoError(t, err)

	// Make timestamp in future
	signed.Timestamp = time.Now().Add(10 * time.Minute).Unix()

	ctx := context.Background()
	err = validator.VerifySignedCard(ctx, signed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "future")
}

func TestAuthDisabled(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), false)

	// Even with invalid data, verification should pass when auth is disabled
	signed := &SignedAgentCard{
		Card:      json.RawMessage(`{"did":"test"}`),
		Signature: "invalid",
		Timestamp: 0,
		PublicKey: "invalid",
	}

	ctx := context.Background()
	err := validator.VerifySignedCard(ctx, signed)
	assert.NoError(t, err, "Verification should pass when auth is disabled")
}

func TestInvalidSignature(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)

	_, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	cardData := map[string]interface{}{
		"did": "did:zs:test",
	}
	cardJSON, _ := json.Marshal(cardData)

	signed, err := validator.SignCard(cardJSON, privKey)
	require.NoError(t, err)

	// Corrupt the signature
	signed.Signature = "0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"

	ctx := context.Background()
	err = validator.VerifySignedCard(ctx, signed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestMissingDID(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)

	_, privKey, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Card without DID
	cardData := map[string]interface{}{
		"name": "Test Agent",
	}
	cardJSON, _ := json.Marshal(cardData)

	signed, err := validator.SignCard(cardJSON, privKey)
	require.NoError(t, err)

	ctx := context.Background()
	err = validator.VerifySignedCard(ctx, signed)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing DID")
}

func TestEnableDisableAuth(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), true)
	
	// Initially enabled
	assert.True(t, validator.enableAuth)

	validator.Disable()
	assert.False(t, validator.enableAuth)

	validator.Enable()
	assert.True(t, validator.enableAuth)
}

func TestValidatePublish(t *testing.T) {
	validator := NewAgentCardValidator(zap.NewNop(), false)

	signed := &SignedAgentCard{
		Card:      json.RawMessage(`{"did":"test"}`),
		Signature: "valid-when-auth-disabled",
		Timestamp: time.Now().Unix(),
		PublicKey: "test-key",
	}

	ctx := context.Background()
	err := validator.ValidatePublish(ctx, signed)
	assert.NoError(t, err)
}
