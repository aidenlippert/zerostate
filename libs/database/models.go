package database

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// FAANG-LEVEL DATABASE MODELS
// Following best practices:
// - Use UUID for all primary keys (better distribution, security)
// - Use sql.Null* types for optional fields
// - Use time.Time for timestamps with timezone support
// - Use json.RawMessage for JSONB flexibility
// - Implement proper validation at model layer
// - Include audit fields (created_at, updated_at) on all tables

// ============================================================================
// USERS & AUTHENTICATION
// ============================================================================

// User represents a user account in the system
type User struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	DID          string          `db:"did" json:"did"`
	Email        sql.NullString  `db:"email" json:"email,omitempty"`
	PasswordHash sql.NullString  `db:"password_hash" json:"-"` // Never expose password hash
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	LastLoginAt  sql.NullTime    `db:"last_login_at" json:"last_login_at,omitempty"`
	IsActive     bool            `db:"is_active" json:"is_active"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// Validate validates user model
func (u *User) Validate() error {
	if u.DID == "" {
		return ErrInvalidDID
	}
	if u.Email.Valid && !isValidEmail(u.Email.String) {
		return ErrInvalidEmail
	}
	return nil
}

// RefreshToken represents a JWT refresh token
type RefreshToken struct {
	ID        uuid.UUID      `db:"id" json:"id"`
	UserID    uuid.UUID      `db:"user_id" json:"user_id"`
	TokenHash string         `db:"token_hash" json:"-"` // Never expose token hash
	ExpiresAt time.Time      `db:"expires_at" json:"expires_at"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
	RevokedAt sql.NullTime   `db:"revoked_at" json:"revoked_at,omitempty"`
	IPAddress sql.NullString `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent sql.NullString `db:"user_agent" json:"user_agent,omitempty"`
}

// IsValid checks if token is still valid
func (rt *RefreshToken) IsValid() bool {
	return !rt.RevokedAt.Valid && time.Now().Before(rt.ExpiresAt)
}

// APIKey represents an API key for agent authentication
type APIKey struct {
	ID         uuid.UUID       `db:"id" json:"id"`
	UserID     uuid.UUID       `db:"user_id" json:"user_id"`
	KeyHash    string          `db:"key_hash" json:"-"` // Never expose key hash
	KeyPrefix  string          `db:"key_prefix" json:"key_prefix"`
	Name       sql.NullString  `db:"name" json:"name,omitempty"`
	Scopes     json.RawMessage `db:"scopes" json:"scopes"`
	CreatedAt  time.Time       `db:"created_at" json:"created_at"`
	ExpiresAt  sql.NullTime    `db:"expires_at" json:"expires_at,omitempty"`
	LastUsedAt sql.NullTime    `db:"last_used_at" json:"last_used_at,omitempty"`
	RevokedAt  sql.NullTime    `db:"revoked_at" json:"revoked_at,omitempty"`
	IsActive   bool            `db:"is_active" json:"is_active"`
}

// IsValid checks if API key is still valid
func (ak *APIKey) IsValid() bool {
	if !ak.IsActive || ak.RevokedAt.Valid {
		return false
	}
	if ak.ExpiresAt.Valid && time.Now().After(ak.ExpiresAt.Time) {
		return false
	}
	return true
}

// ============================================================================
// PAYMENT SYSTEM
// ============================================================================

// Account represents a user or agent account with balance
type Account struct {
	ID             uuid.UUID       `db:"id" json:"id"`
	DID            string          `db:"did" json:"did"`
	Balance        float64         `db:"balance" json:"balance"`
	TotalDeposited float64         `db:"total_deposited" json:"total_deposited"`
	TotalWithdrawn float64         `db:"total_withdrawn" json:"total_withdrawn"`
	CreatedAt      time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at" json:"updated_at"`
	Metadata       json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// ChannelState represents payment channel state
type ChannelState string

const (
	ChannelStateOpen     ChannelState = "open"
	ChannelStateEscrowed ChannelState = "escrowed"
	ChannelStateSettling ChannelState = "settling"
	ChannelStateClosed   ChannelState = "closed"
)

// PaymentChannel represents a payment channel between two parties
type PaymentChannel struct {
	ID             uuid.UUID      `db:"id" json:"id"`
	PayerDID       string         `db:"payer_did" json:"payer_did"`
	PayeeDID       string         `db:"payee_did" json:"payee_did"`
	AuctionID      sql.NullString `db:"auction_id" json:"auction_id,omitempty"`
	TotalDeposit   float64        `db:"total_deposit" json:"total_deposit"`
	CurrentBalance float64        `db:"current_balance" json:"current_balance"`
	EscrowedAmount float64        `db:"escrowed_amount" json:"escrowed_amount"`
	TotalSettled   float64        `db:"total_settled" json:"total_settled"`
	PendingRefund  float64        `db:"pending_refund" json:"pending_refund"`
	State          ChannelState   `db:"state" json:"state"`
	TaskID         sql.NullString `db:"task_id" json:"task_id,omitempty"`
	EscrowReleased bool           `db:"escrow_released" json:"escrow_released"`
	SequenceNumber int64          `db:"sequence_number" json:"sequence_number"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
	ClosedAt       sql.NullTime   `db:"closed_at" json:"closed_at,omitempty"`
}

// ChannelTransaction represents a transaction in a payment channel
type ChannelTransaction struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	ChannelID       uuid.UUID       `db:"channel_id" json:"channel_id"`
	TransactionType string          `db:"transaction_type" json:"transaction_type"`
	Amount          float64         `db:"amount" json:"amount"`
	TaskID          sql.NullString  `db:"task_id" json:"task_id,omitempty"`
	Reason          sql.NullString  `db:"reason" json:"reason,omitempty"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	Metadata        json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// ============================================================================
// MARKETPLACE & AGENTS
// ============================================================================

// AgentStatus represents agent availability status
type AgentStatus string

const (
	AgentStatusOnline      AgentStatus = "online"
	AgentStatusBusy        AgentStatus = "busy"
	AgentStatusOffline     AgentStatus = "offline"
	AgentStatusMaintenance AgentStatus = "maintenance"
)

// Agent represents an agent in the marketplace
type Agent struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	DID          string          `db:"did" json:"did"`
	Name         string          `db:"name" json:"name"`
	Description  sql.NullString  `db:"description" json:"description,omitempty"`
	Capabilities json.RawMessage `db:"capabilities" json:"capabilities"`
	PricingModel sql.NullString  `db:"pricing_model" json:"pricing_model,omitempty"`
	Status       AgentStatus     `db:"status" json:"status"`
	MaxCapacity  int             `db:"max_capacity" json:"max_capacity"`
	CurrentLoad  int             `db:"current_load" json:"current_load"`
	Region       sql.NullString  `db:"region" json:"region,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at" json:"updated_at"`
	LastSeenAt   sql.NullTime    `db:"last_seen_at" json:"last_seen_at,omitempty"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
	WasmHash     string          `db:"wasm_hash" json:"wasm_hash,omitempty"`
	S3Key        string          `db:"s3_key" json:"s3_key,omitempty"`

	// Backward compatibility fields for meta-agent (not stored in DB, computed on load)
	Price          float64 `db:"-" json:"-"` // Computed from PricingModel
	Rating         float64 `db:"-" json:"-"` // Computed from reputation system
	TasksCompleted int     `db:"-" json:"-"` // Computed from task history
	BinaryURL      string  `db:"-" json:"-"` // Computed from S3 storage path
	BinaryHash     string  `db:"-" json:"-"` // Computed from metadata or stored hash
}

// AuctionType represents type of auction
type AuctionType string

const (
	AuctionTypeFirstPrice  AuctionType = "first_price"
	AuctionTypeSecondPrice AuctionType = "second_price"
	AuctionTypeReserve     AuctionType = "reserve"
)

// AuctionStatus represents auction status
type AuctionStatus string

const (
	AuctionStatusOpen     AuctionStatus = "open"
	AuctionStatusClosed   AuctionStatus = "closed"
	AuctionStatusAwarded  AuctionStatus = "awarded"
	AuctionStatusCanceled AuctionStatus = "canceled"
	AuctionStatusExpired  AuctionStatus = "expired"
)

// Auction represents a task auction
type Auction struct {
	ID              uuid.UUID       `db:"id" json:"id"`
	TaskID          string          `db:"task_id" json:"task_id"`
	UserID          string          `db:"user_id" json:"user_id"`
	AuctionType     AuctionType     `db:"auction_type" json:"auction_type"`
	Status          AuctionStatus   `db:"status" json:"status"`
	DurationSeconds int             `db:"duration_seconds" json:"duration_seconds"`
	ExpiresAt       time.Time       `db:"expires_at" json:"expires_at"`
	ReservePrice    sql.NullFloat64 `db:"reserve_price" json:"reserve_price,omitempty"`
	MaxPrice        sql.NullFloat64 `db:"max_price" json:"max_price,omitempty"`
	MinReputation   sql.NullFloat64 `db:"min_reputation" json:"min_reputation,omitempty"`
	Capabilities    json.RawMessage `db:"capabilities" json:"capabilities"`
	WinningBidID    uuid.NullUUID   `db:"winning_bid_id" json:"winning_bid_id,omitempty"`
	FinalPrice      sql.NullFloat64 `db:"final_price" json:"final_price,omitempty"`
	CreatedAt       time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at" json:"updated_at"`
	Metadata        json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// Bid represents a bid on an auction
type Bid struct {
	ID                   uuid.UUID       `db:"id" json:"id"`
	AuctionID            uuid.UUID       `db:"auction_id" json:"auction_id"`
	AgentDID             string          `db:"agent_did" json:"agent_did"`
	Price                float64         `db:"price" json:"price"`
	EstimatedTimeSeconds sql.NullInt32   `db:"estimated_time_seconds" json:"estimated_time_seconds,omitempty"`
	ReputationScore      sql.NullFloat64 `db:"reputation_score" json:"reputation_score,omitempty"`
	QualityScore         sql.NullFloat64 `db:"quality_score" json:"quality_score,omitempty"`
	CompositeScore       sql.NullFloat64 `db:"composite_score" json:"composite_score,omitempty"`
	CreatedAt            time.Time       `db:"created_at" json:"created_at"`
}

// ============================================================================
// REPUTATION SYSTEM
// ============================================================================

// ReputationScore represents an agent's reputation
type ReputationScore struct {
	ID               uuid.UUID `db:"id" json:"id"`
	AgentDID         string    `db:"agent_did" json:"agent_did"`
	OverallScore     float64   `db:"overall_score" json:"overall_score"`
	ReliabilityScore float64   `db:"reliability_score" json:"reliability_score"`
	QualityScore     float64   `db:"quality_score" json:"quality_score"`
	SpeedScore       float64   `db:"speed_score" json:"speed_score"`
	TotalTasks       int       `db:"total_tasks" json:"total_tasks"`
	SuccessfulTasks  int       `db:"successful_tasks" json:"successful_tasks"`
	FailedTasks      int       `db:"failed_tasks" json:"failed_tasks"`
	CreatedAt        time.Time `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at" json:"updated_at"`
}

// SuccessRate calculates success rate percentage
func (rs *ReputationScore) SuccessRate() float64 {
	if rs.TotalTasks == 0 {
		return 0.0
	}
	return float64(rs.SuccessfulTasks) / float64(rs.TotalTasks) * 100.0
}

// ReputationEvent represents a reputation-affecting event
type ReputationEvent struct {
	ID         uuid.UUID       `db:"id" json:"id"`
	AgentDID   string          `db:"agent_did" json:"agent_did"`
	EventType  string          `db:"event_type" json:"event_type"`
	TaskID     sql.NullString  `db:"task_id" json:"task_id,omitempty"`
	ScoreDelta sql.NullFloat64 `db:"score_delta" json:"score_delta,omitempty"`
	CreatedAt  time.Time       `db:"created_at" json:"created_at"`
	Metadata   json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// ============================================================================
// TASK EXECUTION
// ============================================================================

// TaskStatus represents task execution status
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusAssigned  TaskStatus = "assigned"
	TaskStatusExecuting TaskStatus = "executing"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
)

// Task represents a task in the system
type Task struct {
	ID          uuid.UUID       `db:"id" json:"id"`
	TaskID      string          `db:"task_id" json:"task_id"`
	UserDID     string          `db:"user_did" json:"user_did"`
	AgentDID    sql.NullString  `db:"agent_did" json:"agent_did,omitempty"`
	TaskType    string          `db:"task_type" json:"task_type"`
	Status      TaskStatus      `db:"status" json:"status"`
	Input       json.RawMessage `db:"input" json:"input"`
	Output      json.RawMessage `db:"output" json:"output,omitempty"`
	Error       sql.NullString  `db:"error" json:"error,omitempty"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	StartedAt   sql.NullTime    `db:"started_at" json:"started_at,omitempty"`
	CompletedAt sql.NullTime    `db:"completed_at" json:"completed_at,omitempty"`
	TimeoutAt   sql.NullTime    `db:"timeout_at" json:"timeout_at,omitempty"`
	Metadata    json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// ============================================================================
// AUDIT & LOGGING
// ============================================================================

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           uuid.UUID       `db:"id" json:"id"`
	UserID       uuid.NullUUID   `db:"user_id" json:"user_id,omitempty"`
	Action       string          `db:"action" json:"action"`
	ResourceType sql.NullString  `db:"resource_type" json:"resource_type,omitempty"`
	ResourceID   sql.NullString  `db:"resource_id" json:"resource_id,omitempty"`
	IPAddress    sql.NullString  `db:"ip_address" json:"ip_address,omitempty"`
	UserAgent    sql.NullString  `db:"user_agent" json:"user_agent,omitempty"`
	RequestID    sql.NullString  `db:"request_id" json:"request_id,omitempty"`
	StatusCode   sql.NullInt32   `db:"status_code" json:"status_code,omitempty"`
	Error        sql.NullString  `db:"error" json:"error,omitempty"`
	CreatedAt    time.Time       `db:"created_at" json:"created_at"`
	Metadata     json.RawMessage `db:"metadata" json:"metadata,omitempty"`
}

// ============================================================================
// RATE LIMITING
// ============================================================================

// RateLimitBucket represents a rate limiting bucket
type RateLimitBucket struct {
	ID              uuid.UUID `db:"id" json:"id"`
	Key             string    `db:"key" json:"key"`
	Endpoint        string    `db:"endpoint" json:"endpoint"`
	TokensRemaining int       `db:"tokens_remaining" json:"tokens_remaining"`
	WindowStart     time.Time `db:"window_start" json:"window_start"`
	WindowEnd       time.Time `db:"window_end" json:"window_end"`
	CreatedAt       time.Time `db:"created_at" json:"created_at"`
	UpdatedAt       time.Time `db:"updated_at" json:"updated_at"`
}

// ============================================================================
// HELPER FUNCTIONS & VALIDATION
// ============================================================================

// Common validation errors
var (
	ErrInvalidDID   = NewValidationError("DID cannot be empty")
	ErrInvalidEmail = NewValidationError("invalid email format")
)

// ValidationError represents a model validation error
type ValidationError struct {
	message string
}

func (e *ValidationError) Error() string {
	return e.message
}

// NewValidationError creates a new validation error
func NewValidationError(message string) *ValidationError {
	return &ValidationError{message: message}
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	// Simple email validation - in production use more robust validation
	if email == "" {
		return false
	}
	// Basic check for @ symbol and domain
	atIndex := -1
	for i, c := range email {
		if c == '@' {
			if atIndex != -1 {
				return false // Multiple @ symbols
			}
			atIndex = i
		}
	}
	if atIndex <= 0 || atIndex >= len(email)-1 {
		return false
	}
	return true
}

// AgentDeployment placeholder for backward compatibility
type AgentDeployment struct {
	ID          string
	AgentID     string
	UserID      string
	Status      string
	Environment string
	Config      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
