package orchestration

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/multiformats/go-multibase"
	"go.uber.org/zap"
)

// SelectionMode defines built-in auction selection strategies.
type SelectionMode string

const (
	SelectionModeCheapest       SelectionMode = "cheapest"
	SelectionModeFastest        SelectionMode = "fastest"
	SelectionModeBestReputation SelectionMode = "best_reputation"
	SelectionModeCustom         SelectionMode = "custom"
)

// SelectionLogic defines how the winner is chosen among bids.
type SelectionLogic struct {
	Mode             SelectionMode
	PriceWeight      float64
	SpeedWeight      float64
	ReputationWeight float64
}

// BidSummary captures the key comparable attributes of a bid.
type BidSummary struct {
	BidID      string
	AgentDID   agentcard.DID
	Price      float64
	ETAms      int64
	Reputation float64

	RawMessage interface{} // will be *aacl.AACLMessage when wired
}

// AuctionResult represents the outcome of an auction.
type AuctionResult struct {
	CFPID    string
	Winner   *BidSummary
	AllBids  []BidSummary
	TimedOut bool
}

// Auctioneer coordinates CFP broadcasts and bid collection via p2p.
type Auctioneer struct {
	bus    *p2p.MessageBus
	gossip *p2p.GossipService
	logger *zap.Logger
}

// NewAuctioneer creates a new Auctioneer instance.
func NewAuctioneer(gossip *p2p.GossipService, logger *zap.Logger) *Auctioneer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Auctioneer{
		gossip: gossip,
		logger: logger,
	}
}

// StartAuction broadcasts a CFP for a task and collects bids for the given window.
func (a *Auctioneer) StartAuction(
	ctx context.Context,
	task *Task,
	logic SelectionLogic,
	window time.Duration,
) (*AuctionResult, error) {
	if a == nil || a.gossip == nil {
		return &AuctionResult{}, nil
	}

	if len(task.Capabilities) == 0 {
		// Nothing to auction on; caller should fall back immediately.
		return &AuctionResult{}, nil
	}

	primaryCap := task.Capabilities[0]
	cfpTopic := "ainur/v1/market/cfp/" + primaryCap
	bidTopic := fmt.Sprintf("ainur/v1/market/bid/%s", task.ID)

	a.logger.Info("auctioneer broadcasting CFP",
		zap.String("task_id", task.ID),
		zap.String("primary_capability", primaryCap),
		zap.String("mode", string(logic.Mode)),
		zap.Duration("window", window),
		zap.String("cfp_topic", cfpTopic),
		zap.String("bid_topic", bidTopic),
	)

	// Minimal but spec-aligned CFP payload
	payload := map[string]interface{}{
		"cfp_type":          "AACL-CFP-v1",
		"cfp_id":            task.ID,
		"from":              "orchestrator", // TODO: replace with real DID
		"to":                "*",
		"created_at":        time.Now().UTC().Format(time.RFC3339Nano),
		"auction_window_ms": int64(window / time.Millisecond),
		"selection_logic": map[string]interface{}{
			"mode":              string(logic.Mode),
			"price_weight":      logic.PriceWeight,
			"speed_weight":      logic.SpeedWeight,
			"reputation_weight": logic.ReputationWeight,
		},
		"intent": map[string]interface{}{
			"action":                "auction",
			"capabilities_required": task.Capabilities,
			"task_spec": map[string]interface{}{
				"type":       task.Type,
				"input":      task.Input,
				"priority":   task.Priority,
				"timeout_ms": int64(task.Timeout / time.Millisecond),
			},
			"budget": map[string]interface{}{
				"currency": "AINU",
				"amount":   task.Budget,
			},
		},
		"topic": cfpTopic,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		a.logger.Error("failed to marshal CFP payload",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		return &AuctionResult{CFPID: task.ID}, nil
	}

	// Subscribe to bid topic to collect responses
	var mu sync.Mutex
	bids := []BidSummary{}
	bidHandler := func(ctx context.Context, msg *p2p.GossipMessage) error {
		a.logger.Debug("received bid",
			zap.String("cfp_id", task.ID),
			zap.String("type", msg.Type),
		)

		var bid map[string]interface{}
		if err := json.Unmarshal(msg.Payload, &bid); err != nil {
			a.logger.Error("failed to unmarshal bid", zap.Error(err))
			return err
		}

		// Verify bid signature before accepting
		if err := a.verifyBidSignature(bid); err != nil {
			a.logger.Warn("bid signature verification failed, rejecting bid",
				zap.String("cfp_id", task.ID),
				zap.Error(err),
			)
			return nil // Don't return error, just skip this bid
		}

		// Extract key fields
		bidID, _ := bid["bid_id"].(string)
		fromDID, _ := bid["from"].(string)
		intent, _ := bid["intent"].(map[string]interface{})
		priceMap, _ := intent["price"].(map[string]interface{})
		priceAmount, _ := priceMap["amount"].(float64)
		etaMS, _ := intent["estimated_duration_ms"].(float64)

		summary := BidSummary{
			BidID:      bidID,
			AgentDID:   agentcard.DID(fromDID),
			Price:      priceAmount,
			ETAms:      int64(etaMS),
			Reputation: 0.0, // TODO: lookup from registry
			RawMessage: bid,
		}

		mu.Lock()
		bids = append(bids, summary)
		mu.Unlock()

		a.logger.Info("bid received and verified",
			zap.String("cfp_id", task.ID),
			zap.String("bid_id", bidID),
			zap.String("from", fromDID),
			zap.Float64("price", priceAmount),
		)

		return nil
	}

	if err := a.gossip.Subscribe(bidTopic, bidHandler); err != nil {
		a.logger.Error("failed to subscribe to bid topic",
			zap.String("bid_topic", bidTopic),
			zap.Error(err),
		)
		return &AuctionResult{CFPID: task.ID}, err
	}
	defer a.gossip.Unsubscribe(bidTopic)

	// Publish CFP
	cfpMsg := &p2p.GossipMessage{
		Type:      "AACL-CFP-v1",
		Payload:   data,
		Timestamp: time.Now().Unix(),
		PeerID:    "orchestrator", // TODO: use real DID
	}
	if err := a.gossip.Publish(cfpTopic, cfpMsg); err != nil {
		a.logger.Error("failed to broadcast CFP",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)
		return &AuctionResult{CFPID: task.ID}, err
	}

	// Wait for auction window to collect bids
	a.logger.Info("waiting for bids",
		zap.String("cfp_id", task.ID),
		zap.Duration("window", window),
	)
	time.Sleep(window)

	mu.Lock()
	allBids := make([]BidSummary, len(bids))
	copy(allBids, bids)
	mu.Unlock()

	if len(allBids) == 0 {
		a.logger.Warn("auction complete: no bids received",
			zap.String("cfp_id", task.ID),
		)
		return &AuctionResult{
			CFPID:    task.ID,
			Winner:   nil,
			AllBids:  allBids,
			TimedOut: true,
		}, nil
	}

	// Select winner based on selection logic
	winner := a.selectWinner(allBids, logic)

	a.logger.Info("auction complete: winner selected",
		zap.String("cfp_id", task.ID),
		zap.Int("total_bids", len(allBids)),
		zap.String("winner_did", string(winner.AgentDID)),
		zap.Float64("winning_price", winner.Price),
	)

	// Send acceptance to winner
	if err := a.sendAcceptProposal(ctx, task.ID, winner); err != nil {
		a.logger.Error("failed to send accept proposal to winner",
			zap.String("cfp_id", task.ID),
			zap.String("winner_did", string(winner.AgentDID)),
			zap.Error(err),
		)
	}

	// Send rejections to losers
	for i := range allBids {
		if allBids[i].BidID != winner.BidID {
			if err := a.sendRejectProposal(ctx, task.ID, &allBids[i], winner); err != nil {
				a.logger.Error("failed to send reject proposal",
					zap.String("cfp_id", task.ID),
					zap.String("loser_did", string(allBids[i].AgentDID)),
					zap.Error(err),
				)
			}
		}
	}

	return &AuctionResult{
		CFPID:   task.ID,
		Winner:  winner,
		AllBids: allBids,
	}, nil
}

// selectWinner applies the selection logic to pick the winning bid.
func (a *Auctioneer) selectWinner(bids []BidSummary, logic SelectionLogic) *BidSummary {
	if len(bids) == 0 {
		return nil
	}

	switch logic.Mode {
	case SelectionModeCheapest:
		// Find lowest price
		winner := &bids[0]
		for i := range bids {
			if bids[i].Price < winner.Price {
				winner = &bids[i]
			}
		}
		return winner

	case SelectionModeFastest:
		// Find lowest ETA
		winner := &bids[0]
		for i := range bids {
			if bids[i].ETAms < winner.ETAms {
				winner = &bids[i]
			}
		}
		return winner

	case SelectionModeBestReputation:
		// Find highest reputation
		winner := &bids[0]
		for i := range bids {
			if bids[i].Reputation > winner.Reputation {
				winner = &bids[i]
			}
		}
		return winner

	default:
		// Default to cheapest
		winner := &bids[0]
		for i := range bids {
			if bids[i].Price < winner.Price {
				winner = &bids[i]
			}
		}
		return winner
	}
}

// sendAcceptProposal sends an AACL-Accept-Proposal-v1 message to the winning agent.
func (a *Auctioneer) sendAcceptProposal(ctx context.Context, cfpID string, winner *BidSummary) error {
	acceptMsg := map[string]interface{}{
		"@type":        "Response",
		"message_type": "AACL-Accept-Proposal-v1",
		"message_id":   fmt.Sprintf("accept-%s-%d", cfpID, time.Now().UnixNano()),
		"cfp_id":       cfpID,
		"bid_id":       winner.BidID,
		"from":         "orchestrator", // TODO: use real DID
		"to":           string(winner.AgentDID),
		"created_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"intent": map[string]interface{}{
			"action":           "accept",
			"goal":             "Award task execution contract",
			"natural_language": "Your bid has been accepted. Please execute the task.",
			"contract": map[string]interface{}{
				"agreed_price": winner.Price,
				"currency":     "uAINU",
			},
		},
	}

	data, err := json.Marshal(acceptMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal accept proposal: %w", err)
	}

	// Publish to agent-specific accept topic
	acceptTopic := fmt.Sprintf("ainur/v1/market/accept/%s", winner.AgentDID)
	gossipMsg := &p2p.GossipMessage{
		Type:      "AACL-Accept-Proposal-v1",
		Payload:   data,
		Timestamp: time.Now().Unix(),
		PeerID:    "orchestrator",
	}

	if err := a.gossip.Publish(acceptTopic, gossipMsg); err != nil {
		return fmt.Errorf("failed to publish accept proposal: %w", err)
	}

	a.logger.Info("accept proposal sent",
		zap.String("cfp_id", cfpID),
		zap.String("winner_did", string(winner.AgentDID)),
		zap.String("topic", acceptTopic),
	)

	return nil
}

// sendRejectProposal sends an AACL-Reject-Proposal-v1 message to a non-winning agent.
func (a *Auctioneer) sendRejectProposal(ctx context.Context, cfpID string, loser *BidSummary, winner *BidSummary) error {
	rejectMsg := map[string]interface{}{
		"@type":        "Response",
		"message_type": "AACL-Reject-Proposal-v1",
		"message_id":   fmt.Sprintf("reject-%s-%d", cfpID, time.Now().UnixNano()),
		"cfp_id":       cfpID,
		"bid_id":       loser.BidID,
		"from":         "orchestrator", // TODO: use real DID
		"to":           string(loser.AgentDID),
		"created_at":   time.Now().UTC().Format(time.RFC3339Nano),
		"intent": map[string]interface{}{
			"action":           "reject",
			"goal":             "Inform that bid was not selected",
			"natural_language": "Thank you for your bid. Another agent was selected.",
			"reason":           "not_selected",
			"winning_bid": map[string]interface{}{
				"price":    winner.Price,
				"currency": "uAINU",
			},
		},
	}

	data, err := json.Marshal(rejectMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal reject proposal: %w", err)
	}

	// Publish to agent-specific reject topic
	rejectTopic := fmt.Sprintf("ainur/v1/market/reject/%s", loser.AgentDID)
	gossipMsg := &p2p.GossipMessage{
		Type:      "AACL-Reject-Proposal-v1",
		Payload:   data,
		Timestamp: time.Now().Unix(),
		PeerID:    "orchestrator",
	}

	if err := a.gossip.Publish(rejectTopic, gossipMsg); err != nil {
		return fmt.Errorf("failed to publish reject proposal: %w", err)
	}

	a.logger.Debug("reject proposal sent",
		zap.String("cfp_id", cfpID),
		zap.String("loser_did", string(loser.AgentDID)),
		zap.String("topic", rejectTopic),
	)

	return nil
}

// verifyBidSignature verifies the cryptographic signature on a bid.
func (a *Auctioneer) verifyBidSignature(bid map[string]interface{}) error {
	// Extract proof
	proof, ok := bid["proof"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("bid missing proof field")
	}

	proofValue, ok := proof["proof_value"].(string)
	if !ok {
		return fmt.Errorf("bid proof missing proof_value")
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(proofValue)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Extract agent DID
	fromDID, ok := bid["from"].(string)
	if !ok {
		return fmt.Errorf("bid missing from field")
	}

	// Extract public key from DID
	publicKey, err := publicKeyFromDID(fromDID)
	if err != nil {
		return fmt.Errorf("failed to extract public key from DID: %w", err)
	}

	// Create canonical bid (without proof) for verification
	bidCopy := make(map[string]interface{})
	for k, v := range bid {
		if k != "proof" {
			bidCopy[k] = v
		}
	}

	canonical, err := json.Marshal(bidCopy)
	if err != nil {
		return fmt.Errorf("failed to marshal canonical bid: %w", err)
	}

	// Verify signature
	if !ed25519.Verify(publicKey, canonical, signature) {
		return fmt.Errorf("signature verification failed")
	}

	a.logger.Debug("bid signature verified",
		zap.String("from", fromDID),
	)

	return nil
}

// publicKeyFromDID extracts the Ed25519 public key from a did:key DID.
func publicKeyFromDID(did string) (ed25519.PublicKey, error) {
	// Simplified extraction for did:key:z... format
	if len(did) < 13 || did[:9] != "did:key:z" {
		return nil, fmt.Errorf("invalid DID format: expected did:key:z...")
	}

	// Extract the multibase-encoded part
	// did:key:z{multibase-encoded-key}
	encoded := "z" + did[9:] // Re-add the multibase prefix

	// Decode using multibase
	_, decoded, err := multibase.Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DID multibase: %w", err)
	}

	// Check if it's the right size for Ed25519 public key
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: expected %d, got %d", ed25519.PublicKeySize, len(decoded))
	}

	return ed25519.PublicKey(decoded), nil
}
