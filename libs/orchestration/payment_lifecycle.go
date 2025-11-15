package orchestration

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// PaymentStatus is defined in task.go to avoid circular imports
// The constants are re-exported here for convenience

// Payment lifecycle errors
var (
	ErrPaymentNotFound           = errors.New("payment not found")
	ErrInvalidPaymentStatus      = errors.New("invalid payment status")
	ErrPaymentAlreadyProcessed   = errors.New("payment already processed")
	ErrBlockchainUnavailable     = errors.New("blockchain service unavailable")
	ErrInsufficientFunds         = errors.New("insufficient funds for payment")
	ErrPaymentTimeout           = errors.New("payment operation timeout")
	ErrCircuitBreakerOpen       = errors.New("payment circuit breaker is open")
)

// PaymentEvent represents a payment lifecycle event
type PaymentEvent struct {
	TaskID      string        `json:"task_id"`
	EventType   string        `json:"event_type"`
	Status      PaymentStatus `json:"status"`
	Amount      float64       `json:"amount"`
	Timestamp   time.Time     `json:"timestamp"`
	Reason      string        `json:"reason,omitempty"`
	TxHash      string        `json:"tx_hash,omitempty"`
	RetryCount  int           `json:"retry_count,omitempty"`
}

// PaymentInfo represents payment information for a task
type PaymentInfo struct {
	TaskID       string        `json:"task_id"`
	UserID       string        `json:"user_id"`
	AgentID      string        `json:"agent_id,omitempty"`
	Amount       float64       `json:"amount"`
	Status       PaymentStatus `json:"status"`
	EscrowTxHash string        `json:"escrow_tx_hash,omitempty"`
	PaymentTxHash string       `json:"payment_tx_hash,omitempty"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Events       []PaymentEvent `json:"events,omitempty"`
	RetryCount   int           `json:"retry_count"`
	LastError    string        `json:"last_error,omitempty"`
}

// CircuitBreaker implements circuit breaker pattern for payment operations
type CircuitBreaker struct {
	mu             sync.RWMutex
	failureCount   int
	lastFailureTime time.Time
	state          string // "closed", "open", "half-open"
	timeout        time.Duration
	threshold      int
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     "closed",
		timeout:   timeout,
		threshold: threshold,
	}
}

// Call executes a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	if !cb.AllowRequest() {
		return ErrCircuitBreakerOpen
	}

	err := fn()
	cb.RecordResult(err == nil)
	return err
}

// AllowRequest checks if request is allowed
func (cb *CircuitBreaker) AllowRequest() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case "open":
		if time.Since(cb.lastFailureTime) > cb.timeout {
			cb.state = "half-open"
			return true
		}
		return false
	case "half-open", "closed":
		return true
	default:
		return false
	}
}

// RecordResult records the result of a request
func (cb *CircuitBreaker) RecordResult(success bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if success {
		cb.failureCount = 0
		if cb.state == "half-open" {
			cb.state = "closed"
		}
	} else {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		if cb.failureCount >= cb.threshold {
			cb.state = "open"
		}
	}
}

// BlockchainInterface defines the interface for blockchain operations
type BlockchainInterface interface {
	// Payment methods
	ReleasePayment(ctx context.Context, taskID string) (txHash string, err error)
	RefundEscrow(ctx context.Context, taskID string) (txHash string, err error)
	DisputeEscrow(ctx context.Context, taskID string, reason string) (txHash string, err error)

	// Status checks
	IsEnabled() bool
	GetEscrowStatus(ctx context.Context, taskID string) (PaymentStatus, error)
}

// PaymentLifecycleManager manages the complete payment lifecycle
type PaymentLifecycleManager struct {
	blockchain   BlockchainInterface
	circuitBreaker *CircuitBreaker
	logger       *zap.Logger
	mu           sync.RWMutex

	// In-memory payment tracking (in production, this should be persistent storage)
	payments     map[string]*PaymentInfo

	// Configuration
	config       PaymentConfig
}

// PaymentConfig contains payment lifecycle configuration
type PaymentConfig struct {
	RetryMaxAttempts    int           `json:"retry_max_attempts"`
	RetryBaseDelay      time.Duration `json:"retry_base_delay"`
	RetryMaxDelay       time.Duration `json:"retry_max_delay"`
	PaymentTimeout      time.Duration `json:"payment_timeout"`
	CircuitBreakerThreshold int       `json:"circuit_breaker_threshold"`
	CircuitBreakerTimeout   time.Duration `json:"circuit_breaker_timeout"`
}

// DefaultPaymentConfig returns default payment configuration
func DefaultPaymentConfig() PaymentConfig {
	return PaymentConfig{
		RetryMaxAttempts:        3,
		RetryBaseDelay:          1 * time.Second,
		RetryMaxDelay:           10 * time.Second,
		PaymentTimeout:          30 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
	}
}

// NewPaymentLifecycleManager creates a new payment lifecycle manager
func NewPaymentLifecycleManager(
	blockchain BlockchainInterface,
	config PaymentConfig,
	logger *zap.Logger,
) *PaymentLifecycleManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	circuitBreaker := NewCircuitBreaker(
		config.CircuitBreakerThreshold,
		config.CircuitBreakerTimeout,
	)

	return &PaymentLifecycleManager{
		blockchain:     blockchain,
		circuitBreaker: circuitBreaker,
		logger:         logger,
		payments:       make(map[string]*PaymentInfo),
		config:         config,
	}
}

// CreatePayment creates a new payment record for a task
func (pm *PaymentLifecycleManager) CreatePayment(taskID, userID string, amount float64) *PaymentInfo {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment := &PaymentInfo{
		TaskID:    taskID,
		UserID:    userID,
		Amount:    amount,
		Status:    PaymentStatusCreated,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Events:    []PaymentEvent{},
	}

	pm.payments[taskID] = payment
	pm.addEvent(payment, "payment_created", PaymentStatusCreated, "", 0)

	pm.logger.Info("payment created",
		zap.String("task_id", taskID),
		zap.Float64("amount", amount),
	)

	return payment
}

// UpdatePaymentStatus updates payment status and adds event
func (pm *PaymentLifecycleManager) UpdatePaymentStatus(taskID string, status PaymentStatus, reason string, txHash string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, exists := pm.payments[taskID]
	if !exists {
		return ErrPaymentNotFound
	}

	// Prevent status regression (except for disputes)
	if !pm.isValidStatusTransition(payment.Status, status) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidPaymentStatus, payment.Status, status)
	}

	payment.Status = status
	payment.UpdatedAt = time.Now()

	if txHash != "" {
		if status == PaymentStatusReleased || status == PaymentStatusRefunded {
			payment.PaymentTxHash = txHash
		}
	}

	if status == PaymentStatusReleased || status == PaymentStatusRefunded || status == PaymentStatusDisputed {
		now := time.Now()
		payment.CompletedAt = &now
	}

	pm.addEvent(payment, getEventType(status), status, reason, payment.RetryCount)

	pm.logger.Info("payment status updated",
		zap.String("task_id", taskID),
		zap.String("status", string(status)),
		zap.String("reason", reason),
		zap.String("tx_hash", txHash),
	)

	return nil
}

// GetPaymentInfo retrieves payment information
func (pm *PaymentLifecycleManager) GetPaymentInfo(taskID string) (*PaymentInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	payment, exists := pm.payments[taskID]
	if !exists {
		return nil, ErrPaymentNotFound
	}

	// Return a copy to prevent race conditions
	paymentCopy := *payment
	paymentCopy.Events = make([]PaymentEvent, len(payment.Events))
	copy(paymentCopy.Events, payment.Events)

	return &paymentCopy, nil
}

// ReleasePaymentAsync releases payment asynchronously with retry logic
func (pm *PaymentLifecycleManager) ReleasePaymentAsync(ctx context.Context, taskID, agentID string) {
	go func() {
		pm.logger.Info("starting async payment release",
			zap.String("task_id", taskID),
			zap.String("agent_id", agentID),
		)

		err := pm.ReleasePayment(ctx, taskID, agentID)
		if err != nil {
			pm.logger.Error("failed to release payment asynchronously",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
		}
	}()
}

// ReleasePayment releases payment to agent on successful task completion
func (pm *PaymentLifecycleManager) ReleasePayment(ctx context.Context, taskID, agentID string) error {
	pm.logger.Info("releasing payment",
		zap.String("task_id", taskID),
		zap.String("agent_id", agentID),
	)

	// Get payment info
	payment, err := pm.GetPaymentInfo(taskID)
	if err != nil {
		return err
	}

	// Check if payment can be released
	if payment.Status != PaymentStatusAccepted {
		return fmt.Errorf("%w: payment status is %s, expected %s",
			ErrInvalidPaymentStatus, payment.Status, PaymentStatusAccepted)
	}

	// Update agent info
	pm.mu.Lock()
	if p, exists := pm.payments[taskID]; exists {
		p.AgentID = agentID
	}
	pm.mu.Unlock()

	// Execute payment release with retry and circuit breaker
	err = pm.executePaymentWithRetry(ctx, taskID, func() error {
		return pm.circuitBreaker.Call(func() error {
			txHash, err := pm.blockchain.ReleasePayment(ctx, taskID)
			if err != nil {
				return err
			}
			return pm.UpdatePaymentStatus(taskID, PaymentStatusReleased, "task completed successfully", txHash)
		})
	})

	if err != nil {
		pm.UpdatePaymentStatus(taskID, PaymentStatusFailure, fmt.Sprintf("release failed: %v", err), "")
		return err
	}

	return nil
}

// RefundPaymentAsync refunds payment asynchronously
func (pm *PaymentLifecycleManager) RefundPaymentAsync(ctx context.Context, taskID, reason string) {
	go func() {
		pm.logger.Info("starting async payment refund",
			zap.String("task_id", taskID),
			zap.String("reason", reason),
		)

		err := pm.RefundPayment(ctx, taskID, reason)
		if err != nil {
			pm.logger.Error("failed to refund payment asynchronously",
				zap.String("task_id", taskID),
				zap.Error(err),
			)
		}
	}()
}

// RefundPayment refunds payment on task failure or timeout
func (pm *PaymentLifecycleManager) RefundPayment(ctx context.Context, taskID, reason string) error {
	pm.logger.Info("refunding payment",
		zap.String("task_id", taskID),
		zap.String("reason", reason),
	)

	// Get payment info
	payment, err := pm.GetPaymentInfo(taskID)
	if err != nil {
		return err
	}

	// Check if payment can be refunded
	if payment.Status == PaymentStatusReleased || payment.Status == PaymentStatusRefunded {
		return ErrPaymentAlreadyProcessed
	}

	// Execute payment refund with retry and circuit breaker
	err = pm.executePaymentWithRetry(ctx, taskID, func() error {
		return pm.circuitBreaker.Call(func() error {
			txHash, err := pm.blockchain.RefundEscrow(ctx, taskID)
			if err != nil {
				return err
			}
			return pm.UpdatePaymentStatus(taskID, PaymentStatusRefunded, reason, txHash)
		})
	})

	if err != nil {
		pm.UpdatePaymentStatus(taskID, PaymentStatusFailure, fmt.Sprintf("refund failed: %v", err), "")
		return err
	}

	return nil
}

// DisputePayment initiates a payment dispute
func (pm *PaymentLifecycleManager) DisputePayment(ctx context.Context, taskID, reason, initiator string) error {
	pm.logger.Info("disputing payment",
		zap.String("task_id", taskID),
		zap.String("reason", reason),
		zap.String("initiator", initiator),
	)

	// Get payment info
	payment, err := pm.GetPaymentInfo(taskID)
	if err != nil {
		return err
	}

	// Check if payment can be disputed
	if payment.Status == PaymentStatusReleased || payment.Status == PaymentStatusRefunded {
		return ErrPaymentAlreadyProcessed
	}

	// Execute dispute with circuit breaker (no retry for disputes)
	err = pm.circuitBreaker.Call(func() error {
		txHash, err := pm.blockchain.DisputeEscrow(ctx, taskID, reason)
		if err != nil {
			return err
		}
		disputeReason := fmt.Sprintf("disputed by %s: %s", initiator, reason)
		return pm.UpdatePaymentStatus(taskID, PaymentStatusDisputed, disputeReason, txHash)
	})

	if err != nil {
		pm.UpdatePaymentStatus(taskID, PaymentStatusFailure, fmt.Sprintf("dispute failed: %v", err), "")
		return err
	}

	return nil
}

// executePaymentWithRetry executes a payment operation with exponential backoff retry
func (pm *PaymentLifecycleManager) executePaymentWithRetry(ctx context.Context, taskID string, operation func() error) error {
	var lastErr error

	for attempt := 0; attempt <= pm.config.RetryMaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate exponential backoff delay
			delay := time.Duration(float64(pm.config.RetryBaseDelay) * math.Pow(2, float64(attempt-1)))
			if delay > pm.config.RetryMaxDelay {
				delay = pm.config.RetryMaxDelay
			}

			pm.logger.Info("retrying payment operation",
				zap.String("task_id", taskID),
				zap.Int("attempt", attempt),
				zap.Duration("delay", delay),
			)

			// Update retry count
			pm.mu.Lock()
			if payment, exists := pm.payments[taskID]; exists {
				payment.RetryCount = attempt
			}
			pm.mu.Unlock()

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		// Create context with timeout for this attempt
		opCtx, cancel := context.WithTimeout(ctx, pm.config.PaymentTimeout)
		_ = opCtx // TODO: Use opCtx in the operation when needed

		lastErr = operation()
		cancel()

		if lastErr == nil {
			return nil // Success
		}

		// Check if we should retry
		if attempt == pm.config.RetryMaxAttempts || !pm.isRetryableError(lastErr) {
			break
		}
	}

	return fmt.Errorf("payment operation failed after %d attempts: %w", pm.config.RetryMaxAttempts+1, lastErr)
}

// isRetryableError checks if an error is retryable
func (pm *PaymentLifecycleManager) isRetryableError(err error) bool {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return true
	case errors.Is(err, ErrBlockchainUnavailable):
		return true
	case errors.Is(err, ErrPaymentTimeout):
		return true
	case errors.Is(err, ErrCircuitBreakerOpen):
		return false // Don't retry if circuit breaker is open
	default:
		return false
	}
}

// isValidStatusTransition checks if status transition is valid
func (pm *PaymentLifecycleManager) isValidStatusTransition(from, to PaymentStatus) bool {
	validTransitions := map[PaymentStatus][]PaymentStatus{
		PaymentStatusCreated:  {PaymentStatusPending, PaymentStatusAccepted, PaymentStatusRefunded, PaymentStatusDisputed, PaymentStatusFailure},
		PaymentStatusPending:  {PaymentStatusAccepted, PaymentStatusRefunded, PaymentStatusDisputed, PaymentStatusFailure},
		PaymentStatusAccepted: {PaymentStatusReleased, PaymentStatusRefunded, PaymentStatusDisputed, PaymentStatusFailure},
		PaymentStatusReleased: {PaymentStatusDisputed}, // Can only dispute a released payment
		PaymentStatusRefunded: {PaymentStatusDisputed}, // Can only dispute a refunded payment
		PaymentStatusDisputed: {}, // Terminal state
		PaymentStatusFailure:  {PaymentStatusPending, PaymentStatusAccepted}, // Can retry from failure
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if allowed == to {
			return true
		}
	}

	return false
}

// addEvent adds an event to payment history
func (pm *PaymentLifecycleManager) addEvent(payment *PaymentInfo, eventType string, status PaymentStatus, reason string, retryCount int) {
	event := PaymentEvent{
		TaskID:     payment.TaskID,
		EventType:  eventType,
		Status:     status,
		Amount:     payment.Amount,
		Timestamp:  time.Now(),
		Reason:     reason,
		RetryCount: retryCount,
	}

	payment.Events = append(payment.Events, event)
}

// getEventType maps payment status to event type
func getEventType(status PaymentStatus) string {
	switch status {
	case PaymentStatusCreated:
		return "payment_created"
	case PaymentStatusPending:
		return "payment_pending"
	case PaymentStatusAccepted:
		return "payment_accepted"
	case PaymentStatusReleased:
		return "payment_released"
	case PaymentStatusRefunded:
		return "payment_refunded"
	case PaymentStatusDisputed:
		return "payment_disputed"
	case PaymentStatusFailure:
		return "payment_failure"
	default:
		return "payment_unknown"
	}
}

// GetPaymentMetrics returns payment metrics for monitoring
func (pm *PaymentLifecycleManager) GetPaymentMetrics() map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metrics := map[string]interface{}{
		"total_payments": len(pm.payments),
		"circuit_breaker_state": pm.circuitBreaker.state,
	}

	// Count payments by status
	statusCounts := make(map[PaymentStatus]int)
	for _, payment := range pm.payments {
		statusCounts[payment.Status]++
	}

	for status, count := range statusCounts {
		metrics[fmt.Sprintf("payments_%s", status)] = count
	}

	return metrics
}