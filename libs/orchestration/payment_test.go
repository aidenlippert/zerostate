package orchestration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap/zaptest"
)

// MockBlockchain is a mock implementation of BlockchainInterface
type MockBlockchain struct {
	mock.Mock
}

func (m *MockBlockchain) ReleasePayment(ctx context.Context, taskID string) (txHash string, err error) {
	args := m.Called(ctx, taskID)
	return args.String(0), args.Error(1)
}

func (m *MockBlockchain) RefundEscrow(ctx context.Context, taskID string) (txHash string, err error) {
	args := m.Called(ctx, taskID)
	return args.String(0), args.Error(1)
}

func (m *MockBlockchain) DisputeEscrow(ctx context.Context, taskID string, reason string) (txHash string, err error) {
	args := m.Called(ctx, taskID, reason)
	return args.String(0), args.Error(1)
}

func (m *MockBlockchain) IsEnabled() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockBlockchain) GetEscrowStatus(ctx context.Context, taskID string) (PaymentStatus, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).(PaymentStatus), args.Error(1)
}

func TestPaymentLifecycleManager_CreatePayment(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	amount := 10.5

	payment := pm.CreatePayment(taskID, userID, amount)

	assert.Equal(t, taskID, payment.TaskID)
	assert.Equal(t, userID, payment.UserID)
	assert.Equal(t, amount, payment.Amount)
	assert.Equal(t, PaymentStatusCreated, payment.Status)
	assert.Len(t, payment.Events, 1)
	assert.Equal(t, "payment_created", payment.Events[0].EventType)
}

func TestPaymentLifecycleManager_ReleasePayment(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	agentID := "agent-789"
	amount := 10.5

	// Create payment
	pm.CreatePayment(taskID, userID, amount)

	// Update to accepted status first
	err := pm.UpdatePaymentStatus(taskID, PaymentStatusAccepted, "agent selected", "")
	assert.NoError(t, err)

	// Mock blockchain call
	txHash := "0x123abc"
	mockBlockchain.On("ReleasePayment", mock.Anything, taskID).Return(txHash, nil)

	// Release payment
	ctx := context.Background()
	err = pm.ReleasePayment(ctx, taskID, agentID)
	assert.NoError(t, err)

	// Verify payment status
	payment, err := pm.GetPaymentInfo(taskID)
	assert.NoError(t, err)
	assert.Equal(t, PaymentStatusReleased, payment.Status)
	assert.Equal(t, agentID, payment.AgentID)
	assert.Equal(t, txHash, payment.PaymentTxHash)
	assert.NotNil(t, payment.CompletedAt)

	mockBlockchain.AssertExpectations(t)
}

func TestPaymentLifecycleManager_RefundPayment(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	amount := 10.5

	// Create payment
	pm.CreatePayment(taskID, userID, amount)

	// Mock blockchain call
	txHash := "0x456def"
	reason := "task failed"
	mockBlockchain.On("RefundEscrow", mock.Anything, taskID).Return(txHash, nil)

	// Refund payment
	ctx := context.Background()
	err := pm.RefundPayment(ctx, taskID, reason)
	assert.NoError(t, err)

	// Verify payment status
	payment, err := pm.GetPaymentInfo(taskID)
	assert.NoError(t, err)
	assert.Equal(t, PaymentStatusRefunded, payment.Status)
	assert.Equal(t, txHash, payment.PaymentTxHash)
	assert.NotNil(t, payment.CompletedAt)

	mockBlockchain.AssertExpectations(t)
}

func TestPaymentLifecycleManager_DisputePayment(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	amount := 10.5

	// Create payment
	pm.CreatePayment(taskID, userID, amount)

	// Mock blockchain call
	txHash := "0x789ghi"
	reason := "poor quality work"
	initiator := "user-456"
	mockBlockchain.On("DisputeEscrow", mock.Anything, taskID, reason).Return(txHash, nil)

	// Dispute payment
	ctx := context.Background()
	err := pm.DisputePayment(ctx, taskID, reason, initiator)
	assert.NoError(t, err)

	// Verify payment status
	payment, err := pm.GetPaymentInfo(taskID)
	assert.NoError(t, err)
	assert.Equal(t, PaymentStatusDisputed, payment.Status)
	assert.NotNil(t, payment.CompletedAt)

	mockBlockchain.AssertExpectations(t)
}

func TestPaymentLifecycleManager_RetryLogic(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()
	config.RetryMaxAttempts = 2
	config.RetryBaseDelay = 10 * time.Millisecond

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	agentID := "agent-789"
	amount := 10.5

	// Create payment
	pm.CreatePayment(taskID, userID, amount)

	// Update to accepted status
	err := pm.UpdatePaymentStatus(taskID, PaymentStatusAccepted, "agent selected", "")
	assert.NoError(t, err)

	// Mock blockchain call to fail first, then succeed
	mockBlockchain.On("ReleasePayment", mock.Anything, taskID).Return("", errors.New("network error")).Once()
	mockBlockchain.On("ReleasePayment", mock.Anything, taskID).Return("0x123abc", nil).Once()

	// Release payment (should succeed after retry)
	ctx := context.Background()
	err = pm.ReleasePayment(ctx, taskID, agentID)
	assert.NoError(t, err)

	// Verify payment status
	payment, err := pm.GetPaymentInfo(taskID)
	assert.NoError(t, err)
	assert.Equal(t, PaymentStatusReleased, payment.Status)
	assert.Equal(t, 1, payment.RetryCount) // Should have retried once

	mockBlockchain.AssertExpectations(t)
}

func TestPaymentLifecycleManager_CircuitBreaker(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()
	config.CircuitBreakerThreshold = 2
	config.CircuitBreakerTimeout = 100 * time.Millisecond

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID1 := "test-task-1"
	taskID2 := "test-task-2"
	taskID3 := "test-task-3"
	userID := "user-456"
	agentID := "agent-789"
	amount := 10.5

	// Create payments
	pm.CreatePayment(taskID1, userID, amount)
	pm.CreatePayment(taskID2, userID, amount)
	pm.CreatePayment(taskID3, userID, amount)

	// Update to accepted status
	pm.UpdatePaymentStatus(taskID1, PaymentStatusAccepted, "agent selected", "")
	pm.UpdatePaymentStatus(taskID2, PaymentStatusAccepted, "agent selected", "")
	pm.UpdatePaymentStatus(taskID3, PaymentStatusAccepted, "agent selected", "")

	// Mock blockchain calls to fail (trigger circuit breaker)
	mockBlockchain.On("ReleasePayment", mock.Anything, taskID1).Return("", errors.New("blockchain error")).Once()
	mockBlockchain.On("ReleasePayment", mock.Anything, taskID2).Return("", errors.New("blockchain error")).Once()

	ctx := context.Background()

	// First two calls should fail and trigger circuit breaker
	err1 := pm.ReleasePayment(ctx, taskID1, agentID)
	assert.Error(t, err1)

	err2 := pm.ReleasePayment(ctx, taskID2, agentID)
	assert.Error(t, err2)

	// Third call should fail due to circuit breaker being open
	err3 := pm.ReleasePayment(ctx, taskID3, agentID)
	assert.Error(t, err3)
	assert.Contains(t, err3.Error(), "circuit breaker")

	mockBlockchain.AssertExpectations(t)
}

func TestPaymentLifecycleManager_StatusTransitions(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	taskID := "test-task-123"
	userID := "user-456"
	amount := 10.5

	// Create payment
	pm.CreatePayment(taskID, userID, amount)

	// Valid transition: Created -> Pending
	err := pm.UpdatePaymentStatus(taskID, PaymentStatusPending, "waiting for agent", "")
	assert.NoError(t, err)

	// Valid transition: Pending -> Accepted
	err = pm.UpdatePaymentStatus(taskID, PaymentStatusAccepted, "agent selected", "")
	assert.NoError(t, err)

	// Invalid transition: Accepted -> Pending (regression not allowed)
	err = pm.UpdatePaymentStatus(taskID, PaymentStatusPending, "invalid", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payment status")

	// Valid transition: Accepted -> Disputed (disputes always allowed)
	err = pm.UpdatePaymentStatus(taskID, PaymentStatusDisputed, "disputed by user", "0x123")
	assert.NoError(t, err)
}

func TestPaymentLifecycleManager_PaymentNotFound(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	// Try to get non-existent payment
	_, err := pm.GetPaymentInfo("non-existent-task")
	assert.Error(t, err)
	assert.Equal(t, ErrPaymentNotFound, err)

	// Try to update non-existent payment
	err = pm.UpdatePaymentStatus("non-existent-task", PaymentStatusAccepted, "test", "")
	assert.Error(t, err)
	assert.Equal(t, ErrPaymentNotFound, err)
}

func TestCircuitBreaker(t *testing.T) {
	threshold := 2
	timeout := 50 * time.Millisecond

	cb := NewCircuitBreaker(threshold, timeout)

	// Initially closed
	assert.True(t, cb.AllowRequest())

	// Record failures to trigger circuit breaker
	cb.RecordResult(false)            // failure 1
	assert.True(t, cb.AllowRequest()) // still closed

	cb.RecordResult(false)             // failure 2
	assert.False(t, cb.AllowRequest()) // now open

	// Wait for timeout
	time.Sleep(timeout + 10*time.Millisecond)

	// Should be half-open now
	assert.True(t, cb.AllowRequest())

	// Record success to close circuit
	cb.RecordResult(true)
	assert.True(t, cb.AllowRequest()) // closed again
}

func TestPaymentLifecycleManager_GetPaymentMetrics(t *testing.T) {
	mockBlockchain := &MockBlockchain{}
	logger := zaptest.NewLogger(t)
	config := DefaultPaymentConfig()

	pm := NewPaymentLifecycleManager(mockBlockchain, config, logger)

	// Create some test payments
	pm.CreatePayment("task1", "user1", 10.0)
	pm.CreatePayment("task2", "user2", 20.0)

	metrics := pm.GetPaymentMetrics()

	assert.Equal(t, 2, metrics["total_payments"])
	assert.Equal(t, 2, metrics["payments_created"])
	assert.Contains(t, metrics, "circuit_breaker_state")
}

// Integration test with orchestrator
func TestOrchestratorPaymentIntegration(t *testing.T) {
	// Setup
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	// Create task queue
	queue := NewTaskQueue(ctx, 100, logger)

	// Create mock blockchain
	mockBlockchain := &MockBlockchain{}
	mockBlockchain.On("IsEnabled").Return(true)

	// Create blockchain adapter
	blockchainAdapter := &BlockchainAdapter{
		blockchain: nil, // Use nil to trigger mock behavior
		logger:     logger,
	}

	// Create orchestrator with payment manager
	config := DefaultOrchestratorConfig()
	config.NumWorkers = 1

	paymentConfig := DefaultPaymentConfig()
	paymentManager := NewPaymentLifecycleManager(blockchainAdapter, paymentConfig, logger)

	orchestrator := &Orchestrator{
		queue:          queue,
		selector:       nil, // Not needed for this test
		executor:       NewMockTaskExecutor(logger),
		logger:         logger,
		numWorkers:     1,
		ctx:            ctx,
		stopCh:         make(chan struct{}),
		metrics:        &OrchestratorMetrics{},
		paymentManager: paymentManager,
	}
	_ = orchestrator

	// Create and enqueue a task
	task := NewTask("user123", "test", []string{"test"}, map[string]interface{}{"test": "data"})
	task.Budget = 15.5

	err := queue.Enqueue(task)
	assert.NoError(t, err)

	// Verify payment was created
	paymentInfo, err := paymentManager.GetPaymentInfo(task.ID)
	assert.Error(t, err) // Should be ErrPaymentNotFound since processTask wasn't called

	// Simulate task processing would create payment
	paymentManager.CreatePayment(task.ID, task.UserID, task.Budget)

	paymentInfo, err = paymentManager.GetPaymentInfo(task.ID)
	assert.NoError(t, err)
	assert.Equal(t, task.ID, paymentInfo.TaskID)
	assert.Equal(t, task.Budget, paymentInfo.Amount)
	assert.Equal(t, PaymentStatusCreated, paymentInfo.Status)
}
