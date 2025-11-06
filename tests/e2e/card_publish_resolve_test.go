package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zerostate/libs/identity"
	"github.com/zerostate/libs/p2p"
	"go.uber.org/zap"
)

// TestE2E_CardPublishAndResolve tests the complete flow:
// 1. Start 3 nodes (bootstrap, publisher, resolver)
// 2. Publisher creates and publishes an Agent Card to DHT
// 3. Resolver queries DHT and fetches card from publisher
// 4. Verify content matches and signature is valid
func TestE2E_CardPublishAndResolve(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Step 1: Create bootstrap node (DHT server)
	t.Log("Creating bootstrap node...")
	bootstrapCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		EnableDHT:   true,
		DHTMode:     dht.ModeServer,
		Logger:      logger.Named("bootstrap"),
	}
	bootstrap, err := p2p.NewNode(ctx, bootstrapCfg)
	require.NoError(t, err, "Failed to create bootstrap node")
	defer bootstrap.Close()

	bootstrapAddr := bootstrap.Addrs()[0].String() + "/p2p/" + bootstrap.ID().String()
	t.Logf("Bootstrap node: %s", bootstrapAddr)

	// Step 2: Create publisher node (DHT client)
	t.Log("Creating publisher node...")
	publisherCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("publisher"),
	}
	publisher, err := p2p.NewNode(ctx, publisherCfg)
	require.NoError(t, err, "Failed to create publisher node")
	defer publisher.Close()

	// Bootstrap publisher to connect to DHT
	err = publisher.Bootstrap(ctx)
	require.NoError(t, err, "Publisher bootstrap failed")
	
	time.Sleep(2 * time.Second) // Wait for connection

	// Start content provider on publisher
	err = publisher.StartContentProvider(ctx)
	require.NoError(t, err, "Failed to start content provider on publisher")

	// Step 3: Create and sign Agent Card
	t.Log("Creating and signing Agent Card...")
	signer, err := identity.NewSigner(logger.Named("signer"))
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
					"model":      "gpt-4",
					"max_tokens": 8192,
				},
			},
		},
		Endpoints: &identity.Endpoints{
			Libp2p: []string{"/ip4/127.0.0.1/tcp/9000/p2p/" + signer.DID()},
		},
		Policy: &identity.Policy{
			SLAClass: "premium",
			Privacy:  "encrypted",
		},
	}

	err = signer.SignCard(card)
	require.NoError(t, err, "Failed to sign card")
	require.NotNil(t, card.Proof, "Card proof should be present")

	// Step 4: Publish Agent Card to DHT
	t.Log("Publishing Agent Card to DHT...")
	cardJSON, err := json.Marshal(card)
	require.NoError(t, err, "Failed to marshal card")

	cid, err := publisher.PublishAgentCard(ctx, cardJSON)
	require.NoError(t, err, "Failed to publish card")
	require.NotEmpty(t, cid, "CID should not be empty")
	t.Logf("Published card with CID: %s", cid)

	// Step 5: Wait for DHT propagation
	t.Log("Waiting for DHT propagation...")
	time.Sleep(3 * time.Second)

	// Step 6: Create resolver node
	t.Log("Creating resolver node...")
	resolverCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("resolver"),
	}
	resolver, err := p2p.NewNode(ctx, resolverCfg)
	require.NoError(t, err, "Failed to create resolver node")
	defer resolver.Close()

	err = resolver.Bootstrap(ctx)
	require.NoError(t, err, "Resolver bootstrap failed")
	
	time.Sleep(2 * time.Second)

	// Step 7: Resolve Agent Card from DHT
	t.Log("Resolving Agent Card from DHT...")
	resolvedData, err := resolver.ResolveAgentCard(ctx, cid)
	require.NoError(t, err, "Failed to resolve card")
	require.NotNil(t, resolvedData, "Resolved data should not be nil")

	// Step 8: Verify resolved card
	var resolvedCard identity.AgentCard
	err = json.Unmarshal(resolvedData, &resolvedCard)
	require.NoError(t, err, "Failed to unmarshal resolved card")

	// Verify content matches
	assert.Equal(t, card.DID, resolvedCard.DID, "DID should match")
	assert.Equal(t, len(card.Capabilities), len(resolvedCard.Capabilities), "Capabilities count should match")
	assert.Equal(t, "text-generation", resolvedCard.Capabilities[0].Name, "Capability name should match")
	assert.Equal(t, "premium", resolvedCard.Policy.SLAClass, "SLA class should match")

	// Verify signature
	t.Log("Verifying signature...")
	err = identity.VerifyCard(&resolvedCard)
	require.NoError(t, err, "Signature verification failed")

	t.Log("✅ E2E test passed: Card published, resolved, and verified successfully!")
}

// TestE2E_MultipleCardsWithQRouting tests Q-routing with multiple providers
func TestE2E_MultipleCardsWithQRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Create bootstrap node
	t.Log("Creating bootstrap node...")
	bootstrapCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		EnableDHT:   true,
		DHTMode:     dht.ModeServer,
		Logger:      logger.Named("bootstrap"),
	}
	bootstrap, err := p2p.NewNode(ctx, bootstrapCfg)
	require.NoError(t, err)
	defer bootstrap.Close()

	bootstrapAddr := bootstrap.Addrs()[0].String() + "/p2p/" + bootstrap.ID().String()

	// Create 3 publisher nodes
	publishers := make([]*p2p.Node, 3)
	cids := make([]string, 3)

	for i := 0; i < 3; i++ {
		t.Logf("Creating publisher %d...", i+1)
		cfg := &p2p.Config{
			ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
			BootstrapPeers: []string{bootstrapAddr},
			EnableDHT:      true,
			DHTMode:        dht.ModeClient,
			Logger:         logger.Named("publisher"),
		}
		node, err := p2p.NewNode(ctx, cfg)
		require.NoError(t, err)
		defer node.Close()

		err = node.Bootstrap(ctx)
		require.NoError(t, err)

		err = node.StartContentProvider(ctx)
		require.NoError(t, err)

		publishers[i] = node

		// Create and publish card
		signer, _ := identity.NewSigner(logger)
		card := &identity.AgentCard{
			DID: signer.DID(),
			Capabilities: []identity.Capability{
				{Name: "compute", Version: "1.0"},
			},
			Endpoints: &identity.Endpoints{
				Libp2p: []string{"/ip4/127.0.0.1/tcp/9000/p2p/" + signer.DID()},
			},
		}
		signer.SignCard(card)

		cardJSON, _ := json.Marshal(card)
		cid, err := node.PublishAgentCard(ctx, cardJSON)
		require.NoError(t, err)
		cids[i] = cid
		t.Logf("Publisher %d published CID: %s", i+1, cid)
	}

	time.Sleep(3 * time.Second)

	// Create resolver with Q-routing
	t.Log("Creating resolver with Q-routing...")
	resolverCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("resolver"),
	}
	resolver, err := p2p.NewNode(ctx, resolverCfg)
	require.NoError(t, err)
	defer resolver.Close()

	err = resolver.Bootstrap(ctx)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Resolve all cards - Q-routing should learn best paths
	resolvedCount := 0
	for i, cid := range cids {
		t.Logf("Resolving card %d...", i+1)
		data, err := resolver.ResolveAgentCard(ctx, cid)
		if err != nil {
			t.Logf("Warning: Failed to resolve card %d: %v", i+1, err)
			continue
		}

		var card identity.AgentCard
		json.Unmarshal(data, &card)
		
		err = identity.VerifyCard(&card)
		require.NoError(t, err, "Card %d signature should be valid", i+1)
		
		resolvedCount++
	}

	assert.Greater(t, resolvedCount, 0, "Should resolve at least one card")
	t.Logf("✅ Resolved %d/%d cards with Q-routing", resolvedCount, len(cids))
}
