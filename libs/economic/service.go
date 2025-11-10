package economic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/google/uuid"
)

// EconomicService provides real database-backed economic layer operations
type EconomicService struct {
	db *database.Database
}

// NewEconomicService creates a new economic service with database persistence
func NewEconomicService(db *database.Database) *EconomicService {
	return &EconomicService{db: db}
}

// ============================================================================
// AUCTION SERVICE METHODS
// ============================================================================

// AuctionType represents the type of auction
type AuctionType string

const (
	AuctionTypeFirstPrice  AuctionType = "first_price"  // Winner pays their bid
	AuctionTypeSecondPrice AuctionType = "second_price" // Winner pays second-highest bid
	AuctionTypeReserve     AuctionType = "reserve"      // Must meet reserve price
)

// AuctionStatus represents the current state of an auction
type AuctionStatus string

const (
	AuctionStatusOpen     AuctionStatus = "open"     // Accepting bids
	AuctionStatusClosed   AuctionStatus = "closed"   // No longer accepting bids
	AuctionStatusAwarded  AuctionStatus = "awarded"  // Winner selected
	AuctionStatusCanceled AuctionStatus = "canceled" // Auction canceled
	AuctionStatusExpired  AuctionStatus = "expired"  // Duration expired
)

// Auction represents a task auction
type Auction struct {
	ID             uuid.UUID       `json:"id"`
	TaskID         string          `json:"task_id"`
	UserID         string          `json:"user_id"`
	AuctionType    AuctionType     `json:"auction_type"`
	Status         AuctionStatus   `json:"status"`
	DurationSec    int             `json:"duration_seconds"`
	ExpiresAt      time.Time       `json:"expires_at"`
	ReservePrice   *float64        `json:"reserve_price,omitempty"`
	MaxPrice       *float64        `json:"max_price,omitempty"`
	MinReputation  *float64        `json:"min_reputation,omitempty"`
	Capabilities   json.RawMessage `json:"capabilities"`
	WinningBidID   *uuid.UUID      `json:"winning_bid_id,omitempty"`
	FinalPrice     *float64        `json:"final_price,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
}

// Bid represents a bid on an auction
type Bid struct {
	ID              uuid.UUID `json:"id"`
	AuctionID       uuid.UUID `json:"auction_id"`
	AgentDID        string    `json:"agent_did"`
	Price           float64   `json:"price"`
	EstimatedTimeSec *int     `json:"estimated_time_seconds,omitempty"`
	ReputationScore *float64  `json:"reputation_score,omitempty"`
	QualityScore    *float64  `json:"quality_score,omitempty"`
	CompositeScore  *float64  `json:"composite_score,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// CreateAuction creates a new auction with database persistence
func (s *EconomicService) CreateAuction(ctx context.Context, taskID, userID string, auctionType AuctionType, durationSec int, reservePrice, maxPrice, minReputation *float64, capabilities json.RawMessage) (*Auction, error) {
	if durationSec <= 0 {
		return nil, errors.New("duration must be positive")
	}

	auction := &Auction{
		ID:            uuid.New(),
		TaskID:        taskID,
		UserID:        userID,
		AuctionType:   auctionType,
		Status:        AuctionStatusOpen,
		DurationSec:   durationSec,
		ExpiresAt:     time.Now().Add(time.Duration(durationSec) * time.Second),
		ReservePrice:  reservePrice,
		MaxPrice:      maxPrice,
		MinReputation: minReputation,
		Capabilities:  capabilities,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Metadata:      json.RawMessage(`{}`),
	}

	query := `
		INSERT INTO auctions (
			id, task_id, user_id, auction_type, status, duration_seconds,
			expires_at, reserve_price, max_price, min_reputation,
			capabilities, created_at, updated_at, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := s.db.Conn().ExecContext(ctx, query,
		auction.ID, auction.TaskID, auction.UserID, auction.AuctionType,
		auction.Status, auction.DurationSec, auction.ExpiresAt,
		auction.ReservePrice, auction.MaxPrice, auction.MinReputation,
		auction.Capabilities, auction.CreatedAt, auction.UpdatedAt, auction.Metadata,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create auction: %w", err)
	}

	return auction, nil
}

// SubmitBid submits a bid on an auction with database persistence
func (s *EconomicService) SubmitBid(ctx context.Context, auctionID uuid.UUID, agentDID string, price float64, estimatedTimeSec *int) (*Bid, error) {
	if price <= 0 {
		return nil, errors.New("bid price must be positive")
	}

	// Check if auction exists and is open
	var auctionStatus AuctionStatus
	var expiresAt time.Time
	err := s.db.Conn().QueryRowContext(ctx,
		"SELECT status, expires_at FROM auctions WHERE id = $1",
		auctionID,
	).Scan(&auctionStatus, &expiresAt)
	if err == sql.ErrNoRows {
		return nil, errors.New("auction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to check auction status: %w", err)
	}

	if auctionStatus != AuctionStatusOpen {
		return nil, fmt.Errorf("auction is %s, not accepting bids", auctionStatus)
	}

	if time.Now().After(expiresAt) {
		return nil, errors.New("auction has expired")
	}

	// Get agent reputation score if exists
	var reputationScore *float64
	var score float64
	err = s.db.Conn().QueryRowContext(ctx,
		"SELECT overall_score FROM reputation_scores WHERE agent_did = $1",
		agentDID,
	).Scan(&score)
	if err == nil {
		reputationScore = &score
	}

	// Calculate composite score (price + reputation + speed)
	compositeScore := calculateCompositeScore(price, reputationScore, estimatedTimeSec)

	bid := &Bid{
		ID:              uuid.New(),
		AuctionID:       auctionID,
		AgentDID:        agentDID,
		Price:           price,
		EstimatedTimeSec: estimatedTimeSec,
		ReputationScore: reputationScore,
		CompositeScore:  &compositeScore,
		CreatedAt:       time.Now(),
	}

	query := `
		INSERT INTO bids (
			id, auction_id, agent_did, price, estimated_time_seconds,
			reputation_score, composite_score, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = s.db.Conn().ExecContext(ctx, query,
		bid.ID, bid.AuctionID, bid.AgentDID, bid.Price,
		bid.EstimatedTimeSec, bid.ReputationScore, bid.CompositeScore, bid.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit bid: %w", err)
	}

	return bid, nil
}

// GetAuction retrieves an auction by ID
func (s *EconomicService) GetAuction(ctx context.Context, id uuid.UUID) (*Auction, error) {
	query := `
		SELECT id, task_id, user_id, auction_type, status, duration_seconds,
		       expires_at, reserve_price, max_price, min_reputation, capabilities,
		       winning_bid_id, final_price, created_at, updated_at, metadata
		FROM auctions WHERE id = $1
	`
	var auction Auction
	err := s.db.Conn().QueryRowContext(ctx, query, id).Scan(
		&auction.ID, &auction.TaskID, &auction.UserID, &auction.AuctionType,
		&auction.Status, &auction.DurationSec, &auction.ExpiresAt,
		&auction.ReservePrice, &auction.MaxPrice, &auction.MinReputation,
		&auction.Capabilities, &auction.WinningBidID, &auction.FinalPrice,
		&auction.CreatedAt, &auction.UpdatedAt, &auction.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, errors.New("auction not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get auction: %w", err)
	}
	return &auction, nil
}

// GetBidsForAuction retrieves all bids for an auction
func (s *EconomicService) GetBidsForAuction(ctx context.Context, auctionID uuid.UUID) ([]*Bid, error) {
	query := `
		SELECT id, auction_id, agent_did, price, estimated_time_seconds,
		       reputation_score, quality_score, composite_score, created_at
		FROM bids WHERE auction_id = $1
		ORDER BY composite_score DESC
	`
	rows, err := s.db.Conn().QueryContext(ctx, query, auctionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bids: %w", err)
	}
	defer rows.Close()

	var bids []*Bid
	for rows.Next() {
		var bid Bid
		err := rows.Scan(
			&bid.ID, &bid.AuctionID, &bid.AgentDID, &bid.Price,
			&bid.EstimatedTimeSec, &bid.ReputationScore, &bid.QualityScore,
			&bid.CompositeScore, &bid.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		bids = append(bids, &bid)
	}
	return bids, nil
}

// ============================================================================
// PAYMENT CHANNEL SERVICE METHODS
// ============================================================================

// OpenPaymentChannel creates a new payment channel with database persistence
func (s *EconomicService) OpenPaymentChannel(ctx context.Context, payerDID, payeeDID string, initialDeposit float64, auctionID *string) (*database.PaymentChannel, error) {
	if initialDeposit <= 0 {
		return nil, errors.New("initial deposit must be positive")
	}

	channel := &database.PaymentChannel{
		ID:             uuid.New(),
		PayerDID:       payerDID,
		PayeeDID:       payeeDID,
		TotalDeposit:   initialDeposit,
		CurrentBalance: initialDeposit,
		EscrowedAmount: 0,
		TotalSettled:   0,
		PendingRefund:  0,
		State:          "open",
		SequenceNumber: 0,
		EscrowReleased: false,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if auctionID != nil {
		channel.AuctionID = sql.NullString{String: *auctionID, Valid: true}
	}

	repo := database.NewPaymentChannelRepository(s.db)
	err := repo.Create(ctx, channel)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment channel: %w", err)
	}

	return channel, nil
}

// SettlePaymentChannel settles and closes a payment channel
func (s *EconomicService) SettlePaymentChannel(ctx context.Context, channelID uuid.UUID, finalAmount float64) error {
	if finalAmount < 0 {
		return errors.New("final amount cannot be negative")
	}

	repo := database.NewPaymentChannelRepository(s.db)
	channel, err := repo.GetByID(ctx, channelID)
	if err != nil {
		return fmt.Errorf("failed to get channel: %w", err)
	}

	if channel.State == "closed" {
		return errors.New("channel is already closed")
	}

	// Update channel state to settling, then closed
	query := `
		UPDATE payment_channels
		SET total_settled = $1,
		    current_balance = total_deposit - $1,
		    state = 'closed',
		    closed_at = NOW(),
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err = s.db.Conn().ExecContext(ctx, query, finalAmount, channelID)
	if err != nil {
		return fmt.Errorf("failed to settle channel: %w", err)
	}

	// Record transaction
	txRepo := database.NewChannelTransactionRepository(s.db)
	tx := &database.ChannelTransaction{
		ID:              uuid.New(),
		ChannelID:       channelID,
		TransactionType: "settlement",
		Amount:          finalAmount,
		Reason:          sql.NullString{String: "final settlement", Valid: true},
		Metadata:        json.RawMessage(`{}`),
		CreatedAt:       time.Now(),
	}
	err = txRepo.Create(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to record transaction: %w", err)
	}

	return nil
}

// ============================================================================
// REPUTATION SERVICE METHODS
// ============================================================================

// GetAgentReputation retrieves reputation score for an agent
func (s *EconomicService) GetAgentReputation(ctx context.Context, agentDID string) (*database.ReputationScore, error) {
	repo := database.NewReputationRepository(s.db)
	score, err := repo.GetByAgentDID(ctx, agentDID)
	if err == database.ErrNotFound {
		// Create initial reputation score for new agent
		return s.initializeAgentReputation(ctx, agentDID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reputation: %w", err)
	}
	return score, nil
}

// UpdateAgentReputation updates reputation score based on task completion
func (s *EconomicService) UpdateAgentReputation(ctx context.Context, agentDID, taskID string, success bool, rating float64, responseTime int) error {
	// Check if reputation score exists, if not create it
	repo := database.NewReputationRepository(s.db)
	_, err := repo.GetByAgentDID(ctx, agentDID)
	if err == database.ErrNotFound {
		_, err = s.initializeAgentReputation(ctx, agentDID)
		if err != nil {
			return fmt.Errorf("failed to initialize reputation: %w", err)
		}
	}

	// Calculate score delta based on success, rating, and response time
	delta := calculateReputationDelta(success, rating, responseTime)

	// Update reputation score
	err = repo.UpdateScore(ctx, agentDID, delta, success)
	if err != nil {
		return fmt.Errorf("failed to update reputation: %w", err)
	}

	// Record reputation event
	eventID := uuid.New()
	eventType := "task_completed"
	if !success {
		eventType = "task_failed"
	}

	metadata := map[string]interface{}{
		"task_id":       taskID,
		"rating":        rating,
		"response_time": responseTime,
		"success":       success,
	}
	metadataJSON, _ := json.Marshal(metadata)

	query := `
		INSERT INTO reputation_events (
			id, agent_did, event_type, task_id, score_delta, created_at, metadata
		) VALUES ($1, $2, $3, $4, $5, NOW(), $6)
	`
	_, err = s.db.Conn().ExecContext(ctx, query,
		eventID, agentDID, eventType, taskID, delta, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to record reputation event: %w", err)
	}

	return nil
}

// initializeAgentReputation creates initial reputation score for new agent
func (s *EconomicService) initializeAgentReputation(ctx context.Context, agentDID string) (*database.ReputationScore, error) {
	score := &database.ReputationScore{
		ID:              uuid.New(),
		AgentDID:        agentDID,
		OverallScore:    50.0, // Start at neutral score
		ReliabilityScore: 50.0,
		QualityScore:    50.0,
		SpeedScore:      50.0,
		TotalTasks:      0,
		SuccessfulTasks: 0,
		FailedTasks:     0,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	query := `
		INSERT INTO reputation_scores (
			id, agent_did, overall_score, reliability_score, quality_score,
			speed_score, total_tasks, successful_tasks, failed_tasks,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err := s.db.Conn().ExecContext(ctx, query,
		score.ID, score.AgentDID, score.OverallScore, score.ReliabilityScore,
		score.QualityScore, score.SpeedScore, score.TotalTasks,
		score.SuccessfulTasks, score.FailedTasks, score.CreatedAt, score.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize reputation: %w", err)
	}

	return score, nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

// calculateCompositeScore calculates a composite score for auction bidding
// Lower scores are better (lower price + higher reputation + faster time)
func calculateCompositeScore(price float64, reputationScore *float64, estimatedTimeSec *int) float64 {
	// Normalize price (assume $0.01 - $1.00 range)
	priceScore := price * 100 // Convert to 1-100 scale

	// Reputation score (0-100, higher is better, so invert)
	repScore := 50.0 // Default neutral
	if reputationScore != nil {
		repScore = 100.0 - *reputationScore // Invert so lower is better
	}

	// Time score (assume 10-300 seconds range)
	timeScore := 50.0 // Default neutral
	if estimatedTimeSec != nil {
		timeScore = float64(*estimatedTimeSec) / 3.0 // Normalize to ~0-100
	}

	// Weighted composite: 50% price, 30% reputation, 20% time
	return (priceScore * 0.5) + (repScore * 0.3) + (timeScore * 0.2)
}

// calculateReputationDelta calculates reputation score change based on task outcome
func calculateReputationDelta(success bool, rating float64, responseTime int) float64 {
	if !success {
		return -5.0 // Penalty for failure
	}

	// Base score for success
	delta := 2.0

	// Bonus for high rating (1-5 scale)
	if rating >= 4.5 {
		delta += 2.0
	} else if rating >= 4.0 {
		delta += 1.0
	} else if rating >= 3.5 {
		delta += 0.5
	} else if rating < 3.0 {
		delta -= 1.0 // Penalty for low rating
	}

	// Bonus for fast response (< 100ms excellent, < 250ms good)
	if responseTime < 100 {
		delta += 1.0
	} else if responseTime < 250 {
		delta += 0.5
	} else if responseTime > 1000 {
		delta -= 0.5 // Penalty for slow response
	}

	return delta
}
