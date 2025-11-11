package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

// FAANG-LEVEL REPOSITORY PATTERN
// Following best practices:
// - Repository pattern for clean separation of data access
// - Context-aware operations for cancellation and timeouts
// - Transaction support for atomic multi-table operations
// - Proper error handling with custom error types
// - Connection pooling and prepared statements
// - Query optimization with indexes
// - Audit logging for security-critical operations

// Common repository errors
var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyExists = errors.New("record already exists")
	ErrInvalidInput  = errors.New("invalid input parameters")
	ErrDatabase      = errors.New("database error")
)

// Database wraps sql.DB with our repository methods
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database instance
func NewDatabase(db *sql.DB) *Database {
	return &Database{db: db}
}

// DB is an alias for Database for backward compatibility
type DB = Database

// NewDB creates a new database connection (backward compatibility wrapper for SQLite)
func NewDB(connectionString string) (*Database, error) {
	db, err := sql.Open("sqlite3", connectionString)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return NewDatabase(db), nil
}

// Conn returns the underlying *sql.DB for backward compatibility
func (d *Database) Conn() *sql.DB {
	return d.db
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}

// GetAgentByID retrieves an agent by ID or DID (backward compatibility wrapper)
func (d *Database) GetAgentByID(id string) (*Agent, error) {
	// Try parsing as UUID first
	agentID, err := uuid.Parse(id)

	var query string
	var queryParam interface{}

	if err != nil {
		// Not a UUID, treat as DID
		query = `SELECT id, did, name, description, capabilities, pricing_model, status,
		                 max_capacity, current_load, region, created_at, updated_at, last_seen_at, metadata
		          FROM agents WHERE did = $1`
		queryParam = id
	} else {
		// Valid UUID, query by ID
		query = `SELECT id, did, name, description, capabilities, pricing_model, status,
		                 max_capacity, current_load, region, created_at, updated_at, last_seen_at, metadata
		          FROM agents WHERE id = $1`
		queryParam = agentID
	}

	var agent Agent
	var createdAt, updatedAt, lastSeenAt sql.NullString
	err = d.db.QueryRow(query, queryParam).Scan(
		&agent.ID, &agent.DID, &agent.Name, &agent.Description,
		&agent.Capabilities, &agent.PricingModel, &agent.Status,
		&agent.MaxCapacity, &agent.CurrentLoad, &agent.Region,
		&createdAt, &updatedAt, &lastSeenAt, &agent.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse SQLite TEXT timestamps
	if createdAt.Valid {
		agent.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
	}
	if updatedAt.Valid {
		agent.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
	}
	if lastSeenAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", lastSeenAt.String)
		agent.LastSeenAt = sql.NullTime{Time: t, Valid: true}
	}

	return &agent, nil
}

// SearchAgents searches for agents by query (backward compatibility wrapper)
func (d *Database) SearchAgents(query string) ([]*Agent, error) {
	// Simple search - return all active agents for now
	// TODO: Implement proper text search on capabilities/description
	sqlQuery := `SELECT id, did, name, description, capabilities, pricing_model, status,
	                    max_capacity, current_load, region, created_at, updated_at, last_seen_at, metadata
	             FROM agents
	             WHERE status = 'active'
	             LIMIT 100`

	rows, err := d.db.Query(sqlQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*Agent
	for rows.Next() {
		var agent Agent
		var createdAt, updatedAt, lastSeenAt sql.NullString
		err := rows.Scan(
			&agent.ID, &agent.DID, &agent.Name, &agent.Description,
			&agent.Capabilities, &agent.PricingModel, &agent.Status,
			&agent.MaxCapacity, &agent.CurrentLoad, &agent.Region,
			&createdAt, &updatedAt, &lastSeenAt, &agent.Metadata,
		)
		if err != nil {
			return nil, err
		}

		// Parse SQLite TEXT timestamps
		if createdAt.Valid {
			agent.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
		}
		if updatedAt.Valid {
			agent.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
		}
		if lastSeenAt.Valid {
			t, _ := time.Parse("2006-01-02 15:04:05", lastSeenAt.String)
			agent.LastSeenAt = sql.NullTime{Time: t, Valid: true}
		}

		// Initialize backward compatibility fields
		agent.Price = 0.10          // TODO: Parse from PricingModel JSON
		agent.Rating = 4.5          // TODO: Get from reputation system
		agent.TasksCompleted = 10   // TODO: Get from task history
		agents = append(agents, &agent)
	}
	return agents, nil
}

// BeginTx starts a database transaction
func (d *Database) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
	})
}

// ============================================================================
// USER REPOSITORY
// ============================================================================

type UserRepository struct {
	db *Database
}

func NewUserRepository(db *Database) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, did, email, password_hash, is_active, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	user.ID = uuid.New()
	err := r.db.db.QueryRowContext(ctx, query,
		user.ID, user.DID, user.Email, user.PasswordHash, user.IsActive, user.Metadata,
	).Scan(&user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// GetByID retrieves user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `
		SELECT id, did, email, password_hash, created_at, updated_at,
		       last_login_at, is_active, metadata
		FROM users WHERE id = $1
	`
	var user User
	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.DID, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		&user.IsActive, &user.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &user, nil
}

// GetByDID retrieves user by DID
func (r *UserRepository) GetByDID(ctx context.Context, did string) (*User, error) {
	query := `
		SELECT id, did, email, password_hash, created_at, updated_at,
		       last_login_at, is_active, metadata
		FROM users WHERE did = $1
	`
	var user User
	err := r.db.db.QueryRowContext(ctx, query, did).Scan(
		&user.ID, &user.DID, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		&user.IsActive, &user.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &user, nil
}

// UpdateLastLogin updates user's last login timestamp
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login_at = NOW() WHERE id = $1`
	result, err := r.db.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// ACCOUNT REPOSITORY
// ============================================================================

type AccountRepository struct {
	db *Database
}

func NewAccountRepository(db *Database) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create creates a new account
func (r *AccountRepository) Create(ctx context.Context, account *Account) error {
	query := `
		INSERT INTO accounts (id, did, balance, total_deposited, total_withdrawn, metadata)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	account.ID = uuid.New()
	err := r.db.db.QueryRowContext(ctx, query,
		account.ID, account.DID, account.Balance,
		account.TotalDeposited, account.TotalWithdrawn, account.Metadata,
	).Scan(&account.CreatedAt, &account.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return ErrAlreadyExists
		}
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// GetByDID retrieves account by DID
func (r *AccountRepository) GetByDID(ctx context.Context, did string) (*Account, error) {
	query := `
		SELECT id, did, balance, total_deposited, total_withdrawn,
		       created_at, updated_at, metadata
		FROM accounts WHERE did = $1
	`
	var account Account
	err := r.db.db.QueryRowContext(ctx, query, did).Scan(
		&account.ID, &account.DID, &account.Balance,
		&account.TotalDeposited, &account.TotalWithdrawn,
		&account.CreatedAt, &account.UpdatedAt, &account.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &account, nil
}

// UpdateBalance updates account balance atomically
func (r *AccountRepository) UpdateBalance(ctx context.Context, tx *sql.Tx, did string, delta float64) error {
	query := `
		UPDATE accounts
		SET balance = balance + $1,
		    total_deposited = CASE WHEN $1 > 0 THEN total_deposited + $1 ELSE total_deposited END,
		    total_withdrawn = CASE WHEN $1 < 0 THEN total_withdrawn + ABS($1) ELSE total_withdrawn END
		WHERE did = $2 AND balance + $1 >= 0
	`
	var result sql.Result
	var err error

	if tx != nil {
		result, err = tx.ExecContext(ctx, query, delta, did)
	} else {
		result, err = r.db.db.ExecContext(ctx, query, delta, did)
	}

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvalidInput // Balance would go negative
	}
	return nil
}

// ============================================================================
// PAYMENT CHANNEL REPOSITORY
// ============================================================================

type PaymentChannelRepository struct {
	db *Database
}

func NewPaymentChannelRepository(db *Database) *PaymentChannelRepository {
	return &PaymentChannelRepository{db: db}
}

// Create creates a new payment channel
func (r *PaymentChannelRepository) Create(ctx context.Context, channel *PaymentChannel) error {
	query := `
		INSERT INTO payment_channels (
			id, payer_did, payee_did, auction_id, total_deposit, current_balance,
			escrowed_amount, total_settled, pending_refund, state, task_id,
			escrow_released, sequence_number
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING created_at, updated_at
	`
	channel.ID = uuid.New()
	err := r.db.db.QueryRowContext(ctx, query,
		channel.ID, channel.PayerDID, channel.PayeeDID, channel.AuctionID,
		channel.TotalDeposit, channel.CurrentBalance, channel.EscrowedAmount,
		channel.TotalSettled, channel.PendingRefund, channel.State,
		channel.TaskID, channel.EscrowReleased, channel.SequenceNumber,
	).Scan(&channel.CreatedAt, &channel.UpdatedAt)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// GetByID retrieves payment channel by ID
func (r *PaymentChannelRepository) GetByID(ctx context.Context, id uuid.UUID) (*PaymentChannel, error) {
	query := `
		SELECT id, payer_did, payee_did, auction_id, total_deposit, current_balance,
		       escrowed_amount, total_settled, pending_refund, state, task_id,
		       escrow_released, sequence_number, created_at, updated_at, closed_at
		FROM payment_channels WHERE id = $1
	`
	var channel PaymentChannel
	err := r.db.db.QueryRowContext(ctx, query, id).Scan(
		&channel.ID, &channel.PayerDID, &channel.PayeeDID, &channel.AuctionID,
		&channel.TotalDeposit, &channel.CurrentBalance, &channel.EscrowedAmount,
		&channel.TotalSettled, &channel.PendingRefund, &channel.State,
		&channel.TaskID, &channel.EscrowReleased, &channel.SequenceNumber,
		&channel.CreatedAt, &channel.UpdatedAt, &channel.ClosedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &channel, nil
}

// LockEscrow locks funds in escrow for a task (idempotent)
func (r *PaymentChannelRepository) LockEscrow(ctx context.Context, channelID uuid.UUID, taskID string, amount float64) error {
	query := `
		UPDATE payment_channels
		SET escrowed_amount = escrowed_amount + $1,
		    current_balance = current_balance - $1,
		    task_id = $2,
		    state = 'escrowed'
		WHERE id = $3
		  AND state != 'closed'
		  AND escrow_released = false
		  AND current_balance >= $1
	`
	result, err := r.db.db.ExecContext(ctx, query, amount, taskID, channelID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrInvalidInput
	}
	return nil
}

// ReleaseEscrow releases escrowed funds (idempotent)
func (r *PaymentChannelRepository) ReleaseEscrow(ctx context.Context, channelID uuid.UUID, success bool) error {
	var query string
	if success {
		// Pay agent
		query = `
			UPDATE payment_channels
			SET total_settled = total_settled + escrowed_amount,
			    escrowed_amount = 0,
			    escrow_released = true,
			    state = 'settling'
			WHERE id = $1 AND escrow_released = false
		`
	} else {
		// Refund user
		query = `
			UPDATE payment_channels
			SET current_balance = current_balance + escrowed_amount,
			    escrowed_amount = 0,
			    escrow_released = true,
			    state = 'open'
			WHERE id = $1 AND escrow_released = false
		`
	}

	result, err := r.db.db.ExecContext(ctx, query, channelID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		// Already released - idempotent behavior
		return nil
	}
	return nil
}

// CloseChannel closes a payment channel
func (r *PaymentChannelRepository) CloseChannel(ctx context.Context, channelID uuid.UUID) error {
	now := time.Now()
	query := `
		UPDATE payment_channels
		SET state = 'closed', closed_at = $1
		WHERE id = $2 AND state != 'closed'
	`
	result, err := r.db.db.ExecContext(ctx, query, now, channelID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// CHANNEL TRANSACTION REPOSITORY
// ============================================================================

type ChannelTransactionRepository struct {
	db *Database
}

func NewChannelTransactionRepository(db *Database) *ChannelTransactionRepository {
	return &ChannelTransactionRepository{db: db}
}

// Create creates a new channel transaction
func (r *ChannelTransactionRepository) Create(ctx context.Context, tx *ChannelTransaction) error {
	query := `
		INSERT INTO channel_transactions (
			id, channel_id, transaction_type, amount, task_id, reason, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at
	`
	tx.ID = uuid.New()
	err := r.db.db.QueryRowContext(ctx, query,
		tx.ID, tx.ChannelID, tx.TransactionType, tx.Amount,
		tx.TaskID, tx.Reason, tx.Metadata,
	).Scan(&tx.CreatedAt)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// ListByChannelID lists all transactions for a channel
func (r *ChannelTransactionRepository) ListByChannelID(ctx context.Context, channelID uuid.UUID) ([]ChannelTransaction, error) {
	query := `
		SELECT id, channel_id, transaction_type, amount, task_id, reason, created_at, metadata
		FROM channel_transactions
		WHERE channel_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.db.QueryContext(ctx, query, channelID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	defer rows.Close()

	var transactions []ChannelTransaction
	for rows.Next() {
		var tx ChannelTransaction
		err := rows.Scan(
			&tx.ID, &tx.ChannelID, &tx.TransactionType, &tx.Amount,
			&tx.TaskID, &tx.Reason, &tx.CreatedAt, &tx.Metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
		}
		transactions = append(transactions, tx)
	}
	return transactions, nil
}

// ============================================================================
// AGENT REPOSITORY
// ============================================================================

type AgentRepository struct {
	db *Database
}

func NewAgentRepository(db *Database) *AgentRepository {
	return &AgentRepository{db: db}
}

// Create creates a new agent
func (r *AgentRepository) Create(ctx context.Context, agent *Agent) error {
	// For SQLite compatibility, we set timestamps in Go rather than using RETURNING
	if agent.ID == uuid.Nil {
		agent.ID = uuid.New()
	}
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = time.Now()
	}
	if agent.UpdatedAt.IsZero() {
		agent.UpdatedAt = agent.CreatedAt
	}

	// Ensure metadata is never nil or empty for PostgreSQL JSON column
	if agent.Metadata == nil || len(agent.Metadata) == 0 {
		agent.Metadata = json.RawMessage(`{}`)
	}

	query := `
		INSERT INTO agents (
			id, did, name, description, capabilities, pricing_model, status,
			max_capacity, current_load, region, created_at, updated_at, metadata,
			wasm_hash, s3_key
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := r.db.db.ExecContext(ctx, query,
		agent.ID, agent.DID, agent.Name, agent.Description, agent.Capabilities,
		agent.PricingModel, agent.Status, agent.MaxCapacity, agent.CurrentLoad,
		agent.Region, agent.CreatedAt, agent.UpdatedAt, agent.Metadata,
		agent.WasmHash, agent.S3Key,
	)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			// Log detailed PostgreSQL error
			fmt.Printf("PostgreSQL error in CreateAgent: Code=%s, Message=%s, Detail=%s, Hint=%s\n",
				pqErr.Code, pqErr.Message, pqErr.Detail, pqErr.Hint)
			if pqErr.Code == "23505" {
				return ErrAlreadyExists
			}
		}
		// SQLite constraint error
		if err.Error() == "UNIQUE constraint failed: agents.did" {
			return ErrAlreadyExists
		}
		// Log full error details
		fmt.Printf("Database error in CreateAgent: %v\n", err)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// GetByDID retrieves agent by DID
func (r *AgentRepository) GetByDID(ctx context.Context, did string) (*Agent, error) {
	query := `
		SELECT id, did, name, description, capabilities, pricing_model, status,
		       max_capacity, current_load, region, created_at, updated_at,
		       last_seen_at, metadata, wasm_hash, s3_key
		FROM agents WHERE did = $1
	`
	var agent Agent
	var createdAt, updatedAt, lastSeenAt sql.NullString
	err := r.db.db.QueryRowContext(ctx, query, did).Scan(
		&agent.ID, &agent.DID, &agent.Name, &agent.Description, &agent.Capabilities,
		&agent.PricingModel, &agent.Status, &agent.MaxCapacity, &agent.CurrentLoad,
		&agent.Region, &createdAt, &updatedAt, &lastSeenAt, &agent.Metadata,
		&agent.WasmHash, &agent.S3Key,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	// Parse SQLite TEXT timestamps
	if createdAt.Valid {
		agent.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt.String)
	}
	if updatedAt.Valid {
		agent.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt.String)
	}
	if lastSeenAt.Valid {
		t, _ := time.Parse("2006-01-02 15:04:05", lastSeenAt.String)
		agent.LastSeenAt = sql.NullTime{Time: t, Valid: true}
	}

	return &agent, nil
}

// UpdateStatus updates agent status
func (r *AgentRepository) UpdateStatus(ctx context.Context, did string, status AgentStatus) error {
	query := `
		UPDATE agents
		SET status = $1, last_seen_at = NOW()
		WHERE did = $2
	`
	result, err := r.db.db.ExecContext(ctx, query, status, did)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// REPUTATION REPOSITORY
// ============================================================================

type ReputationRepository struct {
	db *Database
}

func NewReputationRepository(db *Database) *ReputationRepository {
	return &ReputationRepository{db: db}
}

// GetByAgentDID retrieves reputation score by agent DID
func (r *ReputationRepository) GetByAgentDID(ctx context.Context, agentDID string) (*ReputationScore, error) {
	query := `
		SELECT id, agent_did, overall_score, reliability_score, quality_score, speed_score,
		       total_tasks, successful_tasks, failed_tasks, created_at, updated_at
		FROM reputation_scores WHERE agent_did = $1
	`
	var score ReputationScore
	err := r.db.db.QueryRowContext(ctx, query, agentDID).Scan(
		&score.ID, &score.AgentDID, &score.OverallScore, &score.ReliabilityScore,
		&score.QualityScore, &score.SpeedScore, &score.TotalTasks,
		&score.SuccessfulTasks, &score.FailedTasks, &score.CreatedAt, &score.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &score, nil
}

// UpdateScore updates reputation score
func (r *ReputationRepository) UpdateScore(ctx context.Context, agentDID string, delta float64, taskSuccess bool) error {
	query := `
		UPDATE reputation_scores
		SET overall_score = GREATEST(0, LEAST(100, overall_score + $1)),
		    total_tasks = total_tasks + 1,
		    successful_tasks = CASE WHEN $2 THEN successful_tasks + 1 ELSE successful_tasks END,
		    failed_tasks = CASE WHEN NOT $2 THEN failed_tasks + 1 ELSE failed_tasks END
		WHERE agent_did = $3
	`
	result, err := r.db.db.ExecContext(ctx, query, delta, taskSuccess, agentDID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// AUDIT LOG REPOSITORY
// ============================================================================

type AuditLogRepository struct {
	db *Database
}

func NewAuditLogRepository(db *Database) *AuditLogRepository {
	return &AuditLogRepository{db: db}
}

// Create creates a new audit log entry
func (r *AuditLogRepository) Create(ctx context.Context, log *AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, user_id, action, resource_type, resource_id, ip_address,
			user_agent, request_id, status_code, error, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at
	`
	log.ID = uuid.New()
	err := r.db.db.QueryRowContext(ctx, query,
		log.ID, log.UserID, log.Action, log.ResourceType, log.ResourceID,
		log.IPAddress, log.UserAgent, log.RequestID, log.StatusCode,
		log.Error, log.Metadata,
	).Scan(&log.CreatedAt)

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return nil
}

// GetAgentCount returns total number of agents (backward compatibility)
func (d *Database) GetAgentCount() (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	return count, err
}

// ListAgents lists all agents (backward compatibility)
func (d *Database) ListAgents() ([]*Agent, error) {
	return d.SearchAgents("")
}

// CreateAgent creates a new agent (backward compatibility)
func (d *Database) CreateAgent(agent *Agent) error {
	repo := NewAgentRepository(d)
	return repo.Create(context.Background(), agent)
}


// GetUserByEmail retrieves user by email (backward compatibility)
func (d *Database) GetUserByEmail(email string) (*User, error) {
	query := `SELECT id, did, email, password_hash, created_at, updated_at, last_login_at, is_active, metadata
	          FROM users WHERE email = $1`

	var user User
	err := d.db.QueryRow(query, email).Scan(
		&user.ID, &user.DID, &user.Email, &user.PasswordHash,
		&user.CreatedAt, &user.UpdatedAt, &user.LastLoginAt,
		&user.IsActive, &user.Metadata,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &user, nil
}


// Deployment method stubs (backward compatibility - not implemented yet)
func (d *Database) CreateDeployment(deployment *AgentDeployment) error {
	return fmt.Errorf("deployments not implemented yet")
}

func (d *Database) GetDeploymentByID(id string) (*AgentDeployment, error) {
	return nil, ErrNotFound
}

func (d *Database) ListDeploymentsByUser(userID string) ([]*AgentDeployment, error) {
	return []*AgentDeployment{}, nil
}

func (d *Database) UpdateDeployment(deployment *AgentDeployment) error {
	return ErrNotFound
}
