package economic

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zerostate/libs/metrics"
)

// EconomicMetrics holds all economic layer metrics
type EconomicMetrics struct {
	// Payment Channel Metrics
	ChannelsTotal          *prometheus.CounterVec   // Total channels by state (opening, active, closed, disputed)
	ChannelsActive         *prometheus.GaugeVec     // Active channels by party
	ChannelDuration        *prometheus.HistogramVec // Channel lifetime duration
	ChannelBalances        *prometheus.GaugeVec     // Channel balance by channel_id and party
	ChannelOperations      *prometheus.CounterVec   // Channel operations (open, activate, close) by result
	
	// Payment Metrics
	PaymentsTotal          *prometheus.CounterVec   // Total payments by status (success, failed, refunded)
	PaymentAmount          *prometheus.HistogramVec // Payment amount distribution by channel
	PaymentSequence        *prometheus.GaugeVec     // Current payment sequence number by channel
	PaymentDuration        *prometheus.HistogramVec // Payment processing duration
	PaymentFailed          *prometheus.CounterVec   // Failed payments by error type
	
	// Reputation Metrics
	ReputationScores       *prometheus.GaugeVec     // Reputation scores by peer
	TasksExecuted          *prometheus.CounterVec   // Tasks executed by peer and result (success, failure)
	SuccessRate            *prometheus.GaugeVec     // Success rate by peer
	AverageDuration        *prometheus.GaugeVec     // Average task duration by peer (seconds)
	AverageCost            *prometheus.GaugeVec     // Average task cost by peer
	BlacklistedPeers       *prometheus.GaugeVec     // Number of blacklisted peers
	BlacklistEvents        *prometheus.CounterVec   // Blacklist events (added, removed, expired)
	ViolationsTotal        *prometheus.CounterVec   // Violations by peer and type
	
	// Settlement Metrics
	SettlementsTotal       *prometheus.CounterVec   // Settlements by result (success, failed)
	SettlementAmount       *prometheus.HistogramVec // Settlement amount distribution
	SettlementDuration     *prometheus.HistogramVec // Time to settle from channel open
	DisputesTotal          *prometheus.CounterVec   // Disputes by resolution (resolved, pending, arbitrated)
	DisputeResolutionTime  *prometheus.HistogramVec // Time to resolve disputes
	
	// Economic Health Metrics
	TotalValueLocked       *prometheus.GaugeVec     // Total currency locked in channels
	ActiveParticipants     *prometheus.GaugeVec     // Number of active economic participants
	TransactionThroughput  *prometheus.HistogramVec // Transactions per second
	ReputationDecay        *prometheus.CounterVec   // Reputation decay events by peer
}

// NewEconomicMetrics creates a new economic metrics instance
func NewEconomicMetrics(reg *metrics.Registry) *EconomicMetrics {
	m := &EconomicMetrics{
		// Payment Channel Metrics
		ChannelsTotal: reg.Counter(
			"economic_channels_total",
			"Total number of payment channels by state",
			"state",
		),
		
		ChannelsActive: reg.Gauge(
			"economic_channels_active",
			"Number of active payment channels by party",
			"party",
		),
		
		ChannelDuration: reg.Histogram(
			"economic_channel_duration_seconds",
			"Payment channel lifetime duration",
			metrics.DurationBuckets,
			"close_reason",
		),
		
		ChannelBalances: reg.Gauge(
			"economic_channel_balance",
			"Current channel balance by channel and party",
			"channel_id", "party",
		),
		
		ChannelOperations: reg.Counter(
			"economic_channel_operations_total",
			"Payment channel operations by type and result",
			"operation", "result",
		),
		
		// Payment Metrics
		PaymentsTotal: reg.Counter(
			"economic_payments_total",
			"Total number of payments by status",
			"status",
		),
		
		PaymentAmount: reg.Histogram(
			"economic_payment_amount",
			"Payment amount distribution by channel",
			metrics.CostBuckets,
			"channel_id",
		),
		
		PaymentSequence: reg.Gauge(
			"economic_payment_sequence",
			"Current payment sequence number by channel",
			"channel_id",
		),
		
		PaymentDuration: reg.Histogram(
			"economic_payment_duration_seconds",
			"Payment processing duration",
			metrics.DurationBuckets,
		),
		
		PaymentFailed: reg.Counter(
			"economic_payment_failures_total",
			"Failed payments by error type",
			"error_type",
		),
		
		// Reputation Metrics
		ReputationScores: reg.Gauge(
			"economic_reputation_score",
			"Reputation score by peer (0.0-1.0)",
			"peer_id",
		),
		
		TasksExecuted: reg.Counter(
			"economic_tasks_executed_total",
			"Tasks executed by peer and result",
			"peer_id", "result",
		),
		
		SuccessRate: reg.Gauge(
			"economic_success_rate",
			"Task success rate by peer (0.0-1.0)",
			"peer_id",
		),
		
		AverageDuration: reg.Gauge(
			"economic_average_duration_seconds",
			"Average task duration by peer",
			"peer_id",
		),
		
		AverageCost: reg.Gauge(
			"economic_average_cost",
			"Average task cost by peer",
			"peer_id",
		),
		
		BlacklistedPeers: reg.Gauge(
			"economic_blacklisted_peers",
			"Number of currently blacklisted peers",
		),
		
		BlacklistEvents: reg.Counter(
			"economic_blacklist_events_total",
			"Blacklist events by type",
			"event_type",
		),
		
		ViolationsTotal: reg.Counter(
			"economic_violations_total",
			"Violations by peer and type",
			"peer_id", "violation_type",
		),
		
		// Settlement Metrics
		SettlementsTotal: reg.Counter(
			"economic_settlements_total",
			"Total settlements by result",
			"result",
		),
		
		SettlementAmount: reg.Histogram(
			"economic_settlement_amount",
			"Settlement amount distribution",
			metrics.CostBuckets,
		),
		
		SettlementDuration: reg.Histogram(
			"economic_settlement_duration_seconds",
			"Time from channel open to settlement",
			metrics.DurationBuckets,
		),
		
		DisputesTotal: reg.Counter(
			"economic_disputes_total",
			"Total disputes by resolution",
			"resolution",
		),
		
		DisputeResolutionTime: reg.Histogram(
			"economic_dispute_resolution_seconds",
			"Time to resolve disputes",
			metrics.DurationBuckets,
			"resolution",
		),
		
		// Economic Health Metrics
		TotalValueLocked: reg.Gauge(
			"economic_total_value_locked",
			"Total currency locked in payment channels",
		),
		
		ActiveParticipants: reg.Gauge(
			"economic_active_participants",
			"Number of active economic participants",
		),
		
		TransactionThroughput: reg.Histogram(
			"economic_transaction_throughput",
			"Transactions per second",
			metrics.CountBuckets,
		),
		
		ReputationDecay: reg.Counter(
			"economic_reputation_decay_total",
			"Reputation decay events by peer",
			"peer_id",
		),
	}
	
	return m
}

// RecordChannelOpened records a new payment channel opening
func (m *EconomicMetrics) RecordChannelOpened(state, party string, depositA, depositB float64) {
	m.ChannelsTotal.WithLabelValues(state).Inc()
	m.ChannelsActive.WithLabelValues(party).Inc()
	m.TotalValueLocked.WithLabelValues().Add(depositA + depositB)
}

// RecordChannelClosed records a payment channel closure
func (m *EconomicMetrics) RecordChannelClosed(party, reason string, duration float64, finalBalanceA, finalBalanceB float64) {
	m.ChannelsTotal.WithLabelValues("closed").Inc()
	m.ChannelsActive.WithLabelValues(party).Dec()
	m.ChannelDuration.WithLabelValues(reason).Observe(duration)
	m.TotalValueLocked.WithLabelValues().Sub(finalBalanceA + finalBalanceB)
}

// RecordChannelActivated records a channel activation
func (m *EconomicMetrics) RecordChannelActivated(result string) {
	m.ChannelOperations.WithLabelValues("activate", result).Inc()
}

// RecordChannelOperation records a generic channel operation
func (m *EconomicMetrics) RecordChannelOperation(operation, result string) {
	m.ChannelOperations.WithLabelValues(operation, result).Inc()
}

// UpdateChannelBalance updates the balance gauge for a channel
func (m *EconomicMetrics) UpdateChannelBalance(channelID, party string, balance float64) {
	m.ChannelBalances.WithLabelValues(channelID, party).Set(balance)
}

// RecordPayment records a successful payment
func (m *EconomicMetrics) RecordPayment(channelID string, amount float64, sequence uint64, duration float64, success bool) {
	if success {
		m.PaymentsTotal.WithLabelValues("success").Inc()
	} else {
		m.PaymentsTotal.WithLabelValues("failed").Inc()
	}
	
	m.PaymentAmount.WithLabelValues(channelID).Observe(amount)
	m.PaymentSequence.WithLabelValues(channelID).Set(float64(sequence))
	m.PaymentDuration.WithLabelValues().Observe(duration)
}

// RecordPaymentFailure records a failed payment with error type
func (m *EconomicMetrics) RecordPaymentFailure(errorType string) {
	m.PaymentFailed.WithLabelValues(errorType).Inc()
	m.PaymentsTotal.WithLabelValues("failed").Inc()
}

// RecordPaymentRefund records a payment refund
func (m *EconomicMetrics) RecordPaymentRefund(channelID string, amount float64) {
	m.PaymentsTotal.WithLabelValues("refunded").Inc()
	m.PaymentAmount.WithLabelValues(channelID).Observe(amount)
}

// UpdateReputationScore updates a peer's reputation score
func (m *EconomicMetrics) UpdateReputationScore(peerID string, score, successRate float64, avgDuration, avgCost float64) {
	m.ReputationScores.WithLabelValues(peerID).Set(score)
	m.SuccessRate.WithLabelValues(peerID).Set(successRate)
	m.AverageDuration.WithLabelValues(peerID).Set(avgDuration)
	m.AverageCost.WithLabelValues(peerID).Set(avgCost)
}

// RecordTaskExecution records a task execution outcome for reputation
func (m *EconomicMetrics) RecordTaskExecution(peerID string, success bool) {
	if success {
		m.TasksExecuted.WithLabelValues(peerID, "success").Inc()
	} else {
		m.TasksExecuted.WithLabelValues(peerID, "failure").Inc()
	}
}

// RecordBlacklistEvent records a blacklist event
func (m *EconomicMetrics) RecordBlacklistEvent(eventType string) {
	m.BlacklistEvents.WithLabelValues(eventType).Inc()
	
	if eventType == "added" {
		m.BlacklistedPeers.WithLabelValues().Inc()
	} else if eventType == "removed" || eventType == "expired" {
		m.BlacklistedPeers.WithLabelValues().Dec()
	}
}

// RecordViolation records a peer violation
func (m *EconomicMetrics) RecordViolation(peerID, violationType string) {
	m.ViolationsTotal.WithLabelValues(peerID, violationType).Inc()
}

// RecordReputationDecay records reputation decay for a peer
func (m *EconomicMetrics) RecordReputationDecay(peerID string) {
	m.ReputationDecay.WithLabelValues(peerID).Inc()
}

// RecordSettlement records a settlement completion
func (m *EconomicMetrics) RecordSettlement(amount, duration float64, success bool) {
	if success {
		m.SettlementsTotal.WithLabelValues("success").Inc()
	} else {
		m.SettlementsTotal.WithLabelValues("failed").Inc()
	}
	
	m.SettlementAmount.WithLabelValues().Observe(amount)
	m.SettlementDuration.WithLabelValues().Observe(duration)
}

// RecordDispute records a dispute
func (m *EconomicMetrics) RecordDispute(resolution string, resolutionTime float64) {
	m.DisputesTotal.WithLabelValues(resolution).Inc()
	m.DisputeResolutionTime.WithLabelValues(resolution).Observe(resolutionTime)
}

// SetActiveParticipants sets the number of active economic participants
func (m *EconomicMetrics) SetActiveParticipants(count int) {
	m.ActiveParticipants.WithLabelValues().Set(float64(count))
}

// RecordTransactionThroughput records transaction throughput
func (m *EconomicMetrics) RecordTransactionThroughput(tps float64) {
	m.TransactionThroughput.WithLabelValues().Observe(tps)
}
