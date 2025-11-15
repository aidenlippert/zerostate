package orchestration

import (
	"context"
	"fmt"
	"sort"
	"time"

	"go.uber.org/zap"
)

// VCGAuctionResult represents the result of a VCG (Vickrey-Clarke-Groves) auction
type VCGAuctionResult struct {
	CFPID          string
	Winner         *BidSummary
	SecondPrice    float64      // VCG second-price payment
	FirstPrice     float64      // What first-price auction would have paid
	Efficiency     float64      // Economic efficiency vs first-price
	AllBids        []*BidSummary
	SocialWelfare  float64      // Total utility to society
	AuctionType    string       // "VCG" or "FirstPrice" for comparison
}

// VCGAuctioneer implements Vickrey-Clarke-Groves second-price sealed-bid auctions
type VCGAuctioneer struct {
	auctioneer *Auctioneer // Embed the existing auctioneer for bid collection
	logger     *zap.Logger
}

// NewVCGAuctioneer creates a new VCG auctioneer
func NewVCGAuctioneer(auctioneer *Auctioneer, logger *zap.Logger) *VCGAuctioneer {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &VCGAuctioneer{
		auctioneer: auctioneer,
		logger:     logger.With(zap.String("component", "vcg-auction")),
	}
}

// StartVCGAuction runs a VCG auction for the given task
func (v *VCGAuctioneer) StartVCGAuction(
	ctx context.Context,
	task *Task,
	window time.Duration,
) (*VCGAuctionResult, error) {

	v.logger.Info("starting VCG auction",
		zap.String("task_id", task.ID),
		zap.Duration("window", window),
		zap.Strings("capabilities", task.Capabilities),
	)

	// Use the embedded auctioneer to collect bids
	// Set to cheapest mode for initial bid collection
	selectionLogic := SelectionLogic{Mode: SelectionModeCheapest}

	auctionResult, err := v.auctioneer.StartAuction(ctx, task, selectionLogic, window)
	if err != nil {
		return nil, fmt.Errorf("failed to collect bids: %w", err)
	}

	if auctionResult == nil || len(auctionResult.AllBids) == 0 {
		v.logger.Info("VCG auction completed with no bids",
			zap.String("task_id", task.ID),
		)
		return &VCGAuctionResult{
			CFPID:       auctionResult.CFPID,
			AuctionType: "VCG",
		}, nil
	}

	// Run VCG mechanism on collected bids
	vcgResult := v.runVCGMechanism(auctionResult, task)

	v.logger.Info("VCG auction completed",
		zap.String("task_id", task.ID),
		zap.String("winner", string(vcgResult.Winner.AgentDID)),
		zap.Float64("second_price", vcgResult.SecondPrice),
		zap.Float64("first_price", vcgResult.FirstPrice),
		zap.Float64("efficiency_gain", vcgResult.Efficiency),
		zap.Int("total_bids", len(vcgResult.AllBids)),
	)

	return vcgResult, nil
}

// StartFirstPriceAuction runs a traditional first-price auction for comparison
func (v *VCGAuctioneer) StartFirstPriceAuction(
	ctx context.Context,
	task *Task,
	window time.Duration,
) (*VCGAuctionResult, error) {

	v.logger.Info("starting first-price auction for comparison",
		zap.String("task_id", task.ID),
		zap.Duration("window", window),
	)

	// Use the embedded auctioneer with cheapest selection
	selectionLogic := SelectionLogic{Mode: SelectionModeCheapest}

	auctionResult, err := v.auctioneer.StartAuction(ctx, task, selectionLogic, window)
	if err != nil {
		return nil, fmt.Errorf("failed to collect bids: %w", err)
	}

	if auctionResult == nil || len(auctionResult.AllBids) == 0 {
		return &VCGAuctionResult{
			CFPID:       auctionResult.CFPID,
			AuctionType: "FirstPrice",
		}, nil
	}

	// For first-price auction, winner pays their bid
	winner := auctionResult.Winner
	firstPrice := winner.Price

	// Convert []BidSummary to []*BidSummary
	allBidsPtr := make([]*BidSummary, len(auctionResult.AllBids))
	for i := range auctionResult.AllBids {
		allBidsPtr[i] = &auctionResult.AllBids[i]
	}

	return &VCGAuctionResult{
		CFPID:         auctionResult.CFPID,
		Winner:        winner,
		SecondPrice:   firstPrice, // In first-price, payment equals bid
		FirstPrice:    firstPrice,
		Efficiency:    0.0, // Base case for efficiency comparison
		AllBids:       allBidsPtr,
		SocialWelfare: v.calculateSocialWelfare(allBidsPtr, winner),
		AuctionType:   "FirstPrice",
	}, nil
}

// runVCGMechanism implements the VCG auction mechanism
func (v *VCGAuctioneer) runVCGMechanism(auctionResult *AuctionResult, task *Task) *VCGAuctionResult {
	bids := auctionResult.AllBids

	// Convert []BidSummary to []*BidSummary
	bidsPtr := make([]*BidSummary, len(bids))
	for i := range bids {
		bidsPtr[i] = &bids[i]
	}

	// Step 1: Sort bids by price (ascending - lowest cost wins)
	sortedBids := make([]*BidSummary, len(bidsPtr))
	copy(sortedBids, bidsPtr)
	sort.Slice(sortedBids, func(i, j int) bool {
		// Primary: lowest price wins
		if sortedBids[i].Price != sortedBids[j].Price {
			return sortedBids[i].Price < sortedBids[j].Price
		}
		// Tiebreaker: highest reputation
		return sortedBids[i].Reputation > sortedBids[j].Reputation
	})

	// Step 2: Winner is the lowest bidder (most efficient)
	winner := sortedBids[0]

	// Step 3: Calculate VCG payment (second-price)
	var secondPrice float64
	var firstPrice float64 = winner.Price

	if len(sortedBids) >= 2 {
		// VCG payment: second-lowest price
		secondPrice = sortedBids[1].Price
	} else {
		// Only one bid: pay the bid price (degenerate case)
		secondPrice = winner.Price
	}

	// Step 4: Calculate efficiency vs first-price auction
	efficiency := v.calculateEfficiency(firstPrice, secondPrice)

	// Step 5: Calculate social welfare
	socialWelfare := v.calculateSocialWelfare(bidsPtr, winner)

	v.logger.Debug("VCG mechanism results",
		zap.String("winner", string(winner.AgentDID)),
		zap.Float64("winner_bid", winner.Price),
		zap.Float64("second_price_payment", secondPrice),
		zap.Float64("efficiency_vs_first_price", efficiency),
		zap.Float64("social_welfare", socialWelfare),
	)

	return &VCGAuctionResult{
		CFPID:         auctionResult.CFPID,
		Winner:        winner,
		SecondPrice:   secondPrice,
		FirstPrice:    firstPrice,
		Efficiency:    efficiency,
		AllBids:       bidsPtr,
		SocialWelfare: socialWelfare,
		AuctionType:   "VCG",
	}
}

// calculateEfficiency calculates the efficiency gain of VCG over first-price auction
func (v *VCGAuctioneer) calculateEfficiency(firstPrice, secondPrice float64) float64 {
	if firstPrice == 0 {
		return 0.0
	}

	// Efficiency = (first_price - second_price) / first_price
	// This represents the cost savings from VCG mechanism
	efficiency := (firstPrice - secondPrice) / firstPrice

	// Clamp to reasonable bounds
	if efficiency < 0 {
		efficiency = 0
	}
	if efficiency > 1 {
		efficiency = 1
	}

	return efficiency
}

// calculateSocialWelfare calculates the total utility to society
func (v *VCGAuctioneer) calculateSocialWelfare(bids []*BidSummary, winner *BidSummary) float64 {
	if len(bids) == 0 || winner == nil {
		return 0.0
	}

	// Social welfare in this context is the utility gained from selecting the winner
	// We model this as: (max_price - winner_price) + reputation_bonus

	maxPrice := 0.0
	totalReputation := 0.0

	for _, bid := range bids {
		if bid.Price > maxPrice {
			maxPrice = bid.Price
		}
		totalReputation += bid.Reputation
	}

	// Utility = cost savings + reputation value
	costSavings := maxPrice - winner.Price
	reputationValue := winner.Reputation / 1000.0 // Normalize reputation (0-1000 -> 0-1)

	socialWelfare := costSavings + reputationValue

	return socialWelfare
}

// CompareAuctionMechanisms runs both VCG and first-price auctions for comparison
func (v *VCGAuctioneer) CompareAuctionMechanisms(
	ctx context.Context,
	task *Task,
	window time.Duration,
) (*VCGComparisonResult, error) {

	v.logger.Info("running auction mechanism comparison",
		zap.String("task_id", task.ID),
	)

	// Note: In a real implementation, you would need to run both auctions
	// simultaneously or use the same set of bids to ensure fair comparison.
	// For this implementation, we'll simulate both mechanisms on the same bid set.

	// Collect bids once
	selectionLogic := SelectionLogic{Mode: SelectionModeCheapest}
	auctionResult, err := v.auctioneer.StartAuction(ctx, task, selectionLogic, window)
	if err != nil {
		return nil, fmt.Errorf("failed to collect bids for comparison: %w", err)
	}

	if auctionResult == nil || len(auctionResult.AllBids) == 0 {
		return &VCGComparisonResult{
			TaskID: task.ID,
			VCGResult: &VCGAuctionResult{
				CFPID:       auctionResult.CFPID,
				AuctionType: "VCG",
			},
			FirstPriceResult: &VCGAuctionResult{
				CFPID:       auctionResult.CFPID,
				AuctionType: "FirstPrice",
			},
		}, nil
	}

	// Run VCG mechanism
	vcgResult := v.runVCGMechanism(auctionResult, task)

	// Simulate first-price auction on same bids
	firstPriceResult := v.simulateFirstPriceAuction(auctionResult)

	comparison := &VCGComparisonResult{
		TaskID:           task.ID,
		VCGResult:        vcgResult,
		FirstPriceResult: firstPriceResult,
		CostSavings:      firstPriceResult.FirstPrice - vcgResult.SecondPrice,
		EfficiencyGain:   vcgResult.Efficiency,
	}

	v.logger.Info("auction mechanism comparison completed",
		zap.String("task_id", task.ID),
		zap.Float64("vcg_payment", vcgResult.SecondPrice),
		zap.Float64("first_price_payment", firstPriceResult.FirstPrice),
		zap.Float64("cost_savings", comparison.CostSavings),
		zap.Float64("efficiency_gain", comparison.EfficiencyGain),
	)

	return comparison, nil
}

// simulateFirstPriceAuction simulates a first-price auction on existing bids
func (v *VCGAuctioneer) simulateFirstPriceAuction(auctionResult *AuctionResult) *VCGAuctionResult {
	if auctionResult == nil || len(auctionResult.AllBids) == 0 {
		return &VCGAuctionResult{AuctionType: "FirstPrice"}
	}

	// Convert []BidSummary to []*BidSummary
	bidsPtr := make([]*BidSummary, len(auctionResult.AllBids))
	for i := range auctionResult.AllBids {
		bidsPtr[i] = &auctionResult.AllBids[i]
	}

	// Sort by lowest price (same logic as VCG)
	sortedBids := make([]*BidSummary, len(bidsPtr))
	copy(sortedBids, bidsPtr)
	sort.Slice(sortedBids, func(i, j int) bool {
		if sortedBids[i].Price != sortedBids[j].Price {
			return sortedBids[i].Price < sortedBids[j].Price
		}
		return sortedBids[i].Reputation > sortedBids[j].Reputation
	})

	winner := sortedBids[0]
	socialWelfare := v.calculateSocialWelfare(bidsPtr, winner)

	return &VCGAuctionResult{
		CFPID:         auctionResult.CFPID,
		Winner:        winner,
		SecondPrice:   winner.Price, // In first-price, payment equals bid
		FirstPrice:    winner.Price,
		Efficiency:    0.0, // Base case for efficiency comparison
		AllBids:       bidsPtr,
		SocialWelfare: socialWelfare,
		AuctionType:   "FirstPrice",
	}
}

// VCGComparisonResult compares VCG and first-price auction results
type VCGComparisonResult struct {
	TaskID           string
	VCGResult        *VCGAuctionResult
	FirstPriceResult *VCGAuctionResult
	CostSavings      float64 // How much VCG saves vs first-price
	EfficiencyGain   float64 // Economic efficiency improvement
}

// GetOverhead calculates the computational overhead of VCG vs first-price auction
func (v *VCGComparisonResult) GetOverhead() float64 {
	// VCG requires sorting and second-price calculation
	// Overhead is typically minimal (<10% computational cost)
	// This is a simplified model
	bidCount := float64(len(v.VCGResult.AllBids))

	// O(n log n) complexity for sorting vs O(n) for first-price
	if bidCount <= 1 {
		return 0.0
	}

	// Logarithmic overhead for sorting
	overhead := (bidCount * logBase2(bidCount)) / bidCount
	return overhead - 1.0 // Subtract base O(n) complexity
}

func logBase2(x float64) float64 {
	if x <= 1 {
		return 0
	}
	// Simple log2 approximation
	result := 0.0
	temp := x
	for temp > 1 {
		temp = temp / 2
		result++
	}
	return result
}