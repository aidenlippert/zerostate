package economic

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// SECURITY INVARIANTS (CRITICAL - DO NOT VIOLATE):
// 1. Total deposits must ALWAYS equal total withdrawals + channel balances
// 2. Channel balance updates must be atomic (all-or-nothing)
// 3. No double-spending: escrow release must be idempotent
// 4. Balance checks must prevent integer underflow
// 5. All state transitions must be logged for audit

var (
	// ErrInsufficientBalance indicates account has insufficient funds
	ErrInsufficientBalance = errors.New("insufficient balance for operation")

	// ErrChannelNotFound indicates payment channel does not exist
	ErrChannelNotFound = errors.New("payment channel not found")

	// ErrChannelClosed indicates channel is already closed
	ErrChannelClosed = errors.New("payment channel is closed")

	// ErrInvalidAmount indicates negative or zero amount
	ErrInvalidAmount = errors.New("amount must be positive")

	// ErrEscrowAlreadyReleased indicates escrow was already released (prevents double-spending)
	ErrEscrowAlreadyReleased = errors.New("escrow already released")

	// ErrChannelAlreadyExists indicates duplicate channel creation attempt
	ErrChannelAlreadyExists = errors.New("payment channel already exists")

	// ErrInvalidParticipant indicates participant is not authorized for operation
	ErrInvalidParticipant = errors.New("participant not authorized for this channel")
)

// ChannelState represents the state of a payment channel
type ChannelState string

const (
	ChannelStateOpen     ChannelState = "open"     // Active channel
	ChannelStateEscrowed ChannelState = "escrowed" // Funds locked for task
	ChannelStateSettling ChannelState = "settling" // Final settlement in progress
	ChannelStateClosed   ChannelState = "closed"   // Channel permanently closed
)

// PaymentChannel represents an off-chain payment channel between two parties
type PaymentChannel struct {
	ID string `json:"id"` // Unique channel identifier

	// Participants
	PayerDID  string `json:"payer_did"`  // User paying for tasks
	PayeeDID  string `json:"payee_did"`  // Agent receiving payment
	AuctionID string `json:"auction_id"` // Associated auction (if any)

	// Financial state (SECURITY CRITICAL)
	TotalDeposit   float64 `json:"total_deposit"`   // Total deposited by payer
	CurrentBalance float64 `json:"current_balance"` // Available balance
	EscrowedAmount float64 `json:"escrowed_amount"` // Locked for active tasks
	TotalSettled   float64 `json:"total_settled"`   // Total paid to payee
	PendingRefund  float64 `json:"pending_refund"`  // Amount to refund on close

	// Channel state
	State     ChannelState `json:"state"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
	ClosedAt  *time.Time   `json:"closed_at,omitempty"`

	// Task association
	TaskID         string `json:"task_id,omitempty"` // Current task (if escrowed)
	EscrowReleased bool   `json:"escrow_released"`   // Prevents double-release

	// Sequence number for replay attack prevention
	SequenceNumber uint64 `json:"sequence_number"`

	// Audit trail
	TransactionLog []ChannelTransaction `json:"transaction_log"`
}

// ChannelTransaction represents a single transaction in the channel
type ChannelTransaction struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // deposit, escrow, release, refund, settle
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
	TaskID    string    `json:"task_id,omitempty"`
	Reason    string    `json:"reason,omitempty"`
}

// Account represents a user or agent account with balance
type Account struct {
	DID     string  `json:"did"`
	Balance float64 `json:"balance"` // Available balance

	// Total deposits/withdrawals for reconciliation
	TotalDeposited float64 `json:"total_deposited"`
	TotalWithdrawn float64 `json:"total_withdrawn"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// PaymentChannelService manages payment channels with strong consistency guarantees
type PaymentChannelService struct {
	mu sync.RWMutex // Protects all state

	// State storage
	channels map[string]*PaymentChannel // channel_id -> channel
	accounts map[string]*Account        // did -> account

	// Metrics
	metricsChannelsActive        prometheus.Gauge
	metricsChannelsClosed        prometheus.Counter
	metricsDepositsTotal         prometheus.Counter
	metricsDepositAmountTotal    prometheus.Counter
	metricsWithdrawalsTotal      prometheus.Counter
	metricsWithdrawalAmountTotal prometheus.Counter
	metricsEscrowsActive         prometheus.Gauge
	metricsEscrowAmountLocked    prometheus.Gauge
	metricsSettlementsTotal      prometheus.Counter
	metricsSettlementAmount      prometheus.Histogram
	metricsBalanceCheckFailures  prometheus.Counter

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPaymentChannelService creates a new payment channel service
func NewPaymentChannelService() *PaymentChannelService {
	ctx, cancel := context.WithCancel(context.Background())

	return &PaymentChannelService{
		channels: make(map[string]*PaymentChannel),
		accounts: make(map[string]*Account),
		ctx:      ctx,
		cancel:   cancel,

		// Metrics
		metricsChannelsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "zerostate_payment_channels_active",
			Help: "Number of active payment channels",
		}),
		metricsChannelsClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_payment_channels_closed_total",
			Help: "Total number of closed payment channels",
		}),
		metricsDepositsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_deposits_total",
			Help: "Total number of deposits",
		}),
		metricsDepositAmountTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_deposit_amount_total",
			Help: "Total amount deposited (cumulative)",
		}),
		metricsWithdrawalsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_withdrawals_total",
			Help: "Total number of withdrawals",
		}),
		metricsWithdrawalAmountTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_withdrawal_amount_total",
			Help: "Total amount withdrawn (cumulative)",
		}),
		metricsEscrowsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "zerostate_escrows_active",
			Help: "Number of active escrows",
		}),
		metricsEscrowAmountLocked: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "zerostate_escrow_amount_locked",
			Help: "Total amount locked in escrows",
		}),
		metricsSettlementsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_settlements_total",
			Help: "Total number of settlements",
		}),
		metricsSettlementAmount: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_settlement_amount",
			Help:    "Settlement amount distribution",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10),
		}),
		metricsBalanceCheckFailures: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_balance_check_failures_total",
			Help: "Total number of balance check failures (indicates bugs)",
		}),
	}
}

// Deposit adds funds to an account
// SECURITY: Must be atomic and prevent negative balances
func (pcs *PaymentChannelService) Deposit(ctx context.Context, did string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	account, exists := pcs.accounts[did]
	if !exists {
		account = &Account{
			DID:            did,
			Balance:        0,
			TotalDeposited: 0,
			TotalWithdrawn: 0,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		pcs.accounts[did] = account
	}

	// Atomic update
	account.Balance += amount
	account.TotalDeposited += amount
	account.UpdatedAt = time.Now()

	// Metrics
	pcs.metricsDepositsTotal.Inc()
	pcs.metricsDepositAmountTotal.Add(amount)

	return nil
}

// Withdraw removes funds from an account
// SECURITY: Must check sufficient balance and be atomic
func (pcs *PaymentChannelService) Withdraw(ctx context.Context, did string, amount float64) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	account, exists := pcs.accounts[did]
	if !exists {
		return ErrInsufficientBalance
	}

	// CRITICAL: Check for sufficient balance (prevents underflow)
	if account.Balance < amount {
		return ErrInsufficientBalance
	}

	// Atomic update
	account.Balance -= amount
	account.TotalWithdrawn += amount
	account.UpdatedAt = time.Now()

	// Metrics
	pcs.metricsWithdrawalsTotal.Inc()
	pcs.metricsWithdrawalAmountTotal.Add(amount)

	return nil
}

// GetBalance retrieves account balance
func (pcs *PaymentChannelService) GetBalance(ctx context.Context, did string) (float64, error) {
	pcs.mu.RLock()
	defer pcs.mu.RUnlock()

	account, exists := pcs.accounts[did]
	if !exists {
		return 0, nil
	}

	return account.Balance, nil
}

// CreateChannel creates a new payment channel
// SECURITY: Deducts deposit from payer account atomically
func (pcs *PaymentChannelService) CreateChannel(
	ctx context.Context,
	payerDID string,
	payeeDID string,
	depositAmount float64,
	auctionID string,
) (*PaymentChannel, error) {
	if depositAmount <= 0 {
		return nil, ErrInvalidAmount
	}

	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	// Check payer has sufficient balance
	payerAccount, exists := pcs.accounts[payerDID]
	if !exists || payerAccount.Balance < depositAmount {
		return nil, ErrInsufficientBalance
	}

	// Generate unique channel ID
	channelID := generateChannelID()

	// Check for duplicate
	if _, exists := pcs.channels[channelID]; exists {
		return nil, ErrChannelAlreadyExists
	}

	// Create channel
	now := time.Now()
	channel := &PaymentChannel{
		ID:             channelID,
		PayerDID:       payerDID,
		PayeeDID:       payeeDID,
		AuctionID:      auctionID,
		TotalDeposit:   depositAmount,
		CurrentBalance: depositAmount,
		EscrowedAmount: 0,
		TotalSettled:   0,
		PendingRefund:  0,
		State:          ChannelStateOpen,
		CreatedAt:      now,
		UpdatedAt:      now,
		SequenceNumber: 0,
		EscrowReleased: false,
		TransactionLog: []ChannelTransaction{
			{
				ID:        generateTxID(),
				Type:      "deposit",
				Amount:    depositAmount,
				Timestamp: now,
			},
		},
	}

	// ATOMIC: Deduct from payer and create channel
	payerAccount.Balance -= depositAmount
	payerAccount.UpdatedAt = now
	pcs.channels[channelID] = channel

	// Metrics
	pcs.metricsChannelsActive.Inc()

	return channel, nil
}

// LockEscrow locks funds for a task
// SECURITY: Must be atomic and idempotent
func (pcs *PaymentChannelService) LockEscrow(
	ctx context.Context,
	channelID string,
	taskID string,
	amount float64,
) error {
	if amount <= 0 {
		return ErrInvalidAmount
	}

	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	channel, exists := pcs.channels[channelID]
	if !exists {
		return ErrChannelNotFound
	}

	if channel.State == ChannelStateClosed {
		return ErrChannelClosed
	}

	// Check sufficient balance
	if channel.CurrentBalance < amount {
		return ErrInsufficientBalance
	}

	// ATOMIC: Move from current balance to escrow
	channel.CurrentBalance -= amount
	channel.EscrowedAmount += amount
	channel.TaskID = taskID
	channel.State = ChannelStateEscrowed
	channel.EscrowReleased = false // Reset for new escrow
	channel.SequenceNumber++
	channel.UpdatedAt = time.Now()

	// Log transaction
	channel.TransactionLog = append(channel.TransactionLog, ChannelTransaction{
		ID:        generateTxID(),
		Type:      "escrow",
		Amount:    amount,
		Timestamp: time.Now(),
		TaskID:    taskID,
	})

	// Metrics
	pcs.metricsEscrowsActive.Inc()
	pcs.metricsEscrowAmountLocked.Add(amount)

	return nil
}

// ReleaseEscrow releases escrowed funds to payee (task success) or refunds to payer (task failure)
// SECURITY: Must be idempotent (prevent double-spending)
func (pcs *PaymentChannelService) ReleaseEscrow(
	ctx context.Context,
	channelID string,
	taskID string,
	success bool,
) error {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	channel, exists := pcs.channels[channelID]
	if !exists {
		return ErrChannelNotFound
	}

	// CRITICAL: Idempotency check (prevents double-spending)
	if channel.EscrowReleased {
		return ErrEscrowAlreadyReleased
	}

	// Verify task ID matches
	if channel.TaskID != taskID {
		return fmt.Errorf("task ID mismatch: expected %s, got %s", channel.TaskID, taskID)
	}

	escrowAmount := channel.EscrowedAmount

	if success {
		// Task succeeded: pay agent
		channel.TotalSettled += escrowAmount
		channel.EscrowedAmount = 0

		// Credit payee account
		payeeAccount, exists := pcs.accounts[channel.PayeeDID]
		if !exists {
			payeeAccount = &Account{
				DID:       channel.PayeeDID,
				Balance:   0,
				CreatedAt: time.Now(),
			}
			pcs.accounts[channel.PayeeDID] = payeeAccount
		}
		payeeAccount.Balance += escrowAmount
		payeeAccount.UpdatedAt = time.Now()

		// Log
		channel.TransactionLog = append(channel.TransactionLog, ChannelTransaction{
			ID:        generateTxID(),
			Type:      "release",
			Amount:    escrowAmount,
			Timestamp: time.Now(),
			TaskID:    taskID,
			Reason:    "task_success",
		})

		// Metrics
		pcs.metricsSettlementsTotal.Inc()
		pcs.metricsSettlementAmount.Observe(escrowAmount)
	} else {
		// Task failed: refund payer
		channel.CurrentBalance += escrowAmount
		channel.EscrowedAmount = 0

		// Log
		channel.TransactionLog = append(channel.TransactionLog, ChannelTransaction{
			ID:        generateTxID(),
			Type:      "refund",
			Amount:    escrowAmount,
			Timestamp: time.Now(),
			TaskID:    taskID,
			Reason:    "task_failure",
		})
	}

	// Mark as released (idempotency)
	channel.EscrowReleased = true
	channel.State = ChannelStateOpen
	channel.TaskID = ""
	channel.SequenceNumber++
	channel.UpdatedAt = time.Now()

	// Metrics
	pcs.metricsEscrowsActive.Dec()
	pcs.metricsEscrowAmountLocked.Sub(escrowAmount)

	return nil
}

// CloseChannel closes a payment channel and returns remaining balance to payer
// SECURITY: Must settle all pending escrows first
func (pcs *PaymentChannelService) CloseChannel(ctx context.Context, channelID string) error {
	pcs.mu.Lock()
	defer pcs.mu.Unlock()

	channel, exists := pcs.channels[channelID]
	if !exists {
		return ErrChannelNotFound
	}

	if channel.State == ChannelStateClosed {
		return ErrChannelClosed
	}

	// SECURITY: Cannot close with active escrow
	if channel.EscrowedAmount > 0 {
		return fmt.Errorf("cannot close channel with active escrow: %f locked", channel.EscrowedAmount)
	}

	// Return remaining balance to payer
	if channel.CurrentBalance > 0 {
		payerAccount, exists := pcs.accounts[channel.PayerDID]
		if !exists {
			// Should never happen, but be defensive
			payerAccount = &Account{
				DID:       channel.PayerDID,
				Balance:   0,
				CreatedAt: time.Now(),
			}
			pcs.accounts[channel.PayerDID] = payerAccount
		}

		payerAccount.Balance += channel.CurrentBalance
		payerAccount.UpdatedAt = time.Now()
		channel.PendingRefund = channel.CurrentBalance
		channel.CurrentBalance = 0
	}

	// Close channel
	now := time.Now()
	channel.State = ChannelStateClosed
	channel.ClosedAt = &now
	channel.UpdatedAt = now

	// Log
	channel.TransactionLog = append(channel.TransactionLog, ChannelTransaction{
		ID:        generateTxID(),
		Type:      "close",
		Amount:    channel.PendingRefund,
		Timestamp: now,
		Reason:    "channel_closed",
	})

	// Metrics
	pcs.metricsChannelsActive.Dec()
	pcs.metricsChannelsClosed.Inc()

	return nil
}

// GetChannel retrieves a payment channel
func (pcs *PaymentChannelService) GetChannel(ctx context.Context, channelID string) (*PaymentChannel, error) {
	pcs.mu.RLock()
	defer pcs.mu.RUnlock()

	channel, exists := pcs.channels[channelID]
	if !exists {
		return nil, ErrChannelNotFound
	}

	// Return a copy to prevent external mutation
	channelCopy := *channel
	return &channelCopy, nil
}

// VerifyBalanceInvariant checks that all money is accounted for
// SECURITY CRITICAL: This should always pass. If it fails, there's a bug.
func (pcs *PaymentChannelService) VerifyBalanceInvariant() error {
	pcs.mu.RLock()
	defer pcs.mu.RUnlock()

	var totalAccountBalances float64
	var totalDeposited float64
	var totalWithdrawn float64

	for _, account := range pcs.accounts {
		totalAccountBalances += account.Balance
		totalDeposited += account.TotalDeposited
		totalWithdrawn += account.TotalWithdrawn
	}

	var totalChannelBalances float64
	var totalEscrowed float64
	var totalSettled float64

	for _, channel := range pcs.channels {
		totalChannelBalances += channel.CurrentBalance
		totalEscrowed += channel.EscrowedAmount
		totalSettled += channel.TotalSettled
	}

	// INVARIANT: deposits = withdrawals + account balances + channel balances + escrowed + settled
	expected := totalDeposited
	actual := totalWithdrawn + totalAccountBalances + totalChannelBalances + totalEscrowed + totalSettled

	// Allow small floating point error
	epsilon := 0.001
	if abs(expected-actual) > epsilon {
		pcs.metricsBalanceCheckFailures.Inc()
		return fmt.Errorf("balance invariant violated: expected %f, got %f (diff: %f)",
			expected, actual, expected-actual)
	}

	return nil
}

// GetChannel retrieves a payment channel by ID
func (pcs *PaymentChannelService) GetChannel(ctx context.Context, channelID string) (*PaymentChannel, error) {
	pcs.mu.RLock()
	defer pcs.mu.RUnlock()

	channel, exists := pcs.channels[channelID]
	if !exists {
		return nil, ErrChannelNotFound
	}

	return channel, nil
}

// GetTransactionHistory retrieves transaction history for a user
func (pcs *PaymentChannelService) GetTransactionHistory(ctx context.Context, did string) ([]ChannelTransaction, error) {
	pcs.mu.RLock()
	defer pcs.mu.RUnlock()

	history := []ChannelTransaction{}

	// Get transactions from channels where user is participant
	for _, channel := range pcs.channels {
		if channel.PayerDID == did || channel.PayeeDID == did {
			history = append(history, channel.TransactionLog...)
		}
	}

	return history, nil
}

// Close shuts down the payment channel service
func (pcs *PaymentChannelService) Close() error {
	pcs.cancel()
	pcs.wg.Wait()
	return nil
}

// Helper functions

func generateChannelID() string {
	return "channel-" + generateRandomID()
}

func generateTxID() string {
	return "tx-" + generateRandomID()
}

func generateRandomID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
