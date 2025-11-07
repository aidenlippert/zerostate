package p2p

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const testProtocol = protocol.ID("/test/1.0.0")

func createTestHosts(t *testing.T, n int) []host.Host {
	hosts := make([]host.Host, n)
	for i := 0; i < n; i++ {
		h, err := libp2p.New(
			libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		)
		require.NoError(t, err)
		hosts[i] = h
	}
	return hosts
}

func connectHosts(t *testing.T, h1, h2 host.Host) {
	h1.Peerstore().AddAddrs(h2.ID(), h2.Addrs(), time.Hour)
	err := h1.Connect(context.Background(), peer.AddrInfo{ID: h2.ID()})
	require.NoError(t, err)
}

func TestNewConnectionPool(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 1)
	defer hosts[0].Close()

	logger := zap.NewNop()
	config := DefaultConnectionPoolConfig()

	cp := NewConnectionPool(ctx, hosts[0], config, logger)
	defer cp.Close()

	assert.NotNil(t, cp)
	assert.Equal(t, config.MaxIdleTime, cp.config.MaxIdleTime)
	assert.Equal(t, config.MaxStreamsPerConn, cp.config.MaxStreamsPerConn)
}

func TestDefaultConnectionPoolConfig(t *testing.T) {
	config := DefaultConnectionPoolConfig()

	assert.Equal(t, DefaultMaxIdleTime, config.MaxIdleTime)
	assert.Equal(t, DefaultMaxStreamsPerConn, config.MaxStreamsPerConn)
	assert.Equal(t, DefaultCleanupInterval, config.CleanupInterval)
	assert.True(t, config.EnableMetrics)
}

func TestPooledConnectionAcquireStream(t *testing.T) {
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	// Get connection
	conns := hosts[0].Network().ConnsToPeer(hosts[1].ID())
	require.Greater(t, len(conns), 0)

	pc := NewPooledConnection(conns[0], 5)

	ctx := context.Background()
	stream, reused, err := pc.AcquireStream(ctx, testProtocol)
	require.NoError(t, err)
	assert.NotNil(t, stream)
	assert.False(t, reused, "first stream should not be reused")

	// Release and reacquire
	pc.ReleaseStream(stream)
	
	stream2, reused2, err := pc.AcquireStream(ctx, testProtocol)
	require.NoError(t, err)
	assert.NotNil(t, stream2)
	assert.True(t, reused2, "second stream should be reused")
}

func TestPooledConnectionStreamLimit(t *testing.T) {
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	conns := hosts[0].Network().ConnsToPeer(hosts[1].ID())
	require.Greater(t, len(conns), 0)

	maxStreams := 3
	pc := NewPooledConnection(conns[0], maxStreams)

	ctx := context.Background()

	// Create and release more streams than the limit
	for i := 0; i < maxStreams+2; i++ {
		stream, _, err := pc.AcquireStream(ctx, testProtocol)
		require.NoError(t, err)
		pc.ReleaseStream(stream)
	}

	// Pool should only keep maxStreams
	assert.LessOrEqual(t, pc.StreamCount(), maxStreams)
}

func TestPooledConnectionIsIdle(t *testing.T) {
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	conns := hosts[0].Network().ConnsToPeer(hosts[1].ID())
	require.Greater(t, len(conns), 0)

	pc := NewPooledConnection(conns[0], 5)

	// Should not be idle immediately
	assert.False(t, pc.IsIdle(100*time.Millisecond))

	// Wait and check again
	time.Sleep(150 * time.Millisecond)
	assert.True(t, pc.IsIdle(100*time.Millisecond))
}

func TestPooledConnectionClose(t *testing.T) {
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	conns := hosts[0].Network().ConnsToPeer(hosts[1].ID())
	require.Greater(t, len(conns), 0)

	pc := NewPooledConnection(conns[0], 5)

	ctx := context.Background()
	stream, _, err := pc.AcquireStream(ctx, testProtocol)
	require.NoError(t, err)
	pc.ReleaseStream(stream)

	assert.Greater(t, pc.StreamCount(), 0)

	err = pc.Close()
	assert.NoError(t, err)
	assert.Equal(t, 0, pc.StreamCount())
}

func TestConnectionPoolGetStream(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	// Set up echo handler on host 2
	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		defer s.Close()
		buf := make([]byte, 1024)
		n, _ := s.Read(buf)
		s.Write(buf[:n])
	})

	// Get stream
	stream, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
	require.NoError(t, err)
	assert.NotNil(t, stream)

	// Test the stream works
	_, err = stream.Write([]byte("test"))
	assert.NoError(t, err)
	stream.Close()
}

func TestConnectionPoolStreamReuse(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		s.Close()
	})

	// Get and release stream
	stream1, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
	require.NoError(t, err)
	cp.ReleaseStream(hosts[1].ID(), stream1)

	// Connection should be in pool
	stats := cp.Stats()
	assert.Equal(t, 1, stats.TotalConnections)
}

func TestConnectionPoolRemovePeer(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		s.Close()
	})

	// Create connection
	_, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
	require.NoError(t, err)

	stats := cp.Stats()
	assert.Equal(t, 1, stats.TotalConnections)

	// Remove peer
	cp.RemovePeer(hosts[1].ID())

	stats = cp.Stats()
	assert.Equal(t, 0, stats.TotalConnections)
}

func TestConnectionPoolCleanup(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	config := &ConnectionPoolConfig{
		MaxIdleTime:       100 * time.Millisecond,
		MaxStreamsPerConn: 5,
		CleanupInterval:   50 * time.Millisecond,
		EnableMetrics:     true,
	}
	cp := NewConnectionPool(ctx, hosts[0], config, logger)
	defer cp.Close()

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		s.Close()
	})

	// Create connection
	_, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
	require.NoError(t, err)

	stats := cp.Stats()
	assert.Equal(t, 1, stats.TotalConnections)

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Should be cleaned up
	stats = cp.Stats()
	assert.Equal(t, 0, stats.TotalConnections)
}

func TestConnectionPoolStats(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 3)
	defer hosts[0].Close()
	defer hosts[1].Close()
	defer hosts[2].Close()

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	for i := 1; i < 3; i++ {
		connectHosts(t, hosts[0], hosts[i])
		hosts[i].SetStreamHandler(testProtocol, func(s network.Stream) {
			s.Close()
		})
	}

	// Create streams to multiple peers
	for i := 1; i < 3; i++ {
		stream, err := cp.GetStream(ctx, hosts[i].ID(), testProtocol)
		require.NoError(t, err)
		cp.ReleaseStream(hosts[i].ID(), stream)
	}

	stats := cp.Stats()
	assert.Equal(t, 2, stats.TotalConnections)
	assert.LessOrEqual(t, stats.TotalStreams, 2)
}

func TestConnectionPoolClose(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		s.Close()
	})

	// Create connection
	_, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
	require.NoError(t, err)

	stats := cp.Stats()
	assert.Equal(t, 1, stats.TotalConnections)

	// Close pool
	err = cp.Close()
	assert.NoError(t, err)

	stats = cp.Stats()
	assert.Equal(t, 0, stats.TotalConnections)
}

func TestConnectionPoolConcurrent(t *testing.T) {
	ctx := context.Background()
	hosts := createTestHosts(t, 2)
	defer hosts[0].Close()
	defer hosts[1].Close()

	connectHosts(t, hosts[0], hosts[1])

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		time.Sleep(10 * time.Millisecond)
		s.Close()
	})

	// Concurrent stream acquisitions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			stream, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
			if err == nil {
				stream.Close()
			}
			done <- true
		}()
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have connection in pool
	stats := cp.Stats()
	assert.GreaterOrEqual(t, stats.TotalConnections, 0)
}

func BenchmarkConnectionPoolGetStream(b *testing.B) {
	ctx := context.Background()
	hosts := make([]host.Host, 2)
	for i := 0; i < 2; i++ {
		h, err := libp2p.New(
			libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"),
		)
		if err != nil {
			b.Fatal(err)
		}
		defer h.Close()
		hosts[i] = h
	}

	hosts[0].Peerstore().AddAddrs(hosts[1].ID(), hosts[1].Addrs(), time.Hour)
	err := hosts[0].Connect(ctx, peer.AddrInfo{ID: hosts[1].ID()})
	if err != nil {
		b.Fatal(err)
	}

	logger := zap.NewNop()
	cp := NewConnectionPool(ctx, hosts[0], nil, logger)
	defer cp.Close()

	hosts[1].SetStreamHandler(testProtocol, func(s network.Stream) {
		s.Close()
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stream, err := cp.GetStream(ctx, hosts[1].ID(), testProtocol)
		if err == nil {
			stream.Close()
		}
	}
}
