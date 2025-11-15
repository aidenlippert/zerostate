// Package substrate - Blockchain Integration Service
// Provides high-level blockchain integration for the ZeroState API
package substrate

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"go.uber.org/zap"
)

// BlockchainService manages blockchain connectivity and client lifecycle
type BlockchainService struct {
	client     *ClientV2
	keyring    *signature.KeyringPair
	did        *DIDClient
	registry   *RegistryClient
	escrow     *EscrowClient
	reputation *ReputationClient
	logger     *zap.Logger
	mu         sync.RWMutex
	enabled    bool

	// Production hardening (Sprint 3)
	circuitBreaker *CircuitBreaker
	retryConfig    RetryConfig
	metrics        *Metrics
}

// NewBlockchainService creates a new blockchain service
// If endpoint is empty or connection fails, service runs in disabled mode
func NewBlockchainService(endpoint string, keystoreSecret string, logger *zap.Logger) (*BlockchainService, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	service := &BlockchainService{
		logger:  logger,
		enabled: false,
	}

	// If no endpoint, run in disabled mode
	if endpoint == "" {
		logger.Warn("blockchain service running in disabled mode - no endpoint configured")
		return service, nil
	}

	// Try to connect to blockchain
	logger.Info("connecting to blockchain", zap.String("endpoint", endpoint))
	client, err := NewClientV2(endpoint)
	if err != nil {
		logger.Warn("failed to connect to blockchain - service disabled",
			zap.Error(err),
			zap.String("endpoint", endpoint),
		)
		return service, nil // Return disabled service, don't fail
	}

	// Get chain info
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := client.GetChainInfo(ctx)
	if err != nil {
		logger.Warn("failed to get chain info - closing connection",
			zap.Error(err),
		)
		client.Close()
		return service, nil
	}

	logger.Info("blockchain connected",
		zap.String("chain", info.Name),
		zap.String("version", info.Version),
		zap.Uint64("block", info.BlockNumber),
	)

	// Create keyring from secret
	keyring, err := signature.KeyringPairFromSecret(keystoreSecret, 42)
	if err != nil {
		logger.Warn("failed to create keyring - blockchain disabled",
			zap.Error(err),
		)
		client.Close()
		return service, nil
	}

	logger.Info("blockchain keyring initialized", zap.String("address", keyring.Address))

	// Initialize production hardening components (Sprint 3)
	circuitBreaker := NewCircuitBreaker(
		5,              // Open circuit after 5 failures
		2,              // Close circuit after 2 successes
		30*time.Second, // Wait 30s before half-open
	)
	retryConfig := DefaultRetryConfig()
	metrics := NewMetrics()

	// Create clients
	service.client = client
	service.keyring = &keyring
	service.did = NewDIDClient(client, &keyring)
	service.registry = NewRegistryClient(client, &keyring)
	service.escrow = NewEscrowClientWithLogger(client, &keyring, logger)
	service.reputation = NewReputationClient(client, &keyring)
	service.circuitBreaker = circuitBreaker
	service.retryConfig = retryConfig
	service.metrics = metrics
	service.enabled = true

	return service, nil
}

// IsEnabled returns whether blockchain integration is active
func (s *BlockchainService) IsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.enabled
}

// DID returns the DID client (or nil if disabled)
func (s *BlockchainService) DID() *DIDClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.did
}

// Registry returns the Registry client (or nil if disabled)
func (s *BlockchainService) Registry() *RegistryClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry
}

// Escrow returns the Escrow client (or nil if disabled)
func (s *BlockchainService) Escrow() *EscrowClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.escrow
}

// Reputation returns the Reputation client (or nil if disabled)
func (s *BlockchainService) Reputation() *ReputationClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reputation
}

// GetPublicKey returns the service keyring public key
func (s *BlockchainService) GetPublicKey() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.keyring == nil {
		return nil
	}
	return s.keyring.PublicKey
}

// Close closes the blockchain connection
func (s *BlockchainService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		s.logger.Info("closing blockchain connection")
		s.client.Close()
		s.enabled = false
	}

	return nil
}

// HealthCheck checks blockchain connectivity
func (s *BlockchainService) HealthCheck(ctx context.Context) error {
	if !s.IsEnabled() {
		return fmt.Errorf("blockchain service disabled")
	}

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("blockchain client not initialized")
	}

	return client.HealthCheck(ctx)
}

// GetChainInfo returns current chain information
func (s *BlockchainService) GetChainInfo(ctx context.Context) (*ChainInfo, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("blockchain service disabled")
	}

	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("blockchain client not initialized")
	}

	return client.GetChainInfo(ctx)
}

// GetCircuitBreakerState returns the current circuit breaker state
func (s *BlockchainService) GetCircuitBreakerState() CircuitState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.circuitBreaker == nil {
		return CircuitClosed
	}
	return s.circuitBreaker.GetState()
}

// GetMetrics returns current blockchain operation metrics
func (s *BlockchainService) GetMetrics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.metrics == nil {
		return map[string]interface{}{}
	}
	return s.metrics.GetStats()
}

// GetCircuitBreakerStats returns circuit breaker statistics
func (s *BlockchainService) GetCircuitBreakerStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.circuitBreaker == nil {
		return map[string]interface{}{}
	}
	return s.circuitBreaker.GetStats()
}

// ExecuteWithRetry executes a blockchain operation with retry logic and circuit breaker
func (s *BlockchainService) ExecuteWithRetry(ctx context.Context, operation string, fn func() error) error {
	if !s.IsEnabled() {
		return fmt.Errorf("blockchain service disabled")
	}

	start := time.Now()
	var lastErr error

	// Execute with circuit breaker
	err := s.circuitBreaker.Call(func() error {
		// Execute with retry
		return RetryWithBackoff(ctx, s.retryConfig, fn)
	})

	duration := time.Since(start)

	// Record metrics
	if err != nil {
		lastErr = err
		if s.circuitBreaker.GetState() == CircuitOpen {
			s.metrics.RecordCircuitBreakerTrip()
		}
	}
	s.metrics.RecordRequest(operation, duration, lastErr)

	if err != nil {
		s.logger.Warn("blockchain operation failed",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
			zap.Error(err),
			zap.String("circuit_state", string(s.circuitBreaker.GetState())),
		)
	} else {
		s.logger.Debug("blockchain operation succeeded",
			zap.String("operation", operation),
			zap.Duration("duration", duration),
		)
	}

	return lastErr
}
