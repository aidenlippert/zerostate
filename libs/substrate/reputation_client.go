package substrate

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"go.uber.org/zap"
)

// ReputationClient handles interactions with the reputation pallet
// Provides reputation staking, task outcome reporting, and slashing functionality
type ReputationClient struct {
	client         *ClientV2
	keyring        *signature.KeyringPair
	palletID       uint8
	logger         *zap.Logger
	retryConfig    RetryConfig
	circuitBreaker *CircuitBreaker
}

// ReputationStake represents an agent's reputation stake information
type ReputationStake struct {
	Staked          Balance     `json:"staked"`           // Amount of AINU tokens staked
	Reputation      uint32      `json:"reputation"`       // Reputation score (0-1000)
	TasksCompleted  uint32      `json:"tasks_completed"`  // Number of successfully completed tasks
	TasksFailed     uint32      `json:"tasks_failed"`     // Number of failed tasks
	Slashed         Balance     `json:"slashed"`          // Total amount slashed due to failures
	ActiveSince     BlockNumber `json:"active_since"`     // Block number when stake became active
}

// OffenseType represents different types of offenses for slashing
type OffenseType uint8

const (
	// OffenseTypeFraudulentResult indicates fraudulent task results (50% slash)
	OffenseTypeFraudulentResult OffenseType = iota
	// OffenseTypeDoubleTaskAcceptance indicates accepting multiple tasks when capacity is full (30% slash)
	OffenseTypeDoubleTaskAcceptance
	// OffenseTypeRepeatedFailures indicates repeated failures in short time (25% slash)
	OffenseTypeRepeatedFailures
	// OffenseTypeProtocolViolation indicates protocol violations (20% slash)
	OffenseTypeProtocolViolation
)

// String returns the string representation of OffenseType
func (ot OffenseType) String() string {
	switch ot {
	case OffenseTypeFraudulentResult:
		return "FraudulentResult"
	case OffenseTypeDoubleTaskAcceptance:
		return "DoubleTaskAcceptance"
	case OffenseTypeRepeatedFailures:
		return "RepeatedFailures"
	case OffenseTypeProtocolViolation:
		return "ProtocolViolation"
	default:
		return "Unknown"
	}
}

// NewReputationClient creates a new reputation client with production-ready configuration
func NewReputationClient(client *ClientV2, keyring *signature.KeyringPair) *ReputationClient {
	logger := zap.NewNop() // Default no-op logger, should be set via SetLogger

	return &ReputationClient{
		client:   client,
		keyring:  keyring,
		palletID: 11, // Reputation pallet index
		logger:   logger,
		retryConfig: RetryConfig{
			MaxRetries:     3,
			InitialBackoff: 100 * time.Millisecond,
			MaxBackoff:     10 * time.Second,
			Multiplier:     2.0,
			Jitter:         true,
		},
		circuitBreaker: NewCircuitBreaker(
			3,              // Open after 3 failures
			2,              // Close after 2 successes
			30*time.Second, // Wait 30s before half-open
		),
	}
}

// SetLogger configures the logger for this client
func (rc *ReputationClient) SetLogger(logger *zap.Logger) {
	if logger != nil {
		rc.logger = logger
	}
}

// BondReputation bonds AINU tokens for reputation
// This is how agents stake tokens to participate in the reputation system
func (rc *ReputationClient) BondReputation(ctx context.Context, amount uint64) error {
	start := time.Now()
	rc.logger.Info("bonding reputation",
		zap.Uint64("amount", amount),
		zap.String("account", rc.keyring.Address),
	)

	err := rc.executeWithRetry(ctx, "bond_reputation", func() error {
		if rc.client == nil {
			return fmt.Errorf("client not initialized")
		}
		meta := rc.client.GetMetadata()
		if meta == nil {
			return fmt.Errorf("metadata not available")
		}
		amountBig := types.NewU128(*new(big.Int).SetUint64(amount))

		call, err := types.NewCall(meta, "Reputation.bond_reputation", amountBig)
		if err != nil {
			return fmt.Errorf("failed to create bond_reputation call: %w", err)
		}

		hash, err := rc.submitTransaction(ctx, call)
		if err != nil {
			return fmt.Errorf("failed to submit bond_reputation transaction: %w", err)
		}

		rc.logger.Info("reputation bonded successfully",
			zap.String("block_hash", hash.Hex()),
			zap.Uint64("amount", amount),
			zap.Duration("duration", time.Since(start)),
		)
		return nil
	})

	if err != nil {
		rc.logger.Error("failed to bond reputation",
			zap.Error(err),
			zap.Uint64("amount", amount),
			zap.Duration("duration", time.Since(start)),
		)
	}

	return err
}

// UnbondReputation unbonds staked tokens
// Allows agents to withdraw previously staked tokens
func (rc *ReputationClient) UnbondReputation(ctx context.Context, amount uint64) error {
	start := time.Now()
	rc.logger.Info("unbonding reputation",
		zap.Uint64("amount", amount),
		zap.String("account", rc.keyring.Address),
	)

	err := rc.executeWithRetry(ctx, "unbond_reputation", func() error {
		if rc.client == nil {
			return fmt.Errorf("client not initialized")
		}
		meta := rc.client.GetMetadata()
		if meta == nil {
			return fmt.Errorf("metadata not available")
		}
		amountBig := types.NewU128(*new(big.Int).SetUint64(amount))

		call, err := types.NewCall(meta, "Reputation.unbond_reputation", amountBig)
		if err != nil {
			return fmt.Errorf("failed to create unbond_reputation call: %w", err)
		}

		hash, err := rc.submitTransaction(ctx, call)
		if err != nil {
			return fmt.Errorf("failed to submit unbond_reputation transaction: %w", err)
		}

		rc.logger.Info("reputation unbonded successfully",
			zap.String("block_hash", hash.Hex()),
			zap.Uint64("amount", amount),
			zap.Duration("duration", time.Since(start)),
		)
		return nil
	})

	if err != nil {
		rc.logger.Error("failed to unbond reputation",
			zap.Error(err),
			zap.Uint64("amount", amount),
			zap.Duration("duration", time.Since(start)),
		)
	}

	return err
}

// ReportOutcome reports task completion outcome (orchestrator only)
// This function is called by orchestrators after task completion to update agent reputation
func (rc *ReputationClient) ReportOutcome(ctx context.Context, agentAccount AccountID, taskID []byte, success bool) error {
	start := time.Now()
	rc.logger.Info("reporting task outcome",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
		zap.String("task_id", fmt.Sprintf("%x", taskID)),
		zap.Bool("success", success),
	)

	err := rc.executeWithRetry(ctx, "report_outcome", func() error {
		if rc.client == nil {
			return fmt.Errorf("client not initialized")
		}
		meta := rc.client.GetMetadata()
		if meta == nil {
			return fmt.Errorf("metadata not available")
		}

		call, err := types.NewCall(meta, "Reputation.report_outcome",
			agentAccount[:],
			types.NewBytes(taskID),
			types.NewBool(success))
		if err != nil {
			return fmt.Errorf("failed to create report_outcome call: %w", err)
		}

		hash, err := rc.submitTransaction(ctx, call)
		if err != nil {
			return fmt.Errorf("failed to submit report_outcome transaction: %w", err)
		}

		rc.logger.Info("task outcome reported successfully",
			zap.String("block_hash", hash.Hex()),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.Bool("success", success),
			zap.Duration("duration", time.Since(start)),
		)
		return nil
	})

	if err != nil {
		rc.logger.Error("failed to report task outcome",
			zap.Error(err),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.Bool("success", success),
			zap.Duration("duration", time.Since(start)),
		)
	}

	return err
}

// SlashSevere applies severe slashing for major offenses (governance only)
// This function can only be called by governance to slash agents for severe misbehavior
func (rc *ReputationClient) SlashSevere(ctx context.Context, agentAccount AccountID, offense OffenseType) error {
	start := time.Now()
	rc.logger.Info("applying severe slash",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
		zap.String("offense", offense.String()),
		zap.Uint8("offense_code", uint8(offense)),
	)

	err := rc.executeWithRetry(ctx, "slash_severe", func() error {
		if rc.client == nil {
			return fmt.Errorf("client not initialized")
		}
		meta := rc.client.GetMetadata()
		if meta == nil {
			return fmt.Errorf("metadata not available")
		}

		call, err := types.NewCall(meta, "Reputation.slash_severe",
			agentAccount[:],
			types.NewU8(uint8(offense)))
		if err != nil {
			return fmt.Errorf("failed to create slash_severe call: %w", err)
		}

		hash, err := rc.submitTransaction(ctx, call)
		if err != nil {
			return fmt.Errorf("failed to submit slash_severe transaction: %w", err)
		}

		rc.logger.Info("severe slash applied successfully",
			zap.String("block_hash", hash.Hex()),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.String("offense", offense.String()),
			zap.Duration("duration", time.Since(start)),
		)
		return nil
	})

	if err != nil {
		rc.logger.Error("failed to apply severe slash",
			zap.Error(err),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.String("offense", offense.String()),
			zap.Duration("duration", time.Since(start)),
		)
	}

	return err
}

// GetReputationScore retrieves just the reputation score for an agent
// Returns the current reputation score (0-1000) for the specified agent
func (rc *ReputationClient) GetReputationScore(ctx context.Context, agentAccount AccountID) (uint32, error) {
	start := time.Now()
	rc.logger.Debug("getting reputation score",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
	)

	var score uint32
	err := rc.executeWithRetry(ctx, "get_reputation_score", func() error {
		stake, err := rc.getReputationStakeInternal(ctx, agentAccount)
		if err != nil {
			return err
		}
		score = stake.Reputation
		return nil
	})

	if err != nil {
		rc.logger.Error("failed to get reputation score",
			zap.Error(err),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.Duration("duration", time.Since(start)),
		)
		return 0, err
	}

	rc.logger.Debug("reputation score retrieved",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
		zap.Uint32("score", score),
		zap.Duration("duration", time.Since(start)),
	)

	return score, nil
}

// GetReputationStake retrieves full reputation stake information for an agent
// Returns complete stake details including staked amount, reputation, task counts, etc.
func (rc *ReputationClient) GetReputationStake(ctx context.Context, agentAccount AccountID) (*ReputationStake, error) {
	start := time.Now()
	rc.logger.Debug("getting reputation stake",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
	)

	var stake *ReputationStake
	err := rc.executeWithRetry(ctx, "get_reputation_stake", func() error {
		var err error
		stake, err = rc.getReputationStakeInternal(ctx, agentAccount)
		return err
	})

	if err != nil {
		rc.logger.Error("failed to get reputation stake",
			zap.Error(err),
			zap.String("agent", fmt.Sprintf("%x", agentAccount)),
			zap.Duration("duration", time.Since(start)),
		)
		return nil, err
	}

	rc.logger.Debug("reputation stake retrieved",
		zap.String("agent", fmt.Sprintf("%x", agentAccount)),
		zap.Uint32("reputation", stake.Reputation),
		zap.Uint32("tasks_completed", stake.TasksCompleted),
		zap.Uint32("tasks_failed", stake.TasksFailed),
		zap.Duration("duration", time.Since(start)),
	)

	return stake, nil
}

// getReputationStakeInternal is the internal implementation for querying reputation stakes
func (rc *ReputationClient) getReputationStakeInternal(ctx context.Context, agentAccount AccountID) (*ReputationStake, error) {
	if rc.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	if rc.client.metadata == nil {
		return nil, fmt.Errorf("metadata not available")
	}

	// Create storage key for ReputationStakes map
	key, err := types.CreateStorageKey(rc.client.metadata, "Reputation", "ReputationStakes", agentAccount[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	// Define the storage result structure matching the pallet's ReputationStake struct
	var result struct {
		Staked          types.U128
		Reputation      types.U32
		TasksCompleted  types.U32
		TasksFailed     types.U32
		Slashed         types.U128
		ActiveSince     types.U32
	}

	// Query the storage
	ok, err := rc.client.api.RPC.State.GetStorageLatest(key, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to query reputation storage: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("reputation stake not found for agent %x", agentAccount)
	}

	// Convert to our ReputationStake struct
	stake := &ReputationStake{
		Staked:          Balance(result.Staked.Int.String()),
		Reputation:      uint32(result.Reputation),
		TasksCompleted:  uint32(result.TasksCompleted),
		TasksFailed:     uint32(result.TasksFailed),
		Slashed:         Balance(result.Slashed.Int.String()),
		ActiveSince:     BlockNumber(result.ActiveSince),
	}

	return stake, nil
}

// executeWithRetry executes a function with retry logic and circuit breaker protection
func (rc *ReputationClient) executeWithRetry(ctx context.Context, operation string, fn func() error) error {
	// Create a context with timeout if none exists
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	return rc.circuitBreaker.Call(func() error {
		return RetryWithBackoff(ctx, rc.retryConfig, func() error {
			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			default:
				return fn()
			}
		})
	})
}

// submitTransaction handles the common transaction submission logic with proper error handling
func (rc *ReputationClient) submitTransaction(ctx context.Context, call types.Call) (types.Hash, error) {
	// Get runtime version
	rv, err := rc.client.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get runtime version: %w", err)
	}

	// Get account info for nonce
	key, err := types.CreateStorageKey(rc.client.metadata, "System", "Account", rc.keyring.PublicKey)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to create storage key for account: %w", err)
	}

	var accountInfo types.AccountInfo
	ok, err := rc.client.api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get account info: %w", err)
	}

	nonce := types.NewUCompactFromUInt(0)
	if ok {
		nonce = types.NewUCompactFromUInt(uint64(accountInfo.Nonce))
	}

	// Create and sign extrinsic
	ext := types.NewExtrinsic(call)
	genesisHash := rc.client.GetGenesisHash()

	blockHash, err := rc.client.api.RPC.Chain.GetBlockHashLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get latest block hash: %w", err)
	}

	o := types.SignatureOptions{
		BlockHash:          blockHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              nonce,
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	err = ext.Sign(*rc.keyring, o)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign extrinsic: %w", err)
	}

	// Submit transaction
	hash, err := rc.client.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to submit extrinsic: %w", err)
	}

	return hash, nil
}