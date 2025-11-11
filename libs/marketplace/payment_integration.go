package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/reputation"
)

// Payment integration errors
var (
	// ErrPaymentChannelRequired indicates payment channel must be created first
	ErrPaymentChannelRequired = errors.New("payment channel required for task execution")

	// ErrInsufficientFunds indicates user has insufficient balance
	ErrInsufficientFunds = errors.New("insufficient funds for task payment")

	// ErrPaymentFailed indicates payment processing failed
	ErrPaymentFailed = errors.New("payment processing failed")
)

// PaymentMarketplaceService integrates payments with marketplace operations
// SECURITY: All payment operations must be atomic with marketplace state
type PaymentMarketplaceService struct {
	mu sync.RWMutex

	// Core services
	marketplaceService *MarketplaceService
	paymentService     *economic.PaymentChannelService
	reputationService  *reputation.ReputationService

	// Payment channel tracking
	auctionToChannel map[string]string // auction_id -> channel_id
	taskToChannel    map[string]string // task_id -> channel_id
}

// NewPaymentMarketplaceService creates payment-integrated marketplace
func NewPaymentMarketplaceService(
	marketplaceService *MarketplaceService,
	paymentService *economic.PaymentChannelService,
	reputationService *reputation.ReputationService,
) *PaymentMarketplaceService {
	return &PaymentMarketplaceService{
		marketplaceService: marketplaceService,
		paymentService:     paymentService,
		reputationService:  reputationService,
		auctionToChannel:   make(map[string]string),
		taskToChannel:      make(map[string]string),
	}
}

// AllocateTaskWithPayment performs auction + payment channel creation atomically
// FLOW: Auction → Winner → Create Channel → Lock Escrow → Execute Task
func (pms *PaymentMarketplaceService) AllocateTaskWithPayment(
	ctx context.Context,
	req *AuctionRequest,
) (*AllocationResult, string, error) {
	// Step 1: Run auction to find winner
	allocation, err := pms.marketplaceService.AllocateTask(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("auction failed: %w", err)
	}

	// Step 2: Check user has sufficient balance
	userBalance, err := pms.paymentService.GetBalance(ctx, req.UserID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check balance: %w", err)
	}

	if userBalance < allocation.FinalPrice {
		return nil, "", ErrInsufficientFunds
	}

	// Step 3: Create payment channel with winner
	channel, err := pms.paymentService.CreateChannel(
		ctx,
		req.UserID,              // Payer (user)
		allocation.WinnerDID,    // Payee (winning agent)
		allocation.FinalPrice,   // Deposit amount
		allocation.AuctionID,    // Auction reference
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create payment channel: %w", err)
	}

	// Step 4: Lock funds in escrow for task execution
	err = pms.paymentService.LockEscrow(
		ctx,
		channel.ID,
		req.TaskID,
		allocation.FinalPrice,
	)
	if err != nil {
		// Rollback: close channel and refund
		pms.paymentService.CloseChannel(ctx, channel.ID)
		return nil, "", fmt.Errorf("failed to lock escrow: %w", err)
	}

	// Step 5: Track channel associations
	pms.mu.Lock()
	pms.auctionToChannel[allocation.AuctionID] = channel.ID
	pms.taskToChannel[req.TaskID] = channel.ID
	pms.mu.Unlock()

	return allocation, channel.ID, nil
}

// CompleteTaskWithPayment handles task completion and payment settlement
// SECURITY: Must be idempotent (can be called multiple times safely)
func (pms *PaymentMarketplaceService) CompleteTaskWithPayment(
	ctx context.Context,
	taskID string,
	agentDID string,
	success bool,
) error {
	// Get channel for task
	pms.mu.RLock()
	channelID, exists := pms.taskToChannel[taskID]
	pms.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no payment channel found for task: %s", taskID)
	}

	// Release escrow (pay agent if success, refund user if failure)
	err := pms.paymentService.ReleaseEscrow(ctx, channelID, taskID, success)
	if err != nil {
		// If already released, this is fine (idempotency)
		if err == economic.ErrEscrowAlreadyReleased {
			return nil
		}
		return fmt.Errorf("failed to release escrow: %w", err)
	}

	// Update marketplace completion tracking
	err = pms.marketplaceService.HandleTaskCompletion(ctx, agentDID, taskID, success)
	if err != nil {
		// Log but don't fail - payment already processed
		fmt.Printf("Warning: marketplace completion tracking failed: %v\n", err)
	}

	// Update reputation based on payment outcome
	if pms.reputationService != nil {
		if success {
			pms.reputationService.RecordSuccess(ctx, agentDID, taskID)
		} else {
			pms.reputationService.RecordFailure(ctx, agentDID, taskID)
		}
	}

	// Close channel after settlement
	err = pms.paymentService.CloseChannel(ctx, channelID)
	if err != nil {
		// Log but don't fail - escrow already released
		fmt.Printf("Warning: failed to close channel: %v\n", err)
	}

	// Cleanup tracking
	pms.mu.Lock()
	delete(pms.taskToChannel, taskID)
	pms.mu.Unlock()

	return nil
}

// GetChannelForTask retrieves payment channel for a task
func (pms *PaymentMarketplaceService) GetChannelForTask(ctx context.Context, taskID string) (string, error) {
	pms.mu.RLock()
	defer pms.mu.RUnlock()

	channelID, exists := pms.taskToChannel[taskID]
	if !exists {
		return "", fmt.Errorf("no channel found for task: %s", taskID)
	}

	return channelID, nil
}

// GetChannelForAuction retrieves payment channel for an auction
func (pms *PaymentMarketplaceService) GetChannelForAuction(ctx context.Context, auctionID string) (string, error) {
	pms.mu.RLock()
	defer pms.mu.RUnlock()

	channelID, exists := pms.auctionToChannel[auctionID]
	if !exists {
		return "", fmt.Errorf("no channel found for auction: %s", auctionID)
	}

	return channelID, nil
}

// RefundFailedTask refunds user if task execution failed
func (pms *PaymentMarketplaceService) RefundFailedTask(ctx context.Context, taskID string) error {
	return pms.CompleteTaskWithPayment(ctx, taskID, "", false)
}

// VerifyPaymentIntegrity checks payment system integrity
// Should be called periodically to detect bugs
func (pms *PaymentMarketplaceService) VerifyPaymentIntegrity(ctx context.Context) error {
	return pms.paymentService.VerifyBalanceInvariant()
}
