package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

var (
	agentMessagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_agent_messages_published_total",
			Help: "Total agent-to-agent messages published",
		},
		[]string{"message_type"},
	)

	agentMessagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_agent_messages_received_total",
			Help: "Total agent-to-agent messages received",
		},
		[]string{"message_type"},
	)

	agentMessageLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "zerostate_agent_message_latency_seconds",
			Help:    "Agent message round-trip latency",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0},
		},
		[]string{"message_type"},
	)

	agentMessageErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zerostate_agent_message_errors_total",
			Help: "Total agent message processing errors",
		},
		[]string{"error_type"},
	)
)

const (
	// Topic for agent-to-agent communication
	TopicAgentMessages = "/zerostate/agents/messages/1.0.0"

	// Message types for agent communication
	MessageTypeRequest     = "REQUEST"      // Agent requests another agent to perform task
	MessageTypeResponse    = "RESPONSE"     // Response to a request
	MessageTypeBroadcast   = "BROADCAST"    // Broadcast message to all agents
	MessageTypeNegotiation = "NEGOTIATION"  // Negotiation for task collaboration
	MessageTypeCoordination = "COORDINATION" // Coordination for multi-agent workflows
	MessageTypeHeartbeat   = "HEARTBEAT"    // Agent availability heartbeat
	MessageTypeAck         = "ACK"          // Acknowledgment of message receipt

	// Delivery guarantees
	DeliveryBestEffort = "BEST_EFFORT" // No guarantee
	DeliveryAtLeastOnce = "AT_LEAST_ONCE" // Ack required
	DeliveryExactlyOnce = "EXACTLY_ONCE"  // Dedup + ack required

	// Default timeouts
	DefaultRequestTimeout  = 30 * time.Second
	DefaultAckTimeout      = 5 * time.Second
)

// AgentMessage represents a message between agents
type AgentMessage struct {
	// Message metadata
	ID              string    `json:"id"`                // Unique message ID
	CorrelationID   string    `json:"correlation_id"`    // For request-response correlation
	Type            string    `json:"type"`              // MessageType*
	Delivery        string    `json:"delivery"`          // Delivery guarantee
	Timestamp       time.Time `json:"timestamp"`         // Send timestamp
	TTL             int64     `json:"ttl"`               // Time-to-live in seconds
	Priority        int       `json:"priority"`          // Message priority (0-10)

	// Routing information
	From            string    `json:"from"`              // Sender agent ID
	To              string    `json:"to"`                // Recipient agent ID (empty for broadcast)
	ReplyTo         string    `json:"reply_to"`          // Agent ID to send response to

	// Payload
	Payload         json.RawMessage `json:"payload"`     // Message content
	PayloadType     string          `json:"payload_type"` // Type hint for payload

	// Metadata
	Metadata        map[string]string `json:"metadata"`  // Additional metadata
}

// TaskRequest represents a request from one agent to another
type TaskRequest struct {
	TaskID          string            `json:"task_id"`
	AgentID         string            `json:"agent_id"`          // Requested agent ID
	Input           json.RawMessage   `json:"input"`             // Task input
	Requirements    map[string]string `json:"requirements"`      // Task requirements
	Deadline        time.Time         `json:"deadline"`          // Task deadline
	Budget          float64           `json:"budget"`            // Max price willing to pay
	Priority        int               `json:"priority"`          // Task priority
}

// TaskResponse represents a response to a task request
type TaskResponse struct {
	TaskID          string            `json:"task_id"`
	Status          string            `json:"status"`            // ACCEPTED, REJECTED, COMPLETED, FAILED
	Result          json.RawMessage   `json:"result"`            // Task result (if completed)
	Error           string            `json:"error"`             // Error message (if failed)
	Price           float64           `json:"price"`             // Actual price charged
	Duration        int64             `json:"duration_ms"`       // Execution duration in ms
	Metadata        map[string]string `json:"metadata"`          // Additional metadata
}

// NegotiationMessage represents a negotiation between agents
type NegotiationMessage struct {
	TaskID          string    `json:"task_id"`
	Phase           string    `json:"phase"`             // BID, COUNTER_BID, ACCEPT, REJECT
	Price           float64   `json:"price"`             // Proposed price
	Deadline        time.Time `json:"deadline"`          // Proposed deadline
	Terms           map[string]string `json:"terms"`     // Negotiation terms
}

// CoordinationMessage represents coordination between agents
type CoordinationMessage struct {
	WorkflowID      string            `json:"workflow_id"`
	Action          string            `json:"action"`          // LOCK, UNLOCK, UPDATE, SYNC
	Resource        string            `json:"resource"`        // Resource to coordinate on
	State           json.RawMessage   `json:"state"`           // Shared state
	Metadata        map[string]string `json:"metadata"`
}

// MessageBus handles agent-to-agent messaging
type MessageBus struct {
	mu              sync.RWMutex
	gossip          *GossipService
	logger          *zap.Logger
	ctx             context.Context
	cancel          context.CancelFunc

	// Message routing
	agentID         string                                  // This agent's ID
	handlers        map[string][]AgentMessageHandler        // Handler per message type
	pendingRequests map[string]chan *AgentMessage          // Pending request-response pairs
	messageCache    map[string]time.Time                   // For exactly-once delivery (message ID -> timestamp)

	// Metrics
	sentMessages    map[string]int64                       // Messages sent per type
	receivedMessages map[string]int64                      // Messages received per type
}

// AgentMessageHandler processes received agent messages
type AgentMessageHandler func(ctx context.Context, msg *AgentMessage) error

// NewMessageBus creates a new agent message bus
func NewMessageBus(ctx context.Context, agentID string, gossip *GossipService, logger *zap.Logger) (*MessageBus, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	busCtx, cancel := context.WithCancel(ctx)

	mb := &MessageBus{
		gossip:          gossip,
		logger:          logger,
		ctx:             busCtx,
		cancel:          cancel,
		agentID:         agentID,
		handlers:        make(map[string][]AgentMessageHandler),
		pendingRequests: make(map[string]chan *AgentMessage),
		messageCache:    make(map[string]time.Time),
		sentMessages:    make(map[string]int64),
		receivedMessages: make(map[string]int64),
	}

	// Subscribe to agent messages topic
	if err := gossip.Subscribe(TopicAgentMessages, mb.handleGossipMessage); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to subscribe to agent messages: %w", err)
	}

	// Start cleanup goroutine
	go mb.cleanupLoop()

	logger.Info("message bus created", zap.String("agent_id", agentID))
	return mb, nil
}

// RegisterHandler registers a handler for a specific message type
func (mb *MessageBus) RegisterHandler(messageType string, handler AgentMessageHandler) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	mb.handlers[messageType] = append(mb.handlers[messageType], handler)
	mb.logger.Info("registered message handler", zap.String("type", messageType))
}

// SendMessage sends a message to another agent
func (mb *MessageBus) SendMessage(ctx context.Context, msg *AgentMessage) error {
	// Set defaults
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}
	if msg.From == "" {
		msg.From = mb.agentID
	}
	if msg.Delivery == "" {
		msg.Delivery = DeliveryBestEffort
	}
	if msg.TTL == 0 {
		msg.TTL = 300 // 5 minutes default
	}

	// Wrap in gossip message
	payload, err := json.Marshal(msg)
	if err != nil {
		agentMessageErrors.WithLabelValues("marshal_error").Inc()
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	gossipMsg := &GossipMessage{
		Type:      "AGENT_MESSAGE",
		Payload:   payload,
		Timestamp: time.Now().Unix(),
		PeerID:    mb.agentID,
	}

	// Publish to gossip network
	if err := mb.gossip.Publish(TopicAgentMessages, gossipMsg); err != nil {
		agentMessageErrors.WithLabelValues("publish_error").Inc()
		return fmt.Errorf("failed to publish message: %w", err)
	}

	// Track metrics
	agentMessagesPublished.WithLabelValues(msg.Type).Inc()
	mb.mu.Lock()
	mb.sentMessages[msg.Type]++
	mb.mu.Unlock()

	mb.logger.Debug("message sent",
		zap.String("id", msg.ID),
		zap.String("type", msg.Type),
		zap.String("from", msg.From),
		zap.String("to", msg.To),
	)

	return nil
}

// SendRequest sends a request and waits for response
func (mb *MessageBus) SendRequest(ctx context.Context, to string, req *TaskRequest, timeout time.Duration) (*TaskResponse, error) {
	if timeout == 0 {
		timeout = DefaultRequestTimeout
	}

	// Create request message
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	msg := &AgentMessage{
		ID:          uuid.New().String(),
		Type:        MessageTypeRequest,
		From:        mb.agentID,
		To:          to,
		ReplyTo:     mb.agentID,
		Payload:     payload,
		PayloadType: "TaskRequest",
		Delivery:    DeliveryAtLeastOnce,
		TTL:         int64(timeout.Seconds()),
	}

	// Create response channel
	respChan := make(chan *AgentMessage, 1)
	mb.mu.Lock()
	mb.pendingRequests[msg.ID] = respChan
	mb.mu.Unlock()

	// Send request
	if err := mb.SendMessage(ctx, msg); err != nil {
		mb.mu.Lock()
		delete(mb.pendingRequests, msg.ID)
		mb.mu.Unlock()
		return nil, err
	}

	// Wait for response with timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case respMsg := <-respChan:
		// Parse response
		var resp TaskResponse
		if err := json.Unmarshal(respMsg.Payload, &resp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		// Record latency
		latency := time.Since(msg.Timestamp).Seconds()
		agentMessageLatency.WithLabelValues(MessageTypeRequest).Observe(latency)

		return &resp, nil

	case <-timer.C:
		mb.mu.Lock()
		delete(mb.pendingRequests, msg.ID)
		mb.mu.Unlock()
		return nil, fmt.Errorf("request timeout after %v", timeout)

	case <-ctx.Done():
		mb.mu.Lock()
		delete(mb.pendingRequests, msg.ID)
		mb.mu.Unlock()
		return nil, ctx.Err()
	}
}

// SendResponse sends a response to a request
func (mb *MessageBus) SendResponse(ctx context.Context, requestMsg *AgentMessage, resp *TaskResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	respMsg := &AgentMessage{
		ID:            uuid.New().String(),
		CorrelationID: requestMsg.ID,
		Type:          MessageTypeResponse,
		From:          mb.agentID,
		To:            requestMsg.ReplyTo,
		Payload:       payload,
		PayloadType:   "TaskResponse",
		Delivery:      DeliveryBestEffort,
	}

	return mb.SendMessage(ctx, respMsg)
}

// Broadcast sends a message to all agents
func (mb *MessageBus) Broadcast(ctx context.Context, payload json.RawMessage, payloadType string) error {
	msg := &AgentMessage{
		Type:        MessageTypeBroadcast,
		From:        mb.agentID,
		To:          "", // Empty means broadcast
		Payload:     payload,
		PayloadType: payloadType,
		Delivery:    DeliveryBestEffort,
	}

	return mb.SendMessage(ctx, msg)
}

// handleGossipMessage processes messages from gossip network
func (mb *MessageBus) handleGossipMessage(ctx context.Context, gossipMsg *GossipMessage) error {
	// Parse agent message
	var msg AgentMessage
	if err := json.Unmarshal(gossipMsg.Payload, &msg); err != nil {
		agentMessageErrors.WithLabelValues("unmarshal_error").Inc()
		return fmt.Errorf("failed to unmarshal agent message: %w", err)
	}

	// Skip messages from self
	if msg.From == mb.agentID {
		return nil
	}

	// Check if message is for us or broadcast
	if msg.To != "" && msg.To != mb.agentID {
		return nil // Not for us
	}

	// Check TTL
	if time.Since(msg.Timestamp).Seconds() > float64(msg.TTL) {
		mb.logger.Debug("message expired",
			zap.String("id", msg.ID),
			zap.Duration("age", time.Since(msg.Timestamp)),
		)
		return nil
	}

	// Handle exactly-once delivery
	if msg.Delivery == DeliveryExactlyOnce {
		mb.mu.Lock()
		if _, exists := mb.messageCache[msg.ID]; exists {
			mb.mu.Unlock()
			mb.logger.Debug("duplicate message ignored", zap.String("id", msg.ID))
			return nil
		}
		mb.messageCache[msg.ID] = time.Now()
		mb.mu.Unlock()
	}

	// Track metrics
	agentMessagesReceived.WithLabelValues(msg.Type).Inc()
	mb.mu.Lock()
	mb.receivedMessages[msg.Type]++
	mb.mu.Unlock()

	mb.logger.Debug("message received",
		zap.String("id", msg.ID),
		zap.String("type", msg.Type),
		zap.String("from", msg.From),
	)

	// Handle response to pending request
	if msg.Type == MessageTypeResponse && msg.CorrelationID != "" {
		mb.mu.RLock()
		respChan, exists := mb.pendingRequests[msg.CorrelationID]
		mb.mu.RUnlock()

		if exists {
			select {
			case respChan <- &msg:
			default:
				mb.logger.Warn("response channel full", zap.String("id", msg.ID))
			}
			return nil
		}
	}

	// Call registered handlers
	mb.mu.RLock()
	handlers := mb.handlers[msg.Type]
	mb.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, &msg); err != nil {
			agentMessageErrors.WithLabelValues("handler_error").Inc()
			mb.logger.Error("handler error",
				zap.String("id", msg.ID),
				zap.String("type", msg.Type),
				zap.Error(err),
			)
		}
	}

	// Send ack if required
	if msg.Delivery == DeliveryAtLeastOnce || msg.Delivery == DeliveryExactlyOnce {
		ack := &AgentMessage{
			Type:          MessageTypeAck,
			CorrelationID: msg.ID,
			From:          mb.agentID,
			To:            msg.From,
			Delivery:      DeliveryBestEffort,
		}
		if err := mb.SendMessage(ctx, ack); err != nil {
			mb.logger.Error("failed to send ack", zap.Error(err))
		}
	}

	return nil
}

// cleanupLoop periodically cleans up expired cache entries
func (mb *MessageBus) cleanupLoop() {
	ticker := time.NewTicker(DefaultCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mb.cleanup()
		case <-mb.ctx.Done():
			return
		}
	}
}

// cleanup removes expired cache entries
func (mb *MessageBus) cleanup() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	now := time.Now()
	for id, timestamp := range mb.messageCache {
		if now.Sub(timestamp) > 10*time.Minute {
			delete(mb.messageCache, id)
		}
	}

	// Clean up stale pending requests
	for id := range mb.pendingRequests {
		// Note: Timeouts are handled in SendRequest, this is just backup cleanup
		select {
		case <-mb.pendingRequests[id]:
			delete(mb.pendingRequests, id)
		default:
		}
	}

	mb.logger.Debug("cleanup completed",
		zap.Int("cache_size", len(mb.messageCache)),
		zap.Int("pending_requests", len(mb.pendingRequests)),
	)
}

// Close closes the message bus
func (mb *MessageBus) Close() error {
	mb.cancel()
	mb.logger.Info("message bus closed")
	return nil
}

// GetStats returns message bus statistics
func (mb *MessageBus) GetStats() map[string]interface{} {
	mb.mu.RLock()
	defer mb.mu.RUnlock()

	return map[string]interface{}{
		"agent_id":          mb.agentID,
		"sent_messages":     mb.sentMessages,
		"received_messages": mb.receivedMessages,
		"pending_requests":  len(mb.pendingRequests),
		"cache_size":        len(mb.messageCache),
	}
}
