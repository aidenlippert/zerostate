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

// TestE2E_MDNSDiscovery tests mDNS peer discovery on local network
// This simulates edge nodes discovering each other on the same LAN
func TestE2E_MDNSDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Create 3 nodes with mDNS enabled (no bootstrap peers)
	nodes := make([]*p2p.Node, 3)
	
	for i := 0; i < 3; i++ {
		t.Logf("Creating node %d with mDNS...", i+1)
		cfg := &p2p.Config{
			ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
			EnableDHT:   true,
			DHTMode:     dht.ModeClient,
			EnableMDNS:  true,
			Logger:      logger.Named("node"),
		}
		
		node, err := p2p.NewNode(ctx, cfg)
		require.NoError(t, err, "Failed to create node %d", i+1)
		defer node.Close()
		
		nodes[i] = node
		t.Logf("Node %d created: %s", i+1, node.ID().String())
	}

	// Wait for mDNS discovery (can take a few seconds)
	t.Log("Waiting for mDNS discovery...")
	time.Sleep(5 * time.Second)

	// Check that nodes discovered each other
	discoveredCount := 0
	for i, node := range nodes {
		peers := node.Host().Network().Peers()
		t.Logf("Node %d has %d peers: %v", i+1, len(peers), peers)
		
		if len(peers) > 0 {
			discoveredCount++
		}
	}

	// At least some nodes should have discovered each other
	assert.Greater(t, discoveredCount, 0, "At least one node should discover peers via mDNS")
	
	t.Logf("✅ mDNS discovery test passed: %d/%d nodes discovered peers", discoveredCount, len(nodes))
}

// TestE2E_MDNSWithBootstrap tests mDNS combined with bootstrap peers
func TestE2E_MDNSWithBootstrap(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger, _ := zap.NewDevelopment()

	// Create bootstrap node (no mDNS)
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

	// Create edge nodes with both bootstrap and mDNS
	edgeNodes := make([]*p2p.Node, 3)
	
	for i := 0; i < 3; i++ {
		t.Logf("Creating edge node %d...", i+1)
		cfg := &p2p.Config{
			ListenAddrs:    []string{"/ip4/127.0.0.1/tcp/0"},
			BootstrapPeers: []string{bootstrapAddr},
			EnableDHT:      true,
			DHTMode:        dht.ModeClient,
			EnableMDNS:     true,
			Logger:         logger.Named("edge"),
		}
		
		node, err := p2p.NewNode(ctx, cfg)
		require.NoError(t, err)
		defer node.Close()
		
		err = node.Bootstrap(ctx)
		require.NoError(t, err, "Bootstrap failed for edge node %d", i+1)
		
		edgeNodes[i] = node
	}

	// Wait for both bootstrap and mDNS discovery
	t.Log("Waiting for peer discovery...")
	time.Sleep(5 * time.Second)

	// Verify connectivity
	totalPeers := 0
	for i, node := range edgeNodes {
		peers := node.Host().Network().Peers()
		peerCount := len(peers)
		totalPeers += peerCount
		
		t.Logf("Edge node %d has %d peers", i+1, peerCount)
		
		// Each edge node should connect to at least bootstrap
		assert.GreaterOrEqual(t, peerCount, 1, "Edge node %d should have at least bootstrap peer", i+1)
	}

	// Verify bootstrap has connections
	bootstrapPeers := len(bootstrap.Host().Network().Peers())
	t.Logf("Bootstrap node has %d peers", bootstrapPeers)
	assert.Greater(t, bootstrapPeers, 0, "Bootstrap should have peer connections")

	t.Logf("✅ Combined bootstrap + mDNS test passed: Total %d peer connections", totalPeers)
}
