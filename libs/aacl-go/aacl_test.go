package aacl

import (
	"encoding/json"
	"testing"

	"github.com/aidenlippert/zerostate/libs/agentcard-go"
)

func TestIntentBuilder(t *testing.T) {
	intent := NewIntent("compute", "Calculate the sum of two numbers").
		WithNaturalLanguage("Please add 5 and 7").
		WithParameter("operation", "add").
		WithParameter("a", 5).
		WithParameter("b", 7).
		RequiresCapability("math.add").
		WithConfidence(0.95).
		Build()

	if intent.Action != "compute" {
		t.Errorf("Expected action 'compute', got '%s'", intent.Action)
	}

	if intent.Goal != "Calculate the sum of two numbers" {
		t.Errorf("Unexpected goal: %s", intent.Goal)
	}

	if len(intent.Parameters) != 3 {
		t.Errorf("Expected 3 parameters, got %d", len(intent.Parameters))
	}

	if intent.Parameters["operation"] != "add" {
		t.Error("Parameter 'operation' should be 'add'")
	}

	if len(intent.CapabilitiesRequired) != 1 {
		t.Errorf("Expected 1 required capability, got %d", len(intent.CapabilitiesRequired))
	}

	if *intent.Confidence != 0.95 {
		t.Errorf("Expected confidence 0.95, got %f", *intent.Confidence)
	}

	if intent.NaturalLanguage == nil || *intent.NaturalLanguage != "Please add 5 and 7" {
		t.Error("Natural language not set correctly")
	}
}

func TestMessageBuilder(t *testing.T) {
	fromDID := agentcard.NewUserDID("alice")
	toDID := agentcard.NewAgentDID("math-001")

	intent := NewIntent("compute", "Add two numbers").
		WithParameter("a", 10).
		WithParameter("b", 20).
		Build()

	msg, err := NewMessage(MessageTypeRequest, fromDID, toDID).
		WithIntent(intent).
		Build()

	if err != nil {
		t.Fatalf("Failed to build message: %v", err)
	}

	if msg.Type != string(MessageTypeRequest) {
		t.Errorf("Expected type 'Request', got '%s'", msg.Type)
	}

	if msg.From != fromDID {
		t.Errorf("From DID mismatch")
	}

	if msg.To != toDID {
		t.Errorf("To DID mismatch")
	}

	if msg.Intent == nil {
		t.Fatal("Intent should not be nil")
	}

	if msg.Intent.Action != "compute" {
		t.Errorf("Intent action mismatch")
	}

	if msg.ID == "" {
		t.Error("Message ID should not be empty")
	}

	if msg.Context != "https://ainur.network/contexts/aacl/v1" {
		t.Errorf("Unexpected context: %s", msg.Context)
	}
}

func TestRequestMessage(t *testing.T) {
	fromDID := agentcard.NewUserDID("bob")
	toDID := agentcard.NewAgentDID("calculator")

	intent := NewIntent("calculate", "Compute factorial").
		WithParameter("n", 5).
		Build()

	msg, err := RequestMessage(fromDID, toDID, intent)
	if err != nil {
		t.Fatalf("Failed to create request message: %v", err)
	}

	if msg.Type != string(MessageTypeRequest) {
		t.Errorf("Expected Request type, got %s", msg.Type)
	}

	if msg.Intent == nil {
		t.Fatal("Request should have an intent")
	}
}

func TestResponseMessage(t *testing.T) {
	fromDID := agentcard.NewAgentDID("math-001")
	toDID := agentcard.NewUserDID("alice")

	payload := SuccessResponse(
		map[string]interface{}{"sum": 12},
		&ExecutionMetadata{
			DurationMs:      125,
			GasUsed:         100,
			CostUainur:      10,
			AgentVersion:    "1.0.0",
			AgentTrustScore: 95.5,
		},
	)

	msg, err := ResponseMessage(fromDID, toDID, payload)
	if err != nil {
		t.Fatalf("Failed to create response message: %v", err)
	}

	if msg.Type != string(MessageTypeResponse) {
		t.Errorf("Expected Response type, got %s", msg.Type)
	}

	// Type assertion to check payload
	responsePayload, ok := msg.Payload.(ResponsePayload)
	if !ok {
		t.Fatal("Payload is not ResponsePayload")
	}

	if responsePayload.Status != "success" {
		t.Errorf("Expected status 'success', got '%s'", responsePayload.Status)
	}

	if responsePayload.ExecutionMetadata == nil {
		t.Fatal("Execution metadata should not be nil")
	}

	if responsePayload.ExecutionMetadata.GasUsed != 100 {
		t.Errorf("Expected gas used 100, got %d", responsePayload.ExecutionMetadata.GasUsed)
	}
}

func TestErrorMessage(t *testing.T) {
	fromDID := agentcard.NewAgentDID("broken-agent")
	toDID := agentcard.NewUserDID("user")

	errorInfo := ErrorInfo{
		Code:        "INVALID_INPUT",
		Message:     "Parameter 'x' must be a number",
		Recoverable: true,
		RecoverySuggestions: []string{
			"Provide a numeric value for parameter 'x'",
		},
	}

	msg, err := ErrorMessage(fromDID, toDID, errorInfo)
	if err != nil {
		t.Fatalf("Failed to create error message: %v", err)
	}

	if msg.Type != string(MessageTypeError) {
		t.Errorf("Expected Error type, got %s", msg.Type)
	}

	errorPayload, ok := msg.Payload.(ErrorInfo)
	if !ok {
		t.Fatal("Payload is not ErrorInfo")
	}

	if errorPayload.Code != "INVALID_INPUT" {
		t.Errorf("Error code mismatch")
	}

	if !errorPayload.Recoverable {
		t.Error("Error should be recoverable")
	}
}

func TestWorkflowMessage(t *testing.T) {
	fromDID := agentcard.NewUserDID("orchestrator")
	toDID := agentcard.NewAgentDID("workflow-engine")

	workflow := Workflow{
		WorkflowID: "wf-123",
		Goal:       "Process data pipeline",
		Steps: []WorkflowStep{
			{
				StepID:   "step-1",
				AgentDID: agentcard.NewAgentDID("fetch-001"),
				Intent: NewIntent("fetch", "Get data from API").
					WithParameter("url", "https://api.example.com/data").
					Build(),
			},
			{
				StepID:   "step-2",
				AgentDID: agentcard.NewAgentDID("transform-001"),
				Intent: NewIntent("transform", "Convert JSON to CSV").
					Build(),
				DependsOn: []string{"step-1"},
			},
		},
		Dependencies: map[string][]string{
			"step-2": {"step-1"},
		},
	}

	msg, err := WorkflowRequestMessage(fromDID, toDID, workflow)
	if err != nil {
		t.Fatalf("Failed to create workflow message: %v", err)
	}

	if msg.Type != string(MessageTypeWorkflowRequest) {
		t.Errorf("Expected WorkflowRequest type, got %s", msg.Type)
	}

	workflowPayload, ok := msg.Payload.(Workflow)
	if !ok {
		t.Fatal("Payload is not Workflow")
	}

	if len(workflowPayload.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(workflowPayload.Steps))
	}

	if workflowPayload.Steps[1].DependsOn[0] != "step-1" {
		t.Error("Step 2 should depend on step-1")
	}
}

func TestSerialization(t *testing.T) {
	fromDID := agentcard.NewUserDID("test-user")
	toDID := agentcard.NewAgentDID("test-agent")

	intent := NewIntent("test", "Test serialization").
		WithParameter("key", "value").
		Build()

	msg, err := RequestMessage(fromDID, toDID, intent)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Serialize to JSON
	jsonData, err := msg.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	// Deserialize from JSON
	parsedMsg, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to deserialize: %v", err)
	}

	if parsedMsg.Type != msg.Type {
		t.Error("Message type mismatch after round trip")
	}

	if parsedMsg.From != msg.From {
		t.Error("From DID mismatch after round trip")
	}

	if parsedMsg.To != msg.To {
		t.Error("To DID mismatch after round trip")
	}
}

func TestSignAndVerify(t *testing.T) {
	// Generate keypair
	publicKey, privateKey, err := agentcard.GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	fromDID := agentcard.NewAgentDID("signing-agent")
	toDID := agentcard.NewUserDID("user")

	intent := NewIntent("sign_test", "Test message signing").Build()
	msg, err := RequestMessage(fromDID, toDID, intent)
	if err != nil {
		t.Fatalf("Failed to create message: %v", err)
	}

	// Sign the message
	if err := msg.Sign(privateKey); err != nil {
		t.Fatalf("Failed to sign message: %v", err)
	}

	if msg.Signature == nil {
		t.Fatal("Signature should not be nil after signing")
	}

	// Verify signature
	valid, err := msg.Verify(publicKey)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}

	if !valid {
		t.Error("Signature verification failed")
	}

	// Test with wrong key
	wrongKey, _, _ := agentcard.GenerateKeyPair()
	valid, err = msg.Verify(wrongKey)
	if err != nil {
		t.Fatalf("Verification error with wrong key: %v", err)
	}

	if valid {
		t.Error("Signature should not verify with wrong key")
	}
}

func TestConversationContext(t *testing.T) {
	ctx := NewConversation()

	if ctx.ConversationID == "" {
		t.Error("Conversation ID should not be empty")
	}

	// Add messages
	ctx.AddMessage("msg-1")
	ctx.AddMessage("msg-2")

	if len(ctx.PreviousMessages) != 2 {
		t.Errorf("Expected 2 messages in history, got %d", len(ctx.PreviousMessages))
	}

	// Test shared state
	ctx.SetState("user_preference", "dark_mode")
	ctx.SetState("count", 42)

	pref, ok := ctx.GetState("user_preference")
	if !ok {
		t.Fatal("user_preference should exist")
	}

	if pref != "dark_mode" {
		t.Errorf("Expected 'dark_mode', got '%v'", pref)
	}

	count, ok := ctx.GetState("count")
	if !ok {
		t.Fatal("count should exist")
	}

	if count != 42 {
		t.Errorf("Expected 42, got %v", count)
	}

	_, ok = ctx.GetState("nonexistent")
	if ok {
		t.Error("Nonexistent key should return false")
	}
}

func TestMessageWithConversation(t *testing.T) {
	fromDID := agentcard.NewUserDID("conversational-user")
	toDID := agentcard.NewAgentDID("assistant")

	ctx := NewConversation()
	ctx.SetState("topic", "weather")

	intent := NewIntent("query", "Get weather forecast").Build()

	msg, err := NewMessage(MessageTypeRequest, fromDID, toDID).
		WithIntent(intent).
		WithConversationContext(ctx).
		Build()

	if err != nil {
		t.Fatalf("Failed to build message: %v", err)
	}

	if msg.ConversationContext == nil {
		t.Fatal("Conversation context should not be nil")
	}

	if msg.ConversationContext.ConversationID == "" {
		t.Error("Conversation ID should not be empty")
	}

	topic, ok := msg.ConversationContext.GetState("topic")
	if !ok || topic != "weather" {
		t.Error("Topic state not preserved")
	}
}

func TestComplexPayload(t *testing.T) {
	fromDID := agentcard.NewAgentDID("ml-agent")
	toDID := agentcard.NewUserDID("data-scientist")

	// Create complex result
	result := map[string]interface{}{
		"predictions": []float64{0.92, 0.87, 0.95},
		"model":       "random-forest",
		"accuracy":    0.91,
		"features": map[string]interface{}{
			"count": 10,
			"names": []string{"feature1", "feature2", "feature3"},
		},
	}

	metadata := &ExecutionMetadata{
		DurationMs:      1250,
		GasUsed:         5000,
		CostUainur:      500,
		AgentVersion:    "2.1.0",
		AgentTrustScore: 88.5,
		ExecutionNodeID: "node-42",
	}

	payload := SuccessResponse(result, metadata)
	msg, err := ResponseMessage(fromDID, toDID, payload)
	if err != nil {
		t.Fatalf("Failed to create message with complex payload: %v", err)
	}

	// Serialize and deserialize
	jsonData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var parsedMsg AACLMessage
	if err := json.Unmarshal(jsonData, &parsedMsg); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify complex structure survives round trip
	payloadMap, ok := parsedMsg.Payload.(map[string]interface{})
	if !ok {
		t.Fatal("Payload should be a map")
	}

	if payloadMap["status"] != "success" {
		t.Error("Status not preserved")
	}
}
