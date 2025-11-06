package p2p

import (
	"context"
	"testing"
	"time"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewNode(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	cfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{},
		EnableDHT:      false,
		Logger:         logger,
	}

	node, err := NewNode(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, node)
	defer node.Close()

	assert.NotEmpty(t, node.ID())
	assert.NotEmpty(t, node.Addrs())
}

func TestNewNodeWithDHT(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	cfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{},
		EnableDHT:      true,
		DHTMode:        dht.ModeServer,
		Logger:         logger,
	}

	node, err := NewNode(ctx, cfg)
	require.NoError(t, err)
	require.NotNil(t, node)
	defer node.Close()

	assert.NotNil(t, node.dht)
}

func TestNodeBootstrap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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

	// Get bootstrap peer address
	bootAddr := bootNode.host.Addrs()[0].String() + "/p2p/" + bootNode.ID().String()

	// Create client node
	clientCfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{bootAddr},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger,
	}

	clientNode, err := NewNode(ctx, clientCfg)
	require.NoError(t, err)
	defer clientNode.Close()

	// Bootstrap
	err = clientNode.Bootstrap(ctx)
	require.NoError(t, err)

	// Wait for peers
	err = clientNode.WaitForPeers(ctx, 1, 5*time.Second)
	require.NoError(t, err)

	// Verify connection
	peers := clientNode.host.Network().Peers()
	assert.Contains(t, peers, bootNode.ID())
}

func TestNodeInvalidBootstrapAddr(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	cfg := &Config{
		ListenAddrs:    []string{"/ip4/127.0.0.1/udp/0/quic-v1"},
		BootstrapPeers: []string{"invalid-address"},
		EnableDHT:      true,
		DHTMode:        dht.ModeClient,
		Logger:         logger,
	}

	node, err := NewNode(ctx, cfg)
	require.NoError(t, err)
	defer node.Close()

	// Bootstrap should fail with no valid peers
	err = node.Bootstrap(ctx)
	assert.Error(t, err)
}
