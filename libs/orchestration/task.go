package orchestration

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the current state of a task
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusQueued    TaskStatus = "queued"
	TaskStatusAssigned  TaskStatus = "assigned"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCanceled  TaskStatus = "canceled"
)

// PaymentStatus is imported from payment_lifecycle.go but defined here to avoid circular imports
// Note: Keep this in sync with PaymentStatus in payment_lifecycle.go
type PaymentStatus string

const (
	PaymentStatusCreated  PaymentStatus = "created"  // Escrow created
	PaymentStatusPending  PaymentStatus = "pending"  // Waiting for agent
	PaymentStatusAccepted PaymentStatus = "accepted" // Agent selected
	PaymentStatusReleased PaymentStatus = "released" // Payment released on success
	PaymentStatusRefunded PaymentStatus = "refunded" // Payment refunded on failure/timeout
	PaymentStatusDisputed PaymentStatus = "disputed" // Payment is in dispute
	PaymentStatusFailure  PaymentStatus = "failure"  // Payment system failure
)

// TaskPriority represents task execution priority
type TaskPriority int

const (
	PriorityLow      TaskPriority = 0
	PriorityNormal   TaskPriority = 1
	PriorityHigh     TaskPriority = 2
	PriorityCritical TaskPriority = 3
)

// TaskMilestone represents a milestone in milestone-based escrow
type TaskMilestone struct {
	ID                string                 `json:"id"`                     // Unique milestone ID
	Name              string                 `json:"name"`                   // Human-readable milestone name
	Description       string                 `json:"description"`            // Detailed description
	Amount            float64                `json:"amount"`                 // Payment amount for this milestone
	Status            string                 `json:"status"`                 // created, in_progress, completed, approved
	RequiredApprovals int                    `json:"required_approvals"`     // Number of approvals needed
	Approvals         []MilestoneApproval    `json:"approvals,omitempty"`    // List of approvals
	CompletedAt       *time.Time             `json:"completed_at,omitempty"` // When milestone was completed
	ApprovedAt        *time.Time             `json:"approved_at,omitempty"`  // When milestone was approved
	Evidence          map[string]interface{} `json:"evidence,omitempty"`     // Evidence of completion
	Order             int                    `json:"order"`                  // Execution order
}

// MilestoneApproval represents an approval for a milestone
type MilestoneApproval struct {
	ApproverDID string    `json:"approver_did"`       // DID of the approver
	ApprovedAt  time.Time `json:"approved_at"`        // When approved
	Comments    string    `json:"comments,omitempty"` // Optional comments
	Evidence    string    `json:"evidence,omitempty"` // Evidence provided
}

// Task represents a computational task to be executed by agents
type Task struct {
	// Identity
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Task Specification
	Type         string                 `json:"type"`         // e.g., "image-classification", "data-processing"
	Description  string                 `json:"description"`  // Human-readable description
	Capabilities []string               `json:"capabilities"` // Required agent capabilities
	Input        map[string]interface{} `json:"input"`        // Task input data
	Metadata     map[string]interface{} `json:"metadata"`     // Additional metadata

	// Execution
	Priority    TaskPriority           `json:"priority"`
	Status      TaskStatus             `json:"status"`
	AssignedTo  string                 `json:"assigned_to,omitempty"` // Agent DID
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`

	// Resource Constraints
	MaxCPU     string        `json:"max_cpu,omitempty"`    // e.g., "500m"
	MaxMemory  string        `json:"max_memory,omitempty"` // e.g., "128Mi"
	Timeout    time.Duration `json:"timeout"`              // Task timeout
	MaxRetries int           `json:"max_retries"`          // Maximum retry attempts
	RetryCount int           `json:"retry_count"`          // Current retry count

	// Payment & Economics
	Budget       float64 `json:"budget"`                  // Maximum price user will pay
	ActualCost   float64 `json:"actual_cost,omitempty"`   // Actual cost charged
	PaymentToken string  `json:"payment_token,omitempty"` // Payment reference

	// Payment Lifecycle
	PaymentStatus    PaymentStatus `json:"payment_status,omitempty"`     // Current payment status
	EscrowTxHash     string        `json:"escrow_tx_hash,omitempty"`     // Escrow creation transaction hash
	PaymentTxHash    string        `json:"payment_tx_hash,omitempty"`    // Payment release/refund transaction hash
	PaymentUpdatedAt *time.Time    `json:"payment_updated_at,omitempty"` // Last payment status update

	// Extended Escrow Support
	EscrowType string `json:"escrow_type,omitempty"` // simple, multi_party, milestone, hybrid
	TemplateID string `json:"template_id,omitempty"` // Template used to create this task

	// Multi-party Escrow
	Participants  []string `json:"participants,omitempty"`   // List of participant DIDs
	RequiredVotes int      `json:"required_votes,omitempty"` // Votes needed for approval

	// Milestone Support
	Milestones       []TaskMilestone `json:"milestones,omitempty"`        // Task milestones
	CurrentMilestone int             `json:"current_milestone,omitempty"` // Current milestone index

	// Batch Operations
	BatchID     string `json:"batch_id,omitempty"`      // Batch identifier for grouped tasks
	IsBatchTask bool   `json:"is_batch_task,omitempty"` // Whether this is part of a batch

	// Refund Policy
	RefundPolicyType string `json:"refund_policy_type,omitempty"` // linear, exponential, stepwise, etc.
}

// NewTask creates a new task with default values
func NewTask(userID, taskType string, capabilities []string, input map[string]interface{}) *Task {
	now := time.Now()
	return &Task{
		ID:            uuid.New().String(),
		UserID:        userID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Type:          taskType,
		Capabilities:  capabilities,
		Input:         input,
		Priority:      PriorityNormal,
		Status:        TaskStatusPending,
		Timeout:       30 * time.Second, // Default 30s timeout
		MaxRetries:    3,                // Default 3 retries
		RetryCount:    0,
		Metadata:      make(map[string]interface{}),
		PaymentStatus: PaymentStatusCreated, // Initialize payment status
	}
}

// CanRetry returns whether the task can be retried
func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetries && t.Status == TaskStatusFailed
}

// UpdateStatus updates the task status and timestamp
func (t *Task) UpdateStatus(status TaskStatus) {
	t.Status = status
	t.UpdatedAt = time.Now()

	switch status {
	case TaskStatusRunning:
		now := time.Now()
		t.StartedAt = &now
	case TaskStatusCompleted, TaskStatusFailed, TaskStatusCanceled:
		now := time.Now()
		t.CompletedAt = &now
	}
}

// IsTerminal returns whether the task is in a terminal state
func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted ||
		t.Status == TaskStatusFailed ||
		t.Status == TaskStatusCanceled
}

// TaskResult represents the result of a completed task
type TaskResult struct {
	TaskID      string                 `json:"task_id"`
	Status      TaskStatus             `json:"status"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	ExecutionMS int64                  `json:"execution_ms"` // Execution time in milliseconds
	AgentDID    string                 `json:"agent_did"`
	Timestamp   time.Time              `json:"timestamp"`
	Cost        float64                `json:"cost,omitempty"`
}

// TaskFilter represents filtering criteria for task queries
type TaskFilter struct {
	UserID        string       `json:"user_id,omitempty"`
	Status        TaskStatus   `json:"status,omitempty"`
	Priority      TaskPriority `json:"priority,omitempty"`
	Type          string       `json:"type,omitempty"`
	AssignedTo    string       `json:"assigned_to,omitempty"`
	CreatedAfter  *time.Time   `json:"created_after,omitempty"`
	CreatedBefore *time.Time   `json:"created_before,omitempty"`
	Limit         int          `json:"limit,omitempty"`
	Offset        int          `json:"offset,omitempty"`
}
