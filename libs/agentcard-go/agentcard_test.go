package agentcard

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAgentCardBuilder(t *testing.T) {
	agentDID := NewAgentDID("math-agent-001")

	capabilities := Capabilities{
		Domains: []string{"mathematics", "computation"},
		Operations: []Operation{
			{
				Name:        "add",
				Category:    "arithmetic",
				GasEstimate: 100,
			},
			{
				Name:        "multiply",
				Category:    "arithmetic",
				GasEstimate: 150,
			},
		},
		Constraints: CapabilityConstraints{
			MaxInputSize:       1024,
			MaxExecutionTimeMs: 5000,
		},
		Interfaces: []string{"http", "p2p"},
	}

	runtime := RuntimeInfo{
		Protocol:       "ainur-v1",
		Implementation: "reference-runtime",
		Version:        "0.1.0",
		WasmEngine:     "wasmtime",
		WasmVersion:    "23.0.0",
		ModuleHash:     "sha256:abc123",
		ExecutionEnvironment: ExecutionEnvironment{
			MemoryLimitMB:     128,
			CPUQuotaMs:        1000,
			NetworkEnabled:    true,
			FilesystemEnabled: false,
		},
		Endpoints: []Endpoint{
			{
				Protocol: "http",
				Address:  "http://localhost:8080",
			},
		},
	}

	network := Network{
		P2P: P2PConfig{
			PeerID:          "12D3KooWTest",
			ListenAddresses: []string{"/ip4/0.0.0.0/tcp/4001"},
			Protocols:       []string{"/ainur/gossipsub/1.0.0"},
		},
		Discovery: Discovery{
			Methods: []string{"mdns", "dht"},
		},
		Availability: Availability{
			Regions: []string{"us-west", "eu-central"},
			LatencyTargets: LatencyTargets{
				P50Ms: 50,
				P95Ms: 100,
				P99Ms: 200,
			},
		},
	}

	card, err := NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Math Agent").
		SetDescription("Agent for mathematical computations").
		SetVersion("1.0.0").
		SetCapabilities(capabilities).
		SetRuntime(runtime).
		SetNetwork(network).
		SetExpirationDays(365).
		Build()

	if err != nil {
		t.Fatalf("Failed to build AgentCard: %v", err)
	}

	if card.CredentialSubject.ID != agentDID {
		t.Errorf("Expected DID %s, got %s", agentDID, card.CredentialSubject.ID)
	}

	if card.CredentialSubject.Name != "Math Agent" {
		t.Errorf("Expected name 'Math Agent', got '%s'", card.CredentialSubject.Name)
	}

	if len(card.CredentialSubject.Capabilities.Operations) != 2 {
		t.Errorf("Expected 2 operations, got %d", len(card.CredentialSubject.Capabilities.Operations))
	}

	if card.CredentialSubject.Reputation.TrustScore != 50.0 {
		t.Errorf("Expected default trust score 50.0, got %f", card.CredentialSubject.Reputation.TrustScore)
	}

	if card.Proof != nil {
		t.Error("Expected no proof before signing")
	}
}

func TestAgentCardSerialization(t *testing.T) {
	agentDID := NewAgentDID("test-001")

	card, err := NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Test Agent").
		SetDescription("Agent for testing").
		SetCapabilities(Capabilities{
			Domains: []string{"testing"},
			Operations: []Operation{
				{Name: "test", Category: "verification", GasEstimate: 50},
			},
			Constraints: CapabilityConstraints{
				MaxInputSize:       512,
				MaxExecutionTimeMs: 1000,
			},
			Interfaces: []string{"http"},
		}).
		SetRuntime(RuntimeInfo{
			Protocol:       "ainur-v1",
			Implementation: "test",
			Version:        "0.1.0",
			WasmEngine:     "wasmtime",
			WasmVersion:    "23.0.0",
			ModuleHash:     "sha256:test",
			ExecutionEnvironment: ExecutionEnvironment{
				MemoryLimitMB:  64,
				CPUQuotaMs:     500,
				NetworkEnabled: false,
			},
			Endpoints: []Endpoint{},
		}).
		SetNetwork(Network{
			P2P: P2PConfig{
				PeerID:          "12D3KooWTest",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/ainur/gossipsub/1.0.0"},
			},
			Discovery: Discovery{
				Methods: []string{"mdns"},
			},
			Availability: Availability{
				Regions:        []string{"local"},
				LatencyTargets: LatencyTargets{P50Ms: 10, P95Ms: 50, P99Ms: 100},
			},
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build card: %v", err)
	}

	// Test JSON serialization
	jsonData, err := card.ToJSON()
	if err != nil {
		t.Fatalf("Failed to serialize to JSON: %v", err)
	}

	// Test JSON deserialization
	parsedCard, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("Failed to deserialize from JSON: %v", err)
	}

	if parsedCard.CredentialSubject.ID != agentDID {
		t.Errorf("Deserialized DID mismatch: expected %s, got %s", agentDID, parsedCard.CredentialSubject.ID)
	}

	if parsedCard.CredentialSubject.Name != card.CredentialSubject.Name {
		t.Errorf("Deserialized name mismatch")
	}
}

func TestSignAndVerify(t *testing.T) {
	// Generate keypair
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate keypair: %v", err)
	}

	agentDID := NewAgentDID("signed-agent-001")

	card, err := NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Signed Agent").
		SetDescription("Agent with signature").
		SetCapabilities(Capabilities{
			Domains: []string{"testing"},
			Operations: []Operation{
				{Name: "sign_test", Category: "crypto", GasEstimate: 200},
			},
			Constraints: CapabilityConstraints{
				MaxInputSize:       1024,
				MaxExecutionTimeMs: 2000,
			},
			Interfaces: []string{"http"},
		}).
		SetRuntime(RuntimeInfo{
			Protocol:       "ainur-v1",
			Implementation: "test",
			Version:        "0.1.0",
			WasmEngine:     "wasmtime",
			WasmVersion:    "23.0.0",
			ModuleHash:     "sha256:signing",
			ExecutionEnvironment: ExecutionEnvironment{
				MemoryLimitMB:  64,
				CPUQuotaMs:     500,
				NetworkEnabled: false,
			},
			Endpoints: []Endpoint{},
		}).
		SetNetwork(Network{
			P2P: P2PConfig{
				PeerID:          "12D3KooWTestSigning",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/ainur/gossipsub/1.0.0"},
			},
			Discovery: Discovery{
				Methods: []string{"mdns"},
			},
			Availability: Availability{
				Regions:        []string{"local"},
				LatencyTargets: LatencyTargets{P50Ms: 10, P95Ms: 50, P99Ms: 100},
			},
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build card: %v", err)
	}

	// Sign the card
	if err := card.Sign(privateKey); err != nil {
		t.Fatalf("Failed to sign card: %v", err)
	}

	// Verify proof exists
	if card.Proof == nil {
		t.Fatal("Expected proof after signing")
	}

	if card.Proof.Type != "Ed25519Signature2020" {
		t.Errorf("Expected proof type Ed25519Signature2020, got %s", card.Proof.Type)
	}

	// Verify signature
	valid, err := card.Verify(publicKey)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}

	if !valid {
		t.Error("Signature verification failed")
	}

	// Test with wrong key
	wrongKey, _, _ := GenerateKeyPair()
	valid, err = card.Verify(wrongKey)
	if err != nil {
		t.Fatalf("Verification error with wrong key: %v", err)
	}

	if valid {
		t.Error("Signature should not verify with wrong key")
	}
}

func TestHash(t *testing.T) {
	agentDID := NewAgentDID("hash-test-001")

	card, err := NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Hash Test Agent").
		SetDescription("Agent for hash testing").
		SetCapabilities(Capabilities{
			Domains: []string{"testing"},
			Operations: []Operation{
				{Name: "hash_test", Category: "crypto", GasEstimate: 100},
			},
			Constraints: CapabilityConstraints{
				MaxInputSize:       512,
				MaxExecutionTimeMs: 1000,
			},
			Interfaces: []string{"http"},
		}).
		SetRuntime(RuntimeInfo{
			Protocol:       "ainur-v1",
			Implementation: "test",
			Version:        "0.1.0",
			WasmEngine:     "wasmtime",
			WasmVersion:    "23.0.0",
			ModuleHash:     "sha256:hash",
			ExecutionEnvironment: ExecutionEnvironment{
				MemoryLimitMB:  64,
				CPUQuotaMs:     500,
				NetworkEnabled: false,
			},
			Endpoints: []Endpoint{},
		}).
		SetNetwork(Network{
			P2P: P2PConfig{
				PeerID:          "12D3KooWHashTest",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/ainur/gossipsub/1.0.0"},
			},
			Discovery: Discovery{
				Methods: []string{"mdns"},
			},
			Availability: Availability{
				Regions:        []string{"local"},
				LatencyTargets: LatencyTargets{P50Ms: 10, P95Ms: 50, P99Ms: 100},
			},
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build card: %v", err)
	}

	hash1, err := card.Hash()
	if err != nil {
		t.Fatalf("Failed to hash card: %v", err)
	}

	if hash1 == "" {
		t.Error("Hash should not be empty")
	}

	if len(hash1) < 70 { // sha256: + 64 hex chars
		t.Errorf("Hash seems too short: %s", hash1)
	}

	// Hash again to verify determinism
	hash2, err := card.Hash()
	if err != nil {
		t.Fatalf("Failed to hash card second time: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("Hash is not deterministic: %s != %s", hash1, hash2)
	}
}

func TestDIDFormats(t *testing.T) {
	tests := []struct {
		name     string
		did      DID
		expected string
	}{
		{"Agent DID", NewAgentDID("test-001"), "did:ainur:agent:test-001"},
		{"User DID", NewUserDID("alice"), "did:ainur:user:alice"},
		{"Network DID", NewNetworkDID("mainnet"), "did:ainur:network:mainnet"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.did.String() != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.did.String())
			}
		})
	}
}

func TestReputationDefaults(t *testing.T) {
	rep := NewReputation()

	if rep.TrustScore != 50.0 {
		t.Errorf("Expected default trust score 50.0, got %f", rep.TrustScore)
	}

	if rep.TotalTasks != 0 {
		t.Errorf("Expected 0 total tasks for new agent, got %d", rep.TotalTasks)
	}

	if rep.UptimePercentage != 100.0 {
		t.Errorf("Expected 100%% uptime for new agent, got %f", rep.UptimePercentage)
	}

	if time.Since(rep.CreatedAt) > time.Second {
		t.Error("CreatedAt should be recent")
	}
}

func TestMarshalUnmarshalRoundTrip(t *testing.T) {
	agentDID := NewAgentDID("roundtrip-001")
	now := time.Now().Truncate(time.Second)

	originalCard, err := NewAgentCardBuilder().
		SetAgentDID(agentDID).
		SetName("Round Trip Test").
		SetDescription("Testing serialization round trip").
		SetCapabilities(Capabilities{
			Domains: []string{"testing"},
			Operations: []Operation{
				{Name: "roundtrip", Category: "test", GasEstimate: 50},
			},
			Constraints: CapabilityConstraints{
				MaxInputSize:       256,
				MaxExecutionTimeMs: 500,
			},
			Interfaces: []string{"http"},
		}).
		SetRuntime(RuntimeInfo{
			Protocol:       "ainur-v1",
			Implementation: "test",
			Version:        "0.1.0",
			WasmEngine:     "wasmtime",
			WasmVersion:    "23.0.0",
			ModuleHash:     "sha256:roundtrip",
			ExecutionEnvironment: ExecutionEnvironment{
				MemoryLimitMB:  32,
				CPUQuotaMs:     250,
				NetworkEnabled: false,
			},
			Endpoints: []Endpoint{},
		}).
		SetReputation(Reputation{
			TrustScore:       75.5,
			TotalTasks:       100,
			SuccessfulTasks:  95,
			FailedTasks:      5,
			SuccessRate:      0.95,
			UptimePercentage: 99.5,
			CreatedAt:        now,
			LastActive:       now,
			Badges:           []Badge{},
			SlashingHistory:  []map[string]interface{}{},
		}).
		SetNetwork(Network{
			P2P: P2PConfig{
				PeerID:          "12D3KooWRoundTrip",
				ListenAddresses: []string{"/ip4/127.0.0.1/tcp/4001"},
				Protocols:       []string{"/ainur/gossipsub/1.0.0"},
			},
			Discovery: Discovery{
				Methods: []string{"mdns"},
			},
			Availability: Availability{
				Regions:        []string{"local"},
				LatencyTargets: LatencyTargets{P50Ms: 5, P95Ms: 25, P99Ms: 50},
			},
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build original card: %v", err)
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(originalCard)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal back
	var restoredCard AgentCard
	if err := json.Unmarshal(jsonBytes, &restoredCard); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify key fields match
	if restoredCard.CredentialSubject.ID != originalCard.CredentialSubject.ID {
		t.Errorf("DID mismatch after round trip")
	}

	if restoredCard.CredentialSubject.Reputation.TrustScore != 75.5 {
		t.Errorf("Trust score mismatch: expected 75.5, got %f", restoredCard.CredentialSubject.Reputation.TrustScore)
	}

	if restoredCard.CredentialSubject.Reputation.TotalTasks != 100 {
		t.Errorf("Total tasks mismatch: expected 100, got %d", restoredCard.CredentialSubject.Reputation.TotalTasks)
	}
}
