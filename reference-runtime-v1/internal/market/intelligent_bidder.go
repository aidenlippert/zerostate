package market

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"go.uber.org/zap"
)

// IntelligentBidder combines RL-based pricing with P2P communication
// This is the production-ready bidder that:
// 1. Listens for CFPs via P2P MessageBus
// 2. Uses RLBidder for intelligent pricing
// 3. Submits bids via AACL protocol
// 4. Learns from outcomes (win/loss, completion/failure)
type IntelligentBidder struct {
	// P2P communication (optional in reference implementation)
	bus       MessageBus
	agentCard *agentcard.AgentCard

	// RL pricing engine
	rlBidder *RLBidder

	// State tracking
	activeCFPs map[string]*CFPContext // CFP ID → context
	activeBids map[string]*BidContext // Bid ID → context

	// Logger
	logger *zap.Logger
}

// CFPContext tracks context for a specific CFP
type CFPContext struct {
	CFP          *CFP
	ReceivedAt   time.Time
	BidSubmitted bool
	BidPrice     float64
	BidID        string
}

// BidContext tracks context for a submitted bid
type BidContext struct {
	CFP         *CFP
	BidPrice    float64
	SubmittedAt time.Time
	Won         bool
	TaskResult  *TaskOutcome
	BidID       string
}

// NewIntelligentBidder creates an intelligent bidder with RL pricing
func NewIntelligentBidder(
	bus MessageBus,
	card *agentcard.AgentCard,
	logger *zap.Logger,
) *IntelligentBidder {
	if logger == nil {
		logger = zap.NewNop()
	}

	// Extract capabilities from agent card
	capabilities := extractCapabilities(card)

	// Create RL bidder
	rlBidder := NewRLBidder(
		agentDID(card),
		capabilities,
		logger.With(zap.String("component", "rl-bidder")),
	)

	return &IntelligentBidder{
		bus:        bus,
		agentCard:  card,
		rlBidder:   rlBidder,
		activeCFPs: make(map[string]*CFPContext),
		activeBids: make(map[string]*BidContext),
		logger:     logger,
	}
}

// Start subscribes to CFP topics and begins bidding
func (ib *IntelligentBidder) Start(ctx context.Context) error {
	if ib.agentCard == nil {
		ib.logger.Warn("IntelligentBidder.Start called with nil agentCard")
		return fmt.Errorf("agent card is nil")
	}

	// Subscribe to CFP topics for each capability
	if ib.bus == nil {
		ib.logger.Warn("no message bus available - skipping CFP subscriptions")
		return nil
	}

	capabilities := extractCapabilities(ib.agentCard)

	for _, capability := range capabilities {
		topic := fmt.Sprintf("ainur/v1/market/cfp/%s", capability)

		// Subscribe to CFP topic
		err := ib.bus.Subscribe(topic, func(ctx context.Context, msg *Message) error {
			return ib.handleCFP(msg)
		})
		if err != nil {
			ib.logger.Error("failed to subscribe to CFP topic",
				zap.String("topic", topic),
				zap.Error(err),
			)
			continue
		}

		ib.logger.Info("subscribed to CFP topic",
			zap.String("topic", topic),
			zap.String("capability", capability),
		)
	}

	// Subscribe to bid acceptance topic
	acceptTopic := fmt.Sprintf("ainur/v1/market/accept/%s", agentDID(ib.agentCard))
	err := ib.bus.Subscribe(acceptTopic, func(ctx context.Context, msg *Message) error {
		return ib.handleBidAcceptance(msg)
	})
	if err != nil {
		ib.logger.Error("failed to subscribe to acceptance topic",
			zap.String("topic", acceptTopic),
			zap.Error(err),
		)
	}

	ib.logger.Info("IntelligentBidder started",
		zap.String("did", agentDID(ib.agentCard)),
		zap.Int("capabilities", len(capabilities)),
	)

	return nil
}

// handleCFP processes incoming CFP messages
func (ib *IntelligentBidder) handleCFP(msg *Message) error {
	// Parse CFP from message
	var cfp CFP
	err := json.Unmarshal(msg.Data, &cfp)
	if err != nil {
		ib.logger.Error("failed to parse CFP",
			zap.Error(err),
		)
		return err
	}

	ib.logger.Info("received CFP",
		zap.String("cfp_id", cfp.ID),
		zap.String("capability", cfp.Capability),
		zap.Float64("budget", cfp.Budget),
		zap.Int64("deadline", cfp.Deadline),
	)

	// Store CFP context
	ib.activeCFPs[cfp.ID] = &CFPContext{
		CFP:        &cfp,
		ReceivedAt: time.Now(),
	}

	// Decide whether to bid (async to avoid blocking message handler)
	go ib.decideBid(context.Background(), &cfp)

	return nil
}

// decideBid uses RL pricing to decide bid price and submit bid
func (ib *IntelligentBidder) decideBid(ctx context.Context, cfp *CFP) {
	// Check if we have the required capability
	if !ib.hasCapability(cfp.Capability) {
		ib.logger.Debug("skipping CFP - missing capability",
			zap.String("cfp_id", cfp.ID),
			zap.String("capability", cfp.Capability),
		)
		return
	}

	// Use RL bidder to select price
	bidPrice, err := ib.rlBidder.SelectBidPrice(ctx, cfp)
	if err != nil {
		ib.logger.Error("failed to select bid price",
			zap.String("cfp_id", cfp.ID),
			zap.Error(err),
		)
		return
	}

	// Create AACL bid message
	bid := &BidMessage{
		BidID:     generateBidID(),
		CFPID:     cfp.ID,
		AgentDID:  agentDID(ib.agentCard),
		Price:     bidPrice,
		Deadline:  cfp.Deadline, // Match CFP deadline
		Timestamp: time.Now().Unix(),
	}

	// Submit bid via P2P
	if ib.bus != nil {
		bidTopic := fmt.Sprintf("ainur/v1/market/bid/%s", cfp.ID)
		bidData, _ := json.Marshal(bid)

		err = ib.bus.Publish(bidTopic, bidData)
		if err != nil {
			ib.logger.Error("failed to publish bid",
				zap.String("cfp_id", cfp.ID),
				zap.Error(err),
			)
			return
		}
	}

	// Update CFP context
	if ctx, exists := ib.activeCFPs[cfp.ID]; exists {
		ctx.BidSubmitted = true
		ctx.BidPrice = bidPrice
		ctx.BidID = bid.BidID
	}

	// Store bid context
	ib.activeBids[bid.BidID] = &BidContext{
		CFP:         cfp,
		BidPrice:    bidPrice,
		SubmittedAt: time.Now(),
		Won:         false,
		BidID:       bid.BidID,
	}

	ib.logger.Info("submitted bid",
		zap.String("cfp_id", cfp.ID),
		zap.String("bid_id", bid.BidID),
		zap.Float64("price", bidPrice),
		zap.Float64("budget", cfp.Budget),
		zap.Float64("utilization", bidPrice/cfp.Budget),
	)
}

// handleBidAcceptance processes bid acceptance messages
func (ib *IntelligentBidder) handleBidAcceptance(msg *Message) error {
	// Parse acceptance message
	var acceptance AcceptProposal
	err := json.Unmarshal(msg.Data, &acceptance)
	if err != nil {
		ib.logger.Error("failed to parse acceptance",
			zap.Error(err),
		)
		return err
	}

	ib.logger.Info("bid accepted!",
		zap.String("cfp_id", acceptance.CFPID),
		zap.String("task_id", acceptance.TaskID),
	)

	// Find corresponding bid
	for bidID, bidCtx := range ib.activeBids {
		if bidCtx.CFP.ID == acceptance.CFPID {
			bidCtx.Won = true

			ib.logger.Info("won auction",
				zap.String("bid_id", bidID),
				zap.String("task_id", acceptance.TaskID),
				zap.Float64("price", bidCtx.BidPrice),
			)

			// TODO: Execute task and report outcome
			// For now, simulate success
			go ib.simulateTaskExecution(bidCtx)

			break
		}
	}

	return nil
}

// simulateTaskExecution simulates task execution and reports outcome to RL bidder
// TODO: Replace with actual task execution via ARI-v1
func (ib *IntelligentBidder) simulateTaskExecution(bidCtx *BidContext) {
	// Simulate execution time
	time.Sleep(100 * time.Millisecond)

	// Simulate success (90% success rate)
	completed := rand.Float64() < 0.9

	// Create outcome
	cost := bidCtx.BidPrice * 0.3
	outcome := TaskOutcome{
		TaskID:     bidCtx.CFP.ID,
		CFPId:      bidCtx.CFP.ID,
		BidID:      bidCtx.BidID,
		Capability: bidCtx.CFP.Capability,
		BidPrice:   bidCtx.BidPrice,
		ActualCost: cost,
		Profit:     bidCtx.BidPrice - cost,
		Success:    completed,
		Won:        true,
		Timestamp:  time.Now(),
	}

	// Report to RL bidder for learning
	ib.rlBidder.Learn(outcome)

	ib.logger.Info("task execution complete",
		zap.String("cfp_id", bidCtx.CFP.ID),
		zap.Bool("completed", completed),
		zap.Float64("profit", outcome.Profit),
	)
}

// ReportTaskOutcome manually reports task outcome (called after real execution)
func (ib *IntelligentBidder) ReportTaskOutcome(cfpID string, completed bool, actualCost float64) {
	// Find bid context
	for _, bidCtx := range ib.activeBids {
		if bidCtx.CFP.ID == cfpID && bidCtx.Won {
			outcome := TaskOutcome{
				TaskID:     cfpID,
				CFPId:      cfpID,
				BidID:      bidCtx.BidID,
				Capability: bidCtx.CFP.Capability,
				BidPrice:   bidCtx.BidPrice,
				ActualCost: actualCost,
				Profit:     bidCtx.BidPrice - actualCost,
				Success:    completed,
				Won:        true,
				Timestamp:  time.Now(),
			}

			ib.rlBidder.Learn(outcome)

			ib.logger.Info("reported task outcome to RL bidder",
				zap.String("cfp_id", cfpID),
				zap.Bool("completed", completed),
				zap.Float64("profit", outcome.Profit),
			)

			break
		}
	}
}

// GetStats returns bidder statistics
func (ib *IntelligentBidder) GetStats() map[string]interface{} {
	rlStats := ib.rlBidder.GetStats()

	// Add high-level stats
	rlStats["active_cfps"] = len(ib.activeCFPs)
	rlStats["active_bids"] = len(ib.activeBids)

	// Calculate win rate
	totalBids := len(ib.activeBids)
	wonBids := 0
	for _, bidCtx := range ib.activeBids {
		if bidCtx.Won {
			wonBids++
		}
	}

	if totalBids > 0 {
		rlStats["session_win_rate"] = float64(wonBids) / float64(totalBids)
	}

	return rlStats
}

// Helper functions

func extractCapabilities(card *agentcard.AgentCard) []string {
	// TODO: Extract from actual agent card structure
	// For now, return placeholder
	return []string{"math", "string", "json"}
}

func (ib *IntelligentBidder) hasCapability(capability string) bool {
	capabilities := extractCapabilities(ib.agentCard)
	for _, cap := range capabilities {
		if cap == capability {
			return true
		}
	}
	return false
}

func generateBidID() string {
	return fmt.Sprintf("bid-%d", time.Now().UnixNano())
}

// CFP represents a Call for Proposals (simplified for now)
// TODO: Use actual AACL CFP structure
type CFP struct {
	ID           string
	Capability   string
	Budget       float64
	Deadline     int64
	Capabilities []string
	TaskData     map[string]interface{}
}

type BidMessage struct {
	BidID     string  `json:"bid_id"`
	CFPID     string  `json:"cfp_id"`
	AgentDID  string  `json:"agent_did"`
	Price     float64 `json:"price"`
	Deadline  int64   `json:"deadline"`
	Timestamp int64   `json:"timestamp"`
}

type AcceptProposal struct {
	CFPID  string `json:"cfp_id"`
	TaskID string `json:"task_id"`
}
