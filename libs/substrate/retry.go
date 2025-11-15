package substrate

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior
type RetryConfig struct {
	MaxRetries     int           // Maximum number of retry attempts
	InitialBackoff time.Duration // Initial backoff duration
	MaxBackoff     time.Duration // Maximum backoff duration
	Multiplier     float64       // Backoff multiplier
	Jitter         bool          // Add jitter to prevent thundering herd
}

// DefaultRetryConfig returns sensible defaults for retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2.0,
		Jitter:         true,
	}
}

// RetryWithBackoff executes a function with exponential backoff retry logic
func RetryWithBackoff(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		default:
		}

		// Try the operation
		err := fn()
		if err == nil {
			return nil // Success!
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt >= config.MaxRetries {
			break
		}

		// Calculate backoff duration
		backoff := calculateBackoff(attempt, config)

		// Sleep with context cancellation support
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled during backoff: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return fmt.Errorf("max retries (%d) exceeded: %w", config.MaxRetries, lastErr)
}

// calculateBackoff calculates the backoff duration for a given attempt
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	// Exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(config.InitialBackoff) * math.Pow(config.Multiplier, float64(attempt))

	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter if enabled (±25% randomization)
	if config.Jitter {
		jitter := backoff * 0.25
		backoff = backoff - jitter + (rand.Float64() * jitter * 2)
	}

	return time.Duration(backoff)
}

// IsRetryable determines if an error should be retried
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Add specific error checks here
	errStr := err.Error()

	// Retry on network errors
	if contains(errStr, "connection refused") ||
		contains(errStr, "connection reset") ||
		contains(errStr, "timeout") ||
		contains(errStr, "temporary failure") ||
		contains(errStr, "no such host") {
		return true
	}

	// Retry on RPC errors
	if contains(errStr, "rpc") ||
		contains(errStr, "websocket") ||
		contains(errStr, "dial") {
		return true
	}

	// Don't retry on validation errors
	if contains(errStr, "invalid") ||
		contains(errStr, "malformed") ||
		contains(errStr, "unauthorized") {
		return false
	}

	// Default: retry on unknown errors
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s != "" && substr != "" &&
		(s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
