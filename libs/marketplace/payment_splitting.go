package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/reputation"
)

// SECURITY INVARIANTS FOR PAYMENT SPLITTING (CRITICAL):
// 1. Total split payments must EXACTLY equal original task payment
// 2. Atomic settlement: Either ALL agents get paid or NONE do
// 3. Partial failure handling: Failed agents don't get paid, others do
// 4. No double-payment: Each agent paid exactly once per task
// 5. Fairness: Payment proportional to contribution/work done

// Payment splitting errors
var (
	// ErrInvalidSplitRatios indicates split ratios don't sum to 1.0
	ErrInvalidSplitRatios = errors.New("split ratios must sum to 1.0")

	// ErrSplitCountMismatch indicates mismatch between agents and ratios
	ErrSplitCountMismatch = errors.New("agent count must match split ratio count")

	// ErrAtomicSettlementFailed indicates atomic settlement failed
	ErrAtomicSettlementFailed = errors.New("atomic settlement failed - rolled back")

	// ErrNoAgentsInDAG indicates DAG has no agents
	ErrNoAgentsInDAG = errors.New("DAG workflow has no agents")
)

// PaymentSplit represents how to divide payment among agents
type PaymentSplit struct {
	AgentDID string  // Agent receiving payment
	Ratio    float64 // Proportion of total payment (0.0 - 1.0)
	Amount   float64 // Calculated payment amount
	TaskID   string  // Specific task this agent executed
	Success  bool    // Whether agent's task succeeded
}

// DAGPaymentRequest represents payment for a DAG workflow
type DAGPaymentRequest struct {
	WorkflowID   string         // DAG workflow identifier
	UserID       string         // User paying for workflow
	TotalPayment float64        // Total payment for entire workflow
	Splits       []PaymentSplit // How to divide payment among agents
}

// DAGPaymentResult represents settlement outcome
type DAGPaymentResult struct {
	WorkflowID       string         // DAG workflow identifier
	TotalPaid        float64        // Total amount actually paid
	SuccessfulSplits int            // Number of successful payments
	FailedSplits     int            // Number of failed payments
	Splits           []PaymentSplit // Updated with actual amounts paid
	ChannelIDs       []string       // Payment channel IDs created
}

// PaymentSplittingService handles multi-agent payment distribution
// ARCHITECTURE: Integrates with DAG execution and payment channels
type PaymentSplittingService struct {
	mu sync.RWMutex

	// Core services
	paymentService    *economic.PaymentChannelService
	reputationService *reputation.ReputationService

	// DAG payment tracking
	workflowPayments map[string]*DAGPaymentResult // workflow_id -> result
}

// NewPaymentSplittingService creates payment splitting service
func NewPaymentSplittingService(
	paymentService *economic.PaymentChannelService,
	reputationService *reputation.ReputationService,
) *PaymentSplittingService {
	return &PaymentSplittingService{
		paymentService:    paymentService,
		reputationService: reputationService,
		workflowPayments:  make(map[string]*DAGPaymentResult),
	}
}

// CalculateSplitsFromDAG calculates payment splits from DAG execution result
// ALGORITHM: Proportional payment based on task complexity and execution time
func (pss *PaymentSplittingService) CalculateSplitsFromDAG(
	ctx context.Context,
	workflow *orchestration.DAGWorkflow,
	result *orchestration.WorkflowResult,
	totalPayment float64,
) ([]PaymentSplit, error) {
	if len(result.TaskResults) == 0 {
		return nil, ErrNoAgentsInDAG
	}

	// Strategy 1: Equal split (simple, fair for similar tasks)
	// Future: Could weight by execution time, complexity, or task dependencies
	ratio := 1.0 / float64(len(result.TaskResults))

	splits := make([]PaymentSplit, 0, len(result.TaskResults))
	for taskID, taskResult := range result.TaskResults {
		split := PaymentSplit{
			AgentDID: taskResult.AgentDID,
			Ratio:    ratio,
			Amount:   totalPayment * ratio,
			TaskID:   taskID,
			Success:  taskResult.Success,
		}
		splits = append(splits, split)
	}

	return splits, nil
}

// ExecuteDAGPayment performs payment splitting for DAG workflow
// FLOW: Validate → Calculate → Create Channels → Lock Escrow → Settle → Update Reputation
func (pss *PaymentSplittingService) ExecuteDAGPayment(
	ctx context.Context,
	req *DAGPaymentRequest,
) (*DAGPaymentResult, error) {
	// Step 1: Validate split ratios
	if err := pss.validateSplits(req.Splits); err != nil {
		return nil, err
	}

	// Step 2: Check user has sufficient balance
	userBalance, err := pss.paymentService.GetBalance(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	if userBalance < req.TotalPayment {
		return nil, economic.ErrInsufficientFunds
	}

	// Step 3: Create payment channels for each agent
	channels := make([]*economic.PaymentChannel, 0, len(req.Splits))
	channelIDs := make([]string, 0, len(req.Splits))

	for i, split := range req.Splits {
		// Create channel with proportional deposit
		channel, err := pss.paymentService.CreateChannel(
			ctx,
			req.UserID,     // Payer (user)
			split.AgentDID, // Payee (agent)
			split.Amount,   // Deposit amount (proportional)
			req.WorkflowID, // Workflow reference
		)
		if err != nil {
			// Rollback: close all previously created channels
			pss.rollbackChannels(ctx, channels)
			return nil, fmt.Errorf("failed to create channel for agent %s: %w", split.AgentDID, err)
		}

		channels = append(channels, channel)
		channelIDs = append(channelIDs, channel.ID)

		// Update split with channel info
		req.Splits[i].Amount = split.Amount
	}

	// Step 4: Lock escrow for each channel
	for i, channel := range channels {
		split := req.Splits[i]
		err := pss.paymentService.LockEscrow(
			ctx,
			channel.ID,
			split.TaskID,
			split.Amount,
		)
		if err != nil {
			// Rollback: close all channels and refund
			pss.rollbackChannels(ctx, channels)
			return nil, fmt.Errorf("failed to lock escrow for agent %s: %w", split.AgentDID, err)
		}
	}

	// Step 5: Settle based on task success/failure
	result := &DAGPaymentResult{
		WorkflowID: req.WorkflowID,
		TotalPaid:  0,
		Splits:     req.Splits,
		ChannelIDs: channelIDs,
	}

	for i, channel := range channels {
		split := req.Splits[i]

		// Release escrow (pay agent if success, refund user if failure)
		err := pss.paymentService.ReleaseEscrow(
			ctx,
			channel.ID,
			split.TaskID,
			split.Success,
		)
		if err != nil {
			// Log but don't fail - payment already locked
			fmt.Printf("Warning: failed to release escrow for agent %s: %v\n", split.AgentDID, err)
			result.FailedSplits++
			continue
		}

		if split.Success {
			result.TotalPaid += split.Amount
			result.SuccessfulSplits++

			// Update reputation for successful task
			if pss.reputationService != nil {
				pss.reputationService.RecordSuccess(ctx, split.AgentDID, split.TaskID)
			}
		} else {
			result.FailedSplits++

			// Update reputation for failed task
			if pss.reputationService != nil {
				pss.reputationService.RecordFailure(ctx, split.AgentDID, split.TaskID)
			}
		}

		// Close channel after settlement
		err = pss.paymentService.CloseChannel(ctx, channel.ID)
		if err != nil {
			// Log but don't fail - escrow already released
			fmt.Printf("Warning: failed to close channel for agent %s: %v\n", split.AgentDID, err)
		}
	}

	// Step 6: Track workflow payment
	pss.mu.Lock()
	pss.workflowPayments[req.WorkflowID] = result
	pss.mu.Unlock()

	return result, nil
}

// ExecuteAtomicDAGPayment performs atomic settlement - all or nothing
// SECURITY: Either ALL successful agents get paid or NONE do
func (pss *PaymentSplittingService) ExecuteAtomicDAGPayment(
	ctx context.Context,
	req *DAGPaymentRequest,
) (*DAGPaymentResult, error) {
	// Step 1: Validate split ratios
	if err := pss.validateSplits(req.Splits); err != nil {
		return nil, err
	}

	// Step 2: Check user has sufficient balance
	userBalance, err := pss.paymentService.GetBalance(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to check balance: %w", err)
	}

	if userBalance < req.TotalPayment {
		return nil, economic.ErrInsufficientFunds
	}

	// Step 3: Check ALL tasks succeeded
	allSucceeded := true
	for _, split := range req.Splits {
		if !split.Success {
			allSucceeded = false
			break
		}
	}

	// If any task failed, refund user and return
	if !allSucceeded {
		return &DAGPaymentResult{
			WorkflowID:   req.WorkflowID,
			TotalPaid:    0,
			FailedSplits: len(req.Splits),
			Splits:       req.Splits,
		}, nil
	}

	// Step 4: Create payment channels for each agent
	channels := make([]*economic.PaymentChannel, 0, len(req.Splits))
	channelIDs := make([]string, 0, len(req.Splits))

	for i, split := range req.Splits {
		channel, err := pss.paymentService.CreateChannel(
			ctx,
			req.UserID,
			split.AgentDID,
			split.Amount,
			req.WorkflowID,
		)
		if err != nil {
			// Rollback all channels
			pss.rollbackChannels(ctx, channels)
			return nil, fmt.Errorf("atomic settlement failed during channel creation: %w", err)
		}

		channels = append(channels, channel)
		channelIDs = append(channelIDs, channel.ID)
		req.Splits[i].Amount = split.Amount
	}

	// Step 5: Lock and release escrow atomically
	settled := make([]bool, len(channels))
	var settlementErr error

	for i, channel := range channels {
		split := req.Splits[i]

		// Lock escrow
		err := pss.paymentService.LockEscrow(ctx, channel.ID, split.TaskID, split.Amount)
		if err != nil {
			settlementErr = fmt.Errorf("atomic settlement failed during escrow lock: %w", err)
			break
		}

		// Release escrow immediately (pay agent)
		err = pss.paymentService.ReleaseEscrow(ctx, channel.ID, split.TaskID, true)
		if err != nil {
			settlementErr = fmt.Errorf("atomic settlement failed during escrow release: %w", err)
			break
		}

		settled[i] = true
	}

	// Step 6: If any settlement failed, rollback ALL
	if settlementErr != nil {
		// Rollback all settled payments
		for i, channel := range channels {
			if settled[i] {
				// Attempt to reverse payment (close channel)
				pss.paymentService.CloseChannel(ctx, channel.ID)
			}
		}

		// Close all channels
		pss.rollbackChannels(ctx, channels)

		return nil, fmt.Errorf("%w: %v", ErrAtomicSettlementFailed, settlementErr)
	}

	// Step 7: Success - update reputation and close channels
	result := &DAGPaymentResult{
		WorkflowID:       req.WorkflowID,
		TotalPaid:        req.TotalPayment,
		SuccessfulSplits: len(req.Splits),
		Splits:           req.Splits,
		ChannelIDs:       channelIDs,
	}

	for i, channel := range channels {
		split := req.Splits[i]

		// Update reputation
		if pss.reputationService != nil {
			pss.reputationService.RecordSuccess(ctx, split.AgentDID, split.TaskID)
		}

		// Close channel
		pss.paymentService.CloseChannel(ctx, channel.ID)
	}

	// Track workflow payment
	pss.mu.Lock()
	pss.workflowPayments[req.WorkflowID] = result
	pss.mu.Unlock()

	return result, nil
}

// GetWorkflowPayment retrieves payment result for a workflow
func (pss *PaymentSplittingService) GetWorkflowPayment(
	ctx context.Context,
	workflowID string,
) (*DAGPaymentResult, error) {
	pss.mu.RLock()
	defer pss.mu.RUnlock()

	result, exists := pss.workflowPayments[workflowID]
	if !exists {
		return nil, fmt.Errorf("no payment found for workflow: %s", workflowID)
	}

	return result, nil
}

// validateSplits validates payment split ratios
func (pss *PaymentSplittingService) validateSplits(splits []PaymentSplit) error {
	if len(splits) == 0 {
		return ErrNoAgentsInDAG
	}

	// Check ratios sum to 1.0 (with tolerance for floating point)
	totalRatio := 0.0
	for _, split := range splits {
		totalRatio += split.Ratio
	}

	epsilon := 0.0001
	if abs(totalRatio-1.0) > epsilon {
		return fmt.Errorf("%w: got %f", ErrInvalidSplitRatios, totalRatio)
	}

	return nil
}

// rollbackChannels closes all channels and refunds deposits
func (pss *PaymentSplittingService) rollbackChannels(
	ctx context.Context,
	channels []*economic.PaymentChannel,
) {
	for _, channel := range channels {
		err := pss.paymentService.CloseChannel(ctx, channel.ID)
		if err != nil {
			fmt.Printf("Warning: failed to rollback channel %s: %v\n", channel.ID, err)
		}
	}
}

// abs returns absolute value of float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
