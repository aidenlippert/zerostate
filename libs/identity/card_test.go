package identity

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewSigner(t *testing.T) {
	logger := zaptest.NewLogger(t)
	signer, err := NewSigner(logger)
	require.NoError(t, err)
	require.NotNil(t, signer)

	assert.NotEmpty(t, signer.DID())
	assert.Contains(t, signer.DID(), "did:key:z")
	assert.NotEmpty(t, signer.PublicKeyBase58())
}

func TestSignAndVerifyCard(t *testing.T) {
	logger := zaptest.NewLogger(t)
	signer, err := NewSigner(logger)
	require.NoError(t, err)

	card := &AgentCard{
		DID: signer.DID(),
		Endpoints: &Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/udp/4001/quic-v1"},
			Region: "us-west-1",
		},
		Capabilities: []Capability{
			{
				Name:    "test.capability",
				Version: "1.0.0",
			},
		},
	}

	// Sign
	err = signer.SignCard(card)
	require.NoError(t, err)
	require.NotNil(t, card.Proof)
	assert.Equal(t, "Ed25519Signature2020", card.Proof.Type)
	assert.NotEmpty(t, card.Proof.JWS)

	// Verify
	err = VerifyCard(card)
	require.NoError(t, err)
}

func TestVerifyCardTampered(t *testing.T) {
	logger := zaptest.NewLogger(t)
	signer, err := NewSigner(logger)
	require.NoError(t, err)

	card := &AgentCard{
		DID: signer.DID(),
		Endpoints: &Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/udp/4001/quic-v1"},
		},
		Capabilities: []Capability{
			{
				Name:    "test.capability",
				Version: "1.0.0",
			},
		},
	}

	err = signer.SignCard(card)
	require.NoError(t, err)

	// Tamper with the card
	card.Capabilities[0].Name = "tampered.capability"

	// Verify should fail
	err = VerifyCard(card)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestVerifyCardNoProof(t *testing.T) {
	card := &AgentCard{
		DID: "did:key:z6Mktest",
		Endpoints: &Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/udp/4001/quic-v1"},
		},
		Capabilities: []Capability{},
	}

	err := VerifyCard(card)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no proof")
}

func TestCardSerialization(t *testing.T) {
	logger := zaptest.NewLogger(t)
	signer, err := NewSigner(logger)
	require.NoError(t, err)

	card := &AgentCard{
		Context: "https://www.w3.org/2018/credentials/v1",
		Type:    "zs:AgentCard",
		DID:     signer.DID(),
		Endpoints: &Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/udp/4001/quic-v1"},
			Region: "us-west-1",
		},
		Capabilities: []Capability{
			{
				Name:    "embeddings.hnsw.query",
				Version: "1.0.0",
				Cost: &Cost{
					Unit:  "req",
					Price: 0.0001,
				},
			},
		},
		Reputation: &Reputation{
			Score: 0.85,
		},
		Policy: &Policy{
			SLAClass: "regional",
			Privacy:  "guild",
		},
	}

	err = signer.SignCard(card)
	require.NoError(t, err)

	// Serialize to JSON
	data, err := json.MarshalIndent(card, "", "  ")
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Deserialize
	var card2 AgentCard
	err = json.Unmarshal(data, &card2)
	require.NoError(t, err)

	// Verify deserialized card
	err = VerifyCard(&card2)
	require.NoError(t, err)
}

func TestNewSignerFromKey(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Generate a key
	_, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	// Create signer from existing key
	signer, err := NewSignerFromKey(priv, logger)
	require.NoError(t, err)
	require.NotNil(t, signer)

	assert.NotEmpty(t, signer.DID())
	assert.Contains(t, signer.DID(), "did:key:z")
}
