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

// TaskPriority represents task execution priority
type TaskPriority int

const (
	PriorityLow    TaskPriority = 0
	PriorityNormal TaskPriority = 1
	PriorityHigh   TaskPriority = 2
	PriorityCritical TaskPriority = 3
)

// Task represents a computational task to be executed by agents
type Task struct {
	// Identity
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Task Specification
	Type         string                 `json:"type"`          // e.g., "image-classification", "data-processing"
	Description  string                 `json:"description"`   // Human-readable description
	Capabilities []string               `json:"capabilities"`  // Required agent capabilities
	Input        map[string]interface{} `json:"input"`         // Task input data
	Metadata     map[string]interface{} `json:"metadata"`      // Additional metadata

	// Execution
	Priority     TaskPriority           `json:"priority"`
	Status       TaskStatus             `json:"status"`
	AssignedTo   string                 `json:"assigned_to,omitempty"` // Agent DID
	Result       map[string]interface{} `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	StartedAt    *time.Time             `json:"started_at,omitempty"`
	CompletedAt  *time.Time             `json:"completed_at,omitempty"`

	// Resource Constraints
	MaxCPU       string        `json:"max_cpu,omitempty"`        // e.g., "500m"
	MaxMemory    string        `json:"max_memory,omitempty"`     // e.g., "128Mi"
	Timeout      time.Duration `json:"timeout"`                  // Task timeout
	MaxRetries   int           `json:"max_retries"`              // Maximum retry attempts
	RetryCount   int           `json:"retry_count"`              // Current retry count

	// Payment & Economics
	Budget       float64 `json:"budget"`                        // Maximum price user will pay
	ActualCost   float64 `json:"actual_cost,omitempty"`         // Actual cost charged
	PaymentToken string  `json:"payment_token,omitempty"`       // Payment reference
}

// NewTask creates a new task with default values
func NewTask(userID, taskType string, capabilities []string, input map[string]interface{}) *Task {
	now := time.Now()
	return &Task{
		ID:           uuid.New().String(),
		UserID:       userID,
		CreatedAt:    now,
		UpdatedAt:    now,
		Type:         taskType,
		Capabilities: capabilities,
		Input:        input,
		Priority:     PriorityNormal,
		Status:       TaskStatusPending,
		Timeout:      30 * time.Second, // Default 30s timeout
		MaxRetries:   3,                // Default 3 retries
		RetryCount:   0,
		Metadata:     make(map[string]interface{}),
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
}

// TaskFilter represents filtering criteria for task queries
type TaskFilter struct {
	UserID       string       `json:"user_id,omitempty"`
	Status       TaskStatus   `json:"status,omitempty"`
	Priority     TaskPriority `json:"priority,omitempty"`
	Type         string       `json:"type,omitempty"`
	AssignedTo   string       `json:"assigned_to,omitempty"`
	CreatedAfter *time.Time   `json:"created_after,omitempty"`
	CreatedBefore *time.Time  `json:"created_before,omitempty"`
	Limit        int          `json:"limit,omitempty"`
	Offset       int          `json:"offset,omitempty"`
}
