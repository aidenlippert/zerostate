package orchestration

import (
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/aacl-go"
	"github.com/aidenlippert/zerostate/libs/agentcard-go"
	"go.uber.org/zap"
)

func TestAACLAdapter_ParseRequest(t *testing.T) {
	adapter := NewAACLAdapter(zap.NewNop())

	// Create a sample AACL request
	fromDID := agentcard.NewUserDID("alice")
	toDID := agentcard.NewAgentDID("math-001")

	intent := aacl.NewIntent("compute", "Calculate the sum of two numbers").
		WithNaturalLanguage("Please add 5 and 7").
		WithParameter("operation", "add").
		WithParameter("a", float64(5)).
		WithParameter("b", float64(7)).
		RequiresCapability("math.add").
		WithConfidence(0.95).
		Build()

	msg, err := aacl.RequestMessage(fromDID, toDID, intent)
	if err != nil {
		t.Fatalf("Failed to create request message: %v", err)
	}

	// Parse into task
	task, err := adapter.ParseAACLRequest(msg)
	if err != nil {
		t.Fatalf("Failed to parse AACL request: %v", err)
	}

	// Verify task fields
	if task.UserID != string(fromDID) {
		t.Errorf("Expected user ID %s, got %s", fromDID, task.UserID)
	}

	if task.Type != "compute" {
		t.Errorf("Expected type 'compute', got '%s'", task.Type)
	}

	if task.Description != "Calculate the sum of two numbers" {
		t.Errorf("Unexpected description: %s", task.Description)
	}

	if len(task.Capabilities) != 1 || task.Capabilities[0] != "math.add" {
		t.Errorf("Expected capability 'math.add', got %v", task.Capabilities)
	}

	if task.Input["operation"] != "add" {
		t.Error("Operation parameter not preserved")
	}

	if task.Input["a"] != float64(5) {
		t.Errorf("Parameter 'a' mismatch: expected 5, got %v", task.Input["a"])
	}

	if task.Metadata["aacl_message_id"] != msg.ID {
		t.Error("Message ID not stored in metadata")
	}

	t.Logf("Task parsed successfully: ID=%s, Type=%s, Caps=%v", task.ID, task.Type, task.Capabilities)
}

func TestAACLAdapter_FormatResponse(t *testing.T) {
	adapter := NewAACLAdapter(zap.NewNop())

	// Create a sample task
	task := NewTask("did:ainur:user:alice", "compute", []string{"math.add"}, map[string]interface{}{
		"a": 5,
		"b": 7,
	})
	task.Description = "Add two numbers"
	task.Status = TaskStatusCompleted
	now := time.Now()
	task.StartedAt = &now
	completed := now.Add(125 * time.Millisecond)
	task.CompletedAt = &completed
	task.ActualCost = 10.0
	task.Metadata["aacl_message_id"] = "urn:uuid:original-message"

	// Create result
	result := &TaskResult{
		TaskID: task.ID,
		Status: TaskStatusCompleted,
		Result: map[string]interface{}{
			"sum": 12,
		},
		ExecutionMS: 125,
		AgentDID:    "did:ainur:agent:math-001",
		Timestamp:   *task.CompletedAt,
	}

	// Create sample AgentCard
	agentDID := agentcard.NewAgentDID("math-001")
	rep := agentcard.NewReputation()
	rep.TrustScore = 95.5

	card, _ := agentcard.NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Math Agent").
		SetDescription("Test agent").
		SetVersion("1.0.0").
		SetCapabilities(agentcard.Capabilities{
			Domains: []string{"math"},
			Operations: []agentcard.Operation{
				{Name: "add", Category: "arithmetic", GasEstimate: 100},
			},
			Constraints: agentcard.CapabilityConstraints{
				MaxInputSize:       1024,
				MaxExecutionTimeMs: 5000,
			},
			Interfaces: []string{"grpc"},
		}).
		SetRuntime(agentcard.RuntimeInfo{
			Protocol:       "ari-v1",
			Implementation: "test",
			Version:        "1.0.0",
			WasmEngine:     "wasmtime",
			WasmVersion:    "23.0.0",
			ModuleHash:     "test",
			ExecutionEnvironment: agentcard.ExecutionEnvironment{
				MemoryLimitMB:  128,
				CPUQuotaMs:     1000,
				NetworkEnabled: true,
			},
			Endpoints: []agentcard.Endpoint{},
		}).
		SetNetwork(agentcard.Network{
			P2P: agentcard.P2PConfig{
				PeerID:          "12D3Test",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/test/1.0.0"},
			},
			Discovery: agentcard.Discovery{
				Methods: []string{"mdns"},
			},
			Availability: agentcard.Availability{
				Regions:        []string{"local"},
				LatencyTargets: agentcard.LatencyTargets{P50Ms: 50, P95Ms: 100, P99Ms: 200},
			},
		}).
		SetReputation(rep).
		Build()

	// Format response
	fromDID := agentcard.NewAgentDID("math-001")
	toDID := agentcard.NewUserDID("alice")

	msg, err := adapter.FormatAACLResponse(task, result, fromDID, toDID, card)
	if err != nil {
		t.Fatalf("Failed to format AACL response: %v", err)
	}

	// Verify response
	if msg.Type != string(aacl.MessageTypeResponse) {
		t.Errorf("Expected Response type, got %s", msg.Type)
	}

	if msg.From != fromDID {
		t.Error("From DID mismatch")
	}

	if msg.To != toDID {
		t.Error("To DID mismatch")
	}

	// Check payload
	payload, ok := msg.Payload.(aacl.ResponsePayload)
	if !ok {
		t.Fatal("Payload is not ResponsePayload")
	}

	if payload.Status != "success" {
		t.Errorf("Expected success status, got %s", payload.Status)
	}

	if payload.Result == nil {
		t.Fatal("Result should not be nil")
	}

	if payload.ExecutionMetadata == nil {
		t.Fatal("Execution metadata should not be nil")
	}

	if payload.ExecutionMetadata.AgentTrustScore != 95.5 {
		t.Errorf("Trust score mismatch: expected 95.5, got %f", payload.ExecutionMetadata.AgentTrustScore)
	}

	if payload.ExecutionMetadata.DurationMs != 125 {
		t.Errorf("Duration mismatch: expected 125ms, got %d", payload.ExecutionMetadata.DurationMs)
	}

	t.Logf("Response formatted successfully: ID=%s, Status=%s, TrustScore=%.1f",
		msg.ID, payload.Status, payload.ExecutionMetadata.AgentTrustScore)
}

func TestAACLAdapter_InferCapabilities(t *testing.T) {
	adapter := NewAACLAdapter(zap.NewNop())

	tests := []struct {
		action       string
		goal         string
		expectedCaps []string
	}{
		{"compute", "add two numbers", []string{"math.add"}},
		{"calculate", "multiply 5 by 7", []string{"math.multiply"}},
		{"compute", "divide 10 by 2", []string{"math.divide"}},
		{"process", "transform data", []string{"data.process"}},
		{"query", "search database", []string{"data.query"}},
		{"unknown", "do something", []string{"compute"}},
	}

	for _, tt := range tests {
		t.Run(tt.action+"_"+tt.goal, func(t *testing.T) {
			caps := adapter.inferCapabilities(tt.action, tt.goal)
			if len(caps) != len(tt.expectedCaps) {
				t.Errorf("Expected %d capabilities, got %d", len(tt.expectedCaps), len(caps))
				return
			}
			for i, expected := range tt.expectedCaps {
				if caps[i] != expected {
					t.Errorf("Expected capability '%s', got '%s'", expected, caps[i])
				}
			}
		})
	}
}

func TestAACLAdapter_ExtractCapabilities(t *testing.T) {
	adapter := NewAACLAdapter(zap.NewNop())

	agentDID := agentcard.NewAgentDID("test")
	card, _ := agentcard.NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Test Agent").
		SetDescription("Test").
		SetCapabilities(agentcard.Capabilities{
			Domains: []string{"math", "data"},
			Operations: []agentcard.Operation{
				{Name: "add", Category: "arithmetic"},
				{Name: "multiply", Category: "arithmetic"},
				{Name: "process", Category: "transform"},
			},
			Constraints: agentcard.CapabilityConstraints{
				MaxInputSize:       1024,
				MaxExecutionTimeMs: 5000,
			},
			Interfaces: []string{"grpc"},
		}).
		SetRuntime(agentcard.RuntimeInfo{
			Protocol:       "test",
			Implementation: "test",
			Version:        "1.0.0",
			WasmEngine:     "test",
			WasmVersion:    "1.0.0",
			ModuleHash:     "test",
			ExecutionEnvironment: agentcard.ExecutionEnvironment{
				MemoryLimitMB:  64,
				CPUQuotaMs:     500,
				NetworkEnabled: false,
			},
			Endpoints: []agentcard.Endpoint{},
		}).
		SetNetwork(agentcard.Network{
			P2P: agentcard.P2PConfig{
				PeerID:          "test",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/test/1.0.0"},
			},
			Discovery: agentcard.Discovery{
				Methods: []string{"mdns"},
			},
			Availability: agentcard.Availability{
				Regions:        []string{"local"},
				LatencyTargets: agentcard.LatencyTargets{P50Ms: 10, P95Ms: 50, P99Ms: 100},
			},
		}).
		Build()

	caps := adapter.ExtractCapabilitiesFromAgentCard(card)

	expectedCaps := []string{"arithmetic.add", "arithmetic.multiply", "transform.process"}
	if len(caps) != len(expectedCaps) {
		t.Errorf("Expected %d capabilities, got %d", len(expectedCaps), len(caps))
	}

	for i, expected := range expectedCaps {
		if caps[i] != expected {
			t.Errorf("Capability %d: expected '%s', got '%s'", i, expected, caps[i])
		}
	}
}
