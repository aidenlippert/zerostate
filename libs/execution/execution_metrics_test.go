package execution

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zerostate/libs/metrics"
)

func TestNewExecutionMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	require.NotNil(t, m)
	assert.NotNil(t, m.GuildsTotal)
	assert.NotNil(t, m.TasksTotal)
	assert.NotNil(t, m.WasmExecutions)
	assert.NotNil(t, m.ReceiptsGenerated)
	assert.NotNil(t, m.TaskCostEstimated)
}

func TestRecordGuildOperations(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Create guild
	m.RecordGuildCreated("success", 0.1)
	
	active := testutil.ToFloat64(m.GuildsTotal.WithLabelValues("active"))
	assert.Equal(t, 1.0, active)
	
	created := testutil.ToFloat64(m.GuildOperations.WithLabelValues("create", "success"))
	assert.Equal(t, 1.0, created)

	// Create another guild
	m.RecordGuildCreated("success", 0.2)
	active = testutil.ToFloat64(m.GuildsTotal.WithLabelValues("active"))
	assert.Equal(t, 2.0, active)

	// Dissolve guild
	m.RecordGuildDissolved(0.05)
	active = testutil.ToFloat64(m.GuildsTotal.WithLabelValues("active"))
	assert.Equal(t, 1.0, active)
	
	dissolved := testutil.ToFloat64(m.GuildsTotal.WithLabelValues("dissolved"))
	assert.Equal(t, 1.0, dissolved)
}

func TestRecordGuildMembers(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Member joins
	m.RecordGuildMemberJoin("guild1", "success", 0.05)
	m.RecordGuildMemberJoin("guild1", "success", 0.06)
	
	members := testutil.ToFloat64(m.GuildMembers.WithLabelValues("guild1"))
	assert.Equal(t, 2.0, members)

	// Member leaves
	m.RecordGuildMemberLeave("guild1")
	members = testutil.ToFloat64(m.GuildMembers.WithLabelValues("guild1"))
	assert.Equal(t, 1.0, members)

	// Failed join doesn't increment
	m.RecordGuildMemberJoin("guild1", "failed", 0.05)
	members = testutil.ToFloat64(m.GuildMembers.WithLabelValues("guild1"))
	assert.Equal(t, 1.0, members)
}

func TestRecordTasks(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Submit tasks
	m.RecordTaskSubmitted("compute")
	m.RecordTaskSubmitted("compute")
	
	submitted := testutil.ToFloat64(m.TasksTotal.WithLabelValues("submitted"))
	assert.Equal(t, 2.0, submitted)
	
	active := testutil.ToFloat64(m.TasksActive.WithLabelValues("compute"))
	assert.Equal(t, 2.0, active)

	// Complete successfully
	m.RecordTaskCompleted("compute", 1.5, true)
	completed := testutil.ToFloat64(m.TasksTotal.WithLabelValues("completed"))
	assert.Equal(t, 1.0, completed)
	
	active = testutil.ToFloat64(m.TasksActive.WithLabelValues("compute"))
	assert.Equal(t, 1.0, active)

	// Complete with failure
	m.RecordTaskCompleted("compute", 0.5, false)
	failed := testutil.ToFloat64(m.TasksTotal.WithLabelValues("failed"))
	assert.Equal(t, 1.0, failed)
	
	active = testutil.ToFloat64(m.TasksActive.WithLabelValues("compute"))
	assert.Equal(t, 0.0, active)
}

func TestTaskQueueMetrics(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Set queue depth
	m.SetTaskQueueDepth("high", 10)
	m.SetTaskQueueDepth("low", 5)
	
	highDepth := testutil.ToFloat64(m.TaskQueueDepth.WithLabelValues("high"))
	assert.Equal(t, 10.0, highDepth)
	
	lowDepth := testutil.ToFloat64(m.TaskQueueDepth.WithLabelValues("low"))
	assert.Equal(t, 5.0, lowDepth)

	// Record queue latency
	m.RecordTaskQueueLatency("high", 0.1)
	m.RecordTaskQueueLatency("low", 0.05)
	
	// Histograms just verify they exist
	assert.NotNil(t, m.TaskQueueLatency.WithLabelValues("high"))
}

func TestRecordWasmExecution(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Successful execution
	m.RecordWasmExecution("success", 0.001, 1024*1024, 0)
	
	success := testutil.ToFloat64(m.WasmExecutions.WithLabelValues("success"))
	assert.Equal(t, 1.0, success)
	
	exitCode0 := testutil.ToFloat64(m.WasmExitCodes.WithLabelValues("0"))
	assert.Equal(t, 1.0, exitCode0)

	// Failed execution
	m.RecordWasmExecution("failed", 0.002, 2048*1024, 1)
	
	failed := testutil.ToFloat64(m.WasmExecutions.WithLabelValues("failed"))
	assert.Equal(t, 1.0, failed)
	
	exitCode1 := testutil.ToFloat64(m.WasmExitCodes.WithLabelValues("1"))
	assert.Equal(t, 1.0, exitCode1)
}

func TestRecordWasmValidation(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	m.RecordWasmValidation(0.001)
	m.RecordWasmValidation(0.002)
	
	// Histogram - just verify it exists
	assert.NotNil(t, m.WasmValidationTime.WithLabelValues())
}

func TestRecordManifests(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Create manifests
	m.RecordManifestCreated("task", 256)
	m.RecordManifestCreated("task", 512)
	
	created := testutil.ToFloat64(m.ManifestsCreated.WithLabelValues("task"))
	assert.Equal(t, 2.0, created)

	// Validate manifests
	m.RecordManifestValidation("success")
	m.RecordManifestValidation("success")
	m.RecordManifestValidation("failed")
	
	validSuccess := testutil.ToFloat64(m.ManifestValidation.WithLabelValues("success"))
	assert.Equal(t, 2.0, validSuccess)
	
	validFailed := testutil.ToFloat64(m.ManifestValidation.WithLabelValues("failed"))
	assert.Equal(t, 1.0, validFailed)
}

func TestRecordReceipts(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Generate receipts
	m.RecordReceiptGenerated(512)
	m.RecordReceiptGenerated(1024)
	
	generated := testutil.ToFloat64(m.ReceiptsGenerated.WithLabelValues())
	assert.Equal(t, 2.0, generated)

	// Sign receipts
	m.RecordReceiptSigned("executor")
	m.RecordReceiptSigned("executor")
	m.RecordReceiptSigned("witness")
	
	executorSigs := testutil.ToFloat64(m.ReceiptsSigned.WithLabelValues("executor"))
	assert.Equal(t, 2.0, executorSigs)
	
	witnessSigs := testutil.ToFloat64(m.ReceiptsSigned.WithLabelValues("witness"))
	assert.Equal(t, 1.0, witnessSigs)

	// Verify receipts
	m.RecordReceiptVerified("success")
	m.RecordReceiptVerified("failed")
	
	verifySuccess := testutil.ToFloat64(m.ReceiptsVerified.WithLabelValues("success"))
	assert.Equal(t, 1.0, verifySuccess)
}

func TestRecordTaskCost(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Task with savings (actual < estimated)
	m.RecordTaskCost("compute", 10.0, 5.0, false)
	
	// Task capped
	m.RecordTaskCost("compute", 20.0, 15.0, true)
	
	capped := testutil.ToFloat64(m.TaskCostCapped.WithLabelValues("compute"))
	assert.Equal(t, 1.0, capped)

	// Task over estimate (no savings)
	m.RecordTaskCost("compute", 5.0, 10.0, false)
	
	// Histograms - just verify they exist
	assert.NotNil(t, m.TaskCostEstimated.WithLabelValues("compute"))
	assert.NotNil(t, m.TaskCostActual.WithLabelValues("compute"))
	assert.NotNil(t, m.CostSavings.WithLabelValues())
}

func TestRecordResourceUsage(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	m.RecordResourceUsage("wasm", 0.5, 1024*1024)
	m.RecordResourceUsage("wasm", 1.0, 2048*1024)
	
	// Histograms - just verify they exist
	assert.NotNil(t, m.CPUTime.WithLabelValues("wasm"))
	assert.NotNil(t, m.MemoryAllocated.WithLabelValues("wasm"))
}

func TestSetStorageUsed(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	m.SetStorageUsed("receipts", 1024*1024*10) // 10MB
	
	storage := testutil.ToFloat64(m.StorageUsed.WithLabelValues("receipts"))
	assert.Equal(t, float64(1024*1024*10), storage)

	m.SetStorageUsed("receipts", 1024*1024*20) // 20MB
	storage = testutil.ToFloat64(m.StorageUsed.WithLabelValues("receipts"))
	assert.Equal(t, float64(1024*1024*20), storage)
}

func TestRecordErrors(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Execution errors
	m.RecordExecutionError("wasm", "out_of_memory")
	m.RecordExecutionError("wasm", "out_of_memory")
	m.RecordExecutionError("wasm", "invalid_opcode")
	
	oom := testutil.ToFloat64(m.ExecutionErrors.WithLabelValues("wasm", "out_of_memory"))
	assert.Equal(t, 2.0, oom)
	
	opcode := testutil.ToFloat64(m.ExecutionErrors.WithLabelValues("wasm", "invalid_opcode"))
	assert.Equal(t, 1.0, opcode)

	// Validation errors
	m.RecordValidationError("manifest", "missing_field")
	
	valErr := testutil.ToFloat64(m.ValidationErrors.WithLabelValues("manifest", "missing_field"))
	assert.Equal(t, 1.0, valErr)

	// Timeout errors
	m.RecordTimeoutError("execution")
	m.RecordTimeoutError("execution")
	
	timeouts := testutil.ToFloat64(m.TimeoutErrors.WithLabelValues("execution"))
	assert.Equal(t, 2.0, timeouts)
}

func TestExecutionMetricsConcurrency(t *testing.T) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	// Concurrent task submissions and completions
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.RecordTaskSubmitted("compute")
				m.RecordTaskCompleted("compute", 0.5, true)
			}
			done <- true
		}()
	}

	// Wait for completion
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify counts
	submitted := testutil.ToFloat64(m.TasksTotal.WithLabelValues("submitted"))
	assert.Equal(t, 1000.0, submitted)
	
	completed := testutil.ToFloat64(m.TasksTotal.WithLabelValues("completed"))
	assert.Equal(t, 1000.0, completed)
}

func BenchmarkRecordTaskSubmitted(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordTaskSubmitted("compute")
	}
}

func BenchmarkRecordWasmExecution(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordWasmExecution("success", 0.001, 1024*1024, 0)
	}
}

func BenchmarkRecordTaskCost(b *testing.B) {
	reg := metrics.NewRegistry()
	m := NewExecutionMetrics(reg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordTaskCost("compute", 10.0, 5.0, false)
	}
}
