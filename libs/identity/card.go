// Package identity provides DID-based identity and Agent Card signing/verification.
package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/multiformats/go-multibase"
	"go.uber.org/zap"
)

// AgentCard represents a signed agent identity card (simplified schema)
type AgentCard struct {
	Context      interface{}            `json:"@context,omitempty"`
	ID           string                 `json:"id,omitempty"`
	Type         interface{}            `json:"type,omitempty"`
	DID          string                 `json:"did"`
	Keys         *Keys                  `json:"keys,omitempty"`
	Endpoints    *Endpoints             `json:"endpoints"`
	Capabilities []Capability           `json:"capabilities"`
	Embeddings   *Embeddings            `json:"embeddings,omitempty"`
	Reputation   *Reputation            `json:"reputation,omitempty"`
	Policy       *Policy                `json:"policy,omitempty"`
	Proof        *Proof                 `json:"proof"`
}

// Keys holds public keys
type Keys struct {
	Signing     string `json:"signing"`
	Encryption  string `json:"encryption,omitempty"`
	PostQuantum string `json:"postQuantum,omitempty"`
}

// Endpoints holds network endpoints
type Endpoints struct {
	Libp2p []string `json:"libp2p"`
	HTTP   []string `json:"http,omitempty"`
	Region string   `json:"region,omitempty"`
}

// Capability describes an agent capability
type Capability struct {
	Name     string                 `json:"name"`
	Version  string                 `json:"version"`
	Cost     *Cost                  `json:"cost,omitempty"`
	Limits   map[string]interface{} `json:"limits,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Cost describes pricing
type Cost struct {
	Unit  string  `json:"unit"`
	Price float64 `json:"price"`
}

// Embeddings holds capability vectors
type Embeddings struct {
	Model   string      `json:"model"`
	Vectors [][]float64 `json:"vectors"`
}

// Reputation holds reputation info
type Reputation struct {
	Score         float64 `json:"score"`
	ZKAccumulator string  `json:"zkAccumulator,omitempty"`
}

// Policy holds operational policy
type Policy struct {
	SLAClass     string `json:"slaClass,omitempty"`
	EnergyBudget string `json:"energyBudget,omitempty"`
	Privacy      string `json:"privacy,omitempty"`
}

// Proof holds the cryptographic proof
type Proof struct {
	Type               string `json:"type"`
	Created            string `json:"created"`
	ProofPurpose       string `json:"proofPurpose"`
	VerificationMethod string `json:"verificationMethod"`
	JWS                string `json:"jws"`
}

// Signer handles signing operations
type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	did        string
	logger     *zap.Logger
}

// NewSigner creates a new signer with a fresh keypair
func NewSigner(logger *zap.Logger) (*Signer, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keypair: %w", err)
	}

	// Generate DID (simplified did:key)
	encoded, err := multibase.Encode(multibase.Base58BTC, pub)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	did := fmt.Sprintf("did:key:z%s", encoded[1:]) // Skip the multibase prefix

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Signer{
		privateKey: priv,
		publicKey:  pub,
		did:        did,
		logger:     logger,
	}, nil
}

// NewSignerFromKey creates a signer from an existing private key
func NewSignerFromKey(privateKey ed25519.PrivateKey, logger *zap.Logger) (*Signer, error) {
	pub := privateKey.Public().(ed25519.PublicKey)

	encoded, err := multibase.Encode(multibase.Base58BTC, pub)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	did := fmt.Sprintf("did:key:z%s", encoded[1:])

	if logger == nil {
		logger = zap.NewNop()
	}

	return &Signer{
		privateKey: privateKey,
		publicKey:  pub,
		did:        did,
		logger:     logger,
	}, nil
}

// DID returns the signer's DID
func (s *Signer) DID() string {
	return s.did
}

// PublicKeyBase58 returns the base58-encoded public key
func (s *Signer) PublicKeyBase58() string {
	encoded, _ := multibase.Encode(multibase.Base58BTC, s.publicKey)
	return encoded
}

// SignCard signs an agent card
func (s *Signer) SignCard(card *AgentCard) error {
	// Set DID if not set
	if card.DID == "" {
		card.DID = s.did
	}

	// Create canonical JSON (without proof)
	cardCopy := *card
	cardCopy.Proof = nil

	canonical, err := json.Marshal(cardCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}

	// Sign
	signature := ed25519.Sign(s.privateKey, canonical)
	jws := base64.RawURLEncoding.EncodeToString(signature)

	// Add proof
	card.Proof = &Proof{
		Type:               "Ed25519Signature2020",
		Created:            time.Now().UTC().Format(time.RFC3339),
		ProofPurpose:       "assertionMethod",
		VerificationMethod: s.did + "#signing",
		JWS:                jws,
	}

	s.logger.Debug("agent card signed",
		zap.String("did", card.DID),
		zap.String("proof_type", card.Proof.Type),
	)

	return nil
}

// VerifyCard verifies an agent card signature
func VerifyCard(card *AgentCard) error {
	if card.Proof == nil {
		return fmt.Errorf("no proof attached to card")
	}

	// Extract public key from DID (simplified for did:key)
	pubKey, err := publicKeyFromDID(card.DID)
	if err != nil {
		return fmt.Errorf("failed to extract public key from DID: %w", err)
	}

	// Decode signature
	signature, err := base64.RawURLEncoding.DecodeString(card.Proof.JWS)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Create canonical JSON (without proof)
	cardCopy := *card
	cardCopy.Proof = nil

	canonical, err := json.Marshal(cardCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal card: %w", err)
	}

	// Verify
	if !ed25519.Verify(pubKey, canonical, signature) {
		return fmt.Errorf("signature verification failed")
	}

	return nil
}

// publicKeyFromDID extracts the public key from a did:key DID
func publicKeyFromDID(did string) (ed25519.PublicKey, error) {
	// Simplified extraction for did:key:z...
	if len(did) < 13 || did[:9] != "did:key:z" {
		return nil, fmt.Errorf("invalid DID format")
	}

	encoded := "z" + did[9:] // Re-add multibase prefix
	_, decoded, err := multibase.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DID: %w", err)
	}

	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	return ed25519.PublicKey(decoded), nil
}
