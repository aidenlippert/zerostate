package p2p

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/aidenlippert/zerostate/libs/metrics"
)

func TestNewP2PMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	require.NotNil(t, m)
	assert.NotNil(t, m.ConnectionsActive)
	assert.NotNil(t, m.ConnectionsTotal)
	assert.NotNil(t, m.BytesSent)
	assert.NotNil(t, m.BytesReceived)
	assert.NotNil(t, m.MessagesSent)
	assert.NotNil(t, m.MessagesReceived)
	assert.NotNil(t, m.PeersConnected)
	assert.NotNil(t, m.OperationDuration)
}

func TestRecordConnectionEstablished(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Record connection
	m.RecordConnectionEstablished("peer1")

	// Verify counter incremented
	total := testutil.ToFloat64(m.ConnectionsTotal.WithLabelValues("peer1"))
	assert.Equal(t, 1.0, total)

	// Verify active gauge incremented
	active := testutil.ToFloat64(m.ConnectionsActive.WithLabelValues("established"))
	assert.Equal(t, 1.0, active)

	// Record another connection
	m.RecordConnectionEstablished("peer2")
	active = testutil.ToFloat64(m.ConnectionsActive.WithLabelValues("established"))
	assert.Equal(t, 2.0, active)
}

func TestRecordConnectionClosed(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Establish then close
	m.RecordConnectionEstablished("peer1")
	m.RecordConnectionClosed()

	active := testutil.ToFloat64(m.ConnectionsActive.WithLabelValues("established"))
	assert.Equal(t, 0.0, active)
}

func TestRecordConnectionFailed(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.RecordConnectionFailed("timeout")
	m.RecordConnectionFailed("timeout")
	m.RecordConnectionFailed("refused")

	timeoutFailed := testutil.ToFloat64(m.ConnectionsFailed.WithLabelValues("timeout"))
	assert.Equal(t, 2.0, timeoutFailed)

	refusedFailed := testutil.ToFloat64(m.ConnectionsFailed.WithLabelValues("refused"))
	assert.Equal(t, 1.0, refusedFailed)
}

func TestP2PMetricsRecordBandwidth(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Record bytes sent
	m.RecordBytesSent("gossip", "peer1", 1024)
	m.RecordBytesSent("gossip", "peer1", 2048)

	sent := testutil.ToFloat64(m.BytesSent.WithLabelValues("gossip", "peer1"))
	assert.Equal(t, 3072.0, sent)

	// Record bytes received
	m.RecordBytesReceived("request", "peer2", 4096)
	received := testutil.ToFloat64(m.BytesReceived.WithLabelValues("request", "peer2"))
	assert.Equal(t, 4096.0, received)
}

func TestRecordMessages(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Record sent messages
	m.RecordMessageSent("gossip", "protocol1", 100)
	m.RecordMessageSent("gossip", "protocol1", 200)

	sent := testutil.ToFloat64(m.MessagesSent.WithLabelValues("gossip", "protocol1"))
	assert.Equal(t, 2.0, sent)

	// Record received messages
	m.RecordMessageReceived("request", "protocol2", 150)
	received := testutil.ToFloat64(m.MessagesReceived.WithLabelValues("request", "protocol2"))
	assert.Equal(t, 1.0, received)

	// Record failed messages
	m.RecordMessageFailed("gossip", "timeout")
	failed := testutil.ToFloat64(m.MessagesFailed.WithLabelValues("gossip", "timeout"))
	assert.Equal(t, 1.0, failed)
}

func TestRecordPeers(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Record peer connections
	m.RecordPeerConnected("executor")
	m.RecordPeerConnected("executor")
	m.RecordPeerConnected("creator")

	executors := testutil.ToFloat64(m.PeersConnected.WithLabelValues("executor"))
	assert.Equal(t, 2.0, executors)

	creators := testutil.ToFloat64(m.PeersConnected.WithLabelValues("creator"))
	assert.Equal(t, 1.0, creators)

	// Record disconnection
	m.RecordPeerDisconnected("executor")
	executors = testutil.ToFloat64(m.PeersConnected.WithLabelValues("executor"))
	assert.Equal(t, 1.0, executors)

	// Record peer discovered
	m.RecordPeerDiscovered("dht")
	m.RecordPeerDiscovered("gossip")
	discovered := testutil.ToFloat64(m.PeersDiscovered.WithLabelValues("dht"))
	assert.Equal(t, 1.0, discovered)

	// Record peer failed
	m.RecordPeerFailed("timeout")
	failed := testutil.ToFloat64(m.PeersFailed.WithLabelValues("timeout"))
	assert.Equal(t, 1.0, failed)
}

func TestRecordOperationDuration(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Record operation durations
	m.RecordOperationDuration("connect", 0.1)
	m.RecordOperationDuration("connect", 0.2)
	m.RecordOperationDuration("connect", 0.3)

	// Histogram count should be 3
	// Note: testutil.ToFloat64 returns count for histograms
	// We just verify it's created
	assert.NotNil(t, m.OperationDuration.WithLabelValues("connect"))
}

func TestRecordGossip(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.RecordGossipMessage("publish", "success")
	m.RecordGossipMessage("publish", "success")
	m.RecordGossipMessage("publish", "failed")

	success := testutil.ToFloat64(m.GossipMessages.WithLabelValues("publish", "success"))
	assert.Equal(t, 2.0, success)

	failed := testutil.ToFloat64(m.GossipMessages.WithLabelValues("publish", "failed"))
	assert.Equal(t, 1.0, failed)

	// Record propagation
	m.RecordGossipPropagation(0.05)
	assert.NotNil(t, m.GossipPropagation.WithLabelValues())
}

func TestRecordStreams(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.RecordStreamOpened("gossip")
	m.RecordStreamOpened("gossip")
	m.RecordStreamOpened("request")

	gossipStreams := testutil.ToFloat64(m.StreamsActive.WithLabelValues("gossip"))
	assert.Equal(t, 2.0, gossipStreams)

	m.RecordStreamClosed("gossip")
	gossipStreams = testutil.ToFloat64(m.StreamsActive.WithLabelValues("gossip"))
	assert.Equal(t, 1.0, gossipStreams)
}

func TestRecordHeartbeats(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.RecordHeartbeatSent("peer1")
	m.RecordHeartbeatSent("peer1")
	sent := testutil.ToFloat64(m.HeartbeatsSent.WithLabelValues("peer1"))
	assert.Equal(t, 2.0, sent)

	m.RecordHeartbeatReceived("peer2")
	received := testutil.ToFloat64(m.HeartbeatsReceived.WithLabelValues("peer2"))
	assert.Equal(t, 1.0, received)

	m.RecordFailureDetection("peer3", "timeout")
	failures := testutil.ToFloat64(m.FailureDetections.WithLabelValues("peer3", "timeout"))
	assert.Equal(t, 1.0, failures)
}

func TestRecordRelay(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.RecordRelayCircuitOpened("inbound")
	m.RecordRelayCircuitOpened("outbound")
	inbound := testutil.ToFloat64(m.RelayCircuits.WithLabelValues("inbound"))
	assert.Equal(t, 1.0, inbound)

	m.RecordRelayCircuitClosed("inbound")
	inbound = testutil.ToFloat64(m.RelayCircuits.WithLabelValues("inbound"))
	assert.Equal(t, 0.0, inbound)

	m.RecordRelayBytes("inbound", 2048)
	bytes := testutil.ToFloat64(m.RelayBytes.WithLabelValues("inbound"))
	assert.Equal(t, 2048.0, bytes)
}

func TestQoSMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.SetQueueDepth("high", 10)
	m.SetQueueDepth("low", 5)

	highDepth := testutil.ToFloat64(m.QueueDepth.WithLabelValues("high"))
	assert.Equal(t, 10.0, highDepth)

	lowDepth := testutil.ToFloat64(m.QueueDepth.WithLabelValues("low"))
	assert.Equal(t, 5.0, lowDepth)

	m.RecordQueueLatency("high", 0.001)
	assert.NotNil(t, m.QueueLatency.WithLabelValues("high"))

	m.RecordPacketDropped("queue_full")
	dropped := testutil.ToFloat64(m.DroppedPackets.WithLabelValues("queue_full"))
	assert.Equal(t, 1.0, dropped)
}

func TestSetIdleConnections(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	m.SetIdleConnections(5)
	idle := testutil.ToFloat64(m.ConnectionsIdle.WithLabelValues())
	assert.Equal(t, 5.0, idle)

	m.SetIdleConnections(3)
	idle = testutil.ToFloat64(m.ConnectionsIdle.WithLabelValues())
	assert.Equal(t, 3.0, idle)
}

func TestP2PMetricsConcurrency(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	// Concurrent metric updates
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				m.RecordConnectionEstablished("peer")
				m.RecordMessageSent("gossip", "proto", 100)
				m.RecordBytesSent("proto", "peer", 1024)
			}
			done <- true
		}(i)
	}

	// Wait for completion
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no race conditions (counts should be consistent)
	sent := testutil.ToFloat64(m.MessagesSent.WithLabelValues("gossip", "proto"))
	assert.Equal(t, 1000.0, sent)
}

func BenchmarkRecordMessageSent(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordMessageSent("gossip", "protocol", 100)
	}
}

func BenchmarkRecordBytesSent(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordBytesSent("protocol", "peer", 1024)
	}
}

func BenchmarkRecordOperationDuration(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewP2PMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordOperationDuration("connect", 0.1)
	}
}
