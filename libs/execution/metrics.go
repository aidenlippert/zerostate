package execution

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/aidenlippert/zerostate/libs/metrics"
)

// ExecutionMetrics holds all execution-related Prometheus metrics
type ExecutionMetrics struct {
	// Guild metrics
	GuildsTotal      *prometheus.GaugeVec
	GuildOperations  *prometheus.CounterVec
	GuildMembers     *prometheus.GaugeVec
	GuildDuration    *prometheus.HistogramVec
	
	// Task metrics
	TasksTotal       *prometheus.CounterVec
	TasksActive      *prometheus.GaugeVec
	TaskDuration     *prometheus.HistogramVec
	TaskQueueDepth   *prometheus.GaugeVec
	TaskQueueLatency *prometheus.HistogramVec
	
	// WASM execution metrics
	WasmExecutions      *prometheus.CounterVec
	WasmDuration        *prometheus.HistogramVec
	WasmMemoryBytes     *prometheus.HistogramVec
	WasmExitCodes       *prometheus.CounterVec
	WasmValidationTime  *prometheus.HistogramVec
	
	// Manifest metrics
	ManifestsCreated   *prometheus.CounterVec
	ManifestValidation *prometheus.CounterVec
	ManifestSize       *prometheus.HistogramVec
	
	// Receipt metrics
	ReceiptsGenerated *prometheus.CounterVec
	ReceiptsSigned    *prometheus.CounterVec
	ReceiptsVerified  *prometheus.CounterVec
	ReceiptSize       *prometheus.HistogramVec
	
	// Cost metrics
	TaskCostEstimated *prometheus.HistogramVec
	TaskCostActual    *prometheus.HistogramVec
	TaskCostCapped    *prometheus.CounterVec
	CostSavings       *prometheus.HistogramVec
	
	// Resource metrics
	CPUTime          *prometheus.HistogramVec
	MemoryAllocated  *prometheus.HistogramVec
	StorageUsed      *prometheus.GaugeVec
	
	// Error metrics
	ExecutionErrors  *prometheus.CounterVec
	ValidationErrors *prometheus.CounterVec
	TimeoutErrors    *prometheus.CounterVec
}

// NewExecutionMetrics creates and registers all execution metrics
func NewExecutionMetrics(registry *metrics.Registry) *ExecutionMetrics {
	if registry == nil {
		registry = metrics.Default()
	}

	return &ExecutionMetrics{
		// Guild metrics
		GuildsTotal: registry.Gauge(
			"guild_total",
			"Total number of guilds by state",
			"state",
		),
		GuildOperations: registry.Counter(
			"guild_operations_total",
			"Total guild operations",
			"operation", "status",
		),
		GuildMembers: registry.Gauge(
			"guild_members",
			"Number of guild members",
			"guild_id",
		),
		GuildDuration: registry.Histogram(
			"guild_operation_duration_seconds",
			"Guild operation duration",
			metrics.DurationBuckets,
			"operation",
		),

		// Task metrics
		TasksTotal: registry.Counter(
			"tasks_total",
			"Total tasks by status",
			"status",
		),
		TasksActive: registry.Gauge(
			"tasks_active",
			"Currently active tasks",
			"type",
		),
		TaskDuration: registry.Histogram(
			"task_duration_seconds",
			"Task execution duration",
			metrics.DurationBuckets,
			"type",
		),
		TaskQueueDepth: registry.Gauge(
			"task_queue_depth",
			"Number of tasks in queue",
			"priority",
		),
		TaskQueueLatency: registry.Histogram(
			"task_queue_latency_seconds",
			"Time tasks spend waiting in queue",
			metrics.DurationBuckets,
			"priority",
		),

		// WASM execution metrics
		WasmExecutions: registry.Counter(
			"wasm_executions_total",
			"Total WASM executions",
			"status",
		),
		WasmDuration: registry.Histogram(
			"wasm_execution_duration_seconds",
			"WASM execution duration",
			metrics.DurationBuckets,
		),
		WasmMemoryBytes: registry.Histogram(
			"wasm_memory_bytes",
			"WASM memory usage",
			metrics.BytesBuckets,
		),
		WasmExitCodes: registry.Counter(
			"wasm_exit_codes_total",
			"WASM exit codes",
			"code",
		),
		WasmValidationTime: registry.Histogram(
			"wasm_validation_duration_seconds",
			"WASM validation duration",
			metrics.DurationBuckets,
		),

		// Manifest metrics
		ManifestsCreated: registry.Counter(
			"manifests_created_total",
			"Total manifests created",
			"type",
		),
		ManifestValidation: registry.Counter(
			"manifest_validation_total",
			"Total manifest validations",
			"result",
		),
		ManifestSize: registry.Histogram(
			"manifest_size_bytes",
			"Manifest size distribution",
			metrics.BytesBuckets,
		),

		// Receipt metrics
		ReceiptsGenerated: registry.Counter(
			"receipts_generated_total",
			"Total receipts generated",
		),
		ReceiptsSigned: registry.Counter(
			"receipts_signed_total",
			"Total receipts signed",
			"signer",
		),
		ReceiptsVerified: registry.Counter(
			"receipts_verified_total",
			"Total receipt verifications",
			"result",
		),
		ReceiptSize: registry.Histogram(
			"receipt_size_bytes",
			"Receipt size distribution",
			metrics.BytesBuckets,
		),

		// Cost metrics
		TaskCostEstimated: registry.Histogram(
			"task_cost_estimated_units",
			"Estimated task cost",
			metrics.CostBuckets,
			"type",
		),
		TaskCostActual: registry.Histogram(
			"task_cost_actual_units",
			"Actual task cost",
			metrics.CostBuckets,
			"type",
		),
		TaskCostCapped: registry.Counter(
			"task_cost_capped_total",
			"Total tasks with capped cost",
			"type",
		),
		CostSavings: registry.Histogram(
			"task_cost_savings_units",
			"Cost savings from faster execution",
			metrics.CostBuckets,
		),

		// Resource metrics
		CPUTime: registry.Histogram(
			"execution_cpu_seconds",
			"CPU time used by execution",
			metrics.DurationBuckets,
			"type",
		),
		MemoryAllocated: registry.Histogram(
			"execution_memory_allocated_bytes",
			"Memory allocated for execution",
			metrics.BytesBuckets,
			"type",
		),
		StorageUsed: registry.Gauge(
			"execution_storage_used_bytes",
			"Storage used by executions",
			"type",
		),

		// Error metrics
		ExecutionErrors: registry.Counter(
			"execution_errors_total",
			"Total execution errors",
			"type", "reason",
		),
		ValidationErrors: registry.Counter(
			"validation_errors_total",
			"Total validation errors",
			"type", "reason",
		),
		TimeoutErrors: registry.Counter(
			"timeout_errors_total",
			"Total timeout errors",
			"type",
		),
	}
}

// Guild Metrics Methods

// RecordGuildCreated records a guild creation
func (m *ExecutionMetrics) RecordGuildCreated(status string, duration float64) {
	m.GuildOperations.WithLabelValues("create", status).Inc()
	m.GuildDuration.WithLabelValues("create").Observe(duration)
	if status == "success" {
		m.GuildsTotal.WithLabelValues("active").Inc()
	}
}

// RecordGuildDissolved records a guild dissolution
func (m *ExecutionMetrics) RecordGuildDissolved(duration float64) {
	m.GuildOperations.WithLabelValues("dissolve", "success").Inc()
	m.GuildDuration.WithLabelValues("dissolve").Observe(duration)
	m.GuildsTotal.WithLabelValues("active").Dec()
	m.GuildsTotal.WithLabelValues("dissolved").Inc()
}

// RecordGuildMemberJoin records a member joining
func (m *ExecutionMetrics) RecordGuildMemberJoin(guildID string, status string, duration float64) {
	m.GuildOperations.WithLabelValues("join", status).Inc()
	m.GuildDuration.WithLabelValues("join").Observe(duration)
	if status == "success" {
		m.GuildMembers.WithLabelValues(guildID).Inc()
	}
}

// RecordGuildMemberLeave records a member leaving
func (m *ExecutionMetrics) RecordGuildMemberLeave(guildID string) {
	m.GuildMembers.WithLabelValues(guildID).Dec()
}

// Task Metrics Methods

// RecordTaskSubmitted records a task submission
func (m *ExecutionMetrics) RecordTaskSubmitted(taskType string) {
	m.TasksTotal.WithLabelValues("submitted").Inc()
	m.TasksActive.WithLabelValues(taskType).Inc()
}

// RecordTaskCompleted records a task completion
func (m *ExecutionMetrics) RecordTaskCompleted(taskType string, duration float64, success bool) {
	if success {
		m.TasksTotal.WithLabelValues("completed").Inc()
	} else {
		m.TasksTotal.WithLabelValues("failed").Inc()
	}
	m.TasksActive.WithLabelValues(taskType).Dec()
	m.TaskDuration.WithLabelValues(taskType).Observe(duration)
}

// SetTaskQueueDepth sets the current task queue depth
func (m *ExecutionMetrics) SetTaskQueueDepth(priority string, depth int) {
	m.TaskQueueDepth.WithLabelValues(priority).Set(float64(depth))
}

// RecordTaskQueueLatency records task queue wait time
func (m *ExecutionMetrics) RecordTaskQueueLatency(priority string, latency float64) {
	m.TaskQueueLatency.WithLabelValues(priority).Observe(latency)
}

// WASM Metrics Methods

// RecordWasmExecution records a WASM execution
func (m *ExecutionMetrics) RecordWasmExecution(status string, duration float64, memoryBytes int64, exitCode int32) {
	m.WasmExecutions.WithLabelValues(status).Inc()
	m.WasmDuration.WithLabelValues().Observe(duration)
	m.WasmMemoryBytes.WithLabelValues().Observe(float64(memoryBytes))
	m.WasmExitCodes.WithLabelValues(string(rune(exitCode + '0'))).Inc()
}

// RecordWasmValidation records WASM validation
func (m *ExecutionMetrics) RecordWasmValidation(duration float64) {
	m.WasmValidationTime.WithLabelValues().Observe(duration)
}

// Manifest Metrics Methods

// RecordManifestCreated records manifest creation
func (m *ExecutionMetrics) RecordManifestCreated(manifestType string, size int) {
	m.ManifestsCreated.WithLabelValues(manifestType).Inc()
	m.ManifestSize.WithLabelValues().Observe(float64(size))
}

// RecordManifestValidation records manifest validation
func (m *ExecutionMetrics) RecordManifestValidation(result string) {
	m.ManifestValidation.WithLabelValues(result).Inc()
}

// Receipt Metrics Methods

// RecordReceiptGenerated records receipt generation
func (m *ExecutionMetrics) RecordReceiptGenerated(size int) {
	m.ReceiptsGenerated.WithLabelValues().Inc()
	m.ReceiptSize.WithLabelValues().Observe(float64(size))
}

// RecordReceiptSigned records receipt signing
func (m *ExecutionMetrics) RecordReceiptSigned(signer string) {
	m.ReceiptsSigned.WithLabelValues(signer).Inc()
}

// RecordReceiptVerified records receipt verification
func (m *ExecutionMetrics) RecordReceiptVerified(result string) {
	m.ReceiptsVerified.WithLabelValues(result).Inc()
}

// Cost Metrics Methods

// RecordTaskCost records estimated and actual task cost
func (m *ExecutionMetrics) RecordTaskCost(taskType string, estimated, actual float64, capped bool) {
	m.TaskCostEstimated.WithLabelValues(taskType).Observe(estimated)
	m.TaskCostActual.WithLabelValues(taskType).Observe(actual)
	
	if capped {
		m.TaskCostCapped.WithLabelValues(taskType).Inc()
	}
	
	if actual < estimated {
		savings := estimated - actual
		m.CostSavings.WithLabelValues().Observe(savings)
	}
}

// Resource Metrics Methods

// RecordResourceUsage records CPU and memory usage
func (m *ExecutionMetrics) RecordResourceUsage(execType string, cpuSeconds float64, memoryBytes int64) {
	m.CPUTime.WithLabelValues(execType).Observe(cpuSeconds)
	m.MemoryAllocated.WithLabelValues(execType).Observe(float64(memoryBytes))
}

// SetStorageUsed sets storage usage
func (m *ExecutionMetrics) SetStorageUsed(storageType string, bytes int64) {
	m.StorageUsed.WithLabelValues(storageType).Set(float64(bytes))
}

// Error Metrics Methods

// RecordExecutionError records an execution error
func (m *ExecutionMetrics) RecordExecutionError(errType, reason string) {
	m.ExecutionErrors.WithLabelValues(errType, reason).Inc()
}

// RecordValidationError records a validation error
func (m *ExecutionMetrics) RecordValidationError(errType, reason string) {
	m.ValidationErrors.WithLabelValues(errType, reason).Inc()
}

// RecordTimeoutError records a timeout error
func (m *ExecutionMetrics) RecordTimeoutError(errType string) {
	m.TimeoutErrors.WithLabelValues(errType).Inc()
}
