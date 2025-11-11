package economic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EscrowStatus represents the state of an escrow transaction
type EscrowStatus string

const (
	EscrowStatusCreated   EscrowStatus = "created"   // Initial state
	EscrowStatusFunded    EscrowStatus = "funded"    // Funds locked in escrow
	EscrowStatusReleased  EscrowStatus = "released"  // Funds released to recipient
	EscrowStatusRefunded  EscrowStatus = "refunded"  // Funds returned to sender
	EscrowStatusDisputed  EscrowStatus = "disputed"  // Under dispute resolution
	EscrowStatusCancelled EscrowStatus = "cancelled" // Cancelled before funding
)

// DisputeStatus represents the state of a dispute
type DisputeStatus string

const (
	DisputeStatusOpen     DisputeStatus = "open"     // Dispute opened
	DisputeStatusReviewing DisputeStatus = "reviewing" // Under review
	DisputeStatusResolved DisputeStatus = "resolved" // Dispute resolved
	DisputeStatusClosed   DisputeStatus = "closed"   // Dispute closed
)

// Escrow represents an escrow transaction
type Escrow struct {
	ID              uuid.UUID
	TaskID          string
	PayerID         string
	PayeeID         string
	Amount          float64
	Status          EscrowStatus
	FundedAt        *time.Time
	ReleasedAt      *time.Time
	RefundedAt      *time.Time
	DisputeID       *uuid.UUID
	ExpiresAt       time.Time
	AutoReleaseAt   *time.Time
	Conditions      string // JSON conditions for automatic release
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Error           *string
}

// Dispute represents a dispute on an escrow transaction
type Dispute struct {
	ID            uuid.UUID
	EscrowID      uuid.UUID
	InitiatorID   string
	Reason        string
	Status        DisputeStatus
	ReviewerID    *string
	Resolution    *string
	ResolvedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DisputeEvidence represents evidence submitted for a dispute
type DisputeEvidence struct {
	ID          uuid.UUID
	DisputeID   uuid.UUID
	SubmitterID string
	EvidenceType string // "text", "file", "screenshot", "log"
	Content     string
	FileURL     *string
	CreatedAt   time.Time
}

// EscrowService handles escrow transactions and dispute resolution
type EscrowService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewEscrowService creates a new escrow service
func NewEscrowService(db *sql.DB, logger *zap.Logger) *EscrowService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &EscrowService{
		db:     db,
		logger: logger,
	}
}

// CreateEscrow creates a new escrow transaction
func (s *EscrowService) CreateEscrow(
	ctx context.Context,
	taskID string,
	payerID string,
	payeeID string,
	amount float64,
	expirationMinutes int,
	autoReleaseMinutes *int,
	conditions string,
) (*Escrow, error) {
	escrowID := uuid.New()
	now := time.Now()
	expiresAt := now.Add(time.Duration(expirationMinutes) * time.Minute)

	var autoReleaseAt *time.Time
	if autoReleaseMinutes != nil {
		releaseTime := now.Add(time.Duration(*autoReleaseMinutes) * time.Minute)
		autoReleaseAt = &releaseTime
	}

	query := `
		INSERT INTO escrows (
			id, task_id, payer_id, payee_id, amount, status,
			expires_at, auto_release_at, conditions,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	_, err := s.db.ExecContext(ctx, query,
		escrowID, taskID, payerID, payeeID, amount, EscrowStatusCreated,
		expiresAt, autoReleaseAt, conditions, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create escrow: %w", err)
	}

	escrow := &Escrow{
		ID:            escrowID,
		TaskID:        taskID,
		PayerID:       payerID,
		PayeeID:       payeeID,
		Amount:        amount,
		Status:        EscrowStatusCreated,
		ExpiresAt:     expiresAt,
		AutoReleaseAt: autoReleaseAt,
		Conditions:    conditions,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.logger.Info("escrow created",
		zap.String("escrow_id", escrowID.String()),
		zap.String("task_id", taskID),
		zap.Float64("amount", amount),
	)

	return escrow, nil
}

// FundEscrow marks an escrow as funded (transition: created → funded)
func (s *EscrowService) FundEscrow(ctx context.Context, escrowID uuid.UUID, signature string) error {
	now := time.Now()

	// Verify current status is created
	var currentStatus EscrowStatus
	err := s.db.QueryRowContext(ctx,
		"SELECT status FROM escrows WHERE id = $1",
		escrowID,
	).Scan(&currentStatus)

	if err == sql.ErrNoRows {
		return fmt.Errorf("escrow not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get escrow status: %w", err)
	}

	if currentStatus != EscrowStatusCreated {
		return fmt.Errorf("invalid state transition: escrow status is %s, expected created", currentStatus)
	}

	query := `
		UPDATE escrows
		SET status = $1,
		    funded_at = $2,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = s.db.ExecContext(ctx, query, EscrowStatusFunded, now, escrowID)
	if err != nil {
		return fmt.Errorf("failed to fund escrow: %w", err)
	}

	s.logger.Info("escrow funded",
		zap.String("escrow_id", escrowID.String()),
		zap.Time("funded_at", now),
	)

	return nil
}

// ReleaseEscrow releases funds to payee (transition: funded → released)
func (s *EscrowService) ReleaseEscrow(ctx context.Context, escrowID uuid.UUID, releasedBy string) error {
	now := time.Now()

	// Verify current status is funded
	var currentStatus EscrowStatus
	var payerID string
	err := s.db.QueryRowContext(ctx,
		"SELECT status, payer_id FROM escrows WHERE id = $1",
		escrowID,
	).Scan(&currentStatus, &payerID)

	if err == sql.ErrNoRows {
		return fmt.Errorf("escrow not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get escrow: %w", err)
	}

	if currentStatus != EscrowStatusFunded {
		return fmt.Errorf("invalid state transition: escrow status is %s, expected funded", currentStatus)
	}

	// Only payer can release funds (or system for auto-release)
	if releasedBy != payerID && releasedBy != "system" {
		return fmt.Errorf("unauthorized: only payer or system can release escrow")
	}

	query := `
		UPDATE escrows
		SET status = $1,
		    released_at = $2,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = s.db.ExecContext(ctx, query, EscrowStatusReleased, now, escrowID)
	if err != nil {
		return fmt.Errorf("failed to release escrow: %w", err)
	}

	s.logger.Info("escrow released",
		zap.String("escrow_id", escrowID.String()),
		zap.String("released_by", releasedBy),
		zap.Time("released_at", now),
	)

	return nil
}

// RefundEscrow refunds funds to payer (transition: funded/disputed → refunded)
func (s *EscrowService) RefundEscrow(ctx context.Context, escrowID uuid.UUID, refundedBy string) error {
	now := time.Now()

	// Verify current status is funded or disputed
	var currentStatus EscrowStatus
	var payeeID string
	err := s.db.QueryRowContext(ctx,
		"SELECT status, payee_id FROM escrows WHERE id = $1",
		escrowID,
	).Scan(&currentStatus, &payeeID)

	if err == sql.ErrNoRows {
		return fmt.Errorf("escrow not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get escrow: %w", err)
	}

	if currentStatus != EscrowStatusFunded && currentStatus != EscrowStatusDisputed {
		return fmt.Errorf("invalid state transition: escrow status is %s, expected funded or disputed", currentStatus)
	}

	// Only payee or system can initiate refund
	if refundedBy != payeeID && refundedBy != "system" {
		return fmt.Errorf("unauthorized: only payee or system can refund escrow")
	}

	query := `
		UPDATE escrows
		SET status = $1,
		    refunded_at = $2,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = s.db.ExecContext(ctx, query, EscrowStatusRefunded, now, escrowID)
	if err != nil {
		return fmt.Errorf("failed to refund escrow: %w", err)
	}

	s.logger.Info("escrow refunded",
		zap.String("escrow_id", escrowID.String()),
		zap.String("refunded_by", refundedBy),
		zap.Time("refunded_at", now),
	)

	return nil
}

// OpenDispute opens a dispute on an escrow (transition: funded → disputed)
func (s *EscrowService) OpenDispute(
	ctx context.Context,
	escrowID uuid.UUID,
	initiatorID string,
	reason string,
) (*Dispute, error) {
	disputeID := uuid.New()
	now := time.Now()

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Verify escrow is funded
	var currentStatus EscrowStatus
	var payerID, payeeID string
	err = tx.QueryRowContext(ctx,
		"SELECT status, payer_id, payee_id FROM escrows WHERE id = $1",
		escrowID,
	).Scan(&currentStatus, &payerID, &payeeID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("escrow not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow: %w", err)
	}

	if currentStatus != EscrowStatusFunded {
		return nil, fmt.Errorf("cannot dispute escrow with status: %s", currentStatus)
	}

	// Verify initiator is either payer or payee
	if initiatorID != payerID && initiatorID != payeeID {
		return nil, fmt.Errorf("unauthorized: only payer or payee can open dispute")
	}

	// Create dispute record
	disputeQuery := `
		INSERT INTO disputes (
			id, escrow_id, initiator_id, reason, status,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = tx.ExecContext(ctx, disputeQuery,
		disputeID, escrowID, initiatorID, reason, DisputeStatusOpen,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create dispute: %w", err)
	}

	// Update escrow status to disputed
	escrowQuery := `
		UPDATE escrows
		SET status = $1,
		    dispute_id = $2,
		    updated_at = $3
		WHERE id = $4
	`

	_, err = tx.ExecContext(ctx, escrowQuery,
		EscrowStatusDisputed, disputeID, now, escrowID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update escrow: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	dispute := &Dispute{
		ID:          disputeID,
		EscrowID:    escrowID,
		InitiatorID: initiatorID,
		Reason:      reason,
		Status:      DisputeStatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.logger.Info("dispute opened",
		zap.String("dispute_id", disputeID.String()),
		zap.String("escrow_id", escrowID.String()),
		zap.String("initiator_id", initiatorID),
	)

	return dispute, nil
}

// SubmitEvidence submits evidence for a dispute
func (s *EscrowService) SubmitEvidence(
	ctx context.Context,
	disputeID uuid.UUID,
	submitterID string,
	evidenceType string,
	content string,
	fileURL *string,
) (*DisputeEvidence, error) {
	evidenceID := uuid.New()
	now := time.Now()

	// Verify dispute exists and is open
	var disputeStatus DisputeStatus
	var escrowID uuid.UUID
	var initiatorID string
	err := s.db.QueryRowContext(ctx,
		"SELECT status, escrow_id, initiator_id FROM disputes WHERE id = $1",
		disputeID,
	).Scan(&disputeStatus, &escrowID, &initiatorID)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dispute not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dispute: %w", err)
	}

	if disputeStatus != DisputeStatusOpen && disputeStatus != DisputeStatusReviewing {
		return nil, fmt.Errorf("cannot submit evidence: dispute status is %s", disputeStatus)
	}

	// Verify submitter is involved in the escrow
	var payerID, payeeID string
	err = s.db.QueryRowContext(ctx,
		"SELECT payer_id, payee_id FROM escrows WHERE id = $1",
		escrowID,
	).Scan(&payerID, &payeeID)

	if err != nil {
		return nil, fmt.Errorf("failed to get escrow: %w", err)
	}

	if submitterID != payerID && submitterID != payeeID {
		return nil, fmt.Errorf("unauthorized: only payer or payee can submit evidence")
	}

	query := `
		INSERT INTO dispute_evidence (
			id, dispute_id, submitter_id, evidence_type,
			content, file_url, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err = s.db.ExecContext(ctx, query,
		evidenceID, disputeID, submitterID, evidenceType,
		content, fileURL, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit evidence: %w", err)
	}

	evidence := &DisputeEvidence{
		ID:           evidenceID,
		DisputeID:    disputeID,
		SubmitterID:  submitterID,
		EvidenceType: evidenceType,
		Content:      content,
		FileURL:      fileURL,
		CreatedAt:    now,
	}

	s.logger.Info("evidence submitted",
		zap.String("evidence_id", evidenceID.String()),
		zap.String("dispute_id", disputeID.String()),
		zap.String("submitter_id", submitterID),
		zap.String("type", evidenceType),
	)

	return evidence, nil
}

// ResolveDispute resolves a dispute
func (s *EscrowService) ResolveDispute(
	ctx context.Context,
	disputeID uuid.UUID,
	reviewerID string,
	resolution string,
	outcome string, // "release" or "refund"
) error {
	now := time.Now()

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get dispute and verify status
	var disputeStatus DisputeStatus
	var escrowID uuid.UUID
	err = tx.QueryRowContext(ctx,
		"SELECT status, escrow_id FROM disputes WHERE id = $1",
		disputeID,
	).Scan(&disputeStatus, &escrowID)

	if err == sql.ErrNoRows {
		return fmt.Errorf("dispute not found")
	}
	if err != nil {
		return fmt.Errorf("failed to get dispute: %w", err)
	}

	if disputeStatus == DisputeStatusResolved || disputeStatus == DisputeStatusClosed {
		return fmt.Errorf("dispute already resolved")
	}

	// Update dispute
	disputeQuery := `
		UPDATE disputes
		SET status = $1,
		    reviewer_id = $2,
		    resolution = $3,
		    resolved_at = $4,
		    updated_at = $4
		WHERE id = $5
	`

	_, err = tx.ExecContext(ctx, disputeQuery,
		DisputeStatusResolved, reviewerID, resolution, now, disputeID,
	)
	if err != nil {
		return fmt.Errorf("failed to update dispute: %w", err)
	}

	// Apply outcome to escrow
	var escrowQuery string
	var newStatus EscrowStatus

	if outcome == "release" {
		newStatus = EscrowStatusReleased
		escrowQuery = `
			UPDATE escrows
			SET status = $1,
			    released_at = $2,
			    updated_at = $2
			WHERE id = $3
		`
	} else if outcome == "refund" {
		newStatus = EscrowStatusRefunded
		escrowQuery = `
			UPDATE escrows
			SET status = $1,
			    refunded_at = $2,
			    updated_at = $2
			WHERE id = $3
		`
	} else {
		return fmt.Errorf("invalid outcome: must be 'release' or 'refund'")
	}

	_, err = tx.ExecContext(ctx, escrowQuery, newStatus, now, escrowID)
	if err != nil {
		return fmt.Errorf("failed to update escrow: %w", err)
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("dispute resolved",
		zap.String("dispute_id", disputeID.String()),
		zap.String("escrow_id", escrowID.String()),
		zap.String("reviewer_id", reviewerID),
		zap.String("outcome", outcome),
	)

	return nil
}

// GetEscrow retrieves an escrow by ID
func (s *EscrowService) GetEscrow(ctx context.Context, escrowID uuid.UUID) (*Escrow, error) {
	query := `
		SELECT id, task_id, payer_id, payee_id, amount, status,
		       funded_at, released_at, refunded_at, dispute_id,
		       expires_at, auto_release_at, conditions,
		       created_at, updated_at, error
		FROM escrows
		WHERE id = $1
	`

	var e Escrow
	err := s.db.QueryRowContext(ctx, query, escrowID).Scan(
		&e.ID, &e.TaskID, &e.PayerID, &e.PayeeID, &e.Amount, &e.Status,
		&e.FundedAt, &e.ReleasedAt, &e.RefundedAt, &e.DisputeID,
		&e.ExpiresAt, &e.AutoReleaseAt, &e.Conditions,
		&e.CreatedAt, &e.UpdatedAt, &e.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("escrow not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow: %w", err)
	}

	return &e, nil
}

// GetEscrowByTaskID retrieves an escrow by task ID
func (s *EscrowService) GetEscrowByTaskID(ctx context.Context, taskID string) (*Escrow, error) {
	query := `
		SELECT id, task_id, payer_id, payee_id, amount, status,
		       funded_at, released_at, refunded_at, dispute_id,
		       expires_at, auto_release_at, conditions,
		       created_at, updated_at, error
		FROM escrows
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var e Escrow
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&e.ID, &e.TaskID, &e.PayerID, &e.PayeeID, &e.Amount, &e.Status,
		&e.FundedAt, &e.ReleasedAt, &e.RefundedAt, &e.DisputeID,
		&e.ExpiresAt, &e.AutoReleaseAt, &e.Conditions,
		&e.CreatedAt, &e.UpdatedAt, &e.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("escrow not found for task %s", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get escrow: %w", err)
	}

	return &e, nil
}

// GetDispute retrieves a dispute by ID
func (s *EscrowService) GetDispute(ctx context.Context, disputeID uuid.UUID) (*Dispute, error) {
	query := `
		SELECT id, escrow_id, initiator_id, reason, status,
		       reviewer_id, resolution, resolved_at,
		       created_at, updated_at
		FROM disputes
		WHERE id = $1
	`

	var d Dispute
	err := s.db.QueryRowContext(ctx, query, disputeID).Scan(
		&d.ID, &d.EscrowID, &d.InitiatorID, &d.Reason, &d.Status,
		&d.ReviewerID, &d.Resolution, &d.ResolvedAt,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("dispute not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get dispute: %w", err)
	}

	return &d, nil
}

// GetDisputeEvidence retrieves all evidence for a dispute
func (s *EscrowService) GetDisputeEvidence(ctx context.Context, disputeID uuid.UUID) ([]DisputeEvidence, error) {
	query := `
		SELECT id, dispute_id, submitter_id, evidence_type,
		       content, file_url, created_at
		FROM dispute_evidence
		WHERE dispute_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, disputeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query evidence: %w", err)
	}
	defer rows.Close()

	evidence := make([]DisputeEvidence, 0)
	for rows.Next() {
		var e DisputeEvidence
		err = rows.Scan(
			&e.ID, &e.DisputeID, &e.SubmitterID, &e.EvidenceType,
			&e.Content, &e.FileURL, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan evidence: %w", err)
		}
		evidence = append(evidence, e)
	}

	return evidence, nil
}

// ProcessAutoReleases processes escrows that should be auto-released
func (s *EscrowService) ProcessAutoReleases(ctx context.Context) (int, error) {
	now := time.Now()

	// Find escrows that are funded and past auto-release time
	query := `
		SELECT id
		FROM escrows
		WHERE status = $1
		  AND auto_release_at IS NOT NULL
		  AND auto_release_at <= $2
	`

	rows, err := s.db.QueryContext(ctx, query, EscrowStatusFunded, now)
	if err != nil {
		return 0, fmt.Errorf("failed to query auto-releases: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var escrowID uuid.UUID
		if err := rows.Scan(&escrowID); err != nil {
			s.logger.Error("failed to scan escrow ID", zap.Error(err))
			continue
		}

		if err := s.ReleaseEscrow(ctx, escrowID, "system"); err != nil {
			s.logger.Error("failed to auto-release escrow",
				zap.String("escrow_id", escrowID.String()),
				zap.Error(err),
			)
			continue
		}

		count++
	}

	if count > 0 {
		s.logger.Info("processed auto-releases", zap.Int("count", count))
	}

	return count, nil
}

// CompleteEscrow marks an escrow as completed (used when payment channel was used instead)
func (s *EscrowService) CompleteEscrow(ctx context.Context, escrowID uuid.UUID) error {
	query := `
		UPDATE escrows
		SET status = $1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $2
	`

	result, err := s.db.ExecContext(ctx, query, "completed", escrowID)
	if err != nil {
		return fmt.Errorf("failed to complete escrow: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("escrow not found: %s", escrowID.String())
	}

	s.logger.Info("escrow completed",
		zap.String("escrow_id", escrowID.String()),
	)

	return nil
}
