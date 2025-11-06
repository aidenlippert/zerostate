package p2p

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

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

// UpdateMetrics updates Prometheus metrics for the node
func (n *Node) UpdateMetrics(ctx context.Context) {
	if n.host != nil {
		peerCount := len(n.host.Network().Peers())
		peerConnections.Set(float64(peerCount))
	}
}
