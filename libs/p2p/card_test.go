package p2p

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestPublishAndResolveAgentCard(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger := zaptest.NewLogger(t)

	// Create bootstrap node
	bootCfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{},
		EnableDHT:      true,
		DHTMode:        dht.ModeServer,
		Logger:         logger,
	}

	bootNode, err := NewNode(ctx, bootCfg)
	require.NoError(t, err)
	defer bootNode.Close()

	bootAddr := bootNode.host.Addrs()[0].String() + "/p2p/" + bootNode.ID().String()

	// Create publisher node
	pubCfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{bootAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger,
	}

	pubNode, err := NewNode(ctx, pubCfg)
	require.NoError(t, err)
	defer pubNode.Close()

	err = pubNode.Bootstrap(ctx)
	require.NoError(t, err)

	// Create resolver node
	resCfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{bootAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger,
	}

	resNode, err := NewNode(ctx, resCfg)
	require.NoError(t, err)
	defer resNode.Close()

	err = resNode.Bootstrap(ctx)
	require.NoError(t, err)

	// Wait for DHT to stabilize
	time.Sleep(2 * time.Second)

	// Publish agent card
	card := map[string]interface{}{
		"did":      "did:key:z6MkTest",
		"endpoint": "/ip4/127.0.0.1/udp/4001/quic-v1",
	}

	cardJSON, err := json.Marshal(card)
	require.NoError(t, err)

	cidStr, err := pubNode.PublishAgentCard(ctx, cardJSON)
	require.NoError(t, err)
	assert.NotEmpty(t, cidStr)

	t.Logf("Published card with CID: %s", cidStr)

	// Give DHT time to propagate
	time.Sleep(2 * time.Second)

	// Resolve agent card
	resolved, err := resNode.ResolveAgentCard(ctx, cidStr)
	require.NoError(t, err)
	assert.NotEmpty(t, resolved)

	t.Logf("Resolved: %s", string(resolved))
}
