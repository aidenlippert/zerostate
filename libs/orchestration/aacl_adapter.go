package orchestration

import (
	"fmt"
	"strings"

	"github.com/aidenlippert/zerostate/libs/aacl-go"
	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"go.uber.org/zap"
)

// AACLAdapter adapts between AACL messages and orchestration tasks
type AACLAdapter struct {
	logger *zap.Logger
}

// NewAACLAdapter creates a new AACL adapter
func NewAACLAdapter(logger *zap.Logger) *AACLAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &AACLAdapter{
		logger: logger,
	}
}

// ParseAACLRequest converts an AACL Request message into an orchestration Task
func (a *AACLAdapter) ParseAACLRequest(msg *aacl.AACLMessage) (*Task, error) {
	if msg.Type != string(aacl.MessageTypeRequest) {
		return nil, fmt.Errorf("expected Request message, got %s", msg.Type)
	}

	if msg.Intent == nil {
		return nil, fmt.Errorf("request message missing intent")
	}

	intent := msg.Intent

	// Extract user ID from DID
	userID := string(msg.From)

	// Determine task type from intent action
	taskType := intent.Action
	if taskType == "" {
		taskType = "general"
	}

	// Extract capabilities from intent
	capabilities := intent.CapabilitiesRequired
	if capabilities == nil {
		capabilities = []string{}
	}

	// If no explicit capabilities, infer from action
	if len(capabilities) == 0 {
		capabilities = a.inferCapabilities(intent.Action, intent.Goal)
	}

	// Build input from intent parameters
	input := make(map[string]interface{})
	for k, v := range intent.Parameters {
		input[k] = v
	}

	// Add natural language if present
	if intent.NaturalLanguage != nil {
		input["natural_language"] = *intent.NaturalLanguage
	}

	// Add structured parsed data if present
	if intent.Parsed != nil {
		input["parsed"] = intent.Parsed
	}

	// Create task
	task := NewTask(userID, taskType, capabilities, input)
	task.Description = intent.Goal

	// Set metadata from message
	task.Metadata["aacl_message_id"] = msg.ID
	task.Metadata["intent_action"] = intent.Action
	task.Metadata["intent_goal"] = intent.Goal
	if intent.Confidence != nil {
		task.Metadata["intent_confidence"] = *intent.Confidence
	}

	// Handle conversation context if present
	if msg.ConversationContext != nil {
		task.Metadata["conversation_id"] = msg.ConversationContext.ConversationID
		task.Metadata["previous_messages"] = msg.ConversationContext.PreviousMessages
		if msg.ConversationContext.SharedState != nil {
			task.Metadata["conversation_state"] = msg.ConversationContext.SharedState
		}
	}

	a.logger.Debug("parsed AACL request into task",
		zap.String("message_id", msg.ID),
		zap.String("task_id", task.ID),
		zap.String("action", intent.Action),
		zap.String("goal", intent.Goal),
		zap.Int("capabilities", len(capabilities)),
	)

	return task, nil
}

// inferCapabilities attempts to infer required capabilities from intent
func (a *AACLAdapter) inferCapabilities(action, goal string) []string {
	action = strings.ToLower(action)
	goal = strings.ToLower(goal)

	// Simple heuristics for capability inference
	if strings.Contains(action, "compute") || strings.Contains(action, "calculate") {
		if strings.Contains(goal, "add") || strings.Contains(goal, "sum") {
			return []string{"math.add"}
		}
		if strings.Contains(goal, "multiply") || strings.Contains(goal, "product") {
			return []string{"math.multiply"}
		}
		if strings.Contains(goal, "divide") {
			return []string{"math.divide"}
		}
		return []string{"math.compute"}
	}

	if strings.Contains(action, "process") || strings.Contains(action, "transform") {
		return []string{"data.process"}
	}

	if strings.Contains(action, "query") || strings.Contains(action, "search") {
		return []string{"data.query"}
	}

	// Default to general computation
	return []string{"compute"}
}

// FormatAACLResponse converts a TaskResult into an AACL Response message
func (a *AACLAdapter) FormatAACLResponse(
	task *Task,
	result *TaskResult,
	fromDID agentcard.DID,
	toDID agentcard.DID,
	agentCard *agentcard.AgentCard,
) (*aacl.AACLMessage, error) {
	var payload aacl.ResponsePayload

	if result.Status == TaskStatusCompleted {
		// Calculate execution time
		var durationMs uint64
		if task.StartedAt != nil && task.CompletedAt != nil {
			durationMs = uint64(task.CompletedAt.Sub(*task.StartedAt).Milliseconds())
		} else {
			durationMs = uint64(result.ExecutionMS)
		}

		// Build execution metadata
		metadata := &aacl.ExecutionMetadata{
			DurationMs:      durationMs,
			GasUsed:         calculateGasUsed(task),
			CostUainur:      uint64(task.ActualCost),
			AgentVersion:    "1.0.0",
			AgentTrustScore: 50.0, // Default, will be updated with real reputation
			ExecutionNodeID: result.AgentDID,
		}

		// Add agent trust score if available
		if agentCard != nil {
			metadata.AgentTrustScore = agentCard.CredentialSubject.Reputation.TrustScore
			metadata.AgentVersion = agentCard.CredentialSubject.Version
		}

		payload = aacl.SuccessResponse(result.Result, metadata)
	} else {
		// Task failed
		errorInfo := aacl.ErrorInfo{
			Code:        "EXECUTION_FAILED",
			Message:     result.Error,
			Recoverable: task.CanRetry(),
		}

		if task.CanRetry() {
			errorInfo.RecoverySuggestions = []string{
				"Task will be automatically retried",
				fmt.Sprintf("Retry %d of %d", task.RetryCount, task.MaxRetries),
			}
		}

		payload = aacl.ErrorResponse(errorInfo)
	}

	// Create response message
	msg, err := aacl.ResponseMessage(fromDID, toDID, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create response message: %w", err)
	}

	// Add conversation context if original had it
	if convID, ok := task.Metadata["conversation_id"].(string); ok {
		ctx := aacl.ConversationContext{
			ConversationID: convID,
		}
		if prevMsgs, ok := task.Metadata["previous_messages"].([]string); ok {
			ctx.PreviousMessages = prevMsgs
		}
		if msgID, ok := task.Metadata["aacl_message_id"].(string); ok {
			ctx.AddMessage(msgID)
		}
		ctx.AddMessage(msg.ID)
		msg.ConversationContext = &ctx
	}

	a.logger.Debug("formatted AACL response",
		zap.String("task_id", task.ID),
		zap.String("message_id", msg.ID),
		zap.String("status", string(result.Status)),
	)

	return msg, nil
}

// ParseAACLRequestJSON parses AACL request from JSON bytes
func (a *AACLAdapter) ParseAACLRequestJSON(data []byte) (*Task, error) {
	msg, err := aacl.FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse AACL message: %w", err)
	}

	return a.ParseAACLRequest(msg)
}

// FormatAACLResponseJSON formats AACL response as JSON bytes
func (a *AACLAdapter) FormatAACLResponseJSON(
	task *Task,
	result *TaskResult,
	fromDID agentcard.DID,
	toDID agentcard.DID,
	agentCard *agentcard.AgentCard,
) ([]byte, error) {
	msg, err := a.FormatAACLResponse(task, result, fromDID, toDID, agentCard)
	if err != nil {
		return nil, err
	}

	return msg.ToJSON()
}

// calculateGasUsed estimates gas used for task execution
func calculateGasUsed(task *Task) uint64 {
	// Simple heuristic: base cost + per-capability cost
	baseCost := uint64(100)
	capabilityCost := uint64(len(task.Capabilities)) * 50

	// Add cost based on execution time
	var timeCost uint64
	if task.StartedAt != nil && task.CompletedAt != nil {
		durationMs := task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
		timeCost = uint64(durationMs) / 10 // 1 gas per 10ms
	}

	return baseCost + capabilityCost + timeCost
}

// CreateAACLWorkflowFromDAG converts a DAG into an AACL Workflow
func (a *AACLAdapter) CreateAACLWorkflowFromDAG(
	dag *DAGWorkflow,
	fromDID agentcard.DID,
	toDID agentcard.DID,
) (*aacl.AACLMessage, error) {
	workflow := aacl.Workflow{
		WorkflowID:   dag.ID,
		Goal:         fmt.Sprintf("Execute DAG with %d nodes", len(dag.Nodes)),
		Steps:        make([]aacl.WorkflowStep, 0, len(dag.Nodes)),
		Dependencies: make(map[string][]string),
	}

	// Convert DAG nodes to workflow steps
	for nodeID, node := range dag.Nodes {
		goal := node.Name
		if goal == "" {
			goal = fmt.Sprintf("Execute node %s", nodeID)
		}

		intent := aacl.NewIntent(node.TaskType, goal).
			WithParameter("node_id", node.ID).
			Build()

		for k, v := range node.Input {
			intent.Parameters[k] = v
		}

		step := aacl.WorkflowStep{
			StepID:   node.ID,
			AgentDID: agentcard.DID(node.AssignedTo),
			Intent:   intent,
		}

		// Add dependencies
		if len(node.Dependencies) > 0 {
			step.DependsOn = node.Dependencies
			workflow.Dependencies[node.ID] = node.Dependencies
		}

		workflow.Steps = append(workflow.Steps, step)
	}

	// Create workflow request message
	return aacl.WorkflowRequestMessage(fromDID, toDID, workflow)
}

// SupportedAACLFormats returns supported AACL message formats
func (a *AACLAdapter) SupportedAACLFormats() []string {
	return []string{
		"aacl-v1",
		"application/aacl+json",
		"application/ld+json",
	}
}

// ValidateAACLMessage validates an AACL message structure
func (a *AACLAdapter) ValidateAACLMessage(msg *aacl.AACLMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	if msg.ID == "" {
		return fmt.Errorf("message ID is required")
	}

	if msg.From == "" {
		return fmt.Errorf("from DID is required")
	}

	if msg.To == "" {
		return fmt.Errorf("to DID is required")
	}

	if msg.Type == "" {
		return fmt.Errorf("message type is required")
	}

	if msg.Type == string(aacl.MessageTypeRequest) && msg.Intent == nil {
		return fmt.Errorf("request message must have intent")
	}

	return nil
}

// ExtractCapabilitiesFromAgentCard extracts capability strings from an AgentCard
func (a *AACLAdapter) ExtractCapabilitiesFromAgentCard(card *agentcard.AgentCard) []string {
	if card == nil {
		return []string{}
	}

	capabilities := make([]string, 0, len(card.CredentialSubject.Capabilities.Operations))
	for _, op := range card.CredentialSubject.Capabilities.Operations {
		// Format: domain.operation or category.operation
		capString := fmt.Sprintf("%s.%s", op.Category, op.Name)
		capabilities = append(capabilities, capString)
	}

	return capabilities
}
