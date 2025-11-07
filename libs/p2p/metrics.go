package p2p

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/zerostate/libs/metrics"
)

// Legacy metrics (kept for backward compatibility)
var (
	dhtLookupsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_dht_lookups_total",
			Help: "Total number of DHT lookups",
		},
		[]string{"operation", "status"},
	)

	dhtLookupDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_dht_lookup_duration_seconds",
			Help:    "DHT lookup duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	agentCardPublishTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "zerostate_agent_card_publish_total",
			Help: "Total number of agent cards published",
		},
	)

	peerConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "zerostate_peer_connections",
			Help: "Current number of peer connections",
		},
	)
)

// P2PMetrics holds all comprehensive P2P-related Prometheus metrics
type P2PMetrics struct {
	// Connection metrics
	ConnectionsActive *prometheus.GaugeVec
	ConnectionsTotal  *prometheus.CounterVec
	ConnectionsFailed *prometheus.CounterVec
	ConnectionsIdle   *prometheus.GaugeVec

	// Bandwidth metrics
	BytesSent     *prometheus.CounterVec
	BytesReceived *prometheus.CounterVec
	
	// Message metrics
	MessagesSent     *prometheus.CounterVec
	MessagesReceived *prometheus.CounterVec
	MessagesFailed   *prometheus.CounterVec
	MessageSize      *prometheus.HistogramVec

	// Peer metrics
	PeersConnected  *prometheus.GaugeVec
	PeersDiscovered *prometheus.CounterVec
	PeersFailed     *prometheus.CounterVec

	// Operation latency
	OperationDuration *prometheus.HistogramVec
	
	// Protocol-specific metrics
	GossipMessages    *prometheus.CounterVec
	GossipPropagation *prometheus.HistogramVec
	StreamsActive     *prometheus.GaugeVec
	
	// Health metrics
	HeartbeatsSent     *prometheus.CounterVec
	HeartbeatsReceived *prometheus.CounterVec
	FailureDetections  *prometheus.CounterVec
	
	// Relay metrics
	RelayCircuits *prometheus.GaugeVec
	RelayBytes    *prometheus.CounterVec
	
	// QoS metrics
	QueueDepth     *prometheus.GaugeVec
	QueueLatency   *prometheus.HistogramVec
	DroppedPackets *prometheus.CounterVec
}

// NewP2PMetrics creates and registers all P2P metrics
func NewP2PMetrics(registry *metrics.Registry) *P2PMetrics {
	if registry == nil {
		registry = metrics.Default()
	}

	return &P2PMetrics{
		// Connection metrics
		ConnectionsActive: registry.Gauge(
			"p2p_connections_active",
			"Number of active P2P connections",
			"state",
		),
		ConnectionsTotal: registry.Counter(
			"p2p_connections_total",
			"Total P2P connections established",
			"peer_id",
		),
		ConnectionsFailed: registry.Counter(
			"p2p_connections_failed",
			"Total P2P connection failures",
			"reason",
		),
		ConnectionsIdle: registry.Gauge(
			"p2p_connections_idle",
			"Number of idle connections in pool",
		),

		// Bandwidth metrics
		BytesSent: registry.Counter(
			"p2p_bandwidth_bytes_sent",
			"Total bytes sent over P2P network",
			"protocol", "peer_id",
		),
		BytesReceived: registry.Counter(
			"p2p_bandwidth_bytes_received",
			"Total bytes received over P2P network",
			"protocol", "peer_id",
		),

		// Message metrics
		MessagesSent: registry.Counter(
			"p2p_messages_sent_total",
			"Total P2P messages sent",
			"type", "protocol",
		),
		MessagesReceived: registry.Counter(
			"p2p_messages_received_total",
			"Total P2P messages received",
			"type", "protocol",
		),
		MessagesFailed: registry.Counter(
			"p2p_messages_failed_total",
			"Total P2P message failures",
			"type", "reason",
		),
		MessageSize: registry.Histogram(
			"p2p_message_size_bytes",
			"P2P message size distribution",
			metrics.BytesBuckets,
			"type",
		),

		// Peer metrics
		PeersConnected: registry.Gauge(
			"p2p_peers_connected",
			"Number of connected peers",
			"role",
		),
		PeersDiscovered: registry.Counter(
			"p2p_peers_discovered_total",
			"Total peers discovered",
			"source",
		),
		PeersFailed: registry.Counter(
			"p2p_peers_failed_total",
			"Total peer connection failures",
			"reason",
		),

		// Operation latency
		OperationDuration: registry.Histogram(
			"p2p_operation_duration_seconds",
			"P2P operation duration",
			metrics.DurationBuckets,
			"operation",
		),

		// Protocol-specific metrics
		GossipMessages: registry.Counter(
			"p2p_gossip_messages_total",
			"Total gossip messages",
			"action", "status",
		),
		GossipPropagation: registry.Histogram(
			"p2p_gossip_propagation_seconds",
			"Time for gossip message propagation",
			metrics.DurationBuckets,
		),
		StreamsActive: registry.Gauge(
			"p2p_streams_active",
			"Number of active P2P streams",
			"protocol",
		),

		// Health metrics
		HeartbeatsSent: registry.Counter(
			"p2p_heartbeats_sent_total",
			"Total heartbeats sent",
			"peer_id",
		),
		HeartbeatsReceived: registry.Counter(
			"p2p_heartbeats_received_total",
			"Total heartbeats received",
			"peer_id",
		),
		FailureDetections: registry.Counter(
			"p2p_failure_detections_total",
			"Total peer failure detections",
			"peer_id", "reason",
		),

		// Relay metrics
		RelayCircuits: registry.Gauge(
			"p2p_relay_circuits",
			"Number of active relay circuits",
			"direction",
		),
		RelayBytes: registry.Counter(
			"p2p_relay_bytes_total",
			"Total bytes relayed",
			"direction",
		),

		// QoS metrics
		QueueDepth: registry.Gauge(
			"p2p_queue_depth",
			"Current queue depth",
			"priority",
		),
		QueueLatency: registry.Histogram(
			"p2p_queue_latency_seconds",
			"Time messages spend in queue",
			metrics.DurationBuckets,
			"priority",
		),
		DroppedPackets: registry.Counter(
			"p2p_packets_dropped_total",
			"Total packets dropped",
			"reason",
		),
	}
}

// RecordConnectionEstablished records a new connection
func (m *P2PMetrics) RecordConnectionEstablished(peerID string) {
	m.ConnectionsTotal.WithLabelValues(peerID).Inc()
	m.ConnectionsActive.WithLabelValues("established").Inc()
}

// RecordConnectionClosed records a connection closure
func (m *P2PMetrics) RecordConnectionClosed() {
	m.ConnectionsActive.WithLabelValues("established").Dec()
}

// RecordConnectionFailed records a connection failure
func (m *P2PMetrics) RecordConnectionFailed(reason string) {
	m.ConnectionsFailed.WithLabelValues(reason).Inc()
}

// RecordBytesSent records bytes sent
func (m *P2PMetrics) RecordBytesSent(protocol, peerID string, bytes int64) {
	m.BytesSent.WithLabelValues(protocol, peerID).Add(float64(bytes))
}

// RecordBytesReceived records bytes received
func (m *P2PMetrics) RecordBytesReceived(protocol, peerID string, bytes int64) {
	m.BytesReceived.WithLabelValues(protocol, peerID).Add(float64(bytes))
}

// RecordMessageSent records a sent message
func (m *P2PMetrics) RecordMessageSent(msgType, protocol string, size int) {
	m.MessagesSent.WithLabelValues(msgType, protocol).Inc()
	m.MessageSize.WithLabelValues(msgType).Observe(float64(size))
}

// RecordMessageReceived records a received message
func (m *P2PMetrics) RecordMessageReceived(msgType, protocol string, size int) {
	m.MessagesReceived.WithLabelValues(msgType, protocol).Inc()
	m.MessageSize.WithLabelValues(msgType).Observe(float64(size))
}

// RecordMessageFailed records a message failure
func (m *P2PMetrics) RecordMessageFailed(msgType, reason string) {
	m.MessagesFailed.WithLabelValues(msgType, reason).Inc()
}

// RecordPeerConnected records a peer connection
func (m *P2PMetrics) RecordPeerConnected(role string) {
	m.PeersConnected.WithLabelValues(role).Inc()
}

// RecordPeerDisconnected records a peer disconnection
func (m *P2PMetrics) RecordPeerDisconnected(role string) {
	m.PeersConnected.WithLabelValues(role).Dec()
}

// RecordPeerDiscovered records a peer discovery
func (m *P2PMetrics) RecordPeerDiscovered(source string) {
	m.PeersDiscovered.WithLabelValues(source).Inc()
}

// RecordPeerFailed records a peer failure
func (m *P2PMetrics) RecordPeerFailed(reason string) {
	m.PeersFailed.WithLabelValues(reason).Inc()
}

// RecordOperationDuration records operation duration
func (m *P2PMetrics) RecordOperationDuration(operation string, durationSec float64) {
	m.OperationDuration.WithLabelValues(operation).Observe(durationSec)
}

// RecordGossipMessage records a gossip message
func (m *P2PMetrics) RecordGossipMessage(action, status string) {
	m.GossipMessages.WithLabelValues(action, status).Inc()
}

// RecordGossipPropagation records gossip propagation time
func (m *P2PMetrics) RecordGossipPropagation(durationSec float64) {
	m.GossipPropagation.WithLabelValues().Observe(durationSec)
}

// RecordStreamOpened records a new stream
func (m *P2PMetrics) RecordStreamOpened(protocol string) {
	m.StreamsActive.WithLabelValues(protocol).Inc()
}

// RecordStreamClosed records a stream closure
func (m *P2PMetrics) RecordStreamClosed(protocol string) {
	m.StreamsActive.WithLabelValues(protocol).Dec()
}

// RecordHeartbeatSent records a heartbeat sent
func (m *P2PMetrics) RecordHeartbeatSent(peerID string) {
	m.HeartbeatsSent.WithLabelValues(peerID).Inc()
}

// RecordHeartbeatReceived records a heartbeat received
func (m *P2PMetrics) RecordHeartbeatReceived(peerID string) {
	m.HeartbeatsReceived.WithLabelValues(peerID).Inc()
}

// RecordFailureDetection records a failure detection
func (m *P2PMetrics) RecordFailureDetection(peerID, reason string) {
	m.FailureDetections.WithLabelValues(peerID, reason).Inc()
}

// RecordRelayCircuitOpened records a relay circuit opening
func (m *P2PMetrics) RecordRelayCircuitOpened(direction string) {
	m.RelayCircuits.WithLabelValues(direction).Inc()
}

// RecordRelayCircuitClosed records a relay circuit closing
func (m *P2PMetrics) RecordRelayCircuitClosed(direction string) {
	m.RelayCircuits.WithLabelValues(direction).Dec()
}

// RecordRelayBytes records bytes relayed
func (m *P2PMetrics) RecordRelayBytes(direction string, bytes int64) {
	m.RelayBytes.WithLabelValues(direction).Add(float64(bytes))
}

// SetQueueDepth sets the current queue depth
func (m *P2PMetrics) SetQueueDepth(priority string, depth int) {
	m.QueueDepth.WithLabelValues(priority).Set(float64(depth))
}

// RecordQueueLatency records queue latency
func (m *P2PMetrics) RecordQueueLatency(priority string, durationSec float64) {
	m.QueueLatency.WithLabelValues(priority).Observe(durationSec)
}

// RecordPacketDropped records a dropped packet
func (m *P2PMetrics) RecordPacketDropped(reason string) {
	m.DroppedPackets.WithLabelValues(reason).Inc()
}

// SetIdleConnections sets the number of idle connections
func (m *P2PMetrics) SetIdleConnections(count int) {
	m.ConnectionsIdle.WithLabelValues().Set(float64(count))
}

// UpdateMetrics updates Prometheus metrics for the node
func (n *Node) UpdateMetrics(ctx context.Context) {
	if n.host != nil {
		peerCount := len(n.host.Network().Peers())
		peerConnections.Set(float64(peerCount))
	}
}

