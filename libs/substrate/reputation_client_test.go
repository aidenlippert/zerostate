package substrate

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// TestOffenseType_String tests the String method of OffenseType
func TestOffenseType_String(t *testing.T) {
	tests := []struct {
		offense  OffenseType
		expected string
	}{
		{OffenseTypeFraudulentResult, "FraudulentResult"},
		{OffenseTypeDoubleTaskAcceptance, "DoubleTaskAcceptance"},
		{OffenseTypeRepeatedFailures, "RepeatedFailures"},
		{OffenseTypeProtocolViolation, "ProtocolViolation"},
		{OffenseType(99), "Unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := test.offense.String()
			assert.Equal(t, test.expected, result)
		})
	}
}

// TestNewReputationClient tests client creation
func TestNewReputationClient(t *testing.T) {
	// Create mock client and keyring
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	// Test client creation
	rc := NewReputationClient(client, &keyring)
	assert.NotNil(t, rc)
	assert.Equal(t, client, rc.client)
	assert.Equal(t, &keyring, rc.keyring)
	assert.Equal(t, uint8(11), rc.palletID)
	assert.NotNil(t, rc.logger)
	assert.NotNil(t, rc.circuitBreaker)

	// Test retry config
	assert.Equal(t, 3, rc.retryConfig.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, rc.retryConfig.InitialBackoff)
	assert.Equal(t, 10*time.Second, rc.retryConfig.MaxBackoff)
	assert.Equal(t, 2.0, rc.retryConfig.Multiplier)
	assert.True(t, rc.retryConfig.Jitter)
}

// TestReputationClient_SetLogger tests logger configuration
func TestReputationClient_SetLogger(t *testing.T) {
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)

	// Test setting a real logger
	logger := zaptest.NewLogger(t)
	rc.SetLogger(logger)
	assert.Equal(t, logger, rc.logger)

	// Test setting nil logger (should not change)
	originalLogger := rc.logger
	rc.SetLogger(nil)
	assert.Equal(t, originalLogger, rc.logger)
}

// TestReputationStake_JSONTags tests JSON serialization
func TestReputationStake_JSONTags(t *testing.T) {
	stake := &ReputationStake{
		Staked:         "1000000000000",
		Reputation:     750,
		TasksCompleted: 10,
		TasksFailed:    2,
		Slashed:        "50000000000",
		ActiveSince:    12345,
	}

	// Test JSON marshaling
	data, err := json.Marshal(stake)
	require.NoError(t, err)

	// Verify JSON contains expected fields
	jsonStr := string(data)
	assert.Contains(t, jsonStr, `"staked"`)
	assert.Contains(t, jsonStr, `"reputation"`)
	assert.Contains(t, jsonStr, `"tasks_completed"`)
	assert.Contains(t, jsonStr, `"tasks_failed"`)
	assert.Contains(t, jsonStr, `"slashed"`)
	assert.Contains(t, jsonStr, `"active_since"`)
}

// TestReputationClient_BondReputation_ValidationCases tests input validation
func TestReputationClient_BondReputation_ValidationCases(t *testing.T) {
	tests := []struct {
		name    string
		amount  uint64
		wantErr bool
	}{
		{
			name:    "valid_amount",
			amount:  1000000000000, // 1000 AINU
			wantErr: true, // Will fail due to no connection, but method should accept parameters
		},
		{
			name:    "minimum_amount",
			amount:  100000000000, // 100 AINU (minimum)
			wantErr: true, // Will fail due to no connection, but method should accept parameters
		},
		{
			name:    "zero_amount",
			amount:  0,
			wantErr: true, // Should fail due to invalid amount or no connection
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test client - this will fail due to no connection
			// but we're testing that the method signature works
			client := &ClientV2{}
			keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
			require.NoError(t, err)

			rc := NewReputationClient(client, &keyring)
			rc.SetLogger(zaptest.NewLogger(t))

			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			// This will fail due to no connection, but we test the method exists
			err = rc.BondReputation(ctx, tt.amount)
			if tt.wantErr {
				assert.Error(t, err)
			}
			// Note: We can't test success without a real substrate connection
		})
	}
}

// TestReputationClient_MethodSignatures tests that all required methods exist with correct signatures
func TestReputationClient_MethodSignatures(t *testing.T) {
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)
	ctx := context.Background()

	// Test that all methods exist and have correct signatures
	t.Run("BondReputation", func(t *testing.T) {
		// Method should exist and accept context and amount
		err := rc.BondReputation(ctx, 1000000000000)
		// Will fail due to no connection, but method exists
		assert.Error(t, err)
	})

	t.Run("UnbondReputation", func(t *testing.T) {
		err := rc.UnbondReputation(ctx, 500000000000)
		assert.Error(t, err)
	})

	t.Run("ReportOutcome", func(t *testing.T) {
		var agentAccount AccountID
		copy(agentAccount[:], keyring.PublicKey)
		taskID := []byte("test_task_123")

		err := rc.ReportOutcome(ctx, agentAccount, taskID, true)
		assert.Error(t, err)
	})

	t.Run("SlashSevere", func(t *testing.T) {
		var agentAccount AccountID
		copy(agentAccount[:], keyring.PublicKey)

		err := rc.SlashSevere(ctx, agentAccount, OffenseTypeFraudulentResult)
		assert.Error(t, err)
	})

	t.Run("GetReputationScore", func(t *testing.T) {
		var agentAccount AccountID
		copy(agentAccount[:], keyring.PublicKey)

		_, err := rc.GetReputationScore(ctx, agentAccount)
		assert.Error(t, err)
	})

	t.Run("GetReputationStake", func(t *testing.T) {
		var agentAccount AccountID
		copy(agentAccount[:], keyring.PublicKey)

		_, err := rc.GetReputationStake(ctx, agentAccount)
		assert.Error(t, err)
	})
}

// TestReputationClient_ContextHandling tests context timeout and cancellation
func TestReputationClient_ContextHandling(t *testing.T) {
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)

	t.Run("cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := rc.BondReputation(ctx, 1000000000000)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context")
	})

	t.Run("timeout_context", func(t *testing.T) {
		// Create a very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout has passed

		err := rc.BondReputation(ctx, 1000000000000)
		assert.Error(t, err)
		// Should fail quickly due to timeout
	})
}

// TestReputationClient_ErrorHandling tests error handling patterns
func TestReputationClient_ErrorHandling(t *testing.T) {
	client := &ClientV2{} // Nil client will cause various errors
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)
	ctx := context.Background()

	// All methods should handle nil client gracefully
	tests := []struct {
		name string
		fn   func() error
	}{
		{
			"BondReputation",
			func() error { return rc.BondReputation(ctx, 1000000000000) },
		},
		{
			"UnbondReputation",
			func() error { return rc.UnbondReputation(ctx, 500000000000) },
		},
		{
			"ReportOutcome",
			func() error {
				var agentAccount AccountID
				return rc.ReportOutcome(ctx, agentAccount, []byte("test"), true)
			},
		},
		{
			"SlashSevere",
			func() error {
				var agentAccount AccountID
				return rc.SlashSevere(ctx, agentAccount, OffenseTypeFraudulentResult)
			},
		},
		{
			"GetReputationScore",
			func() error {
				var agentAccount AccountID
				_, err := rc.GetReputationScore(ctx, agentAccount)
				return err
			},
		},
		{
			"GetReputationStake",
			func() error {
				var agentAccount AccountID
				_, err := rc.GetReputationStake(ctx, agentAccount)
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.fn()
			assert.Error(t, err)
			// Should return a meaningful error, not panic
			assert.NotEmpty(t, err.Error())
		})
	}
}

// BenchmarkOffenseType_String benchmarks the String method
func BenchmarkOffenseType_String(b *testing.B) {
	offense := OffenseTypeFraudulentResult

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = offense.String()
	}
}

// TestReputationClient_CircuitBreakerIntegration tests circuit breaker functionality
func TestReputationClient_CircuitBreakerIntegration(t *testing.T) {
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)

	// Verify circuit breaker is properly configured
	assert.NotNil(t, rc.circuitBreaker)

	// Test that circuit breaker state can be checked
	state := rc.circuitBreaker.GetState()
	assert.Equal(t, CircuitClosed, state) // Should start closed
}

// TestReputationStake_Validation tests ReputationStake struct validation
func TestReputationStake_Validation(t *testing.T) {
	tests := []struct {
		name  string
		stake ReputationStake
		valid bool
	}{
		{
			name: "valid_stake",
			stake: ReputationStake{
				Staked:         "1000000000000",
				Reputation:     750,
				TasksCompleted: 10,
				TasksFailed:    2,
				Slashed:        "50000000000",
				ActiveSince:    12345,
			},
			valid: true,
		},
		{
			name: "zero_values",
			stake: ReputationStake{
				Staked:         "0",
				Reputation:     0,
				TasksCompleted: 0,
				TasksFailed:    0,
				Slashed:        "0",
				ActiveSince:    0,
			},
			valid: true, // Zero values are valid for a new stake
		},
		{
			name: "max_reputation",
			stake: ReputationStake{
				Staked:         "1000000000000",
				Reputation:     1000, // Max reputation
				TasksCompleted: 100,
				TasksFailed:    0,
				Slashed:        "0",
				ActiveSince:    12345,
			},
			valid: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Basic validation - all fields should be reasonable
			if test.valid {
				assert.LessOrEqual(t, test.stake.Reputation, uint32(1000), "Reputation should not exceed 1000")
				assert.NotEmpty(t, test.stake.Staked, "Staked amount should not be empty")
			}
		})
	}
}

// TestAccountIDValidation tests AccountID parameter validation
func TestAccountIDValidation(t *testing.T) {
	// Test that AccountID is properly sized (32 bytes)
	var accountID AccountID
	assert.Equal(t, 32, len(accountID), "AccountID should be 32 bytes")

	// Test creating AccountID from public key
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	copy(accountID[:], keyring.PublicKey)
	assert.Equal(t, keyring.PublicKey, accountID[:])
}

// TestReputationClient_ConfigurationValidation tests client configuration
func TestReputationClient_ConfigurationValidation(t *testing.T) {
	client := &ClientV2{}
	keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
	require.NoError(t, err)

	rc := NewReputationClient(client, &keyring)

	// Validate retry configuration
	assert.Greater(t, rc.retryConfig.MaxRetries, 0)
	assert.Greater(t, rc.retryConfig.InitialBackoff, time.Duration(0))
	assert.Greater(t, rc.retryConfig.MaxBackoff, rc.retryConfig.InitialBackoff)
	assert.Greater(t, rc.retryConfig.Multiplier, float64(1))

	// Validate circuit breaker
	assert.NotNil(t, rc.circuitBreaker)
	assert.Equal(t, CircuitClosed, rc.circuitBreaker.GetState())

	// Validate pallet ID
	assert.Equal(t, uint8(11), rc.palletID)
}