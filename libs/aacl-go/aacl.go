package aacl

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"github.com/google/uuid"
)

// MessageType defines the type of AACL message
type MessageType string

const (
	MessageTypeRequest          MessageType = "Request"
	MessageTypeResponse         MessageType = "Response"
	MessageTypeQuery            MessageType = "Query"
	MessageTypeNotification     MessageType = "Notification"
	MessageTypeNegotiation      MessageType = "Negotiation"
	MessageTypeError            MessageType = "Error"
	MessageTypeAcknowledgment   MessageType = "Acknowledgment"
	MessageTypeWorkflowRequest  MessageType = "WorkflowRequest"
	MessageTypeWorkflowResponse MessageType = "WorkflowResponse"
	MessageTypeWorkflowStatus   MessageType = "WorkflowStatus"
	MessageTypeStreaming        MessageType = "Streaming"
)

// Intent describes what the user wants to accomplish
type Intent struct {
	Action               string                 `json:"action"`
	Goal                 string                 `json:"goal"`
	NaturalLanguage      *string                `json:"natural_language,omitempty"`
	Parsed               map[string]interface{} `json:"parsed,omitempty"`
	Confidence           *float64               `json:"confidence,omitempty"`
	CapabilitiesRequired []string               `json:"capabilities_required,omitempty"`
	Parameters           map[string]interface{} `json:"parameters"`
}

// IntentBuilder helps construct Intents
type IntentBuilder struct {
	action               string
	goal                 string
	naturalLanguage      *string
	parsed               map[string]interface{}
	confidence           *float64
	capabilitiesRequired []string
	parameters           map[string]interface{}
}

// NewIntent creates a new IntentBuilder
func NewIntent(action, goal string) *IntentBuilder {
	return &IntentBuilder{
		action:     action,
		goal:       goal,
		parameters: make(map[string]interface{}),
	}
}

// WithNaturalLanguage sets the natural language description
func (b *IntentBuilder) WithNaturalLanguage(nl string) *IntentBuilder {
	b.naturalLanguage = &nl
	return b
}

// WithParsed sets parsed structured data
func (b *IntentBuilder) WithParsed(parsed map[string]interface{}) *IntentBuilder {
	b.parsed = parsed
	return b
}

// WithConfidence sets the confidence score
func (b *IntentBuilder) WithConfidence(conf float64) *IntentBuilder {
	b.confidence = &conf
	return b
}

// RequiresCapability adds a required capability
func (b *IntentBuilder) RequiresCapability(cap string) *IntentBuilder {
	b.capabilitiesRequired = append(b.capabilitiesRequired, cap)
	return b
}

// WithParameter adds a parameter
func (b *IntentBuilder) WithParameter(key string, value interface{}) *IntentBuilder {
	b.parameters[key] = value
	return b
}

// Build constructs the Intent
func (b *IntentBuilder) Build() Intent {
	return Intent{
		Action:               b.action,
		Goal:                 b.goal,
		NaturalLanguage:      b.naturalLanguage,
		Parsed:               b.parsed,
		Confidence:           b.confidence,
		CapabilitiesRequired: b.capabilitiesRequired,
		Parameters:           b.parameters,
	}
}

// ExecutionMetadata tracks execution details
type ExecutionMetadata struct {
	DurationMs      uint64  `json:"duration_ms"`
	GasUsed         uint64  `json:"gas_used"`
	CostUainur      uint64  `json:"cost_uainur"`
	AgentVersion    string  `json:"agent_version"`
	AgentTrustScore float64 `json:"agent_trust_score"`
	ExecutionNodeID string  `json:"execution_node_id"`
	RetryCount      uint32  `json:"retry_count,omitempty"`
}

// ResponseResult represents the result of an execution
type ResponseResult struct {
	Value      interface{} `json:"value"`
	Type       string      `json:"type,omitempty"`
	Confidence *float64    `json:"confidence,omitempty"`
}

// ResponsePayload is the payload for Response messages
type ResponsePayload struct {
	Status            string             `json:"status"`
	Result            *ResponseResult    `json:"result,omitempty"`
	Error             *ErrorInfo         `json:"error,omitempty"`
	ExecutionMetadata *ExecutionMetadata `json:"execution_metadata,omitempty"`
}

// ErrorInfo describes an error
type ErrorInfo struct {
	Code                string                 `json:"code"`
	Message             string                 `json:"message"`
	Details             map[string]interface{} `json:"details,omitempty"`
	Recoverable         bool                   `json:"recoverable"`
	RecoverySuggestions []string               `json:"recovery_suggestions,omitempty"`
}

// ConversationContext maintains conversation state
type ConversationContext struct {
	ConversationID   string                 `json:"conversation_id"`
	PreviousMessages []string               `json:"previous_messages,omitempty"`
	SharedState      map[string]interface{} `json:"shared_state,omitempty"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	StepID      string                 `json:"step_id"`
	AgentDID    agentcard.DID          `json:"agent_did"`
	Intent      Intent                 `json:"intent"`
	DependsOn   []string               `json:"depends_on,omitempty"`
	Timeout     *uint64                `json:"timeout,omitempty"`
	RetryPolicy map[string]interface{} `json:"retry_policy,omitempty"`
}

// Workflow orchestrates multi-agent tasks
type Workflow struct {
	WorkflowID   string                 `json:"workflow_id"`
	Goal         string                 `json:"goal"`
	Steps        []WorkflowStep         `json:"steps"`
	Dependencies map[string][]string    `json:"dependencies"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AgentMatch represents a discovered agent
type AgentMatch struct {
	AgentDID     agentcard.DID `json:"agent_did"`
	TrustScore   float64       `json:"trust_score"`
	Price        uint64        `json:"price"`
	Latency      uint64        `json:"latency_ms"`
	Capabilities []string      `json:"capabilities"`
}

// CapabilityQuery queries for agents with specific capabilities
type CapabilityQuery struct {
	RequiredCapabilities []string `json:"required_capabilities"`
	MinTrustScore        *float64 `json:"min_trust_score,omitempty"`
	MaxPrice             *uint64  `json:"max_price,omitempty"`
	MaxLatency           *uint64  `json:"max_latency_ms,omitempty"`
	PreferredRegions     []string `json:"preferred_regions,omitempty"`
}

// AACLMessage is the core message structure
type AACLMessage struct {
	Context             string               `json:"@context"`
	Type                string               `json:"@type"`
	ID                  string               `json:"id"`
	From                agentcard.DID        `json:"from"`
	To                  agentcard.DID        `json:"to"`
	Timestamp           time.Time            `json:"timestamp"`
	Intent              *Intent              `json:"intent,omitempty"`
	Payload             interface{}          `json:"payload"`
	ConversationContext *ConversationContext `json:"conversation_context,omitempty"`
	Signature           *string              `json:"signature,omitempty"`
}

// MessageBuilder helps construct AACLMessages
type MessageBuilder struct {
	msgType             MessageType
	from                agentcard.DID
	to                  agentcard.DID
	intent              *Intent
	payload             interface{}
	conversationContext *ConversationContext
}

// NewMessage creates a new MessageBuilder
func NewMessage(msgType MessageType, from, to agentcard.DID) *MessageBuilder {
	return &MessageBuilder{
		msgType: msgType,
		from:    from,
		to:      to,
	}
}

// WithIntent sets the intent
func (b *MessageBuilder) WithIntent(intent Intent) *MessageBuilder {
	b.intent = &intent
	return b
}

// WithPayload sets the payload
func (b *MessageBuilder) WithPayload(payload interface{}) *MessageBuilder {
	b.payload = payload
	return b
}

// WithConversationContext sets conversation context
func (b *MessageBuilder) WithConversationContext(ctx ConversationContext) *MessageBuilder {
	b.conversationContext = &ctx
	return b
}

// Build constructs the AACLMessage
func (b *MessageBuilder) Build() (*AACLMessage, error) {
	if b.from == "" {
		return nil, fmt.Errorf("from DID is required")
	}
	if b.to == "" {
		return nil, fmt.Errorf("to DID is required")
	}

	return &AACLMessage{
		Context:             "https://ainur.network/contexts/aacl/v1",
		Type:                string(b.msgType),
		ID:                  fmt.Sprintf("urn:uuid:%s", uuid.New().String()),
		From:                b.from,
		To:                  b.to,
		Timestamp:           time.Now(),
		Intent:              b.intent,
		Payload:             b.payload,
		ConversationContext: b.conversationContext,
		Signature:           nil,
	}, nil
}

// RequestMessage creates a Request message
func RequestMessage(from, to agentcard.DID, intent Intent) (*AACLMessage, error) {
	return NewMessage(MessageTypeRequest, from, to).
		WithIntent(intent).
		Build()
}

// ResponseMessage creates a Response message
func ResponseMessage(from, to agentcard.DID, payload ResponsePayload) (*AACLMessage, error) {
	return NewMessage(MessageTypeResponse, from, to).
		WithPayload(payload).
		Build()
}

// ErrorMessage creates an Error message
func ErrorMessage(from, to agentcard.DID, errorInfo ErrorInfo) (*AACLMessage, error) {
	return NewMessage(MessageTypeError, from, to).
		WithPayload(errorInfo).
		Build()
}

// WorkflowRequestMessage creates a WorkflowRequest message
func WorkflowRequestMessage(from, to agentcard.DID, workflow Workflow) (*AACLMessage, error) {
	return NewMessage(MessageTypeWorkflowRequest, from, to).
		WithPayload(workflow).
		Build()
}

// Sign signs an AACL message with an Ed25519 private key
func (m *AACLMessage) Sign(privateKey ed25519.PrivateKey) error {
	// Remove existing signature
	m.Signature = nil

	// Create canonical JSON
	msgJSON, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Sign the JSON
	signature := ed25519.Sign(privateKey, msgJSON)

	// Encode signature as base64
	signatureStr := base64.StdEncoding.EncodeToString(signature)
	m.Signature = &signatureStr

	return nil
}

// Verify verifies the message signature
func (m *AACLMessage) Verify(publicKey ed25519.PublicKey) (bool, error) {
	if m.Signature == nil {
		return false, fmt.Errorf("message has no signature")
	}

	// Decode signature
	signature, err := base64.StdEncoding.DecodeString(*m.Signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Remove signature for verification
	sigBackup := m.Signature
	m.Signature = nil
	msgJSON, err := json.Marshal(m)
	m.Signature = sigBackup
	if err != nil {
		return false, fmt.Errorf("failed to marshal message: %w", err)
	}

	// Verify signature
	return ed25519.Verify(publicKey, msgJSON, signature), nil
}

// ToJSON converts the message to JSON
func (m *AACLMessage) ToJSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

// FromJSON parses an AACLMessage from JSON
func FromJSON(data []byte) (*AACLMessage, error) {
	var msg AACLMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// SuccessResponse creates a successful ResponsePayload
func SuccessResponse(result interface{}, metadata *ExecutionMetadata) ResponsePayload {
	return ResponsePayload{
		Status: "success",
		Result: &ResponseResult{
			Value: result,
		},
		ExecutionMetadata: metadata,
	}
}

// ErrorResponse creates an error ResponsePayload
func ErrorResponse(errorInfo ErrorInfo) ResponsePayload {
	return ResponsePayload{
		Status: "error",
		Error:  &errorInfo,
	}
}

// NewConversation creates a new ConversationContext
func NewConversation() ConversationContext {
	return ConversationContext{
		ConversationID:   uuid.New().String(),
		PreviousMessages: []string{},
		SharedState:      make(map[string]interface{}),
	}
}

// AddMessage adds a message ID to the conversation history
func (c *ConversationContext) AddMessage(messageID string) {
	c.PreviousMessages = append(c.PreviousMessages, messageID)
}

// SetState sets a value in the shared state
func (c *ConversationContext) SetState(key string, value interface{}) {
	c.SharedState[key] = value
}

// GetState retrieves a value from shared state
func (c *ConversationContext) GetState(key string) (interface{}, bool) {
	val, ok := c.SharedState[key]
	return val, ok
}
