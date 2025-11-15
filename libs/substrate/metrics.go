package substrate

import (
	"math"
	"sync"
	"time"
)

// Metrics tracks blockchain operation metrics
type Metrics struct {
	mu sync.RWMutex

	// Request counts
	TotalRequests      int64
	SuccessfulRequests int64
	FailedRequests     int64

	// Per-operation counts
	DIDOperations      int64
	RegistryOperations int64
	EscrowOperations   int64

	// Latency tracking
	TotalLatency time.Duration
	MinLatency   time.Duration
	MaxLatency   time.Duration
	AvgLatency   time.Duration

	// Error tracking
	ConnectionErrors int64
	TimeoutErrors    int64
	ValidationErrors int64
	OtherErrors      int64

	// Circuit breaker stats
	CircuitBreakerTrips int64

	// Last operation
	LastOperationTime   time.Time
	LastOperationType   string
	LastOperationStatus string
}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		MinLatency: time.Duration(math.MaxInt64),
	}
}

// RecordRequest records a blockchain request
func (m *Metrics) RecordRequest(operation string, duration time.Duration, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	m.LastOperationTime = time.Now()
	m.LastOperationType = operation

	// Update latency stats
	m.TotalLatency += duration
	if duration < m.MinLatency {
		m.MinLatency = duration
	}
	if duration > m.MaxLatency {
		m.MaxLatency = duration
	}
	if m.TotalRequests > 0 {
		m.AvgLatency = m.TotalLatency / time.Duration(m.TotalRequests)
	}

	// Track operation type
	switch {
	case contains(operation, "DID") || contains(operation, "did"):
		m.DIDOperations++
	case contains(operation, "Registry") || contains(operation, "registry") || contains(operation, "Agent"):
		m.RegistryOperations++
	case contains(operation, "Escrow") || contains(operation, "escrow"):
		m.EscrowOperations++
	}

	// Track success/failure
	if err != nil {
		m.FailedRequests++
		m.LastOperationStatus = "failed"

		// Classify error type
		errStr := err.Error()
		switch {
		case contains(errStr, "connection") || contains(errStr, "dial"):
			m.ConnectionErrors++
		case contains(errStr, "timeout"):
			m.TimeoutErrors++
		case contains(errStr, "invalid") || contains(errStr, "validation"):
			m.ValidationErrors++
		default:
			m.OtherErrors++
		}
	} else {
		m.SuccessfulRequests++
		m.LastOperationStatus = "success"
	}
}

// RecordCircuitBreakerTrip records a circuit breaker opening
func (m *Metrics) RecordCircuitBreakerTrip() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CircuitBreakerTrips++
}

// GetStats returns current metrics as a map
func (m *Metrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	successRate := 0.0
	if m.TotalRequests > 0 {
		successRate = float64(m.SuccessfulRequests) / float64(m.TotalRequests) * 100
	}

	return map[string]interface{}{
		"total_requests":      m.TotalRequests,
		"successful_requests": m.SuccessfulRequests,
		"failed_requests":     m.FailedRequests,
		"success_rate":        successRate,

		"did_operations":      m.DIDOperations,
		"registry_operations": m.RegistryOperations,
		"escrow_operations":   m.EscrowOperations,

		"avg_latency_ms": m.AvgLatency.Milliseconds(),
		"min_latency_ms": m.MinLatency.Milliseconds(),
		"max_latency_ms": m.MaxLatency.Milliseconds(),

		"connection_errors": m.ConnectionErrors,
		"timeout_errors":    m.TimeoutErrors,
		"validation_errors": m.ValidationErrors,
		"other_errors":      m.OtherErrors,

		"circuit_breaker_trips": m.CircuitBreakerTrips,

		"last_operation_time":   m.LastOperationTime,
		"last_operation_type":   m.LastOperationType,
		"last_operation_status": m.LastOperationStatus,
	}
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	*m = Metrics{
		MinLatency: time.Duration(math.MaxInt64),
	}
}
