package economic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// DelegationStatus represents the state of a delegated task
type DelegationStatus string

const (
	DelegationStatusPending    DelegationStatus = "pending"
	DelegationStatusPlanning   DelegationStatus = "planning"
	DelegationStatusInProgress DelegationStatus = "in_progress"
	DelegationStatusCompleted  DelegationStatus = "completed"
	DelegationStatusFailed     DelegationStatus = "failed"
	DelegationStatusCancelled  DelegationStatus = "cancelled"
)

// SubtaskStatus represents the state of an individual subtask
type SubtaskStatus string

const (
	SubtaskStatusPending    SubtaskStatus = "pending"
	SubtaskStatusAssigned   SubtaskStatus = "assigned"
	SubtaskStatusInProgress SubtaskStatus = "in_progress"
	SubtaskStatusCompleted  SubtaskStatus = "completed"
	SubtaskStatusFailed     SubtaskStatus = "failed"
)

// Delegation represents a meta-orchestrator delegation
type Delegation struct {
	ID                   uuid.UUID
	TaskID               string
	UserID               string
	Query                string
	Capabilities         []string
	Budget               float64
	Priority             string
	Status               DelegationStatus
	AgentsCount          int
	EstimatedCompletion  time.Time
	ActualCompletion     *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Error                *string
}

// Subtask represents an individual subtask within a delegation
type Subtask struct {
	ID           uuid.UUID
	DelegationID uuid.UUID
	TaskID       string
	Description  string
	AgentID      *string
	Status       SubtaskStatus
	BudgetShare  float64
	StartedAt    *time.Time
	CompletedAt  *time.Time
	Result       *string
	Error        *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// MetaOrchestratorService handles task decomposition and multi-agent coordination
type MetaOrchestratorService struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewMetaOrchestratorService creates a new meta-orchestrator service
func NewMetaOrchestratorService(db *sql.DB, logger *zap.Logger) *MetaOrchestratorService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MetaOrchestratorService{
		db:     db,
		logger: logger,
	}
}

// CreateDelegation creates a new task delegation with subtasks
func (s *MetaOrchestratorService) CreateDelegation(
	ctx context.Context,
	taskID string,
	userID string,
	query string,
	capabilities []string,
	budget float64,
	priority string,
) (*Delegation, []Subtask, error) {
	delegationID := uuid.New()
	now := time.Now()

	// Decompose the query into subtasks
	subtaskDescriptions := s.decomposeQuery(query, capabilities)

	// Calculate budget per subtask
	budgetPerSubtask := budget / float64(len(subtaskDescriptions))

	// Estimate completion time based on number of subtasks and priority
	estimatedDuration := s.estimateCompletionTime(len(subtaskDescriptions), priority)
	estimatedCompletion := now.Add(estimatedDuration)

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create delegation record
	delegationQuery := `
		INSERT INTO delegations (
			id, task_id, user_id, query, capabilities, budget, priority,
			status, agents_count, estimated_completion, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	capabilitiesJSON, _ := json.Marshal(capabilities)
	_, err = tx.ExecContext(ctx, delegationQuery,
		delegationID, taskID, userID, query, capabilitiesJSON, budget, priority,
		DelegationStatusPlanning, len(subtaskDescriptions), estimatedCompletion, now, now,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create delegation: %w", err)
	}

	// Create subtasks
	subtasks := make([]Subtask, 0, len(subtaskDescriptions))
	subtaskQuery := `
		INSERT INTO subtasks (
			id, delegation_id, task_id, description, status, budget_share,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for i, desc := range subtaskDescriptions {
		subtaskID := uuid.New()
		subtaskTaskID := fmt.Sprintf("%s-subtask-%d", taskID, i+1)

		_, err = tx.ExecContext(ctx, subtaskQuery,
			subtaskID, delegationID, subtaskTaskID, desc, SubtaskStatusPending,
			budgetPerSubtask, now, now,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create subtask %d: %w", i, err)
		}

		subtasks = append(subtasks, Subtask{
			ID:           subtaskID,
			DelegationID: delegationID,
			TaskID:       subtaskTaskID,
			Description:  desc,
			Status:       SubtaskStatusPending,
			BudgetShare:  budgetPerSubtask,
			CreatedAt:    now,
			UpdatedAt:    now,
		})
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	delegation := &Delegation{
		ID:                  delegationID,
		TaskID:              taskID,
		UserID:              userID,
		Query:               query,
		Capabilities:        capabilities,
		Budget:              budget,
		Priority:            priority,
		Status:              DelegationStatusPlanning,
		AgentsCount:         len(subtaskDescriptions),
		EstimatedCompletion: estimatedCompletion,
		CreatedAt:           now,
		UpdatedAt:           now,
	}

	s.logger.Info("delegation created",
		zap.String("delegation_id", delegationID.String()),
		zap.String("task_id", taskID),
		zap.Int("subtasks", len(subtasks)),
	)

	return delegation, subtasks, nil
}

// GetDelegation retrieves a delegation by ID
func (s *MetaOrchestratorService) GetDelegation(ctx context.Context, delegationID uuid.UUID) (*Delegation, error) {
	query := `
		SELECT id, task_id, user_id, query, capabilities, budget, priority,
		       status, agents_count, estimated_completion, actual_completion,
		       created_at, updated_at, error
		FROM delegations
		WHERE id = $1
	`

	var d Delegation
	var capabilitiesJSON []byte
	err := s.db.QueryRowContext(ctx, query, delegationID).Scan(
		&d.ID, &d.TaskID, &d.UserID, &d.Query, &capabilitiesJSON, &d.Budget,
		&d.Priority, &d.Status, &d.AgentsCount, &d.EstimatedCompletion,
		&d.ActualCompletion, &d.CreatedAt, &d.UpdatedAt, &d.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("delegation not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delegation: %w", err)
	}

	json.Unmarshal(capabilitiesJSON, &d.Capabilities)
	return &d, nil
}

// GetDelegationByTaskID retrieves a delegation by task ID
func (s *MetaOrchestratorService) GetDelegationByTaskID(ctx context.Context, taskID string) (*Delegation, error) {
	query := `
		SELECT id, task_id, user_id, query, capabilities, budget, priority,
		       status, agents_count, estimated_completion, actual_completion,
		       created_at, updated_at, error
		FROM delegations
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	var d Delegation
	var capabilitiesJSON []byte
	err := s.db.QueryRowContext(ctx, query, taskID).Scan(
		&d.ID, &d.TaskID, &d.UserID, &d.Query, &capabilitiesJSON, &d.Budget,
		&d.Priority, &d.Status, &d.AgentsCount, &d.EstimatedCompletion,
		&d.ActualCompletion, &d.CreatedAt, &d.UpdatedAt, &d.Error,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("delegation not found for task %s", taskID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delegation: %w", err)
	}

	json.Unmarshal(capabilitiesJSON, &d.Capabilities)
	return &d, nil
}

// GetSubtasks retrieves all subtasks for a delegation
func (s *MetaOrchestratorService) GetSubtasks(ctx context.Context, delegationID uuid.UUID) ([]Subtask, error) {
	query := `
		SELECT id, delegation_id, task_id, description, agent_id, status,
		       budget_share, started_at, completed_at, result, error,
		       created_at, updated_at
		FROM subtasks
		WHERE delegation_id = $1
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, delegationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subtasks: %w", err)
	}
	defer rows.Close()

	subtasks := make([]Subtask, 0)
	for rows.Next() {
		var st Subtask
		err = rows.Scan(
			&st.ID, &st.DelegationID, &st.TaskID, &st.Description, &st.AgentID,
			&st.Status, &st.BudgetShare, &st.StartedAt, &st.CompletedAt,
			&st.Result, &st.Error, &st.CreatedAt, &st.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subtask: %w", err)
		}
		subtasks = append(subtasks, st)
	}

	return subtasks, nil
}

// UpdateSubtaskStatus updates the status of a subtask
func (s *MetaOrchestratorService) UpdateSubtaskStatus(
	ctx context.Context,
	subtaskID uuid.UUID,
	status SubtaskStatus,
	agentID *string,
	result *string,
	errorMsg *string,
) error {
	now := time.Now()

	query := `
		UPDATE subtasks
		SET status = $1,
		    agent_id = COALESCE($2, agent_id),
		    started_at = CASE
		        WHEN $1 = 'in_progress' AND started_at IS NULL THEN $3
		        ELSE started_at
		    END,
		    completed_at = CASE
		        WHEN $1 IN ('completed', 'failed') THEN $3
		        ELSE completed_at
		    END,
		    result = COALESCE($4, result),
		    error = $5,
		    updated_at = $3
		WHERE id = $6
	`

	_, err := s.db.ExecContext(ctx, query,
		status, agentID, now, result, errorMsg, subtaskID,
	)
	if err != nil {
		return fmt.Errorf("failed to update subtask status: %w", err)
	}

	s.logger.Info("subtask status updated",
		zap.String("subtask_id", subtaskID.String()),
		zap.String("status", string(status)),
	)

	return nil
}

// UpdateDelegationStatus updates the delegation status based on subtask progress
func (s *MetaOrchestratorService) UpdateDelegationStatus(ctx context.Context, delegationID uuid.UUID) error {
	// Get all subtasks
	subtasks, err := s.GetSubtasks(ctx, delegationID)
	if err != nil {
		return err
	}

	// Calculate status based on subtasks
	var status DelegationStatus
	completedCount := 0
	failedCount := 0

	for _, st := range subtasks {
		switch st.Status {
		case SubtaskStatusCompleted:
			completedCount++
		case SubtaskStatusFailed:
			failedCount++
		}
	}

	if completedCount == len(subtasks) {
		status = DelegationStatusCompleted
	} else if failedCount > 0 && (completedCount+failedCount) == len(subtasks) {
		status = DelegationStatusFailed
	} else if completedCount > 0 || failedCount > 0 {
		status = DelegationStatusInProgress
	} else {
		status = DelegationStatusPending
	}

	// Update delegation
	now := time.Now()
	query := `
		UPDATE delegations
		SET status = $1,
		    actual_completion = CASE WHEN $1 IN ('completed', 'failed') THEN $2 ELSE actual_completion END,
		    updated_at = $2
		WHERE id = $3
	`

	_, err = s.db.ExecContext(ctx, query, status, now, delegationID)
	if err != nil {
		return fmt.Errorf("failed to update delegation status: %w", err)
	}

	return nil
}

// decomposeQuery breaks down a query into subtasks based on complexity and capabilities
func (s *MetaOrchestratorService) decomposeQuery(query string, capabilities []string) []string {
	// Simple heuristic-based decomposition
	// In production, this could use LLM or more sophisticated analysis

	queryLower := strings.ToLower(query)
	subtasks := make([]string, 0)

	// Detect common patterns
	hasCompute := contains(capabilities, "compute") || contains(capabilities, "processing")
	hasStorage := contains(capabilities, "storage") || contains(capabilities, "database")
	hasNetwork := contains(capabilities, "network") || contains(capabilities, "api")

	// Data processing pattern
	if strings.Contains(queryLower, "process") || strings.Contains(queryLower, "analyze") {
		if hasStorage {
			subtasks = append(subtasks, "Fetch and prepare input data")
		}
		if hasCompute {
			subtasks = append(subtasks, "Perform core computation/analysis")
		}
		if hasStorage {
			subtasks = append(subtasks, "Store and aggregate results")
		}
	}

	// API/Network pattern
	if strings.Contains(queryLower, "api") || strings.Contains(queryLower, "fetch") || strings.Contains(queryLower, "request") {
		if hasNetwork {
			subtasks = append(subtasks, "Execute API requests")
		}
		if hasCompute {
			subtasks = append(subtasks, "Process API responses")
		}
	}

	// Default: break into 3 phases if we didn't match specific patterns
	if len(subtasks) == 0 {
		subtasks = []string{
			"Initialize and validate inputs",
			"Execute primary task logic",
			"Finalize and return results",
		}
	}

	return subtasks
}

// estimateCompletionTime estimates how long the delegation will take
func (s *MetaOrchestratorService) estimateCompletionTime(subtaskCount int, priority string) time.Duration {
	baseTime := time.Duration(subtaskCount) * 2 * time.Minute

	switch priority {
	case "high":
		return baseTime / 2
	case "low":
		return baseTime * 2
	default: // normal
		return baseTime
	}
}

// contains checks if a slice contains a string (case-insensitive)
func contains(slice []string, item string) bool {
	itemLower := strings.ToLower(item)
	for _, s := range slice {
		if strings.ToLower(s) == itemLower {
			return true
		}
	}
	return false
}
