package orchestration

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/aidenlippert/zerostate/libs/substrate"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

var (
	ErrNoSuitableAgent     = errors.New("no suitable agent found for task")
	ErrAgentUnavailable    = errors.New("agent is unavailable")
	ErrExecutionTimeout    = errors.New("task execution timeout")
	ErrOrchestratorStopped = errors.New("orchestrator has been stopped")
)

// AgentSelector selects the best agent for a given task
type AgentSelector interface {
	SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error)
}

// TaskExecutor executes tasks on agents
type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error)
}

// Orchestrator manages task routing and execution
type Orchestrator struct {
	// Core components
	queue      *TaskQueue
	selector   AgentSelector
	executor   TaskExecutor
	auctioneer *Auctioneer
	cqRouter   *CQRouter // Confidence-based Q-Routing for intelligent agent discovery
	logger     *zap.Logger

	// Worker pool
	numWorkers int
	workers    []*worker
	workerWg   sync.WaitGroup

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	stopCh chan struct{}

	// Metrics
	metrics *OrchestratorMetrics
	mu      sync.RWMutex

	// Blockchain integration
	blockchain *substrate.BlockchainService

	// Payment lifecycle management
	paymentManager *PaymentLifecycleManager

	// Extended escrow functionality
	escrowClient *substrate.EscrowClient
}

// SetAuctioneer attaches an Auctioneer to the orchestrator after construction.
// This allows wiring in components that depend on P2P messaging without
// changing the core constructor signature.
func (o *Orchestrator) SetAuctioneer(a *Auctioneer) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.auctioneer = a
}

// OrchestratorMetrics tracks orchestration statistics
type OrchestratorMetrics struct {
	TasksProcessed   int64
	TasksSucceeded   int64
	TasksFailed      int64
	TasksTimedOut    int64
	AuctionsStarted  int64 // Total auctions initiated
	AuctionSuccesses int64 // Auctions that received bids and selected winner
	AuctionFailures  int64 // Auctions that failed to run
	AuctionNoBids    int64 // Auctions that completed but received no bids
	DBFallbacks      int64 // Times DB selector was used as fallback
	AvgExecutionTime time.Duration
	ActiveWorkers    int
	// Reputation metrics
	ReputationUpdates  int64 // Successful reputation updates
	ReputationFailures int64 // Failed reputation updates

	// Payment metrics
	PaymentsReleased              int64         // Successful payment releases
	PaymentsRefunded              int64         // Successful payment refunds
	PaymentsDisputed              int64         // Payment disputes initiated
	PaymentReleaseLatency         time.Duration // Average payment release latency
	PaymentCircuitBreakerFailures int64         // Payment circuit breaker failures
}

// OrchestratorConfig configures the orchestrator
type OrchestratorConfig struct {
	NumWorkers       int           // Number of worker goroutines
	TaskTimeout      time.Duration // Default task timeout
	RetryAttempts    int           // Max retry attempts for failed tasks
	RetryBackoff     time.Duration // Initial retry backoff
	MaxRetryBackoff  time.Duration // Maximum retry backoff
	WorkerPollPeriod time.Duration // How often workers poll for tasks
}

// DefaultOrchestratorConfig returns default configuration
func DefaultOrchestratorConfig() *OrchestratorConfig {
	return &OrchestratorConfig{
		NumWorkers:       5,
		TaskTimeout:      30 * time.Second,
		RetryAttempts:    3,
		RetryBackoff:     1 * time.Second,
		MaxRetryBackoff:  10 * time.Second,
		WorkerPollPeriod: 100 * time.Millisecond,
	}
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(
	ctx context.Context,
	queue *TaskQueue,
	selector AgentSelector,
	executor TaskExecutor,
	config *OrchestratorConfig,
	logger *zap.Logger,
) *Orchestrator {
	return NewOrchestratorWithBlockchain(ctx, queue, selector, executor, config, logger, nil)
}

// NewOrchestratorWithBlockchain creates a new orchestrator with blockchain integration
func NewOrchestratorWithBlockchain(
	ctx context.Context,
	queue *TaskQueue,
	selector AgentSelector,
	executor TaskExecutor,
	config *OrchestratorConfig,
	logger *zap.Logger,
	blockchain *substrate.BlockchainService,
) *Orchestrator {
	if config == nil {
		config = DefaultOrchestratorConfig()
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	orchCtx, cancel := context.WithCancel(ctx)

	// Initialize CQ-Router for intelligent routing
	cqRouter := NewCQRouter(logger.With(zap.String("component", "cq-router")))

	// Initialize payment lifecycle manager
	var paymentManager *PaymentLifecycleManager
	if blockchain != nil {
		blockchainAdapter := NewBlockchainAdapter(blockchain)
		paymentConfig := DefaultPaymentConfig()
		paymentManager = NewPaymentLifecycleManager(
			blockchainAdapter,
			paymentConfig,
			logger.With(zap.String("component", "payment-lifecycle")),
		)
	}

	// Initialize escrow client
	var escrowClient *substrate.EscrowClient
	if blockchain != nil {
		escrowClient = blockchain.Escrow()
	}

	return &Orchestrator{
		queue:          queue,
		selector:       selector,
		executor:       executor,
		auctioneer:     nil, // Can be wired later when MessageBus is available
		cqRouter:       cqRouter,
		logger:         logger,
		numWorkers:     config.NumWorkers,
		ctx:            orchCtx,
		cancel:         cancel,
		stopCh:         make(chan struct{}),
		metrics:        &OrchestratorMetrics{},
		blockchain:     blockchain,
		paymentManager: paymentManager,
		escrowClient:   escrowClient,
	}
}

// Start starts the orchestrator workers
func (o *Orchestrator) Start() error {
	o.logger.Info("starting orchestrator",
		zap.Int("num_workers", o.numWorkers),
	)

	// Start worker goroutines
	o.workers = make([]*worker, o.numWorkers)
	for i := 0; i < o.numWorkers; i++ {
		w := &worker{
			id:           i,
			orchestrator: o,
			logger:       o.logger.With(zap.Int("worker_id", i)),
		}
		o.workers[i] = w

		o.workerWg.Add(1)
		go w.run()
	}

	// Update active workers metric
	o.mu.Lock()
	o.metrics.ActiveWorkers = o.numWorkers
	o.mu.Unlock()

	o.logger.Info("orchestrator started successfully",
		zap.Int("workers", o.numWorkers),
	)

	return nil
}

// Stop gracefully stops the orchestrator
func (o *Orchestrator) Stop() error {
	o.logger.Info("stopping orchestrator")

	// Signal workers to stop
	o.cancel()
	close(o.stopCh)

	// Wait for all workers to finish
	o.workerWg.Wait()

	// Update active workers metric
	o.mu.Lock()
	o.metrics.ActiveWorkers = 0
	o.mu.Unlock()

	o.logger.Info("orchestrator stopped")
	return nil
}

// GetMetrics returns current orchestrator metrics
func (o *Orchestrator) GetMetrics() *OrchestratorMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	return &OrchestratorMetrics{
		TasksProcessed:     o.metrics.TasksProcessed,
		TasksSucceeded:     o.metrics.TasksSucceeded,
		TasksFailed:        o.metrics.TasksFailed,
		TasksTimedOut:      o.metrics.TasksTimedOut,
		AuctionsStarted:    o.metrics.AuctionsStarted,
		AuctionSuccesses:   o.metrics.AuctionSuccesses,
		AuctionFailures:    o.metrics.AuctionFailures,
		AuctionNoBids:      o.metrics.AuctionNoBids,
		DBFallbacks:        o.metrics.DBFallbacks,
		AvgExecutionTime:   o.metrics.AvgExecutionTime,
		ActiveWorkers:      o.metrics.ActiveWorkers,
		ReputationUpdates:  o.metrics.ReputationUpdates,
		ReputationFailures: o.metrics.ReputationFailures,

		// Payment metrics
		PaymentsReleased:              o.metrics.PaymentsReleased,
		PaymentsRefunded:              o.metrics.PaymentsRefunded,
		PaymentsDisputed:              o.metrics.PaymentsDisputed,
		PaymentReleaseLatency:         o.metrics.PaymentReleaseLatency,
		PaymentCircuitBreakerFailures: o.metrics.PaymentCircuitBreakerFailures,
	}
}

// updateMetrics updates orchestrator metrics
func (o *Orchestrator) updateMetrics(result *TaskResult, executionTime time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.metrics.TasksProcessed++

	switch result.Status {
	case TaskStatusCompleted:
		o.metrics.TasksSucceeded++
	case TaskStatusFailed:
		o.metrics.TasksFailed++
	}

	// Update average execution time
	if o.metrics.AvgExecutionTime == 0 {
		o.metrics.AvgExecutionTime = executionTime
	} else {
		// Simple moving average
		o.metrics.AvgExecutionTime = (o.metrics.AvgExecutionTime + executionTime) / 2
	}
}

// worker represents a task processing worker
type worker struct {
	id           int
	orchestrator *Orchestrator
	logger       *zap.Logger
}

// run is the main worker loop
func (w *worker) run() {
	defer w.orchestrator.workerWg.Done()

	w.logger.Info("worker started")

	for {
		select {
		case <-w.orchestrator.stopCh:
			w.logger.Info("worker stopping")
			return
		case <-w.orchestrator.ctx.Done():
			w.logger.Info("worker context canceled")
			return
		default:
			// Try to get a task
			task, err := w.orchestrator.queue.DequeueWait(w.orchestrator.ctx)
			if err != nil {
				if err == context.Canceled || err == ErrQueueClosed {
					w.logger.Info("worker stopping due to context/queue closed")
					return
				}
				w.logger.Error("failed to dequeue task", zap.Error(err))
				time.Sleep(1 * time.Second)
				continue
			}

			if task == nil {
				// No task available, continue
				continue
			}

			// Process the task
			w.processTask(task)
		}
	}
}

// processTask processes a single task
func (w *worker) processTask(task *Task) {
	startTime := time.Now()

	w.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Int("priority", int(task.Priority)),
	)

	// Update task status to assigned
	task.UpdateStatus(TaskStatusAssigned)
	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task status", zap.Error(err))
	}

	// Initialize payment if payment manager is available
	if w.orchestrator.paymentManager != nil {
		w.orchestrator.paymentManager.CreatePayment(task.ID, task.UserID, task.Budget)
		task.PaymentStatus = PaymentStatusCreated
		task.PaymentUpdatedAt = &startTime
	}

	// Select agent for task (skip for executors that don't need DB agent selection)
	var agent *identity.AgentCard
	var err error
	var auctionResult *AuctionResult

	_, isIntelligent := w.orchestrator.executor.(*IntelligentTaskExecutor)
	_, isARI := w.orchestrator.executor.(*ARIExecutor)
	_, isP2PARI := w.orchestrator.executor.(*P2PARIExecutor)

	if isIntelligent || isARI || isP2PARI {
		// These executors don't need agent selection - they handle execution directly
		if isIntelligent {
			w.logger.Info("using intelligent executor - skipping agent selection",
				zap.String("task_id", task.ID),
			)
		} else if isARI {
			w.logger.Info("using ARI executor - skipping agent selection",
				zap.String("task_id", task.ID),
			)
		} else if isP2PARI {
			w.logger.Info("using P2P ARI executor - skipping agent selection, discovering runtime",
				zap.String("task_id", task.ID),
			)
		}
		agent = nil // No agent needed
	} else {
		// Market-based selection: run auction first, only fall back to DB if no bids
		// Always try auction if auctioneer is available and task has capabilities
		if w.orchestrator.auctioneer != nil && len(task.Capabilities) > 0 {
			logic := SelectionLogic{Mode: SelectionModeCheapest}
			window := 500 * time.Millisecond

			w.logger.Info("starting market auction",
				zap.String("task_id", task.ID),
				zap.Strings("capabilities", task.Capabilities),
				zap.Duration("window", window),
			)

			// Track auction start
			w.orchestrator.mu.Lock()
			w.orchestrator.metrics.AuctionsStarted++
			w.orchestrator.mu.Unlock()

			auctionResult, err = w.orchestrator.auctioneer.StartAuction(w.orchestrator.ctx, task, logic, window)
			if err != nil {
				w.logger.Warn("auction failed; falling back to DB selection",
					zap.String("task_id", task.ID),
					zap.Error(err),
				)
				// Track auction failure
				w.orchestrator.mu.Lock()
				w.orchestrator.metrics.AuctionFailures++
				w.orchestrator.mu.Unlock()
			} else if auctionResult != nil && auctionResult.Winner != nil {
				// ✅ AUCTION SUCCESS - Use the market winner
				w.logger.Info("🏆 auction winner selected (MARKET PRIMARY)",
					zap.String("task_id", task.ID),
					zap.String("winner_did", string(auctionResult.Winner.AgentDID)),
					zap.Float64("price", auctionResult.Winner.Price),
					zap.Int64("eta_ms", auctionResult.Winner.ETAms),
					zap.Int("total_bids", len(auctionResult.AllBids)),
				)

				// Track auction success
				w.orchestrator.mu.Lock()
				w.orchestrator.metrics.AuctionSuccesses++
				w.orchestrator.mu.Unlock()

				// Create a minimal AgentCard from the winner for execution
				// TODO: fetch full AgentCard from registry or on-chain DID document
				caps := make([]identity.Capability, len(task.Capabilities))
				for i, cap := range task.Capabilities {
					caps[i] = identity.Capability{Name: cap, Version: "v1"}
				}
				agent = &identity.AgentCard{
					DID:          string(auctionResult.Winner.AgentDID),
					Endpoints:    &identity.Endpoints{HTTP: []string{}},
					Capabilities: caps,
					Proof:        &identity.Proof{},
				}
				task.AssignedTo = agent.DID
			} else {
				// No bids received - this is the only case we fall back to DB
				w.logger.Warn("auction completed but no bids received; falling back to DB",
					zap.String("task_id", task.ID),
					zap.Int("bid_count", len(auctionResult.AllBids)),
				)
				// Track no-bid auction
				w.orchestrator.mu.Lock()
				w.orchestrator.metrics.AuctionNoBids++
				w.orchestrator.mu.Unlock()
			}
		} else {
			// No auctioneer configured - this is expected for non-market mode
			if len(task.Capabilities) > 0 {
				w.logger.Debug("auctioneer not configured, using DB selection",
					zap.String("task_id", task.ID),
				)
			}
		}

		// Fall back to DB selector only if auction didn't produce a winner
		if agent == nil {
			w.logger.Info("using database agent selector (fallback)",
				zap.String("task_id", task.ID),
			)

			// Track DB fallback
			w.orchestrator.mu.Lock()
			w.orchestrator.metrics.DBFallbacks++
			w.orchestrator.mu.Unlock()

			agent, err = w.orchestrator.selector.SelectAgent(w.orchestrator.ctx, task)
			if err != nil {
				w.logger.Error("failed to select agent from database",
					zap.String("task_id", task.ID),
					zap.Error(err),
				)
				w.handleTaskFailure(task, err)
				return
			}

			// Assign agent to task
			task.AssignedTo = agent.DID
			w.logger.Info("agent selected from database",
				zap.String("task_id", task.ID),
				zap.String("agent_id", agent.DID),
			)
		}
	}

	// Update payment status to accepted when agent is assigned
	if w.orchestrator.paymentManager != nil && agent != nil {
		err := w.orchestrator.paymentManager.UpdatePaymentStatus(task.ID, PaymentStatusAccepted, "agent selected", "")
		if err != nil {
			w.logger.Error("failed to update payment status to accepted", zap.Error(err))
		} else {
			task.PaymentStatus = PaymentStatusAccepted
			now := time.Now()
			task.PaymentUpdatedAt = &now
		}
	}

	task.UpdateStatus(TaskStatusRunning)
	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task with agent assignment", zap.Error(err))
	}

	// Execute task with timeout
	execCtx, cancel := context.WithTimeout(w.orchestrator.ctx, task.Timeout)
	defer cancel()

	result, err := w.orchestrator.executor.ExecuteTask(execCtx, task, agent)
	executionTime := time.Since(startTime)

	if err != nil {
		w.logger.Error("task execution failed",
			zap.String("task_id", task.ID),
			zap.String("agent_id", agent.DID),
			zap.Error(err),
			zap.Duration("execution_time", executionTime),
		)
		w.handleTaskFailure(task, err)
		return
	}

	// Update task with result
	task.Result = result.Result
	task.ActualCost = result.Cost
	if task.ActualCost == 0 {
		if auctionResult != nil && auctionResult.Winner != nil {
			task.ActualCost = auctionResult.Winner.Price
		} else if task.Budget > 0 {
			task.ActualCost = task.Budget
		}
	}
	task.UpdateStatus(result.Status)

	if result.Status == TaskStatusFailed {
		task.Error = result.Error
	}

	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update task with result", zap.Error(err))
	}

	// Update metrics
	w.orchestrator.updateMetrics(result, executionTime)

	// Handle payment lifecycle based on task outcome
	if w.orchestrator.paymentManager != nil {
		w.handlePaymentLifecycle(task, agent, result.Status)
	}

	// Report reputation to blockchain (async, don't fail task if reputation fails)
	go w.reportReputationAsync(task, agent, result.Status == TaskStatusCompleted)

	w.logger.Info("task completed",
		zap.String("task_id", task.ID),
		zap.String("status", string(result.Status)),
		zap.Duration("execution_time", executionTime),
	)
}

// handlePaymentLifecycle handles payment release/refund based on task outcome
func (w *worker) handlePaymentLifecycle(task *Task, agent *identity.AgentCard, taskStatus TaskStatus) {
	agentID := ""
	if agent != nil {
		agentID = agent.DID
	}

	// Handle extended escrow types
	switch task.EscrowType {
	case "milestone":
		w.handleMilestonePaymentLifecycle(task, agent, taskStatus)
		return
	case "multi_party":
		w.handleMultiPartyPaymentLifecycle(task, agent, taskStatus)
		return
	default:
		// Handle simple escrow and legacy tasks
	}

	switch taskStatus {
	case TaskStatusCompleted:
		// Release payment to agent on successful completion
		if w.orchestrator.escrowClient != nil {
			w.releasePaymentWithEscrow(task, agentID)
		} else {
			w.orchestrator.paymentManager.ReleasePaymentAsync(w.orchestrator.ctx, task.ID, agentID)
		}

		// Update payment metrics
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.PaymentsReleased++
		w.orchestrator.mu.Unlock()

	case TaskStatusFailed, TaskStatusCanceled:
		// Refund payment on failure or cancellation
		reason := fmt.Sprintf("task %s", taskStatus)
		if task.Error != "" {
			reason = fmt.Sprintf("task failed: %s", task.Error)
		}

		if w.orchestrator.escrowClient != nil {
			w.refundPaymentWithEscrow(task, reason)
		} else {
			w.orchestrator.paymentManager.RefundPaymentAsync(w.orchestrator.ctx, task.ID, reason)
		}

		// Update payment metrics
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.PaymentsRefunded++
		w.orchestrator.mu.Unlock()

	default:
		w.logger.Debug("no payment action for task status",
			zap.String("task_id", task.ID),
			zap.String("status", string(taskStatus)),
		)
	}
}

// handleMilestonePaymentLifecycle handles payment for milestone-based escrow
func (w *worker) handleMilestonePaymentLifecycle(task *Task, agent *identity.AgentCard, taskStatus TaskStatus) {
	w.logger.Info("handling milestone payment lifecycle",
		zap.String("task_id", task.ID),
		zap.String("status", string(taskStatus)),
		zap.Int("current_milestone", task.CurrentMilestone),
	)

	switch taskStatus {
	case TaskStatusCompleted:
		// Check if all milestones are approved
		allApproved := true
		for _, milestone := range task.Milestones {
			if milestone.Status != "approved" {
				allApproved = false
				break
			}
		}

		if allApproved {
			// Release full payment
			w.releasePaymentWithEscrow(task, agent.DID)
			w.logger.Info("all milestones approved, releasing full payment",
				zap.String("task_id", task.ID),
			)
		} else {
			// Only release payment for completed milestones
			w.releaseMilestonePayments(task)
		}

	case TaskStatusFailed, TaskStatusCanceled:
		// Apply refund policy or refund for incomplete milestones
		w.refundUncompletedMilestones(task)
	}
}

// handleMultiPartyPaymentLifecycle handles payment for multi-party escrow
func (w *worker) handleMultiPartyPaymentLifecycle(task *Task, agent *identity.AgentCard, taskStatus TaskStatus) {
	w.logger.Info("handling multi-party payment lifecycle",
		zap.String("task_id", task.ID),
		zap.String("status", string(taskStatus)),
		zap.Int("required_votes", task.RequiredVotes),
	)

	switch taskStatus {
	case TaskStatusCompleted:
		// For multi-party, require approval before payment release
		// This would typically involve off-chain approval collection
		// For now, just release payment (in production, add approval logic)
		w.releasePaymentWithEscrow(task, agent.DID)

	case TaskStatusFailed, TaskStatusCanceled:
		reason := fmt.Sprintf("multi-party task %s", taskStatus)
		w.refundPaymentWithEscrow(task, reason)
	}
}

// releasePaymentWithEscrow releases payment using the escrow client
func (w *worker) releasePaymentWithEscrow(task *Task, agentDID string) {
	if w.orchestrator.escrowClient == nil {
		w.logger.Warn("escrow client not available, falling back to payment manager")
		w.orchestrator.paymentManager.ReleasePaymentAsync(w.orchestrator.ctx, task.ID, agentDID)
		return
	}

	taskIDBytes, err := w.orchestrator.convertStringToTaskID(task.ID)
	if err != nil {
		w.logger.Error("failed to convert task ID for payment release", zap.Error(err))
		return
	}

	err = w.orchestrator.escrowClient.ReleasePayment(w.orchestrator.ctx, taskIDBytes)
	if err != nil {
		w.logger.Error("failed to release payment via escrow client",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
	} else {
		w.logger.Info("payment released via escrow client",
			zap.String("task_id", task.ID),
			zap.String("agent_did", agentDID),
		)
	}
}

// refundPaymentWithEscrow refunds payment using the escrow client
func (w *worker) refundPaymentWithEscrow(task *Task, reason string) {
	if w.orchestrator.escrowClient == nil {
		w.logger.Warn("escrow client not available, falling back to payment manager")
		w.orchestrator.paymentManager.RefundPaymentAsync(w.orchestrator.ctx, task.ID, reason)
		return
	}

	taskIDBytes, err := w.orchestrator.convertStringToTaskID(task.ID)
	if err != nil {
		w.logger.Error("failed to convert task ID for payment refund", zap.Error(err))
		return
	}

	err = w.orchestrator.escrowClient.RefundEscrow(w.orchestrator.ctx, taskIDBytes)
	if err != nil {
		w.logger.Error("failed to refund payment via escrow client",
			zap.String("task_id", task.ID),
			zap.String("reason", reason),
			zap.Error(err),
		)
	} else {
		w.logger.Info("payment refunded via escrow client",
			zap.String("task_id", task.ID),
			zap.String("reason", reason),
		)
	}
}

// releaseMilestonePayments releases payment for completed milestones
func (w *worker) releaseMilestonePayments(task *Task) {
	// TODO: Implement partial milestone payment release
	// This would involve tracking which milestones have been paid
	// and releasing payment only for newly approved milestones
	w.logger.Info("milestone payment release not yet implemented",
		zap.String("task_id", task.ID),
	)
}

// refundUncompletedMilestones refunds payment for uncompleted milestones
func (w *worker) refundUncompletedMilestones(task *Task) {
	// TODO: Implement refund policy for uncompleted milestones
	// This would consider the refund policy type and calculate appropriate refunds
	reason := "task failed with uncompleted milestones"
	w.refundPaymentWithEscrow(task, reason)
}

// handleTaskFailure handles task execution failures
func (w *worker) handleTaskFailure(task *Task, err error) {
	task.Error = err.Error()

	// Handle payment refund for failed task
	if w.orchestrator.paymentManager != nil {
		reason := fmt.Sprintf("task execution failed: %v", err)
		w.orchestrator.paymentManager.RefundPaymentAsync(w.orchestrator.ctx, task.ID, reason)

		// Update payment metrics
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.PaymentsRefunded++
		w.orchestrator.mu.Unlock()
	}

	// Check if task can be retried
	if task.CanRetry() {
		task.RetryCount++
		task.UpdateStatus(TaskStatusPending)

		// Re-enqueue with exponential backoff
		backoff := time.Duration(task.RetryCount) * time.Second
		time.Sleep(backoff)

		if err := w.orchestrator.queue.Enqueue(task); err != nil {
			w.logger.Error("failed to re-enqueue task for retry",
				zap.String("task_id", task.ID),
				zap.Error(err),
			)
			task.UpdateStatus(TaskStatusFailed)
		} else {
			w.logger.Info("task re-enqueued for retry",
				zap.String("task_id", task.ID),
				zap.Int("retry_count", task.RetryCount),
			)
			return
		}
	} else {
		task.UpdateStatus(TaskStatusFailed)
	}

	if err := w.orchestrator.queue.Update(task); err != nil {
		w.logger.Error("failed to update failed task", zap.Error(err))
	}
}

// HNSWAgentSelector uses HNSW semantic search to find best agent
type HNSWAgentSelector struct {
	hnsw   *search.HNSWIndex
	logger *zap.Logger
}

// NewHNSWAgentSelector creates a new HNSW-based agent selector
func NewHNSWAgentSelector(hnsw *search.HNSWIndex, logger *zap.Logger) *HNSWAgentSelector {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &HNSWAgentSelector{
		hnsw:   hnsw,
		logger: logger,
	}
}

// SelectAgent selects the best agent for a task using semantic search
func (s *HNSWAgentSelector) SelectAgent(ctx context.Context, task *Task) (*identity.AgentCard, error) {
	if s.hnsw == nil {
		return nil, fmt.Errorf("HNSW index not initialized")
	}

	// Generate embedding for task capabilities
	embeddingGen := search.NewEmbedding(128)
	taskVector := embeddingGen.EncodeCapabilities(task.Capabilities, nil)

	// Search for similar agents (k=5)
	results := s.hnsw.Search(taskVector, 5)

	if len(results) == 0 {
		s.logger.Warn("no agents found for task",
			zap.String("task_id", task.ID),
			zap.Strings("capabilities", task.Capabilities),
		)
		return nil, ErrNoSuitableAgent
	}

	// Get the best matching agent (highest similarity)
	bestResult := results[0]
	agent, ok := bestResult.Payload.(*identity.AgentCard)
	if !ok {
		return nil, fmt.Errorf("invalid agent card in HNSW result")
	}

	s.logger.Info("agent selected",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
		zap.Float64("similarity", bestResult.Distance),
	)

	return agent, nil
}

// reportReputationAsync reports task outcome to blockchain reputation system asynchronously
func (w *worker) reportReputationAsync(task *Task, agent *identity.AgentCard, success bool) {
	if w.orchestrator.blockchain == nil {
		return // No blockchain service configured
	}

	// Skip reputation reporting for tasks without agents (e.g., intelligent executor)
	if agent == nil || agent.DID == "" {
		return
	}

	// Convert agent DID to AccountID format expected by blockchain
	agentAccount, err := w.convertDIDToAccountID(agent.DID)
	if err != nil {
		w.logger.Warn("failed to convert agent DID to account ID",
			zap.String("task_id", task.ID),
			zap.String("agent_did", agent.DID),
			zap.Error(err),
		)
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.ReputationFailures++
		w.orchestrator.mu.Unlock()
		return
	}

	// Create context with timeout for reputation reporting
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Report task outcome to blockchain
	err = w.orchestrator.ReportTaskOutcomeToBlockchain(ctx, w.orchestrator.blockchain, task, agentAccount, success)
	if err != nil {
		w.logger.Warn("failed to report task outcome to blockchain - continuing without failure",
			zap.String("task_id", task.ID),
			zap.String("agent_did", agent.DID),
			zap.Bool("success", success),
			zap.Error(err),
		)
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.ReputationFailures++
		w.orchestrator.mu.Unlock()
	} else {
		w.logger.Debug("successfully reported task outcome to blockchain",
			zap.String("task_id", task.ID),
			zap.String("agent_did", agent.DID),
			zap.Bool("success", success),
		)
		w.orchestrator.mu.Lock()
		w.orchestrator.metrics.ReputationUpdates++
		w.orchestrator.mu.Unlock()
	}
}

// convertDIDToAccountID converts a DID string to substrate AccountID
// This is a simplified conversion - in production you might need to resolve DID documents
func (w *worker) convertDIDToAccountID(did string) (substrate.AccountID, error) {
	// For now, assume DID is in a format we can parse to get the account ID
	// In a real implementation, you'd resolve the DID document to get the blockchain account

	// Simple implementation: assume DID contains the hex account ID
	// Example: did:substrate:5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY
	if len(did) < 48 { // Basic validation
		return substrate.AccountID{}, fmt.Errorf("invalid DID format: %s", did)
	}

	// Extract the last part which should be the SS58 address
	parts := []string{}
	if len(did) > 13 && did[:13] == "did:substrate" {
		parts = []string{did[14:]} // Skip "did:substrate:"
	} else {
		parts = []string{did} // Use as-is
	}

	if len(parts) == 0 {
		return substrate.AccountID{}, fmt.Errorf("could not extract account from DID: %s", did)
	}

	// For now, create a dummy AccountID from the DID hash
	// In production, you'd properly decode the SS58 address
	var accountID substrate.AccountID
	copy(accountID[:], []byte(parts[0])[:32])

	return accountID, nil
}

// MockTaskExecutor is a simple executor for testing
type MockTaskExecutor struct {
	logger *zap.Logger
}

// NewMockTaskExecutor creates a new mock executor
func NewMockTaskExecutor(logger *zap.Logger) *MockTaskExecutor {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &MockTaskExecutor{
		logger: logger,
	}
}

// ExecuteTask executes a task (mock implementation)
func (e *MockTaskExecutor) ExecuteTask(ctx context.Context, task *Task, agent *identity.AgentCard) (*TaskResult, error) {
	e.logger.Info("executing task (mock)",
		zap.String("task_id", task.ID),
		zap.String("agent_id", agent.DID),
	)

	// Simulate execution time
	select {
	case <-time.After(100 * time.Millisecond):
		// Success
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Return mock result
	return &TaskResult{
		TaskID: task.ID,
		Status: TaskStatusCompleted,
		Result: map[string]interface{}{
			"message": "Task completed successfully (mock)",
			"input":   task.Input,
		},
		ExecutionMS: 100,
		AgentDID:    agent.DID,
		Timestamp:   time.Now(),
	}, nil
}

// =============================================================================
// EXTENDED ESCROW METHODS
// =============================================================================

// CreateMultiPartyTask creates a task with multi-party escrow support
func (o *Orchestrator) CreateMultiPartyTask(
	ctx context.Context,
	userID string,
	taskType string,
	capabilities []string,
	input map[string]interface{},
	participants []string,
	requiredVotes int,
	budget float64,
) (*Task, error) {
	o.logger.Info("creating multi-party task",
		zap.String("user_id", userID),
		zap.String("task_type", taskType),
		zap.Strings("participants", participants),
		zap.Int("required_votes", requiredVotes),
		zap.Float64("budget", budget),
	)

	// Create basic task
	task := NewTask(userID, taskType, capabilities, input)
	task.EscrowType = "multi_party"
	task.Participants = participants
	task.RequiredVotes = requiredVotes
	task.Budget = budget

	// Create multi-party escrow on blockchain if escrow client is available
	if o.escrowClient != nil {
		taskIDBytes, err := o.convertStringToTaskID(task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert task ID: %w", err)
		}

		// Create initial escrow (will be enhanced with participants after agent selection)
		err = o.escrowClient.CreateEscrow(ctx, taskIDBytes, uint64(budget*1e8), [32]byte{}, nil)
		if err != nil {
			o.logger.Error("failed to create escrow on blockchain", zap.Error(err))
			return nil, fmt.Errorf("failed to create escrow: %w", err)
		}

		task.EscrowTxHash = fmt.Sprintf("0x%x", taskIDBytes)
		task.PaymentStatus = PaymentStatusCreated

		o.logger.Info("multi-party escrow created",
			zap.String("task_id", task.ID),
			zap.String("tx_hash", task.EscrowTxHash),
		)
	}

	// Enqueue task
	if err := o.queue.Enqueue(task); err != nil {
		return nil, fmt.Errorf("failed to enqueue multi-party task: %w", err)
	}

	return task, nil
}

// CreateMilestoneTask creates a task with milestone-based escrow
func (o *Orchestrator) CreateMilestoneTask(
	ctx context.Context,
	userID string,
	taskType string,
	capabilities []string,
	input map[string]interface{},
	milestones []TaskMilestone,
	budget float64,
) (*Task, error) {
	o.logger.Info("creating milestone task",
		zap.String("user_id", userID),
		zap.String("task_type", taskType),
		zap.Int("milestones", len(milestones)),
		zap.Float64("budget", budget),
	)

	// Validate milestones
	if len(milestones) == 0 {
		return nil, fmt.Errorf("milestone task must have at least one milestone")
	}

	totalAmount := 0.0
	for _, milestone := range milestones {
		totalAmount += milestone.Amount
	}
	if totalAmount != budget {
		return nil, fmt.Errorf("sum of milestone amounts (%.2f) must equal budget (%.2f)", totalAmount, budget)
	}

	// Create basic task
	task := NewTask(userID, taskType, capabilities, input)
	task.EscrowType = "milestone"
	task.Milestones = milestones
	task.CurrentMilestone = 0
	task.Budget = budget

	// Create milestone escrow on blockchain if escrow client is available
	if o.escrowClient != nil {
		taskIDBytes, err := o.convertStringToTaskID(task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert task ID: %w", err)
		}

		// Create initial escrow
		err = o.escrowClient.CreateEscrow(ctx, taskIDBytes, uint64(budget*1e8), [32]byte{}, nil)
		if err != nil {
			o.logger.Error("failed to create milestone escrow on blockchain", zap.Error(err))
			return nil, fmt.Errorf("failed to create escrow: %w", err)
		}

		task.EscrowTxHash = fmt.Sprintf("0x%x", taskIDBytes)
		task.PaymentStatus = PaymentStatusCreated

		// Add milestones to blockchain
		// Note: Milestone addition is implemented as separate transactions after escrow creation
		for _, milestone := range milestones {
			milestoneDesc := milestone.Description
			milestoneAmt := uint64(milestone.Amount * 1e8)
			milestoneApprovals := uint32(milestone.RequiredApprovals)

			err := o.escrowClient.AddMilestone(ctx, taskIDBytes, milestoneDesc, milestoneAmt, milestoneApprovals)
			if err != nil {
				o.logger.Warn("failed to add milestone to blockchain",
					zap.String("milestone_id", milestone.ID),
					zap.Error(err),
				)
			}
		}

		o.logger.Info("milestone escrow created",
			zap.String("task_id", task.ID),
			zap.String("tx_hash", task.EscrowTxHash),
			zap.Int("milestones_count", len(milestones)),
		)
	}

	// Enqueue task
	if err := o.queue.Enqueue(task); err != nil {
		return nil, fmt.Errorf("failed to enqueue milestone task: %w", err)
	}

	return task, nil
}

// CreateTaskFromTemplate creates a task using a predefined template
func (o *Orchestrator) CreateTaskFromTemplate(
	ctx context.Context,
	userID string,
	templateID string,
	input map[string]interface{},
	budget float64,
) (*Task, error) {
	o.logger.Info("creating task from template",
		zap.String("user_id", userID),
		zap.String("template_id", templateID),
		zap.Float64("budget", budget),
	)

	// TODO: Template functionality not yet implemented in escrow client
	// For now, create a simple task
	o.logger.Warn("template functionality not yet implemented, creating simple task",
		zap.String("template_id", templateID),
	)

	task := NewTask(userID, "template_task", nil, input)
	task.Budget = budget
	task.TemplateID = templateID
	task.EscrowType = "simple"

	// Create basic escrow
	if o.escrowClient != nil {
		taskIDBytes, err := o.convertStringToTaskID(task.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to convert task ID: %w", err)
		}

		err = o.escrowClient.CreateEscrow(ctx, taskIDBytes, uint64(budget*1e8), [32]byte{}, nil)
		if err != nil {
			o.logger.Error("failed to create escrow from template", zap.Error(err))
			return nil, fmt.Errorf("failed to create escrow: %w", err)
		}

		task.EscrowTxHash = fmt.Sprintf("0x%x", taskIDBytes)
		task.PaymentStatus = PaymentStatusCreated
	}

	// Enqueue task
	if err := o.queue.Enqueue(task); err != nil {
		return nil, fmt.Errorf("failed to enqueue template task: %w", err)
	}

	o.logger.Info("task created from template",
		zap.String("task_id", task.ID),
		zap.String("template_id", templateID),
		zap.String("tx_hash", task.EscrowTxHash),
	)

	return task, nil
}

// CreateBatchTasks creates multiple tasks in a batch operation
func (o *Orchestrator) CreateBatchTasks(
	ctx context.Context,
	userID string,
	tasks []BatchTaskRequest,
) (*BatchTaskResult, error) {
	batchID := uuid.New().String()

	o.logger.Info("creating batch tasks",
		zap.String("user_id", userID),
		zap.String("batch_id", batchID),
		zap.Int("task_count", len(tasks)),
	)

	if len(tasks) == 0 {
		return nil, fmt.Errorf("no tasks provided for batch")
	}

	result := &BatchTaskResult{
		BatchID:         batchID,
		SuccessfulTasks: make([]*Task, 0),
		FailedTasks:     make([]BatchTaskError, 0),
		TotalRequested:  len(tasks),
	}

	// Create escrow batch requests for blockchain
	var escrowRequests []substrate.BatchCreateEscrowRequest
	var createdTasks []*Task

	for i, taskReq := range tasks {
		// Create task
		task := NewTask(userID, taskReq.Type, taskReq.Capabilities, taskReq.Input)
		task.Budget = taskReq.Budget
		task.BatchID = batchID
		task.IsBatchTask = true
		task.EscrowType = "simple"

		createdTasks = append(createdTasks, task)

		// Add to escrow batch if escrow client is available
		if o.escrowClient != nil {
			taskIDBytes, err := o.convertStringToTaskID(task.ID)
			if err != nil {
				result.FailedTasks = append(result.FailedTasks, BatchTaskError{
					Index:  i,
					TaskID: task.ID,
					Error:  fmt.Sprintf("failed to convert task ID: %v", err),
				})
				continue
			}

			escrowRequests = append(escrowRequests, substrate.BatchCreateEscrowRequest{
				TaskID:        taskIDBytes,
				Amount:        uint64(taskReq.Budget * 1e8),
				TaskHash:      [32]byte{}, // Empty for now
				TimeoutBlocks: nil,        // No timeout
			})
		}
	}

	// Create batch escrow on blockchain if available
	if o.escrowClient != nil && len(escrowRequests) > 0 {
		batchResult, err := o.escrowClient.BatchCreateEscrow(ctx, escrowRequests)
		if err != nil {
			o.logger.Error("batch escrow creation failed", zap.Error(err))
			// Continue with individual task creation even if batch escrow fails
		} else {
			result.TransactionHash = fmt.Sprintf("0x%x", batchResult.TransactionHash)
			o.logger.Info("batch escrow created",
				zap.String("batch_id", batchID),
				zap.String("tx_hash", result.TransactionHash),
				zap.Uint32("succeeded", batchResult.TotalSucceeded),
			)
		}
	}

	// Enqueue all tasks
	for i, task := range createdTasks {
		if o.escrowClient != nil {
			task.PaymentStatus = PaymentStatusCreated
		}

		if err := o.queue.Enqueue(task); err != nil {
			result.FailedTasks = append(result.FailedTasks, BatchTaskError{
				Index:  i,
				TaskID: task.ID,
				Error:  fmt.Sprintf("failed to enqueue task: %v", err),
			})
		} else {
			result.SuccessfulTasks = append(result.SuccessfulTasks, task)
		}
	}

	result.TotalSucceeded = len(result.SuccessfulTasks)
	result.TotalFailed = len(result.FailedTasks)

	o.logger.Info("batch task creation completed",
		zap.String("batch_id", batchID),
		zap.Int("succeeded", result.TotalSucceeded),
		zap.Int("failed", result.TotalFailed),
	)

	return result, nil
}

// ApproveMilestone approves a milestone for a task
func (o *Orchestrator) ApproveMilestone(
	ctx context.Context,
	taskID string,
	milestoneIndex int,
	approverDID string,
	evidence string,
) error {
	o.logger.Info("approving milestone",
		zap.String("task_id", taskID),
		zap.Int("milestone_index", milestoneIndex),
		zap.String("approver_did", approverDID),
	)

	// Get task
	// Note: GetTask method needs to be implemented in TaskQueue
	// For now, use a placeholder
	task := &Task{ID: taskID, EscrowType: "milestone"}
	var err error
	// TODO: Implement task retrieval from queue
	_ = err // Suppress unused variable warning for now

	if task.EscrowType != "milestone" {
		return fmt.Errorf("task is not a milestone task")
	}

	if milestoneIndex >= len(task.Milestones) {
		return fmt.Errorf("milestone index out of range")
	}

	milestone := &task.Milestones[milestoneIndex]

	// Check if already approved by this DID
	for _, approval := range milestone.Approvals {
		if approval.ApproverDID == approverDID {
			return fmt.Errorf("milestone already approved by %s", approverDID)
		}
	}

	// Add approval
	approval := MilestoneApproval{
		ApproverDID: approverDID,
		ApprovedAt:  time.Now(),
		Evidence:    evidence,
	}
	milestone.Approvals = append(milestone.Approvals, approval)

	// Check if milestone is fully approved
	if len(milestone.Approvals) >= milestone.RequiredApprovals {
		milestone.Status = "approved"
		now := time.Now()
		milestone.ApprovedAt = &now

		// Approve milestone on blockchain
		if o.escrowClient != nil {
			taskIDBytes, err := o.convertStringToTaskID(taskID)
			if err != nil {
				return fmt.Errorf("failed to convert task ID: %w", err)
			}

			err = o.escrowClient.ApproveMilestone(ctx, taskIDBytes, uint32(milestoneIndex))
			if err != nil {
				o.logger.Warn("failed to approve milestone on blockchain",
					zap.String("task_id", taskID),
					zap.Error(err),
				)
			}
		}

		o.logger.Info("milestone fully approved",
			zap.String("task_id", taskID),
			zap.Int("milestone_index", milestoneIndex),
			zap.String("milestone_name", milestone.Name),
		)
	}

	// Update task
	if err := o.queue.Update(task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	return nil
}

// convertStringToTaskID converts a string ID to a 32-byte task ID
func (o *Orchestrator) convertStringToTaskID(id string) ([32]byte, error) {
	var taskID [32]byte

	// Try to parse as hex first
	if strings.HasPrefix(id, "0x") {
		bytes, err := hex.DecodeString(id[2:])
		if err == nil && len(bytes) <= 32 {
			copy(taskID[:], bytes)
			return taskID, nil
		}
	}

	// Fall back to using the string bytes directly
	idBytes := []byte(id)
	if len(idBytes) > 32 {
		// Use first 32 bytes
		copy(taskID[:], idBytes[:32])
	} else {
		// Pad with zeros
		copy(taskID[:], idBytes)
	}

	return taskID, nil
}

// Support types for batch operations
type BatchTaskRequest struct {
	Type         string                 `json:"type"`
	Capabilities []string               `json:"capabilities"`
	Input        map[string]interface{} `json:"input"`
	Budget       float64                `json:"budget"`
}

type BatchTaskResult struct {
	BatchID         string           `json:"batch_id"`
	SuccessfulTasks []*Task          `json:"successful_tasks"`
	FailedTasks     []BatchTaskError `json:"failed_tasks"`
	TotalRequested  int              `json:"total_requested"`
	TotalSucceeded  int              `json:"total_succeeded"`
	TotalFailed     int              `json:"total_failed"`
	TransactionHash string           `json:"transaction_hash,omitempty"`
}

type BatchTaskError struct {
	Index  int    `json:"index"`
	TaskID string `json:"task_id"`
	Error  string `json:"error"`
}

// BlockchainAdapter adapts substrate.BlockchainService to BlockchainInterface
type BlockchainAdapter struct {
	blockchain *substrate.BlockchainService
	logger     *zap.Logger
}

// NewBlockchainAdapter creates a new blockchain adapter
func NewBlockchainAdapter(blockchain *substrate.BlockchainService) *BlockchainAdapter {
	return &BlockchainAdapter{
		blockchain: blockchain,
		logger:     zap.NewNop(),
	}
}

// ReleasePayment releases payment to agent on successful task completion
func (ba *BlockchainAdapter) ReleasePayment(ctx context.Context, taskID string) (txHash string, err error) {
	if ba.blockchain == nil {
		return "", ErrBlockchainUnavailable
	}

	// Convert task ID to [32]byte as expected by the blockchain
	var taskIDBytes [32]byte
	copy(taskIDBytes[:], []byte(taskID))

	// Call the blockchain service to release payment
	// This is a placeholder - the actual implementation depends on the substrate service interface
	ba.logger.Info("releasing payment on blockchain",
		zap.String("task_id", taskID),
	)

	// TODO: Implement actual blockchain call when substrate interface is available
	// For now, return a mock transaction hash
	return fmt.Sprintf("0x%x", taskIDBytes[:8]), nil
}

// RefundEscrow refunds payment on task failure or timeout
func (ba *BlockchainAdapter) RefundEscrow(ctx context.Context, taskID string) (txHash string, err error) {
	if ba.blockchain == nil {
		return "", ErrBlockchainUnavailable
	}

	// Convert task ID to [32]byte as expected by the blockchain
	var taskIDBytes [32]byte
	copy(taskIDBytes[:], []byte(taskID))

	ba.logger.Info("refunding escrow on blockchain",
		zap.String("task_id", taskID),
	)

	// TODO: Implement actual blockchain call when substrate interface is available
	// For now, return a mock transaction hash
	return fmt.Sprintf("0x%x", taskIDBytes[:8]), nil
}

// DisputeEscrow initiates a payment dispute
func (ba *BlockchainAdapter) DisputeEscrow(ctx context.Context, taskID string, reason string) (txHash string, err error) {
	if ba.blockchain == nil {
		return "", ErrBlockchainUnavailable
	}

	// Convert task ID to [32]byte as expected by the blockchain
	var taskIDBytes [32]byte
	copy(taskIDBytes[:], []byte(taskID))

	ba.logger.Info("disputing escrow on blockchain",
		zap.String("task_id", taskID),
		zap.String("reason", reason),
	)

	// TODO: Implement actual blockchain call when substrate interface is available
	// For now, return a mock transaction hash
	return fmt.Sprintf("0x%x", taskIDBytes[:8]), nil
}

// IsEnabled checks if blockchain is available
func (ba *BlockchainAdapter) IsEnabled() bool {
	return ba.blockchain != nil && ba.blockchain.IsEnabled()
}

// GetEscrowStatus checks escrow status
func (ba *BlockchainAdapter) GetEscrowStatus(ctx context.Context, taskID string) (PaymentStatus, error) {
	if ba.blockchain == nil {
		return PaymentStatusFailure, ErrBlockchainUnavailable
	}

	// TODO: Implement actual blockchain status check
	// For now, return created status
	return PaymentStatusCreated, nil
}
