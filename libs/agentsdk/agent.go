package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Agent interface defines the contract for all ZeroState agents
type Agent interface {
	// Identity
	GetDID() string
	GetName() string
	GetCapabilities() []Capability
	GetVersion() string

	// Lifecycle
	Initialize(ctx context.Context, config *Config) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() HealthStatus

	// Task Execution
	HandleTask(ctx context.Context, task *Task) (*TaskResult, error)
	CanHandle(task *Task) bool

	// Communication (optional - provided by BaseAgent)
	SendMessage(ctx context.Context, targetDID string, msg *Message) error
	BroadcastMessage(ctx context.Context, msg *Message) error

	// Collaboration (optional - provided by BaseAgent)
	JoinWorkflow(ctx context.Context, workflowID string) error
	LeaveWorkflow(ctx context.Context, workflowID string) error
}

// Capability represents an agent capability
type Capability struct {
	Name        string                 `json:"name"`
	Version     string                 `json:"version"`
	Description string                 `json:"description,omitempty"`
	Cost        *Cost                  `json:"cost,omitempty"`
	Limits      *Limits                `json:"limits,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Cost represents capability pricing
type Cost struct {
	Unit  string  `json:"unit"`  // "req", "sec", "mb", etc.
	Price float64 `json:"price"` // Price per unit
}

// Limits represents capability rate limits
type Limits struct {
	TPS         int `json:"tps"`         // Transactions per second
	Concurrency int `json:"concurrency"` // Max concurrent operations
	MaxSize     int `json:"max_size,omitempty"`
}

// Task represents a task to be executed by an agent
type Task struct {
	ID           string                 `json:"id"`
	Type         string                 `json:"type"`
	Capabilities []string               `json:"capabilities"`
	Input        json.RawMessage        `json:"input"`
	Budget       float64                `json:"budget"`
	Priority     int                    `json:"priority"`
	Deadline     time.Time              `json:"deadline,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TaskResult represents the result of task execution
type TaskResult struct {
	TaskID       string                 `json:"task_id"`
	Status       TaskStatus             `json:"status"`
	Result       json.RawMessage        `json:"result,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Cost         float64                `json:"cost,omitempty"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ProofOfWork  string                 `json:"proof_of_work,omitempty"`
	AgentVersion string                 `json:"agent_version,omitempty"`
}

// TaskStatus represents task execution status
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "PENDING"
	TaskStatusRunning   TaskStatus = "RUNNING"
	TaskStatusCompleted TaskStatus = "COMPLETED"
	TaskStatusFailed    TaskStatus = "FAILED"
	TaskStatusCanceled  TaskStatus = "CANCELED"
	TaskStatusTimeout   TaskStatus = "TIMEOUT"
)

// Message represents agent-to-agent messages
type Message struct {
	ID          string                 `json:"id"`
	From        string                 `json:"from"`
	To          string                 `json:"to,omitempty"` // Empty for broadcast
	Type        MessageType            `json:"type"`
	Payload     json.RawMessage        `json:"payload"`
	Priority    int                    `json:"priority"`
	Timestamp   time.Time              `json:"timestamp"`
	ReplyTo     string                 `json:"reply_to,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Signature   string                 `json:"signature,omitempty"`
	DeliveryAck bool                   `json:"delivery_ack,omitempty"`
}

// MessageType represents message types
type MessageType string

const (
	MessageTypeRequest      MessageType = "REQUEST"
	MessageTypeResponse     MessageType = "RESPONSE"
	MessageTypeBroadcast    MessageType = "BROADCAST"
	MessageTypeNegotiation  MessageType = "NEGOTIATION"
	MessageTypeCoordination MessageType = "COORDINATION"
	MessageTypeHeartbeat    MessageType = "HEARTBEAT"
	MessageTypeAck          MessageType = "ACK"
)

// HealthStatus represents agent health
type HealthStatus struct {
	Status      string                 `json:"status"` // "healthy", "degraded", "unhealthy"
	Uptime      time.Duration          `json:"uptime"`
	TasksTotal  int64                  `json:"tasks_total"`
	TasksActive int                    `json:"tasks_active"`
	LastError   string                 `json:"last_error,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Config represents agent configuration
type Config struct {
	// Identity
	DID         string       `json:"did,omitempty"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Version     string       `json:"version"`
	Capabilities []Capability `json:"capabilities"`

	// Network
	NetworkEndpoint string   `json:"network_endpoint"`
	Region          string   `json:"region,omitempty"`
	BootstrapPeers  []string `json:"bootstrap_peers,omitempty"`

	// Pricing
	DefaultPrice float64 `json:"default_price"`
	MinBudget    float64 `json:"min_budget,omitempty"`

	// Performance
	MaxConcurrentTasks int           `json:"max_concurrent_tasks"`
	TaskTimeout        time.Duration `json:"task_timeout"`
	HeartbeatInterval  time.Duration `json:"heartbeat_interval"`

	// Logging
	LogLevel string `json:"log_level"`

	// Custom settings
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// BaseAgent provides common agent functionality
type BaseAgent struct {
	config *Config
	logger *zap.Logger

	// Identity
	did          string
	name         string
	version      string
	capabilities []Capability

	// State
	mu          sync.RWMutex
	running     bool
	startTime   time.Time
	tasksTotal  int64
	tasksActive int
	lastError   error

	// Task management
	tasksMu     sync.RWMutex
	activeTasks map[string]context.CancelFunc

	// Communication (to be implemented)
	messageBus MessageBus

	// Workflows
	workflows map[string]*Workflow
}

// MessageBus interface for agent communication
type MessageBus interface {
	Send(ctx context.Context, msg *Message) error
	Broadcast(ctx context.Context, msg *Message) error
	Subscribe(msgType MessageType, handler func(*Message) error) error
	Unsubscribe(msgType MessageType) error
}

// Workflow represents a collaborative workflow
type Workflow struct {
	ID           string
	Participants []string
	Status       string
	CreatedAt    time.Time
}

// NewBaseAgent creates a new base agent
func NewBaseAgent(config *Config, logger *zap.Logger) *BaseAgent {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &BaseAgent{
		config:       config,
		logger:       logger,
		did:          config.DID,
		name:         config.Name,
		version:      config.Version,
		capabilities: config.Capabilities,
		activeTasks:  make(map[string]context.CancelFunc),
		workflows:    make(map[string]*Workflow),
	}
}

// GetDID returns the agent's DID
func (a *BaseAgent) GetDID() string {
	return a.did
}

// GetName returns the agent's name
func (a *BaseAgent) GetName() string {
	return a.name
}

// GetCapabilities returns the agent's capabilities
func (a *BaseAgent) GetCapabilities() []Capability {
	return a.capabilities
}

// GetVersion returns the agent's version
func (a *BaseAgent) GetVersion() string {
	return a.version
}

// Initialize initializes the agent
func (a *BaseAgent) Initialize(ctx context.Context, config *Config) error {
	a.logger.Info("initializing agent",
		zap.String("name", a.name),
		zap.String("did", a.did),
		zap.String("version", a.version),
	)

	// Generate DID if not provided
	if a.did == "" {
		a.did = fmt.Sprintf("did:zerostate:%s", uuid.New().String())
		a.logger.Info("generated DID", zap.String("did", a.did))
	}

	// Validate capabilities
	if len(a.capabilities) == 0 {
		return fmt.Errorf("agent must have at least one capability")
	}

	a.logger.Info("agent initialized successfully",
		zap.Int("capabilities", len(a.capabilities)),
	)

	return nil
}

// Start starts the agent
func (a *BaseAgent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("agent already running")
	}

	a.logger.Info("starting agent", zap.String("name", a.name))

	a.running = true
	a.startTime = time.Now()

	// Start heartbeat
	if a.config.HeartbeatInterval > 0 {
		go a.heartbeatLoop(ctx)
	}

	a.logger.Info("agent started successfully")
	return nil
}

// Stop stops the agent
func (a *BaseAgent) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return fmt.Errorf("agent not running")
	}

	a.logger.Info("stopping agent", zap.String("name", a.name))

	// Cancel all active tasks
	a.tasksMu.Lock()
	for taskID, cancel := range a.activeTasks {
		a.logger.Info("canceling active task", zap.String("task_id", taskID))
		cancel()
	}
	a.tasksMu.Unlock()

	a.running = false
	a.logger.Info("agent stopped successfully")
	return nil
}

// Health returns the agent's health status
func (a *BaseAgent) Health() HealthStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := "healthy"
	if !a.running {
		status = "unhealthy"
	} else if a.lastError != nil {
		status = "degraded"
	}

	var lastErrStr string
	if a.lastError != nil {
		lastErrStr = a.lastError.Error()
	}

	return HealthStatus{
		Status:      status,
		Uptime:      time.Since(a.startTime),
		TasksTotal:  a.tasksTotal,
		TasksActive: a.tasksActive,
		LastError:   lastErrStr,
	}
}

// CanHandle checks if the agent can handle a task
func (a *BaseAgent) CanHandle(task *Task) bool {
	// Check if agent has all required capabilities
	for _, reqCap := range task.Capabilities {
		found := false
		for _, agentCap := range a.capabilities {
			if agentCap.Name == reqCap {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check budget
	if task.Budget < a.config.MinBudget {
		return false
	}

	return true
}

// HandleTask is the default task handler (should be overridden)
func (a *BaseAgent) HandleTask(ctx context.Context, task *Task) (*TaskResult, error) {
	return nil, fmt.Errorf("HandleTask must be implemented by derived agent")
}

// SendMessage sends a message to another agent
func (a *BaseAgent) SendMessage(ctx context.Context, targetDID string, msg *Message) error {
	if a.messageBus == nil {
		return fmt.Errorf("message bus not configured")
	}

	msg.From = a.did
	msg.To = targetDID
	msg.Timestamp = time.Now()

	return a.messageBus.Send(ctx, msg)
}

// BroadcastMessage broadcasts a message to all agents
func (a *BaseAgent) BroadcastMessage(ctx context.Context, msg *Message) error {
	if a.messageBus == nil {
		return fmt.Errorf("message bus not configured")
	}

	msg.From = a.did
	msg.Type = MessageTypeBroadcast
	msg.Timestamp = time.Now()

	return a.messageBus.Broadcast(ctx, msg)
}

// JoinWorkflow joins a collaborative workflow
func (a *BaseAgent) JoinWorkflow(ctx context.Context, workflowID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.workflows[workflowID]; exists {
		return fmt.Errorf("already joined workflow %s", workflowID)
	}

	a.workflows[workflowID] = &Workflow{
		ID:        workflowID,
		Status:    "active",
		CreatedAt: time.Now(),
	}

	a.logger.Info("joined workflow", zap.String("workflow_id", workflowID))
	return nil
}

// LeaveWorkflow leaves a collaborative workflow
func (a *BaseAgent) LeaveWorkflow(ctx context.Context, workflowID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, exists := a.workflows[workflowID]; !exists {
		return fmt.Errorf("not in workflow %s", workflowID)
	}

	delete(a.workflows, workflowID)
	a.logger.Info("left workflow", zap.String("workflow_id", workflowID))
	return nil
}

// ExecuteTask wraps task execution with tracking and error handling
func (a *BaseAgent) ExecuteTask(ctx context.Context, task *Task, handler func(context.Context, *Task) (*TaskResult, error)) (*TaskResult, error) {
	a.logger.Info("executing task",
		zap.String("task_id", task.ID),
		zap.String("type", task.Type),
		zap.Strings("capabilities", task.Capabilities),
	)

	// Track active task
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.tasksMu.Lock()
	a.activeTasks[task.ID] = cancel
	a.tasksActive++
	a.tasksMu.Unlock()

	defer func() {
		a.tasksMu.Lock()
		delete(a.activeTasks, task.ID)
		a.tasksActive--
		a.tasksTotal++
		a.tasksMu.Unlock()
	}()

	startTime := time.Now()

	// Execute with timeout
	if !task.Deadline.IsZero() {
		var timeoutCancel context.CancelFunc
		taskCtx, timeoutCancel = context.WithDeadline(taskCtx, task.Deadline)
		defer timeoutCancel()
	} else if a.config.TaskTimeout > 0 {
		var timeoutCancel context.CancelFunc
		taskCtx, timeoutCancel = context.WithTimeout(taskCtx, a.config.TaskTimeout)
		defer timeoutCancel()
	}

	// Execute task
	result, err := handler(taskCtx, task)

	if err != nil {
		a.mu.Lock()
		a.lastError = err
		a.mu.Unlock()

		a.logger.Error("task execution failed",
			zap.String("task_id", task.ID),
			zap.Error(err),
		)

		return &TaskResult{
			TaskID:      task.ID,
			Status:      TaskStatusFailed,
			Error:       err.Error(),
			StartedAt:   startTime,
			CompletedAt: time.Now(),
		}, err
	}

	result.TaskID = task.ID
	result.StartedAt = startTime
	result.CompletedAt = time.Now()
	result.AgentVersion = a.version

	a.logger.Info("task completed successfully",
		zap.String("task_id", task.ID),
		zap.Duration("duration", time.Since(startTime)),
		zap.Float64("cost", result.Cost),
	)

	return result, nil
}

// heartbeatLoop sends periodic heartbeats
func (a *BaseAgent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			health := a.Health()
			a.logger.Debug("heartbeat",
				zap.String("status", health.Status),
				zap.Int64("tasks_total", health.TasksTotal),
				zap.Int("tasks_active", health.TasksActive),
			)

			// Broadcast heartbeat if message bus configured
			if a.messageBus != nil {
				healthJSON, _ := json.Marshal(health)
				msg := &Message{
					ID:      uuid.New().String(),
					Type:    MessageTypeHeartbeat,
					Payload: healthJSON,
				}
				_ = a.BroadcastMessage(ctx, msg)
			}
		}
	}
}

// SetMessageBus sets the message bus for communication
func (a *BaseAgent) SetMessageBus(bus MessageBus) {
	a.messageBus = bus
}

// GetConfig returns the agent configuration
func (a *BaseAgent) GetConfig() *Config {
	return a.config
}

// GetLogger returns the agent logger
func (a *BaseAgent) GetLogger() *zap.Logger {
	return a.logger
}
