package marketplace

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/aidenlippert/zerostate/libs/reputation"
)

var (
	// ErrNoEligibleAgents indicates no agents meet auction requirements
	ErrNoEligibleAgents = errors.New("no eligible agents found for auction")

	// ErrAuctionNotActive indicates auction cannot accept bids
	ErrAuctionNotActive = errors.New("auction is not active")
)

// MarketplaceService integrates discovery and auction for task allocation
type MarketplaceService struct {
	mu sync.RWMutex

	// Core services
	discoveryService  *DiscoveryService
	auctionService    *AuctionService
	messageBus        p2p.MessageBus
	reputationService *reputation.ReputationService

	// Configuration
	minAgentsForAuction int
	invitationTimeout   time.Duration
	defaultAuctionType  AuctionType

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// AuctionRequest represents a request to allocate a task via auction
type AuctionRequest struct {
	TaskID       string                 `json:"task_id"`
	UserID       string                 `json:"user_id"`
	Capabilities []string               `json:"capabilities"`
	TaskType     string                 `json:"task_type"`
	Input        map[string]interface{} `json:"input"`
	MaxPrice     float64                `json:"max_price"`
	Timeout      time.Duration          `json:"timeout"`

	// Auction configuration
	AuctionType     AuctionType   `json:"auction_type,omitempty"`
	AuctionDuration time.Duration `json:"auction_duration,omitempty"`
	ReservePrice    float64       `json:"reserve_price,omitempty"`
	MinReputation   float64       `json:"min_reputation,omitempty"`

	// Discovery configuration
	PreferredRegions []string `json:"preferred_regions,omitempty"`
	MaxAgents        int      `json:"max_agents,omitempty"`
}

// AllocationResult represents the result of task allocation via auction
type AllocationResult struct {
	AuctionID   string              `json:"auction_id"`
	WinnerDID   string              `json:"winner_did"`
	FinalPrice  float64             `json:"final_price"`
	AgentCard   *identity.AgentCard `json:"agent_card"`
	NumBids     int                 `json:"num_bids"`
	Duration    time.Duration       `json:"duration"`
	StartedAt   time.Time           `json:"started_at"`
	CompletedAt time.Time           `json:"completed_at"`
}

// NewMarketplaceService creates a new integrated marketplace service
func NewMarketplaceService(
	discoveryService *DiscoveryService,
	auctionService *AuctionService,
	messageBus p2p.MessageBus,
	reputationService *reputation.ReputationService,
) *MarketplaceService {
	ctx, cancel := context.WithCancel(context.Background())

	return &MarketplaceService{
		discoveryService:    discoveryService,
		auctionService:      auctionService,
		messageBus:          messageBus,
		reputationService:   reputationService,
		minAgentsForAuction: 3,
		invitationTimeout:   5 * time.Second,
		defaultAuctionType:  AuctionTypeSecondPrice,
		ctx:                 ctx,
		cancel:              cancel,
	}
}

// AllocateTask orchestrates discovery, auction, and task allocation
func (ms *MarketplaceService) AllocateTask(ctx context.Context, req *AuctionRequest) (*AllocationResult, error) {
	startTime := time.Now()

	// Step 1: Discover eligible agents
	eligibleAgents, err := ms.discoverEligibleAgents(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("agent discovery failed: %w", err)
	}

	if len(eligibleAgents) < ms.minAgentsForAuction {
		return nil, fmt.Errorf("%w: found %d agents, need at least %d",
			ErrNoEligibleAgents, len(eligibleAgents), ms.minAgentsForAuction)
	}

	// Step 2: Create auction
	auction, err := ms.createAuctionFromRequest(req, eligibleAgents)
	if err != nil {
		return nil, fmt.Errorf("auction creation failed: %w", err)
	}

	// Step 3: Invite agents to bid
	if err := ms.inviteAgentsToBid(ctx, auction, eligibleAgents); err != nil {
		return nil, fmt.Errorf("agent invitation failed: %w", err)
	}

	// Step 4: Wait for auction to complete
	completedAuction, err := ms.waitForAuctionCompletion(ctx, auction.ID, req.AuctionDuration)
	if err != nil {
		return nil, fmt.Errorf("auction completion failed: %w", err)
	}

	// Step 5: Get winner's agent card
	winnerRecord, exists := ms.discoveryService.index.GetAgent(completedAuction.WinningBid.AgentDID)
	if !exists {
		return nil, fmt.Errorf("winner agent not found: %s", completedAuction.WinningBid.AgentDID)
	}

	// Step 6: Update agent load
	if err := ms.discoveryService.UpdateAgentLoad(ctx, completedAuction.WinningBid.AgentDID, winnerRecord.CurrentLoad+1); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: failed to update agent load: %v\n", err)
	}

	return &AllocationResult{
		AuctionID:   completedAuction.ID,
		WinnerDID:   completedAuction.WinningBid.AgentDID,
		FinalPrice:  completedAuction.FinalPrice,
		AgentCard:   winnerRecord.AgentCard,
		NumBids:     len(completedAuction.Bids),
		Duration:    time.Since(startTime),
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}, nil
}

// discoverEligibleAgents finds agents that match auction requirements
func (ms *MarketplaceService) discoverEligibleAgents(ctx context.Context, req *AuctionRequest) ([]*DiscoveryResult, error) {
	query := &DiscoveryQuery{
		Capabilities:     req.Capabilities,
		MinReputation:    req.MinReputation,
		PreferredRegions: req.PreferredRegions,
		Limit:            req.MaxAgents,
	}

	if query.Limit <= 0 {
		query.Limit = 50 // Default max agents to invite
	}

	return ms.discoveryService.DiscoverAgents(ctx, query)
}

// createAuctionFromRequest creates an auction from allocation request
func (ms *MarketplaceService) createAuctionFromRequest(
	req *AuctionRequest,
	eligibleAgents []*DiscoveryResult,
) (*TaskAuction, error) {
	auctionType := req.AuctionType
	if auctionType == "" {
		auctionType = ms.defaultAuctionType
	}

	duration := req.AuctionDuration
	if duration <= 0 {
		duration = 30 * time.Second // Default auction duration
	}

	auction := &TaskAuction{
		ID:            fmt.Sprintf("auction-%s-%d", req.TaskID, time.Now().Unix()),
		TaskID:        req.TaskID,
		UserID:        req.UserID,
		Type:          auctionType,
		Status:        AuctionStatusOpen,
		Duration:      duration,
		ExpiresAt:     time.Now().Add(duration),
		ReservePrice:  req.ReservePrice,
		MaxPrice:      req.MaxPrice,
		MinReputation: req.MinReputation,
		Capabilities:  req.Capabilities,
		Bids:          make([]*Bid, 0),
	}

	return ms.auctionService.CreateAuction(context.Background(), auction)
}

// inviteAgentsToBid sends bid invitations to eligible agents
func (ms *MarketplaceService) inviteAgentsToBid(
	ctx context.Context,
	auction *TaskAuction,
	eligibleAgents []*DiscoveryResult,
) error {
	// Prepare invitation message
	invitation := &AuctionInvitation{
		AuctionID:    auction.ID,
		TaskID:       auction.TaskID,
		Type:         auction.Type,
		Capabilities: auction.Capabilities,
		MaxPrice:     auction.MaxPrice,
		ReservePrice: auction.ReservePrice,
		Duration:     auction.Duration,
		ExpiresAt:    auction.ExpiresAt,
	}

	// Send invitations in parallel
	var wg sync.WaitGroup
	invitationCtx, cancel := context.WithTimeout(ctx, ms.invitationTimeout)
	defer cancel()

	for _, result := range eligibleAgents {
		wg.Add(1)
		go func(agentDID string) {
			defer wg.Done()
			if err := ms.sendAuctionInvitation(invitationCtx, agentDID, invitation); err != nil {
				// Log but don't fail - some agents may be unreachable
				fmt.Printf("Warning: failed to invite agent %s: %v\n", agentDID, err)
			}
		}(result.Record.AgentCard.DID)
	}

	wg.Wait()
	return nil
}

// sendAuctionInvitation sends an invitation to a specific agent
func (ms *MarketplaceService) sendAuctionInvitation(
	ctx context.Context,
	agentDID string,
	invitation *AuctionInvitation,
) error {
	// Broadcast invitation via P2P
	msg := &p2p.TaskRequest{
		TaskID: invitation.AuctionID,
		Type:   "auction-invitation",
		Input:  invitation,
	}

	_, err := ms.messageBus.SendRequest(ctx, agentDID, msg, ms.invitationTimeout)
	return err
}

// waitForAuctionCompletion waits for auction to close and return results
func (ms *MarketplaceService) waitForAuctionCompletion(
	ctx context.Context,
	auctionID string,
	duration time.Duration,
) (*TaskAuction, error) {
	// Wait for auction duration
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration + 5*time.Second) // Add 5s buffer

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, errors.New("auction timeout")
		case <-ticker.C:
			auction, exists := ms.auctionService.GetAuction(auctionID)
			if !exists {
				return nil, fmt.Errorf("auction %s not found", auctionID)
			}

			if auction.Status == AuctionStatusAwarded {
				return auction, nil
			}

			if auction.Status == AuctionStatusCanceled || auction.Status == AuctionStatusExpired {
				return nil, fmt.Errorf("auction %s failed with status: %s", auctionID, auction.Status)
			}
		}
	}
}

// SubmitBidForAgent submits a bid on behalf of an agent (called by agents)
func (ms *MarketplaceService) SubmitBidForAgent(
	ctx context.Context,
	auctionID string,
	agentDID string,
	price float64,
	estimatedTime time.Duration,
) error {
	// Get agent record for reputation and quality scores
	record, exists := ms.discoveryService.index.GetAgent(agentDID)
	if !exists {
		return ErrAgentNotFound
	}

	// Create bid
	bid := &Bid{
		ID:              fmt.Sprintf("bid-%s-%d", agentDID, time.Now().Unix()),
		AuctionID:       auctionID,
		AgentDID:        agentDID,
		Price:           price,
		EstimatedTime:   estimatedTime,
		ReputationScore: record.ReputationScore,
		QualityScore:    record.QualityScore,
	}

	return ms.auctionService.SubmitBid(ctx, bid)
}

// HandleTaskCompletion updates agent state after task completion
func (ms *MarketplaceService) HandleTaskCompletion(
	ctx context.Context,
	agentDID string,
	taskID string,
	success bool,
) error {
	record, exists := ms.discoveryService.index.GetAgent(agentDID)
	if !exists {
		return ErrAgentNotFound
	}

	// Update agent load
	if record.CurrentLoad > 0 {
		if err := ms.discoveryService.UpdateAgentLoad(ctx, agentDID, record.CurrentLoad-1); err != nil {
			return fmt.Errorf("failed to update agent load: %w", err)
		}
	}

	// Update reputation if reputation service is available
	if ms.reputationService != nil && success {
		// Reward successful completion
		if err := ms.reputationService.RecordSuccess(ctx, agentDID, taskID); err != nil {
			// Log but don't fail
			fmt.Printf("Warning: failed to record reputation success: %v\n", err)
		}
	}

	return nil
}

// Close shuts down the marketplace service
func (ms *MarketplaceService) Close() error {
	ms.cancel()
	ms.wg.Wait()
	return nil
}

// AuctionInvitation represents an invitation to bid on an auction
type AuctionInvitation struct {
	AuctionID    string        `json:"auction_id"`
	TaskID       string        `json:"task_id"`
	Type         AuctionType   `json:"type"`
	Capabilities []string      `json:"capabilities"`
	MaxPrice     float64       `json:"max_price"`
	ReservePrice float64       `json:"reserve_price,omitempty"`
	Duration     time.Duration `json:"duration"`
	ExpiresAt    time.Time     `json:"expires_at"`
}

// MarketplaceOrchestrator integrates marketplace with task execution
type MarketplaceOrchestrator struct {
	marketplace *MarketplaceService
	messageBus  p2p.MessageBus
}

// NewMarketplaceOrchestrator creates a new orchestrator
func NewMarketplaceOrchestrator(
	marketplace *MarketplaceService,
	messageBus p2p.MessageBus,
) *MarketplaceOrchestrator {
	return &MarketplaceOrchestrator{
		marketplace: marketplace,
		messageBus:  messageBus,
	}
}

// ExecuteTask allocates and executes a task via marketplace
func (mo *MarketplaceOrchestrator) ExecuteTask(
	ctx context.Context,
	req *AuctionRequest,
) (*orchestration.TaskResult, error) {
	// Allocate task via auction
	allocation, err := mo.marketplace.AllocateTask(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("task allocation failed: %w", err)
	}

	// Execute task with winning agent
	taskReq := &p2p.TaskRequest{
		TaskID: req.TaskID,
		Type:   req.TaskType,
		Input:  req.Input,
	}

	response, err := mo.messageBus.SendRequest(ctx, allocation.WinnerDID, taskReq, req.Timeout)
	if err != nil {
		// Task execution failed - update agent state
		mo.marketplace.HandleTaskCompletion(ctx, allocation.WinnerDID, req.TaskID, false)
		return nil, fmt.Errorf("task execution failed: %w", err)
	}

	// Task execution succeeded
	mo.marketplace.HandleTaskCompletion(ctx, allocation.WinnerDID, req.TaskID, true)

	return &orchestration.TaskResult{
		TaskID:        req.TaskID,
		Status:        orchestration.TaskStatusCompleted,
		Output:        response.Output,
		AgentDID:      allocation.WinnerDID,
		StartedAt:     allocation.StartedAt,
		CompletedAt:   allocation.CompletedAt,
		ExecutionTime: allocation.Duration,
	}, nil
}
