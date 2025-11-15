package orchestration

import (
	"testing"
)

// TestReputationCircuitBreakerBasic tests the basic circuit breaker functionality
func TestReputationCircuitBreakerBasic(t *testing.T) {
	// Test circuit breaker creation and basic functionality
	cb := &ReputationCircuitBreaker{}

	// Initially should be closed
	if cb.isOpen() {
		t.Error("Expected circuit breaker to be closed initially")
	}

	// Record failures up to but not exceeding the limit
	for i := 0; i < ReputationMaxFailures-1; i++ {
		cb.recordFailure()
	}

	// Should still be closed
	if cb.isOpen() {
		t.Error("Expected circuit breaker to be closed before max failures")
	}

	// One more failure should open it
	cb.recordFailure()
	if !cb.isOpen() {
		t.Error("Expected circuit breaker to be open after max failures")
	}

	// Record success should reset
	cb.recordSuccess()
	if cb.isOpen() {
		t.Error("Expected circuit breaker to be closed after success")
	}
}

// TestReputationConstants tests that the reputation constants are defined correctly
func TestReputationConstants(t *testing.T) {
	if ReputationMaxFailures <= 0 {
		t.Error("Expected ReputationMaxFailures to be positive")
	}

	if ReputationTimeout <= 0 {
		t.Error("Expected ReputationTimeout to be positive")
	}

	if ReputationRetryDelay <= 0 {
		t.Error("Expected ReputationRetryDelay to be positive")
	}
}

// TestOrchestratorMetricsFields tests that the metrics structure has reputation fields
func TestOrchestratorMetricsFields(t *testing.T) {
	metrics := &OrchestratorMetrics{}

	// Check that reputation fields exist and are initialized to zero
	if metrics.ReputationUpdates != 0 {
		t.Error("Expected ReputationUpdates to be initialized to 0")
	}

	if metrics.ReputationFailures != 0 {
		t.Error("Expected ReputationFailures to be initialized to 0")
	}
}

// TestDIDConversionValid tests valid DID conversion
func TestDIDConversionValid(t *testing.T) {
	w := &worker{}

	// Test valid DID format
	validDID := "did:substrate:5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
	account, err := w.convertDIDToAccountID(validDID)
	if err != nil {
		t.Errorf("Expected valid DID to convert successfully: %v", err)
	}

	// Check that we get a non-empty account
	var emptyAccount [32]byte
	if account == emptyAccount {
		t.Error("Expected non-empty account ID")
	}
}

// TestDIDConversionInvalid tests invalid DID conversion
func TestDIDConversionInvalid(t *testing.T) {
	w := &worker{}

	// Test too-short DID
	shortDID := "too:short"
	_, err := w.convertDIDToAccountID(shortDID)
	if err == nil {
		t.Error("Expected short DID to fail conversion")
	}

	// Test empty DID
	_, err = w.convertDIDToAccountID("")
	if err == nil {
		t.Error("Expected empty DID to fail conversion")
	}
}