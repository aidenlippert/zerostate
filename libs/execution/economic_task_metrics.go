package execution

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/aidenlippert/zerostate/libs/metrics"
)

// EconomicTaskMetrics holds all metrics for economic task execution (Sprint 9)
// These metrics track the integrated workflow: escrow creation → WASM execution → automatic settlement
type EconomicTaskMetrics struct {
	// Task Execution Metrics
	TaskExecutionsTotal      *prometheus.CounterVec   // Total economic task executions by status (success, failure)
	TaskExecutionDuration    *prometheus.HistogramVec // Economic task execution duration (includes escrow + WASM + settlement)

	// Escrow Integration Metrics
	EscrowCreations          *prometheus.CounterVec   // Escrow creations by result (success, failed)
	EscrowCreationDuration   *prometheus.HistogramVec // Time to create and fund escrow
	EscrowSettlements        *prometheus.CounterVec   // Escrow settlements by type (release, refund)
	EscrowSettlementDuration *prometheus.HistogramVec // Time to settle escrow (release or refund)
	EscrowAmount             *prometheus.HistogramVec // Escrow amount distribution by outcome

	// WASM Execution Integration
	WasmWithEconomics        *prometheus.CounterVec   // WASM executions with economic context by result
	WasmEconomicDuration     *prometheus.HistogramVec // WASM execution time within economic workflow

	// Payment Flow Metrics
	PaymentsProcessed        *prometheus.CounterVec   // Payments processed by method (escrow, channel)
	PaymentAmount            *prometheus.HistogramVec // Payment amounts by method
	PaymentErrors            *prometheus.CounterVec   // Payment failures by reason

	// Resource Usage Tracking
	ResourceCPUTime          *prometheus.HistogramVec // CPU time per economic task
	ResourceMemoryUsage      *prometheus.HistogramVec // Memory usage per economic task
	ResourceExecutionTime    *prometheus.HistogramVec // Execution time distribution

	// Reputation Integration (placeholder - ready for when service is exported)
	ReputationUpdates        *prometheus.CounterVec   // Reputation updates by type (positive, negative, neutral)
	ReputationDelta          *prometheus.HistogramVec // Reputation score changes

	// Economic Health
	ActiveEconomicTasks      *prometheus.GaugeVec     // Currently executing economic tasks
	EscrowBacklog            *prometheus.GaugeVec     // Pending escrow operations
	SettlementBacklog        *prometheus.GaugeVec     // Pending settlement operations

	// Error Tracking
	PreExecutionErrors       *prometheus.CounterVec   // Pre-execution validation errors by type
	ExecutionErrors          *prometheus.CounterVec   // Execution errors by type
	SettlementErrors         *prometheus.CounterVec   // Settlement errors by type

	// Cost Analysis
	EstimatedCost            *prometheus.HistogramVec // Estimated task cost
	ActualCost               *prometheus.HistogramVec // Actual cost charged
	CostAccuracy             *prometheus.HistogramVec // Difference between estimated and actual
}

// NewEconomicTaskMetrics creates and registers all economic task execution metrics
func NewEconomicTaskMetrics(registry *metrics.Registry) *EconomicTaskMetrics {
	if registry == nil {
		registry = metrics.Default()
	}

	return &EconomicTaskMetrics{
		// Task Execution Metrics
		TaskExecutionsTotal: registry.Counter(
			"economic_task_executions_total",
			"Total number of economic task executions",
			"status", // success, failure
		),
		TaskExecutionDuration: registry.Histogram(
			"economic_task_execution_duration_seconds",
			"Economic task execution duration (end-to-end)",
			metrics.DurationBuckets,
		),

		// Escrow Integration Metrics
		EscrowCreations: registry.Counter(
			"economic_escrow_creations_total",
			"Total escrow creations for economic tasks",
			"result", // success, failed
		),
		EscrowCreationDuration: registry.Histogram(
			"economic_escrow_creation_duration_seconds",
			"Time to create and fund escrow",
			metrics.DurationBuckets,
		),
		EscrowSettlements: registry.Counter(
			"economic_escrow_settlements_total",
			"Total escrow settlements",
			"type", // release, refund
		),
		EscrowSettlementDuration: registry.Histogram(
			"economic_escrow_settlement_duration_seconds",
			"Time to settle escrow (release or refund)",
			metrics.DurationBuckets,
		),
		EscrowAmount: registry.Histogram(
			"economic_escrow_amount",
			"Escrow amount distribution",
			metrics.CostBuckets,
			"outcome", // success, failure
		),

		// WASM Execution Integration
		WasmWithEconomics: registry.Counter(
			"economic_wasm_executions_total",
			"WASM executions with economic integration",
			"result", // success, failure, timeout
		),
		WasmEconomicDuration: registry.Histogram(
			"economic_wasm_duration_seconds",
			"WASM execution time within economic workflow",
			metrics.DurationBuckets,
		),

		// Payment Flow Metrics
		PaymentsProcessed: registry.Counter(
			"economic_payments_processed_total",
			"Payments processed by method",
			"method", // escrow, channel
		),
		PaymentAmount: registry.Histogram(
			"economic_payment_amount",
			"Payment amount distribution",
			metrics.CostBuckets,
			"method",
		),
		PaymentErrors: registry.Counter(
			"economic_payment_errors_total",
			"Payment processing errors",
			"reason",
		),

		// Resource Usage Tracking
		ResourceCPUTime: registry.Histogram(
			"economic_task_cpu_seconds",
			"CPU time per economic task",
			metrics.DurationBuckets,
		),
		ResourceMemoryUsage: registry.Histogram(
			"economic_task_memory_bytes",
			"Memory usage per economic task",
			metrics.BytesBuckets,
		),
		ResourceExecutionTime: registry.Histogram(
			"economic_task_resource_execution_seconds",
			"Resource-tracked execution time",
			metrics.DurationBuckets,
		),

		// Reputation Integration
		ReputationUpdates: registry.Counter(
			"economic_reputation_updates_total",
			"Reputation updates triggered by economic tasks",
			"type", // positive, negative, neutral
		),
		ReputationDelta: registry.Histogram(
			"economic_reputation_delta",
			"Reputation score changes from economic tasks",
			[]float64{-10, -5, -2, -1, 0, 1, 2, 5, 10},
		),

		// Economic Health
		ActiveEconomicTasks: registry.Gauge(
			"economic_tasks_active",
			"Currently executing economic tasks",
		),
		EscrowBacklog: registry.Gauge(
			"economic_escrow_backlog",
			"Pending escrow operations",
			"operation", // create, fund, release, refund
		),
		SettlementBacklog: registry.Gauge(
			"economic_settlement_backlog",
			"Pending settlement operations",
		),

		// Error Tracking
		PreExecutionErrors: registry.Counter(
			"economic_pre_execution_errors_total",
			"Pre-execution validation errors",
			"type", // escrow_not_funded, escrow_not_found, insufficient_funds
		),
		ExecutionErrors: registry.Counter(
			"economic_execution_errors_total",
			"Execution errors during economic tasks",
			"type", // wasm_failure, timeout, out_of_memory
		),
		SettlementErrors: registry.Counter(
			"economic_settlement_errors_total",
			"Settlement processing errors",
			"type", // release_failed, refund_failed, escrow_locked
		),

		// Cost Analysis
		EstimatedCost: registry.Histogram(
			"economic_task_cost_estimated",
			"Estimated task cost",
			metrics.CostBuckets,
		),
		ActualCost: registry.Histogram(
			"economic_task_cost_actual",
			"Actual cost charged",
			metrics.CostBuckets,
		),
		CostAccuracy: registry.Histogram(
			"economic_task_cost_accuracy",
			"Cost estimation accuracy (estimated - actual)",
			[]float64{-1.0, -0.5, -0.1, -0.01, 0, 0.01, 0.1, 0.5, 1.0},
		),
	}
}

// Task Execution Methods

// RecordTaskExecution records a complete economic task execution
func (m *EconomicTaskMetrics) RecordTaskExecution(success bool, duration float64) {
	if success {
		m.TaskExecutionsTotal.WithLabelValues("success").Inc()
	} else {
		m.TaskExecutionsTotal.WithLabelValues("failure").Inc()
	}
	m.TaskExecutionDuration.WithLabelValues().Observe(duration)
}

// RecordTaskStarted records a task starting
func (m *EconomicTaskMetrics) RecordTaskStarted() {
	m.ActiveEconomicTasks.WithLabelValues().Inc()
}

// RecordTaskCompleted records a task completing
func (m *EconomicTaskMetrics) RecordTaskCompleted() {
	m.ActiveEconomicTasks.WithLabelValues().Dec()
}

// Escrow Methods

// RecordEscrowCreation records escrow creation
func (m *EconomicTaskMetrics) RecordEscrowCreation(success bool, duration float64, amount float64) {
	if success {
		m.EscrowCreations.WithLabelValues("success").Inc()
	} else {
		m.EscrowCreations.WithLabelValues("failed").Inc()
	}
	m.EscrowCreationDuration.WithLabelValues().Observe(duration)
	if success {
		m.EscrowAmount.WithLabelValues("pending").Observe(amount)
	}
}

// RecordEscrowSettlement records escrow settlement (release or refund)
func (m *EconomicTaskMetrics) RecordEscrowSettlement(settlementType string, duration float64, amount float64, success bool) {
	m.EscrowSettlements.WithLabelValues(settlementType).Inc()
	m.EscrowSettlementDuration.WithLabelValues().Observe(duration)

	if success {
		outcome := "success"
		if settlementType == "refund" {
			outcome = "failure"
		}
		m.EscrowAmount.WithLabelValues(outcome).Observe(amount)
	}
}

// RecordEscrowBacklog updates escrow operation backlog
func (m *EconomicTaskMetrics) RecordEscrowBacklog(operation string, count int) {
	m.EscrowBacklog.WithLabelValues(operation).Set(float64(count))
}

// WASM Execution Methods

// RecordWasmExecution records WASM execution with economic context
func (m *EconomicTaskMetrics) RecordWasmExecution(result string, duration float64) {
	m.WasmWithEconomics.WithLabelValues(result).Inc()
	m.WasmEconomicDuration.WithLabelValues().Observe(duration)
}

// Payment Methods

// RecordPayment records a payment transaction
func (m *EconomicTaskMetrics) RecordPayment(method string, amount float64, success bool) {
	if success {
		m.PaymentsProcessed.WithLabelValues(method).Inc()
		m.PaymentAmount.WithLabelValues(method).Observe(amount)
	}
}

// RecordPaymentError records a payment error
func (m *EconomicTaskMetrics) RecordPaymentError(reason string) {
	m.PaymentErrors.WithLabelValues(reason).Inc()
}

// Resource Usage Methods

// RecordResourceUsage records resource usage for an economic task
func (m *EconomicTaskMetrics) RecordResourceUsage(cpuTimeSeconds float64, memoryBytes uint64, executionTimeSeconds float64) {
	m.ResourceCPUTime.WithLabelValues().Observe(cpuTimeSeconds)
	if memoryBytes > 0 {
		m.ResourceMemoryUsage.WithLabelValues().Observe(float64(memoryBytes))
	}
	m.ResourceExecutionTime.WithLabelValues().Observe(executionTimeSeconds)
}

// Reputation Methods

// RecordReputationUpdate records a reputation update
func (m *EconomicTaskMetrics) RecordReputationUpdate(updateType string, delta float64) {
	m.ReputationUpdates.WithLabelValues(updateType).Inc()
	m.ReputationDelta.WithLabelValues().Observe(delta)
}

// Error Methods

// RecordPreExecutionError records a pre-execution validation error
func (m *EconomicTaskMetrics) RecordPreExecutionError(errorType string) {
	m.PreExecutionErrors.WithLabelValues(errorType).Inc()
}

// RecordExecutionError records an execution error
func (m *EconomicTaskMetrics) RecordExecutionError(errorType string) {
	m.ExecutionErrors.WithLabelValues(errorType).Inc()
}

// RecordSettlementError records a settlement error
func (m *EconomicTaskMetrics) RecordSettlementError(errorType string) {
	m.SettlementErrors.WithLabelValues(errorType).Inc()
}

// Cost Analysis Methods

// RecordCost records estimated and actual cost for analysis
func (m *EconomicTaskMetrics) RecordCost(estimated, actual float64) {
	m.EstimatedCost.WithLabelValues().Observe(estimated)
	m.ActualCost.WithLabelValues().Observe(actual)

	accuracy := estimated - actual
	m.CostAccuracy.WithLabelValues().Observe(accuracy)
}

// Health Methods

// SetSettlementBacklog sets the settlement backlog gauge
func (m *EconomicTaskMetrics) SetSettlementBacklog(count int) {
	m.SettlementBacklog.WithLabelValues().Set(float64(count))
}
