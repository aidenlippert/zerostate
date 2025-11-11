package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/aidenlippert/zerostate/libs/economic"
	"github.com/aidenlippert/zerostate/libs/identity"
	"github.com/aidenlippert/zerostate/libs/p2p"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	ErrAuctionNotFound    = errors.New("auction not found")
	ErrAuctionExpired     = errors.New("auction has expired")
	ErrAuctionClosed      = errors.New("auction is closed")
	ErrInvalidBid         = errors.New("invalid bid")
	ErrNoBids             = errors.New("no bids received")
	ErrInsufficientScore  = errors.New("agent reputation score too low")
)

// AuctionType represents the type of auction mechanism
type AuctionType string

const (
	AuctionTypeFirstPrice  AuctionType = "first_price"  // Winner pays their bid
	AuctionTypeSecondPrice AuctionType = "second_price" // Winner pays second-highest bid (Vickrey)
	AuctionTypeDutch       AuctionType = "dutch"        // Price decreases over time
	AuctionTypeReserve     AuctionType = "reserve"      // Minimum price threshold
)

// AuctionStatus represents the current state of an auction
type AuctionStatus string

const (
	AuctionStatusOpen      AuctionStatus = "open"
	AuctionStatusClosed    AuctionStatus = "closed"
	AuctionStatusAwarded   AuctionStatus = "awarded"
	AuctionStatusCanceled  AuctionStatus = "canceled"
	AuctionStatusExpired   AuctionStatus = "expired"
)

// TaskAuction represents an auction for task execution
type TaskAuction struct {
	// Identity
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Auction Configuration
	Type           AuctionType   `json:"type"`
	Status         AuctionStatus `json:"status"`
	Duration       time.Duration `json:"duration"`
	ExpiresAt      time.Time     `json:"expires_at"`
	ReservePrice   float64       `json:"reserve_price,omitempty"`   // Minimum acceptable price
	MaxPrice       float64       `json:"max_price"`                 // User's budget
	MinReputation  float64       `json:"min_reputation,omitempty"`  // Minimum reputation score

	// Task Requirements
	Capabilities []string               `json:"capabilities"`
	Requirements map[string]string      `json:"requirements"`
	TaskInput    map[string]interface{} `json:"task_input"`
	Timeout      time.Duration          `json:"timeout"`

	// Bids
	Bids         []*Bid  `json:"bids"`
	WinningBid   *Bid    `json:"winning_bid,omitempty"`
	WinnerDID    string  `json:"winner_did,omitempty"`
	FinalPrice   float64 `json:"final_price,omitempty"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata"`
}

// Bid represents an agent's bid for a task
type Bid struct {
	// Identity
	ID        string    `json:"id"`
	AuctionID string    `json:"auction_id"`
	AgentDID  string    `json:"agent_did"`
	CreatedAt time.Time `json:"created_at"`

	// Bid Details
	Price            float64           `json:"price"`              // Bid price
	EstimatedTime    time.Duration     `json:"estimated_time"`     // Estimated completion time
	ReputationScore  float64           `json:"reputation_score"`   // Agent's reputation
	QualityScore     float64           `json:"quality_score"`      // Past quality metrics
	Metadata         map[string]string `json:"metadata"`

	// Composite Score (calculated)
	CompositeScore   float64           `json:"composite_score"`    // Overall ranking score
}

// AuctionService manages task auctions
type AuctionService struct {
	mu              sync.RWMutex
	messageBus      *p2p.MessageBus
	reputationSvc   *economic.ReputationService
	logger          *zap.Logger

	// Active auctions
	auctions        map[string]*TaskAuction // Auction ID -> Auction
	taskAuctions    map[string]string       // Task ID -> Auction ID

	// Cleanup
	cleanupTicker   *time.Ticker
	stopCh          chan struct{}

	// Metrics
	metricsAuctionsCreated   prometheus.Counter
	metricsAuctionsClosed    prometheus.Counter
	metricsAuctionsAwarded   prometheus.Counter
	metricsBidsReceived      prometheus.Counter
	metricsAuctionDuration   prometheus.Histogram
	metricsBidsPerAuction    prometheus.Histogram
	metricsWinningBidPrice   prometheus.Histogram
}

// NewAuctionService creates a new auction service
func NewAuctionService(
	messageBus *p2p.MessageBus,
	reputationSvc *economic.ReputationService,
	logger *zap.Logger,
) *AuctionService {
	as := &AuctionService{
		messageBus:    messageBus,
		reputationSvc: reputationSvc,
		logger:        logger,
		auctions:      make(map[string]*TaskAuction),
		taskAuctions:  make(map[string]string),
		stopCh:        make(chan struct{}),

		metricsAuctionsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_auctions_created_total",
			Help: "Total number of auctions created",
		}),
		metricsAuctionsClosed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_auctions_closed_total",
			Help: "Total number of auctions closed",
		}),
		metricsAuctionsAwarded: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_auctions_awarded_total",
			Help: "Total number of auctions awarded",
		}),
		metricsBidsReceived: promauto.NewCounter(prometheus.CounterOpts{
			Name: "zerostate_bids_received_total",
			Help: "Total number of bids received",
		}),
		metricsAuctionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_auction_duration_seconds",
			Help:    "Duration of auctions in seconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 8), // 1s to ~2min
		}),
		metricsBidsPerAuction: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_bids_per_auction",
			Help:    "Number of bids per auction",
			Buckets: prometheus.LinearBuckets(0, 1, 20), // 0 to 20 bids
		}),
		metricsWinningBidPrice: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "zerostate_winning_bid_price",
			Help:    "Winning bid price distribution",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // $0.1 to ~$100
		}),
	}

	// Start cleanup goroutine
	as.cleanupTicker = time.NewTicker(10 * time.Second)
	go as.cleanupExpiredAuctions()

	return as
}

// CreateAuction creates a new task auction
func (as *AuctionService) CreateAuction(ctx context.Context, config *TaskAuction) (*TaskAuction, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Generate ID if not provided
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	// Set defaults
	now := time.Now()
	config.CreatedAt = now
	config.UpdatedAt = now
	config.Status = AuctionStatusOpen

	if config.Duration == 0 {
		config.Duration = 30 * time.Second // Default auction duration
	}
	config.ExpiresAt = now.Add(config.Duration)

	if config.Type == "" {
		config.Type = AuctionTypeSecondPrice // Default to Vickrey auction
	}

	config.Bids = make([]*Bid, 0)
	if config.Metadata == nil {
		config.Metadata = make(map[string]interface{})
	}

	// Store auction
	as.auctions[config.ID] = config
	as.taskAuctions[config.TaskID] = config.ID

	as.metricsAuctionsCreated.Inc()
	as.logger.Info("auction created",
		zap.String("auction_id", config.ID),
		zap.String("task_id", config.TaskID),
		zap.String("type", string(config.Type)),
		zap.Duration("duration", config.Duration),
		zap.Float64("max_price", config.MaxPrice),
	)

	// Broadcast auction to network
	go as.broadcastAuction(ctx, config)

	return config, nil
}

// SubmitBid submits a bid for an auction
func (as *AuctionService) SubmitBid(ctx context.Context, auctionID string, bid *Bid) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Get auction
	auction, exists := as.auctions[auctionID]
	if !exists {
		return ErrAuctionNotFound
	}

	// Check auction status
	if auction.Status != AuctionStatusOpen {
		return ErrAuctionClosed
	}

	// Check expiration
	if time.Now().After(auction.ExpiresAt) {
		auction.Status = AuctionStatusExpired
		return ErrAuctionExpired
	}

	// Validate bid
	if bid.Price > auction.MaxPrice {
		return fmt.Errorf("%w: bid price %.2f exceeds max price %.2f",
			ErrInvalidBid, bid.Price, auction.MaxPrice)
	}

	if auction.ReservePrice > 0 && bid.Price < auction.ReservePrice {
		return fmt.Errorf("%w: bid price %.2f below reserve price %.2f",
			ErrInvalidBid, bid.Price, auction.ReservePrice)
	}

	// Check reputation threshold
	if auction.MinReputation > 0 && bid.ReputationScore < auction.MinReputation {
		return fmt.Errorf("%w: reputation %.2f below minimum %.2f",
			ErrInsufficientScore, bid.ReputationScore, auction.MinReputation)
	}

	// Set bid metadata
	bid.ID = uuid.New().String()
	bid.AuctionID = auctionID
	bid.CreatedAt = time.Now()

	// Calculate composite score
	bid.CompositeScore = as.calculateCompositeScore(bid, auction)

	// Add bid
	auction.Bids = append(auction.Bids, bid)
	auction.UpdatedAt = time.Now()

	as.metricsBidsReceived.Inc()
	as.logger.Info("bid received",
		zap.String("auction_id", auctionID),
		zap.String("agent_did", bid.AgentDID),
		zap.Float64("price", bid.Price),
		zap.Float64("composite_score", bid.CompositeScore),
	)

	return nil
}

// CloseAuction closes an auction and selects the winner
func (as *AuctionService) CloseAuction(ctx context.Context, auctionID string) (*Bid, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Get auction
	auction, exists := as.auctions[auctionID]
	if !exists {
		return nil, ErrAuctionNotFound
	}

	// Check if already closed
	if auction.Status != AuctionStatusOpen {
		return auction.WinningBid, nil
	}

	// Check if expired
	if time.Now().After(auction.ExpiresAt) {
		auction.Status = AuctionStatusExpired
	} else {
		auction.Status = AuctionStatusClosed
	}

	// Check for bids
	if len(auction.Bids) == 0 {
		as.logger.Warn("auction closed with no bids", zap.String("auction_id", auctionID))
		as.metricsAuctionsClosed.Inc()
		return nil, ErrNoBids
	}

	// Select winner based on auction type
	winningBid, finalPrice := as.selectWinner(auction)

	auction.WinningBid = winningBid
	auction.WinnerDID = winningBid.AgentDID
	auction.FinalPrice = finalPrice
	auction.Status = AuctionStatusAwarded
	auction.UpdatedAt = time.Now()

	duration := time.Since(auction.CreatedAt)
	as.metricsAuctionsClosed.Inc()
	as.metricsAuctionsAwarded.Inc()
	as.metricsAuctionDuration.Observe(duration.Seconds())
	as.metricsBidsPerAuction.Observe(float64(len(auction.Bids)))
	as.metricsWinningBidPrice.Observe(finalPrice)

	as.logger.Info("auction awarded",
		zap.String("auction_id", auctionID),
		zap.String("winner_did", winningBid.AgentDID),
		zap.Float64("bid_price", winningBid.Price),
		zap.Float64("final_price", finalPrice),
		zap.Int("total_bids", len(auction.Bids)),
		zap.Duration("duration", duration),
	)

	return winningBid, nil
}

// selectWinner selects the winning bid based on auction type
func (as *AuctionService) selectWinner(auction *TaskAuction) (*Bid, float64) {
	// Sort bids by composite score (highest first)
	sortedBids := make([]*Bid, len(auction.Bids))
	copy(sortedBids, auction.Bids)
	sort.Slice(sortedBids, func(i, j int) bool {
		return sortedBids[i].CompositeScore > sortedBids[j].CompositeScore
	})

	winningBid := sortedBids[0]
	var finalPrice float64

	switch auction.Type {
	case AuctionTypeFirstPrice:
		// Winner pays their bid
		finalPrice = winningBid.Price

	case AuctionTypeSecondPrice:
		// Winner pays second-highest bid (Vickrey auction)
		if len(sortedBids) > 1 {
			finalPrice = sortedBids[1].Price
		} else {
			finalPrice = winningBid.Price
		}

	case AuctionTypeReserve:
		// Winner pays their bid, but must meet reserve
		finalPrice = winningBid.Price
		if auction.ReservePrice > 0 && finalPrice < auction.ReservePrice {
			finalPrice = auction.ReservePrice
		}

	default:
		finalPrice = winningBid.Price
	}

	return winningBid, finalPrice
}

// calculateCompositeScore calculates overall ranking score for a bid
func (as *AuctionService) calculateCompositeScore(bid *Bid, auction *TaskAuction) float64 {
	// Composite score formula:
	// Score = w1*(1/normalized_price) + w2*reputation + w3*quality + w4*(1/normalized_time)

	// Weights (sum to 1.0)
	const (
		weightPrice      = 0.40  // 40% - Price is important
		weightReputation = 0.30  // 30% - Reputation matters
		weightQuality    = 0.20  // 20% - Past quality
		weightTime       = 0.10  // 10% - Speed
	)

	// Normalize price (lower is better, so invert)
	// Score ranges from 0 to 1, where 1 is best
	priceScore := 0.0
	if auction.MaxPrice > 0 {
		// Invert: low price = high score
		priceScore = 1.0 - (bid.Price / auction.MaxPrice)
	}

	// Reputation score (0 to 1, where 1 is perfect reputation)
	reputationScore := bid.ReputationScore / 100.0
	if reputationScore > 1.0 {
		reputationScore = 1.0
	}

	// Quality score (0 to 1)
	qualityScore := bid.QualityScore / 100.0
	if qualityScore > 1.0 {
		qualityScore = 1.0
	}

	// Time score (faster is better, so invert)
	timeScore := 0.0
	if auction.Timeout > 0 && bid.EstimatedTime > 0 {
		timeScore = 1.0 - float64(bid.EstimatedTime)/float64(auction.Timeout)
		if timeScore < 0 {
			timeScore = 0
		}
	}

	// Calculate weighted composite score
	compositeScore := (weightPrice * priceScore) +
		(weightReputation * reputationScore) +
		(weightQuality * qualityScore) +
		(weightTime * timeScore)

	return compositeScore
}

// GetAuction retrieves an auction by ID
func (as *AuctionService) GetAuction(auctionID string) (*TaskAuction, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	auction, exists := as.auctions[auctionID]
	if !exists {
		return nil, ErrAuctionNotFound
	}

	return auction, nil
}

// GetAuctionByTask retrieves auction for a task
func (as *AuctionService) GetAuctionByTask(taskID string) (*TaskAuction, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	auctionID, exists := as.taskAuctions[taskID]
	if !exists {
		return nil, ErrAuctionNotFound
	}

	return as.auctions[auctionID], nil
}

// CancelAuction cancels an auction
func (as *AuctionService) CancelAuction(auctionID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	auction, exists := as.auctions[auctionID]
	if !exists {
		return ErrAuctionNotFound
	}

	auction.Status = AuctionStatusCanceled
	auction.UpdatedAt = time.Now()

	as.logger.Info("auction canceled", zap.String("auction_id", auctionID))
	return nil
}

// broadcastAuction broadcasts auction to the P2P network
func (as *AuctionService) broadcastAuction(ctx context.Context, auction *TaskAuction) {
	// Create auction announcement
	announcement := map[string]interface{}{
		"auction_id":    auction.ID,
		"task_id":       auction.TaskID,
		"type":          auction.Type,
		"capabilities":  auction.Capabilities,
		"requirements":  auction.Requirements,
		"max_price":     auction.MaxPrice,
		"reserve_price": auction.ReservePrice,
		"min_reputation": auction.MinReputation,
		"expires_at":    auction.ExpiresAt.Unix(),
		"timeout":       auction.Timeout.Seconds(),
	}

	payload, err := json.Marshal(announcement)
	if err != nil {
		as.logger.Error("failed to marshal auction announcement", zap.Error(err))
		return
	}

	err = as.messageBus.Broadcast(ctx, payload, "auction_announcement")
	if err != nil {
		as.logger.Error("failed to broadcast auction", zap.Error(err))
	}
}

// cleanupExpiredAuctions periodically removes expired auctions
func (as *AuctionService) cleanupExpiredAuctions() {
	for {
		select {
		case <-as.cleanupTicker.C:
			as.mu.Lock()
			now := time.Now()
			for id, auction := range as.auctions {
				if auction.Status == AuctionStatusOpen && now.After(auction.ExpiresAt) {
					auction.Status = AuctionStatusExpired
					auction.UpdatedAt = now
					as.logger.Info("auction expired", zap.String("auction_id", id))
				}
			}
			as.mu.Unlock()

		case <-as.stopCh:
			as.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the auction service
func (as *AuctionService) Stop() {
	close(as.stopCh)
	as.logger.Info("auction service stopped")
}

// AuctionStats returns statistics about auctions
type AuctionStats struct {
	TotalAuctions    int     `json:"total_auctions"`
	OpenAuctions     int     `json:"open_auctions"`
	ClosedAuctions   int     `json:"closed_auctions"`
	AwardedAuctions  int     `json:"awarded_auctions"`
	ExpiredAuctions  int     `json:"expired_auctions"`
	TotalBids        int     `json:"total_bids"`
	AvgBidsPerAuction float64 `json:"avg_bids_per_auction"`
	AvgWinningPrice  float64 `json:"avg_winning_price"`
}

// GetStats returns auction statistics
func (as *AuctionService) GetStats() *AuctionStats {
	as.mu.RLock()
	defer as.mu.RUnlock()

	stats := &AuctionStats{}
	totalBids := 0
	totalWinningPrice := 0.0
	awardedCount := 0

	for _, auction := range as.auctions {
		stats.TotalAuctions++
		totalBids += len(auction.Bids)

		switch auction.Status {
		case AuctionStatusOpen:
			stats.OpenAuctions++
		case AuctionStatusClosed:
			stats.ClosedAuctions++
		case AuctionStatusAwarded:
			stats.AwardedAuctions++
			awardedCount++
			totalWinningPrice += auction.FinalPrice
		case AuctionStatusExpired:
			stats.ExpiredAuctions++
		}
	}

	stats.TotalBids = totalBids
	if stats.TotalAuctions > 0 {
		stats.AvgBidsPerAuction = float64(totalBids) / float64(stats.TotalAuctions)
	}
	if awardedCount > 0 {
		stats.AvgWinningPrice = totalWinningPrice / float64(awardedCount)
	}

	return stats
}
