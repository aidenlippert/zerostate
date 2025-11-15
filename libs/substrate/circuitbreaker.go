package substrate

import (
	"errors"
	"sync"
	"time"
)

// CircuitState represents the state of a circuit breaker
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"    // Normal operation
	CircuitOpen     CircuitState = "open"      // Failing, rejecting requests
	CircuitHalfOpen CircuitState = "half-open" // Testing if service recovered
)

// CircuitBreaker implements the circuit breaker pattern for blockchain operations
type CircuitBreaker struct {
	mu sync.RWMutex

	// Configuration
	failureThreshold int           // Number of failures before opening circuit
	successThreshold int           // Number of successes to close from half-open
	timeout          time.Duration // Time to wait before trying half-open

	// State
	state           CircuitState
	failures        int
	successes       int
	lastFailureTime time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold, successThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
		state:            CircuitClosed,
		lastStateChange:  time.Now(),
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	if err := cb.beforeCall(); err != nil {
		return err
	}

	err := fn()
	cb.afterCall(err)
	return err
}

// beforeCall checks if the circuit breaker allows the call
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitOpen:
		// Check if timeout has elapsed
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = CircuitHalfOpen
			cb.successes = 0
			cb.lastStateChange = time.Now()
			return nil
		}
		return errors.New("circuit breaker is open")

	case CircuitHalfOpen:
		// Allow limited requests to test recovery
		return nil

	case CircuitClosed:
		return nil

	default:
		return nil
	}
}

// afterCall records the result of the call
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.onFailure()
	} else {
		cb.onSuccess()
	}
}

// onFailure handles a failed call
func (cb *CircuitBreaker) onFailure() {
	cb.failures++
	cb.lastFailureTime = time.Now()
	cb.successes = 0

	switch cb.state {
	case CircuitClosed:
		if cb.failures >= cb.failureThreshold {
			cb.state = CircuitOpen
			cb.lastStateChange = time.Now()
		}

	case CircuitHalfOpen:
		// Any failure in half-open state reopens the circuit
		cb.state = CircuitOpen
		cb.lastStateChange = time.Now()
	}
}

// onSuccess handles a successful call
func (cb *CircuitBreaker) onSuccess() {
	cb.successes++
	cb.failures = 0

	if cb.state == CircuitHalfOpen {
		if cb.successes >= cb.successThreshold {
			cb.state = CircuitClosed
			cb.lastStateChange = time.Now()
		}
	}
}

// GetState returns the current circuit state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats returns current circuit breaker statistics
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return map[string]interface{}{
		"state":             cb.state,
		"failures":          cb.failures,
		"successes":         cb.successes,
		"last_failure_time": cb.lastFailureTime,
		"last_state_change": cb.lastStateChange,
		"failure_threshold": cb.failureThreshold,
		"success_threshold": cb.successThreshold,
		"timeout_seconds":   cb.timeout.Seconds(),
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failures = 0
	cb.successes = 0
	cb.lastStateChange = time.Now()
}
