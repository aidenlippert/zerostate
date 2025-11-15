package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics contains all application-specific Prometheus metrics
type PrometheusMetrics struct {
	// Task Metrics
	TasksTotal     *prometheus.CounterVec
	TaskDuration   *prometheus.HistogramVec
	TasksInQueue   prometheus.Gauge
	TaskExecutions *prometheus.CounterVec
	TaskErrors     *prometheus.CounterVec

	// Agent Metrics
	AgentsRegistered   prometheus.Gauge
	AgentSelectionTime *prometheus.HistogramVec
	AgentBidsReceived  *prometheus.CounterVec
	AgentHealthStatus  *prometheus.GaugeVec
	AgentConnections   prometheus.Gauge

	// Auction Metrics
	AuctionsTotal       *prometheus.CounterVec
	AuctionDuration     *prometheus.HistogramVec
	VCGEfficiencyRatio  prometheus.Gauge
	BidProcessingTime   *prometheus.HistogramVec
	AuctionParticipants prometheus.Gauge

	// Reputation Metrics
	ReputationUpdates      *prometheus.CounterVec
	ReputationUpdateTime   *prometheus.HistogramVec
	CircuitBreakerState    *prometheus.GaugeVec
	ReputationScore        *prometheus.GaugeVec
	ReputationRecoveryTime *prometheus.HistogramVec

	// Payment Metrics
	PaymentsTotal   *prometheus.CounterVec
	PaymentAmount   *prometheus.HistogramVec
	PaymentLatency  *prometheus.HistogramVec
	EscrowBalance   prometheus.Gauge
	PaymentFailures *prometheus.CounterVec

	// Blockchain Metrics
	BlockchainCalls        *prometheus.CounterVec
	BlockchainCallDuration *prometheus.HistogramVec
	BlockchainConnection   prometheus.Gauge
	BlockHeight            prometheus.Gauge
	TransactionPool        prometheus.Gauge

	// API & System Metrics
	APIRequests        *prometheus.CounterVec
	APIRequestDuration *prometheus.HistogramVec
	ActiveConnections  prometheus.Gauge

	// Runtime Registry Metrics
	RuntimeCount           prometheus.Gauge
	RuntimeStatus          *prometheus.GaugeVec
	RuntimeCapabilityCount *prometheus.GaugeVec
	RuntimeEvents          *prometheus.CounterVec

	// Go Runtime Metrics (handled by default collector)
	registry *prometheus.Registry
}

var (
	defaultMetrics     *PrometheusMetrics
	defaultMetricsOnce sync.Once
)

// GetDefaultMetrics returns the singleton metrics instance
func GetDefaultMetrics() *PrometheusMetrics {
	defaultMetricsOnce.Do(func() {
		defaultMetrics = NewPrometheusMetrics(prometheus.DefaultRegisterer)
	})
	return defaultMetrics
}

// NewPrometheusMetrics creates a new PrometheusMetrics instance
func NewPrometheusMetrics(registerer prometheus.Registerer) *PrometheusMetrics {
	factory := promauto.With(registerer)

	m := &PrometheusMetrics{
		// Task Metrics
		TasksTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "tasks_total",
				Help:      "Total number of tasks processed by status",
			},
			[]string{"status"}, // pending, running, completed, failed
		),

		TaskDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "task_duration_seconds",
				Help:      "Task execution duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0, 600.0},
			},
			[]string{"task_type", "status"},
		),

		TasksInQueue: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "tasks_in_queue",
				Help:      "Number of tasks currently in queue",
			},
		),

		TaskExecutions: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "task_executions_total",
				Help:      "Total task executions by executor type",
			},
			[]string{"executor_type", "result"},
		),

		TaskErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "task_errors_total",
				Help:      "Total task errors by error type",
			},
			[]string{"error_type", "task_type"},
		),

		// Agent Metrics
		AgentsRegistered: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "agents_registered_total",
				Help:      "Number of registered agents",
			},
		),

		AgentSelectionTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "agent_selection_duration_seconds",
				Help:      "Agent selection duration in seconds",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
			},
			[]string{"selection_type"},
		),

		AgentBidsReceived: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "agent_bids_received_total",
				Help:      "Total agent bids received",
			},
			[]string{"auction_type"},
		),

		AgentHealthStatus: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "agent_health_status",
				Help:      "Agent health status (1=healthy, 0=unhealthy)",
			},
			[]string{"agent_id", "agent_type"},
		),

		AgentConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "agent_connections_active",
				Help:      "Number of active agent connections",
			},
		),

		// Auction Metrics
		AuctionsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "auctions_total",
				Help:      "Total auctions completed by type",
			},
			[]string{"type"}, // vcg, first_price, sealed_bid
		),

		AuctionDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "auction_duration_seconds",
				Help:      "Auction duration in seconds",
				Buckets:   []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0},
			},
			[]string{"type"},
		),

		VCGEfficiencyRatio: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "vcg_efficiency_ratio",
				Help:      "VCG auction efficiency ratio",
			},
		),

		BidProcessingTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "bid_processing_duration_seconds",
				Help:      "Bid processing duration in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
			},
			[]string{"bid_type"},
		),

		AuctionParticipants: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "auction_participants",
				Help:      "Number of auction participants in current auction",
			},
		),

		// Reputation Metrics
		ReputationUpdates: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "reputation_updates_total",
				Help:      "Total reputation updates by result",
			},
			[]string{"result"}, // success, failure, timeout
		),

		ReputationUpdateTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "reputation_update_duration_seconds",
				Help:      "Reputation update duration in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0},
			},
			[]string{"update_type"},
		),

		CircuitBreakerState: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "reputation_circuit_breaker_state",
				Help:      "Circuit breaker state (1=open, 0=closed)",
			},
			[]string{"service", "state"}, // open, closed, half_open
		),

		ReputationScore: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "reputation_score",
				Help:      "Current reputation score of agents",
			},
			[]string{"agent_id"},
		),

		ReputationRecoveryTime: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "reputation_recovery_duration_seconds",
				Help:      "Time to recover from reputation penalties",
				Buckets:   []float64{1.0, 5.0, 10.0, 30.0, 60.0, 300.0, 600.0, 1800.0},
			},
			[]string{"recovery_type"},
		),

		// Payment Metrics
		PaymentsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "payments_total",
				Help:      "Total payments processed by type",
			},
			[]string{"type"}, // released, refunded, disputed, escrowed
		),

		PaymentAmount: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "payment_amount_ainu",
				Help:      "Payment amount in AINU tokens",
				Buckets:   []float64{0.001, 0.01, 0.1, 1.0, 10.0, 100.0, 1000.0, 10000.0},
			},
			[]string{"payment_type"},
		),

		PaymentLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "payment_latency_seconds",
				Help:      "Payment processing latency in seconds",
				Buckets:   []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0},
			},
			[]string{"payment_type"},
		),

		EscrowBalance: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "escrow_balance_ainu",
				Help:      "Total amount in escrow (AINU tokens)",
			},
		),

		PaymentFailures: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "payment_failures_total",
				Help:      "Total payment failures by reason",
			},
			[]string{"failure_reason"},
		),

		// Blockchain Metrics
		BlockchainCalls: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "blockchain_calls_total",
				Help:      "Total blockchain calls by method and result",
			},
			[]string{"method", "result"}, // success, failure, timeout
		),

		BlockchainCallDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "blockchain_call_duration_seconds",
				Help:      "Blockchain call duration in seconds",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0},
			},
			[]string{"method"},
		),

		BlockchainConnection: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "blockchain_connection_status",
				Help:      "Blockchain connection status (1=connected, 0=disconnected)",
			},
		),

		BlockHeight: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "blockchain_block_height",
				Help:      "Current blockchain block height",
			},
		),

		TransactionPool: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "blockchain_transaction_pool_size",
				Help:      "Number of transactions in mempool",
			},
		),

		// API & System Metrics
		APIRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "api_requests_total",
				Help:      "Total API requests by endpoint, method, and status",
			},
			[]string{"endpoint", "method", "status"},
		),

		APIRequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "ainur",
				Name:      "api_request_duration_seconds",
				Help:      "API request duration in seconds",
				Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
			},
			[]string{"endpoint", "method"},
		),

		ActiveConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "api_connections_active",
				Help:      "Number of active API connections",
			},
		),

		// Runtime Registry Metrics
		RuntimeCount: factory.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "runtime_registry_count",
				Help:      "Number of runtimes currently registered",
			},
		),

		RuntimeStatus: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "runtime_registry_status",
				Help:      "Number of runtimes by status",
			},
			[]string{"status"},
		),

		RuntimeCapabilityCount: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "ainur",
				Name:      "runtime_registry_capabilities",
				Help:      "Number of runtimes by advertised capability",
			},
			[]string{"capability"},
		),

		RuntimeEvents: factory.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ainur",
				Name:      "runtime_registry_events_total",
				Help:      "Runtime registry events by type (discovered, updated, removed, timed_out)",
			},
			[]string{"event"},
		),
	}

	// Register Go runtime metrics
	if reg, ok := registerer.(*prometheus.Registry); ok {
		m.registry = reg
		reg.MustRegister(prometheus.NewGoCollector())
		reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	}

	return m
}

// RecordTaskCompletion records task completion metrics
func (m *PrometheusMetrics) RecordTaskCompletion(taskType, status string, duration time.Duration) {
	m.TasksTotal.WithLabelValues(status).Inc()
	m.TaskDuration.WithLabelValues(taskType, status).Observe(duration.Seconds())
}

// RecordAgentSelection records agent selection metrics
func (m *PrometheusMetrics) RecordAgentSelection(selectionType string, duration time.Duration) {
	m.AgentSelectionTime.WithLabelValues(selectionType).Observe(duration.Seconds())
}

// RecordAuction records auction completion metrics
func (m *PrometheusMetrics) RecordAuction(auctionType string, duration time.Duration, participants int) {
	m.AuctionsTotal.WithLabelValues(auctionType).Inc()
	m.AuctionDuration.WithLabelValues(auctionType).Observe(duration.Seconds())
	m.AuctionParticipants.Set(float64(participants))
}

// RecordPayment records payment metrics
func (m *PrometheusMetrics) RecordPayment(paymentType string, amount float64, latency time.Duration) {
	m.PaymentsTotal.WithLabelValues(paymentType).Inc()
	m.PaymentAmount.WithLabelValues(paymentType).Observe(amount)
	m.PaymentLatency.WithLabelValues(paymentType).Observe(latency.Seconds())
}

// RecordBlockchainCall records blockchain call metrics
func (m *PrometheusMetrics) RecordBlockchainCall(method, result string, duration time.Duration) {
	m.BlockchainCalls.WithLabelValues(method, result).Inc()
	m.BlockchainCallDuration.WithLabelValues(method).Observe(duration.Seconds())
}

// RecordAPIRequest records API request metrics
func (m *PrometheusMetrics) RecordAPIRequest(endpoint, method, status string, duration time.Duration) {
	m.APIRequests.WithLabelValues(endpoint, method, status).Inc()
	m.APIRequestDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())
}

// UpdateQueueSize updates the current queue size
func (m *PrometheusMetrics) UpdateQueueSize(size int) {
	m.TasksInQueue.Set(float64(size))
}

// UpdateAgentCount updates the number of registered agents
func (m *PrometheusMetrics) UpdateAgentCount(count int) {
	m.AgentsRegistered.Set(float64(count))
}

// UpdateBlockchainStatus updates blockchain connection status
func (m *PrometheusMetrics) UpdateBlockchainStatus(connected bool) {
	if connected {
		m.BlockchainConnection.Set(1)
	} else {
		m.BlockchainConnection.Set(0)
	}
}

// UpdateCircuitBreaker updates circuit breaker state
func (m *PrometheusMetrics) UpdateCircuitBreaker(service, state string) {
	m.CircuitBreakerState.WithLabelValues(service, state).Set(1)
	// Reset other states
	states := []string{"open", "closed", "half_open"}
	for _, s := range states {
		if s != state {
			m.CircuitBreakerState.WithLabelValues(service, s).Set(0)
		}
	}
}

// UpdateReputationScore updates an agent's reputation score
func (m *PrometheusMetrics) UpdateReputationScore(agentID string, score float64) {
	m.ReputationScore.WithLabelValues(agentID).Set(score)
}

// UpdateActiveConnections updates the number of active connections
func (m *PrometheusMetrics) UpdateActiveConnections(count int) {
	m.ActiveConnections.Set(float64(count))
}

// UpdateRuntimeCount updates the total runtime count in the registry metrics
func (m *PrometheusMetrics) UpdateRuntimeCount(count int) {
	if m == nil || m.RuntimeCount == nil {
		return
	}
	m.RuntimeCount.Set(float64(count))
}

// UpdateRuntimeStatus updates the runtime count for a specific status
func (m *PrometheusMetrics) UpdateRuntimeStatus(status string, count int) {
	if m == nil || m.RuntimeStatus == nil {
		return
	}
	if status == "" {
		status = "unknown"
	}
	m.RuntimeStatus.WithLabelValues(status).Set(float64(count))
}

// UpdateRuntimeCapability updates the runtime count for a specific capability
func (m *PrometheusMetrics) UpdateRuntimeCapability(capability string, count int) {
	if m == nil || m.RuntimeCapabilityCount == nil {
		return
	}
	if capability == "" {
		capability = "unspecified"
	}
	m.RuntimeCapabilityCount.WithLabelValues(capability).Set(float64(count))
}

// RecordRuntimeEvent increments the runtime registry event counter
func (m *PrometheusMetrics) RecordRuntimeEvent(event string) {
	if m == nil || m.RuntimeEvents == nil {
		return
	}
	if event == "" {
		event = "unknown"
	}
	m.RuntimeEvents.WithLabelValues(event).Inc()
}

// GetRuntimeMetrics returns current Go runtime metrics as a map
func (m *PrometheusMetrics) GetRuntimeMetrics() map[string]interface{} {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	return map[string]interface{}{
		"goroutines":     runtime.NumGoroutine(),
		"memory_alloc":   memStats.Alloc,
		"memory_total":   memStats.TotalAlloc,
		"memory_sys":     memStats.Sys,
		"gc_cycles":      memStats.NumGC,
		"gc_pause_total": memStats.PauseTotalNs,
	}
}
