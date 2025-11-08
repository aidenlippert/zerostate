package execution

import (
	"context"
	"fmt"
)

// S3BinaryStore adapter for S3 storage
type S3BinaryStore struct {
	storage S3Storage
	db      AgentDatabase
}

// S3Storage interface that S3Storage implements
type S3Storage interface {
	Download(ctx context.Context, key string) ([]byte, error)
}

// AgentDatabase interface for looking up agent binary URLs
type AgentDatabase interface {
	GetAgentByID(id string) (*Agent, error)
}

// Agent represents minimal agent info needed for binary retrieval
type Agent struct {
	BinaryURL  string
	BinaryHash string
}

// NewS3BinaryStore creates a new S3 binary store adapter
func NewS3BinaryStore(storage S3Storage, db AgentDatabase) *S3BinaryStore {
	return &S3BinaryStore{
		storage: storage,
		db:      db,
	}
}

// GetBinary implements BinaryStore interface
func (s *S3BinaryStore) GetBinary(ctx context.Context, agentID string) ([]byte, error) {
	// Look up agent to get binary URL/key
	agent, err := s.db.GetAgentByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	// Extract S3 key from binary URL
	// Expected format: https://bucket.s3.amazonaws.com/agents/{agentID}/{hash}.wasm
	// or agents/{agentID}/{hash}.wasm for S3 key
	key := fmt.Sprintf("agents/%s/%s.wasm", agentID, agent.BinaryHash)

	// Download from S3
	binary, err := s.storage.Download(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to download binary: %w", err)
	}

	return binary, nil
}

// WebSocketHubAdapter adapts websocket.Hub to WebSocketHub interface
type WebSocketHubAdapter struct {
	hub Hub
}

// Hub interface that websocket.Hub implements
type Hub interface {
	BroadcastTaskUpdate(taskID, status, message string) error
}

// NewWebSocketHubAdapter creates a new WebSocket hub adapter
func NewWebSocketHubAdapter(hub Hub) *WebSocketHubAdapter {
	return &WebSocketHubAdapter{hub: hub}
}

// BroadcastTaskUpdate implements WebSocketHub interface
func (a *WebSocketHubAdapter) BroadcastTaskUpdate(taskID, status, message string) error {
	return a.hub.BroadcastTaskUpdate(taskID, status, message)
}

// TaskQueueAdapter adapts orchestration.TaskQueue to TaskQueue interface
type TaskQueueAdapter struct {
	queue Queue
}

// Queue interface that orchestration.TaskQueue implements
type Queue interface {
	Dequeue(ctx context.Context) (*QueuedTask, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status string) error
}

// QueuedTask represents a task from the orchestration queue
type QueuedTask struct {
	ID        string
	UserID    string
	AgentID   string
	Query     string
	Input     []byte
	Status    string
	CreatedAt interface{}
}

// NewTaskQueueAdapter creates a new task queue adapter
func NewTaskQueueAdapter(queue Queue) *TaskQueueAdapter {
	return &TaskQueueAdapter{queue: queue}
}

// Dequeue implements TaskQueue interface
func (a *TaskQueueAdapter) Dequeue(ctx context.Context) (*Task, error) {
	qt, err := a.queue.Dequeue(ctx)
	if err != nil {
		return nil, err
	}
	if qt == nil {
		return nil, nil
	}

	// Convert QueuedTask to Task
	return &Task{
		ID:      qt.ID,
		UserID:  qt.UserID,
		AgentID: qt.AgentID,
		Query:   qt.Query,
		Input:   qt.Input,
		Status:  qt.Status,
	}, nil
}

// UpdateStatus implements TaskQueue interface
func (a *TaskQueueAdapter) UpdateStatus(ctx context.Context, taskID string, status string) error {
	return a.queue.UpdateTaskStatus(ctx, taskID, status)
}
