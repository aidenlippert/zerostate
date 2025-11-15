package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/substrate"
	"go.uber.org/zap"
)

// ReputationCircuitBreaker manages reputation service availability
type ReputationCircuitBreaker struct {
	failures    int
	lastFailure time.Time
	mu          sync.RWMutex
}

// Circuit breaker states
const (
	ReputationMaxFailures = 5
	ReputationTimeout     = 30 * time.Second
	ReputationRetryDelay  = 1 * time.Second
)

// isOpen returns true if the circuit breaker is open (should block requests)
func (cb *ReputationCircuitBreaker) isOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.failures >= ReputationMaxFailures {
		if time.Since(cb.lastFailure) < ReputationTimeout {
			return true
		}
		// Reset after timeout
		cb.failures = 0
	}
	return false
}

// recordFailure records a failure and updates the circuit breaker state
func (cb *ReputationCircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.lastFailure = time.Now()
}

// recordSuccess resets the failure count
func (cb *ReputationCircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
}

// Global circuit breaker for reputation service
var reputationCircuitBreaker = &ReputationCircuitBreaker{}

func (o *Orchestrator) ReportTaskOutcomeToBlockchain(
	ctx context.Context,
	blockchain *substrate.BlockchainService,
	task *Task,
	agentAccount substrate.AccountID,
	success bool,
) error {
	if blockchain == nil || !blockchain.IsEnabled() {
		o.logger.Debug("blockchain service not available, skipping reputation update")
		return nil
	}

	// Check circuit breaker
	if reputationCircuitBreaker.isOpen() {
		o.logger.Warn("reputation circuit breaker open, skipping reputation update",
			zap.String("task_id", task.ID),
			zap.String("agent", string(agentAccount[:])),
		)
		return fmt.Errorf("reputation circuit breaker open")
	}

	repClient := blockchain.Reputation()
	if repClient == nil {
		o.logger.Warn("reputation client not available")
		return nil
	}

	taskIDBytes := []byte(task.ID)

	// Execute with timeout and retry
	start := time.Now()
	err := blockchain.ExecuteWithRetry(ctx, "report_outcome", func() error {
		return repClient.ReportOutcome(ctx, agentAccount, taskIDBytes, success)
	})
	duration := time.Since(start)

	if err != nil {
		reputationCircuitBreaker.recordFailure()
		o.logger.Error("failed to report task outcome to blockchain",
			zap.String("task_id", task.ID),
			zap.String("agent", string(agentAccount[:])),
			zap.Bool("success", success),
			zap.Duration("duration", duration),
			zap.Int("circuit_failures", reputationCircuitBreaker.failures),
			zap.Error(err),
		)
		return err
	}

	// Record success for circuit breaker
	reputationCircuitBreaker.recordSuccess()

	o.logger.Info("reported task outcome to blockchain reputation system",
		zap.String("task_id", task.ID),
		zap.String("agent", string(agentAccount[:])),
		zap.Bool("success", success),
		zap.Duration("duration", duration),
	)

	return nil
}

func (o *Orchestrator) GetAgentReputation(
	ctx context.Context,
	blockchain *substrate.BlockchainService,
	agentAccount substrate.AccountID,
) (uint32, error) {
	if blockchain == nil || !blockchain.IsEnabled() {
		return 500, nil
	}

	// Check circuit breaker
	if reputationCircuitBreaker.isOpen() {
		o.logger.Debug("reputation circuit breaker open, using default reputation",
			zap.String("agent", string(agentAccount[:])),
		)
		return 500, nil // Return default reputation when circuit is open
	}

	repClient := blockchain.Reputation()
	if repClient == nil {
		return 500, nil
	}

	var reputation uint32
	start := time.Now()

	// Add timeout context for reputation queries
	repCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := blockchain.ExecuteWithRetry(repCtx, "get_reputation", func() error {
		var err error
		reputation, err = repClient.GetReputationScore(repCtx, agentAccount)
		return err
	})

	duration := time.Since(start)

	if err != nil {
		reputationCircuitBreaker.recordFailure()
		o.logger.Debug("failed to get agent reputation from blockchain",
			zap.String("agent", string(agentAccount[:])),
			zap.Duration("duration", duration),
			zap.Int("circuit_failures", reputationCircuitBreaker.failures),
			zap.Error(err),
		)
		return 500, err
	}

	// Record success for circuit breaker
	reputationCircuitBreaker.recordSuccess()

	o.logger.Debug("successfully retrieved agent reputation",
		zap.String("agent", string(agentAccount[:])),
		zap.Uint32("reputation", reputation),
		zap.Duration("duration", duration),
	)

	return reputation, nil
}

func (o *Orchestrator) GetAgentReputationStake(
	ctx context.Context,
	blockchain *substrate.BlockchainService,
	agentAccount substrate.AccountID,
) (*substrate.ReputationStake, error) {
	if blockchain == nil || !blockchain.IsEnabled() {
		return nil, nil
	}

	// Check circuit breaker
	if reputationCircuitBreaker.isOpen() {
		o.logger.Debug("reputation circuit breaker open, skipping stake query",
			zap.String("agent", string(agentAccount[:])),
		)
		return nil, fmt.Errorf("reputation circuit breaker open")
	}

	repClient := blockchain.Reputation()
	if repClient == nil {
		return nil, nil
	}

	var stake *substrate.ReputationStake
	start := time.Now()

	// Add timeout context for reputation stake queries
	repCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	err := blockchain.ExecuteWithRetry(repCtx, "get_reputation_stake", func() error {
		var err error
		stake, err = repClient.GetReputationStake(repCtx, agentAccount)
		return err
	})

	duration := time.Since(start)

	if err != nil {
		reputationCircuitBreaker.recordFailure()
		o.logger.Debug("failed to get agent reputation stake from blockchain",
			zap.String("agent", string(agentAccount[:])),
			zap.Duration("duration", duration),
			zap.Int("circuit_failures", reputationCircuitBreaker.failures),
			zap.Error(err),
		)
		return nil, err
	}

	// Record success for circuit breaker
	reputationCircuitBreaker.recordSuccess()

	o.logger.Debug("successfully retrieved agent reputation stake",
		zap.String("agent", string(agentAccount[:])),
		zap.Duration("duration", duration),
	)

	return stake, nil
}

// GetReputationCircuitBreakerStats returns current circuit breaker statistics
func (o *Orchestrator) GetReputationCircuitBreakerStats() map[string]interface{} {
	reputationCircuitBreaker.mu.RLock()
	defer reputationCircuitBreaker.mu.RUnlock()

	return map[string]interface{}{
		"failures":     reputationCircuitBreaker.failures,
		"last_failure": reputationCircuitBreaker.lastFailure,
		"is_open":      reputationCircuitBreaker.isOpen(),
		"max_failures": ReputationMaxFailures,
		"timeout":      ReputationTimeout.String(),
	}
}
