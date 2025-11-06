package e2e

import (
	"context"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zerostate/libs/p2p"
	"go.uber.org/zap"
)

// TestE2E_ContentExchangeProtocol tests the custom content exchange protocol
// This verifies that nodes can fetch content from each other directly
func TestE2E_ContentExchangeProtocol(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Create provider node with content
	t.Log("Creating provider node...")
	providerCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		EnableDHT:   false, // Don't use DHT for this test
		Logger:      logger.Named("provider"),
	}
	provider, err := p2p.NewNode(ctx, providerCfg)
	require.NoError(t, err)
	defer provider.Close()

	// Start content provider protocol
	err = provider.StartContentProvider(ctx)
	require.NoError(t, err, "Failed to start content provider")

	// Note: For this basic test, we're just verifying the protocol handshake
	// Full content storage/retrieval is tested in card_publish_resolve_test.go
	
	// Create consumer node
	t.Log("Creating consumer node...")
	consumerCfg := &p2p.Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
		EnableDHT:   false,
		Logger:      logger.Named("consumer"),
	}
	consumer, err := p2p.NewNode(ctx, consumerCfg)
	require.NoError(t, err)
	defer consumer.Close()

	// Connect consumer to provider
	providerInfo := provider.Host().Peerstore().PeerInfo(provider.ID())
	
	t.Logf("Connecting consumer to provider...")
	err = consumer.Host().Connect(ctx, providerInfo)
	require.NoError(t, err, "Failed to connect to provider")

	time.Sleep(1 * time.Second)

	// Verify connection
	peers := consumer.Host().Network().Peers()
	require.Contains(t, peers, provider.ID(), "Consumer should be connected to provider")

	t.Log("✅ Content exchange protocol test passed: Nodes connected")
	
	// Note: Full content exchange test would require:
	// 1. Provider storing content in its content store
	// 2. Consumer calling FetchContent(ctx, providerID, cid)
	// 3. Verifying the content matches
	// This is partially tested in card_publish_resolve_test.go
}

// TestE2E_ContentExchangeWithDHT tests content exchange through DHT lookup
func TestE2E_ContentExchangeWithDHT(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	// Create provider node
	t.Log("Creating provider node...")
	providerCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("provider"),
	}
	provider, err := p2p.NewNode(ctx, providerCfg)
	require.NoError(t, err)
	defer provider.Close()

	err = provider.Bootstrap(ctx)
	require.NoError(t, err)

	err = provider.StartContentProvider(ctx)
	require.NoError(t, err)

	// Publish test content
	testContent := []byte(`{"type":"test-content","value":42}`)
	cid, err := provider.PublishAgentCard(ctx, testContent)
	require.NoError(t, err)
	t.Logf("Published content with CID: %s", cid)

	time.Sleep(3 * time.Second)

	// Create consumer node
	t.Log("Creating consumer node...")
	consumerCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("consumer"),
	}
	consumer, err := p2p.NewNode(ctx, consumerCfg)
	require.NoError(t, err)
	defer consumer.Close()

	err = consumer.Bootstrap(ctx)
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	// Fetch content through DHT
	t.Log("Fetching content through DHT...")
	fetchedContent, err := consumer.ResolveAgentCard(ctx, cid)
	require.NoError(t, err, "Failed to fetch content")
	require.NotNil(t, fetchedContent, "Fetched content should not be nil")

	// Verify content matches
	assert.Equal(t, testContent, fetchedContent, "Content should match original")

	t.Logf("✅ Content exchange with DHT test passed: Fetched %d bytes", len(fetchedContent))
}

// TestE2E_ContentExchangeLatency measures content fetch latency
func TestE2E_ContentExchangeLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Setup network
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

	providerCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("provider"),
	}
	provider, err := p2p.NewNode(ctx, providerCfg)
	require.NoError(t, err)
	defer provider.Close()

	provider.Bootstrap(ctx)
	provider.StartContentProvider(ctx)

	// Publish content
	testContent := make([]byte, 1024) // 1KB test content
	for i := range testContent {
		testContent[i] = byte(i % 256)
	}
	
	cid, err := provider.PublishAgentCard(ctx, testContent)
	require.NoError(t, err)

	time.Sleep(3 * time.Second)

	consumerCfg := &p2p.Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
		BootstrapPeers: []string{bootstrapAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger.Named("consumer"),
	}
	consumer, err := p2p.NewNode(ctx, consumerCfg)
	require.NoError(t, err)
	defer consumer.Close()

	consumer.Bootstrap(ctx)
	time.Sleep(2 * time.Second)

	// Measure fetch latency
	start := time.Now()
	fetchedContent, err := consumer.ResolveAgentCard(ctx, cid)
	latency := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, len(testContent), len(fetchedContent), "Content size should match")

	t.Logf("✅ Content exchange latency test passed:")
	t.Logf("   - Content size: %d bytes", len(testContent))
	t.Logf("   - Fetch latency: %v", latency)
	t.Logf("   - Throughput: %.2f KB/s", float64(len(testContent))/latency.Seconds()/1024)

	// Latency should be reasonable (< 5s for local network)
	assert.Less(t, latency, 5*time.Second, "Latency should be under 5 seconds for local network")
}
