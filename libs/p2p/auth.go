package p2p

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	authVerifications = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_auth_verifications_total",
			Help: "Total signature verification attempts",
		},
		[]string{"result"}, // success, failure_signature, failure_did_mismatch, failure_expired
	)

	authPublishAttempts = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_auth_publish_attempts_total",
			Help: "Total authenticated publish attempts",
		},
		[]string{"result"}, // allowed, rejected
	)
)

// SignedAgentCard wraps an Agent Card with signature and timestamp
type SignedAgentCard struct {
	Card      json.RawMessage `json:"card"`       // The actual agent card
	Signature string          `json:"signature"`  // Hex-encoded signature
	Timestamp int64           `json:"timestamp"`  // Unix timestamp
	PublicKey string          `json:"public_key"` // Hex-encoded Ed25519 public key
}

// AgentCardValidator provides signature verification for DHT writes
type AgentCardValidator struct {
	logger      *zap.Logger
	maxAge      time.Duration // Maximum age for signed cards
	enableAuth  bool          // Enable/disable authentication
}

// NewAgentCardValidator creates a new validator
func NewAgentCardValidator(logger *zap.Logger, enableAuth bool) *AgentCardValidator {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AgentCardValidator{
		logger:     logger,
		maxAge:     1 * time.Hour, // Cards expire after 1 hour
		enableAuth: enableAuth,
	}
}

// SignCard signs an agent card with the given private key
func (v *AgentCardValidator) SignCard(card []byte, privKey ed25519.PrivateKey) (*SignedAgentCard, error) {
	timestamp := time.Now().Unix()

	// Create message to sign: card + timestamp
	message := append(card, []byte(fmt.Sprintf("%d", timestamp))...)
	
	// Sign the message
	signature := ed25519.Sign(privKey, message)

	pubKey := privKey.Public().(ed25519.PublicKey)

	signed := &SignedAgentCard{
		Card:      json.RawMessage(card),
		Signature: hex.EncodeToString(signature),
		Timestamp: timestamp,
		PublicKey: hex.EncodeToString(pubKey),
	}

	return signed, nil
}

// VerifySignedCard verifies a signed agent card
func (v *AgentCardValidator) VerifySignedCard(ctx context.Context, signed *SignedAgentCard) error {
	if !v.enableAuth {
		v.logger.Debug("auth disabled, skipping verification")
		authVerifications.WithLabelValues("skipped").Inc()
		return nil
	}

	// Check timestamp
	cardTime := time.Unix(signed.Timestamp, 0)
	age := time.Since(cardTime)
	
	if age > v.maxAge {
		authVerifications.WithLabelValues("failure_expired").Inc()
		return fmt.Errorf("card expired: age=%v, max=%v", age, v.maxAge)
	}

	if age < -5*time.Minute {
		authVerifications.WithLabelValues("failure_future").Inc()
		return fmt.Errorf("card timestamp in future: %v", cardTime)
	}

	// Decode public key
	pubKeyBytes, err := hex.DecodeString(signed.PublicKey)
	if err != nil {
		authVerifications.WithLabelValues("failure_invalid_key").Inc()
		return fmt.Errorf("invalid public key: %w", err)
	}

	pubKey := ed25519.PublicKey(pubKeyBytes)

	// Decode signature
	sigBytes, err := hex.DecodeString(signed.Signature)
	if err != nil {
		authVerifications.WithLabelValues("failure_invalid_signature").Inc()
		return fmt.Errorf("invalid signature: %w", err)
	}

	// Verify signature
	message := append(signed.Card, []byte(fmt.Sprintf("%d", signed.Timestamp))...)
	
	if !ed25519.Verify(pubKey, message, sigBytes) {
		authVerifications.WithLabelValues("failure_signature").Inc()
		return fmt.Errorf("signature verification failed")
	}

	// Parse card to get DID
	var cardData map[string]interface{}
	if err := json.Unmarshal(signed.Card, &cardData); err != nil {
		return fmt.Errorf("failed to parse card: %w", err)
	}

	cardDID, ok := cardData["did"].(string)
	if !ok {
		authVerifications.WithLabelValues("failure_no_did").Inc()
		return fmt.Errorf("card missing DID")
	}

	// Verify DID matches public key
	libp2pKey, err := crypto.UnmarshalEd25519PublicKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to unmarshal libp2p key: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(libp2pKey)
	if err != nil {
		return fmt.Errorf("failed to derive peer ID: %w", err)
	}

	// DID should be "did:zs:<peerID>"
	expectedDID := fmt.Sprintf("did:zs:%s", peerID.String())
	
	if cardDID != expectedDID {
		authVerifications.WithLabelValues("failure_did_mismatch").Inc()
		v.logger.Warn("DID mismatch",
			zap.String("card_did", cardDID),
			zap.String("expected_did", expectedDID),
		)
		return fmt.Errorf("DID mismatch: card=%s, expected=%s", cardDID, expectedDID)
	}

	authVerifications.WithLabelValues("success").Inc()
	
	v.logger.Debug("card signature verified",
		zap.String("did", cardDID),
		zap.Duration("age", age),
	)

	return nil
}

// ValidatePublish validates a card before publishing to DHT
func (v *AgentCardValidator) ValidatePublish(ctx context.Context, signedCard *SignedAgentCard) error {
	if err := v.VerifySignedCard(ctx, signedCard); err != nil {
		authPublishAttempts.WithLabelValues("rejected").Inc()
		return fmt.Errorf("validation failed: %w", err)
	}

	authPublishAttempts.WithLabelValues("allowed").Inc()
	return nil
}

// SetMaxAge configures maximum age for cards
func (v *AgentCardValidator) SetMaxAge(d time.Duration) {
	v.maxAge = d
}

// Enable enables authentication
func (v *AgentCardValidator) Enable() {
	v.enableAuth = true
	v.logger.Info("authentication enabled")
}

// Disable disables authentication (for testing)
func (v *AgentCardValidator) Disable() {
	v.enableAuth = false
	v.logger.Warn("authentication disabled")
}
