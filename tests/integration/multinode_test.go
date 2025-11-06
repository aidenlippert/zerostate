package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"zerostate/libs/identity"
	"zerostate/libs/p2p"
)

// TestMultiNodePublishResolve tests Agent Card publish/resolve across a 3-node DHT network
func TestMultiNodePublishResolve(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Step 1: Create bootstrap node (DHT server)
	t.Log("Creating bootstrap node...")
	bootstrapCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/0.0.0.0/tcp/0"},
		EnableDHT:   true,
		DHTMode:     dht.ModeServer,
	}
	bootstrap, err := p2p.NewNode(ctx, bootstrapCfg)
	require.NoError(t, err, "Failed to create bootstrap node")
	defer bootstrap.Close()

	bootstrapAddr := fmt.Sprintf("%s/p2p/%s", bootstrap.Host().Addrs()[0].String(), bootstrap.Host().ID().String())
	t.Logf("Bootstrap node listening on: %s", bootstrapAddr)

	// Step 2: Create publisher node (DHT client)
	t.Log("Creating publisher node...")
	publisherCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/0.0.0.0/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
	}
	publisher, err := p2p.NewNode(ctx, publisherCfg)
	require.NoError(t, err, "Failed to create publisher node")
	defer publisher.Close()

	// Bootstrap publisher
	err = publisher.Bootstrap(ctx)
	require.NoError(t, err, "Publisher bootstrap failed")
	
	// Wait for publisher to connect to bootstrap
	err = publisher.WaitForPeers(ctx, 1, 10*time.Second)
	require.NoError(t, err, "Publisher failed to connect to bootstrap")
	t.Logf("Publisher connected to bootstrap. Peer count: %d", len(publisher.Host().Network().Peers()))

	// Step 3: Create resolver node (DHT client)
	t.Log("Creating resolver node...")
	resolverCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/0.0.0.0/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
	}
	resolver, err := p2p.NewNode(ctx, resolverCfg)
	require.NoError(t, err, "Failed to create resolver node")
	defer resolver.Close()

	// Bootstrap resolver
	err = resolver.Bootstrap(ctx)
	require.NoError(t, err, "Resolver bootstrap failed")
	
	// Wait for resolver to connect to bootstrap
	err = resolver.WaitForPeers(ctx, 1, 10*time.Second)
	require.NoError(t, err, "Resolver failed to connect to bootstrap")
	t.Logf("Resolver connected to bootstrap. Peer count: %d", len(resolver.Host().Network().Peers()))

	// Give DHT time to stabilize
	time.Sleep(2 * time.Second)

	// Step 4: Create and sign Agent Card on publisher
	t.Log("Creating Agent Card...")
	signer, err := identity.NewSigner(nil)
	require.NoError(t, err, "Failed to create signer")

	card := &identity.AgentCard{
		DID: signer.DID(),
		Capabilities: []identity.Capability{
			{
				Name:    "text-generation",
				Version: "1.0",
				Cost: &identity.Cost{
					Unit:  "token",
					Price: 0.001,
				},
				Metadata: map[string]interface{}{
					"max_tokens": 4096,
					"model":      "gpt-4-turbo",
				},
			},
		},
		Endpoints: &identity.Endpoints{
			Libp2p: []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/9000/p2p/%s", signer.DID())},
			HTTP:   []string{"https://test-agent.example.com/api"},
		},
		Policy: &identity.Policy{
			SLAClass: "premium",
			Privacy:  "encrypted",
		},
	}

	err = signer.SignCard(card)
	require.NoError(t, err, "Failed to sign Agent Card")

	// Verify signature is present
	require.NotNil(t, card.Proof, "Proof should be present after signing")
	assert.Equal(t, "Ed25519Signature2020", card.Proof.Type)
	assert.Equal(t, signer.DID()+"#signing", card.Proof.VerificationMethod)

	// Step 5: Publish Agent Card from publisher node
	t.Log("Publishing Agent Card...")
	cardJSON, err := json.Marshal(card)
	require.NoError(t, err, "Failed to marshal Agent Card")

	cid, err := publisher.PublishAgentCard(ctx, cardJSON)
	require.NoError(t, err, "Failed to publish Agent Card")
	require.NotEmpty(t, cid, "CID should not be empty")
	t.Logf("Published Agent Card with CID: %s", cid)

	// Wait for DHT propagation
	t.Log("Waiting for DHT propagation...")
	time.Sleep(3 * time.Second)

	// Step 6: Resolve Agent Card from resolver node
	t.Log("Resolving Agent Card from resolver node...")
	resolvedData, err := resolver.ResolveAgentCard(ctx, cid)
	require.NoError(t, err, "Failed to resolve Agent Card")
	require.NotNil(t, resolvedData, "Resolved data should not be nil")

	// Step 7: Unmarshal and verify resolved card
	var resolvedCard identity.AgentCard
	err = json.Unmarshal(resolvedData, &resolvedCard)
	require.NoError(t, err, "Failed to unmarshal resolved Agent Card")

	// Verify card content matches original
	assert.Equal(t, card.DID, resolvedCard.DID, "Card DID should match")
	assert.Equal(t, len(card.Capabilities), len(resolvedCard.Capabilities), "Capabilities count should match")
	assert.Equal(t, card.Capabilities[0].Name, resolvedCard.Capabilities[0].Name, "Capability name should match")

	// Step 8: Verify signature on resolved card
	t.Log("Verifying signature on resolved card...")
	err = identity.VerifyCard(&resolvedCard)
	require.NoError(t, err, "Signature verification failed on resolved card")

	t.Log("✅ Multi-node integration test passed!")
	t.Logf("   - Bootstrap node: %d peers", len(bootstrap.Host().Network().Peers()))
	t.Logf("   - Publisher node: %d peers", len(publisher.Host().Network().Peers()))
	t.Logf("   - Resolver node: %d peers", len(resolver.Host().Network().Peers()))
	t.Logf("   - Agent Card CID: %s", cid)
	t.Logf("   - Card signature verified: ✓")
}

// TestMultiNodeWithMultipleCards tests publishing and resolving multiple Agent Cards
func TestMultiNodeWithMultipleCards(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Create bootstrap node
	t.Log("Creating bootstrap node...")
	bootstrapCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/0.0.0.0/tcp/0"},
		EnableDHT:   true,
		DHTMode:     dht.ModeServer,
	}
	bootstrap, err := p2p.NewNode(ctx, bootstrapCfg)
	require.NoError(t, err, "Failed to create bootstrap node")
	defer bootstrap.Close()

	bootstrapAddr := fmt.Sprintf("%s/p2p/%s", bootstrap.Host().Addrs()[0].String(), bootstrap.Host().ID().String())

	// Create 3 publisher nodes
	publishers := make([]*p2p.Node, 3)
	signers := make([]*identity.Signer, 3)
	cids := make([]string, 3)

	for i := 0; i < 3; i++ {
		t.Logf("Creating publisher node %d...", i+1)
		publisherCfg := &p2p.Config{
			ListenAddrs:    []string{"/ip4/0.0.0.0/tcp/0"},
			BootstrapPeers: []string{bootstrapAddr},
			EnableDHT:      true,
			DHTMode:        dht.ModeClient,
		}
		node, err := p2p.NewNode(ctx, publisherCfg)
		require.NoError(t, err, "Failed to create publisher node %d", i+1)
		defer node.Close()

		err = node.Bootstrap(ctx)
		require.NoError(t, err, "Publisher %d bootstrap failed", i+1)
		
		err = node.WaitForPeers(ctx, 1, 10*time.Second)
		require.NoError(t, err, "Publisher %d failed to connect", i+1)

		publishers[i] = node

		// Create unique Agent Card
		signer, err := identity.NewSigner(nil)
		require.NoError(t, err, "Failed to create signer for publisher %d", i+1)
		signers[i] = signer

		card := &identity.AgentCard{
			DID: signer.DID(),
			Capabilities: []identity.Capability{
				{
					Name:    "compute",
					Version: "1.0",
					Metadata: map[string]interface{}{
						"description": fmt.Sprintf("Worker node %d", i+1),
					},
				},
			},
			Endpoints: &identity.Endpoints{
				Libp2p: []string{fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", 9000+i, signer.DID())},
			},
		}

		err = signer.SignCard(card)
		require.NoError(t, err, "Failed to sign card %d", i+1)

		cardJSON, err := json.Marshal(card)
		require.NoError(t, err, "Failed to marshal card %d", i+1)

		cid, err := node.PublishAgentCard(ctx, cardJSON)
		require.NoError(t, err, "Failed to publish card %d", i+1)
		cids[i] = cid

		t.Logf("Publisher %d published card: %s", i+1, cid)
	}

	// Wait for DHT propagation
	time.Sleep(3 * time.Second)

	// Create resolver node
	t.Log("Creating resolver node...")
	resolverCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/0.0.0.0/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
	}
	resolver, err := p2p.NewNode(ctx, resolverCfg)
	require.NoError(t, err, "Failed to create resolver node")
	defer resolver.Close()

	err = resolver.Bootstrap(ctx)
	require.NoError(t, err, "Resolver bootstrap failed")
	
	err = resolver.WaitForPeers(ctx, 1, 10*time.Second)
	require.NoError(t, err, "Resolver failed to connect")

	time.Sleep(2 * time.Second)

	// Resolve all cards
	resolvedCount := 0
	for i, cid := range cids {
		t.Logf("Resolving card %d (CID: %s)...", i+1, cid)
		data, err := resolver.ResolveAgentCard(ctx, cid)
		if err != nil {
			t.Logf("Warning: Failed to resolve card %d: %v", i+1, err)
			continue
		}

		var card identity.AgentCard
		err = json.Unmarshal(data, &card)
		require.NoError(t, err, "Failed to unmarshal card %d", i+1)

		err = identity.VerifyCard(&card)
		require.NoError(t, err, "Card %d signature verification failed", i+1)

		assert.Equal(t, signers[i].DID(), card.DID, "Card %d DID should match", i+1)
		assert.Equal(t, "compute", card.Capabilities[0].Name, "Card %d capability should match", i+1)

		resolvedCount++
		t.Logf("✅ Card %d resolved and verified", i+1)
	}

	assert.Equal(t, 3, resolvedCount, "All 3 cards should be resolved")
	t.Logf("✅ Multi-card integration test passed! Resolved %d/3 cards", resolvedCount)
}
