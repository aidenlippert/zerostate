package agentcard

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DID represents a Decentralized Identifier
type DID string

// NewAgentDID creates a new agent DID with format: did:ainur:agent:<identifier>
func NewAgentDID(identifier string) DID {
	return DID(fmt.Sprintf("did:ainur:agent:%s", identifier))
}

// NewUserDID creates a new user DID with format: did:ainur:user:<identifier>
func NewUserDID(identifier string) DID {
	return DID(fmt.Sprintf("did:ainur:user:%s", identifier))
}

// NewNetworkDID creates a new network DID
func NewNetworkDID(identifier string) DID {
	return DID(fmt.Sprintf("did:ainur:network:%s", identifier))
}

// String returns the DID as a string
func (d DID) String() string {
	return string(d)
}

// Operation defines a capability operation
type Operation struct {
	Name         string                 `json:"name"`
	Category     string                 `json:"category"`
	InputSchema  map[string]interface{} `json:"input_schema,omitempty"`
	OutputSchema map[string]interface{} `json:"output_schema,omitempty"`
	Complexity   string                 `json:"complexity,omitempty"`
	GasEstimate  uint64                 `json:"gas_estimate"`
}

// CapabilityConstraints defines resource limits
type CapabilityConstraints struct {
	MaxInputSize       uint64  `json:"max_input_size"`
	MaxExecutionTimeMs uint64  `json:"max_execution_time_ms"`
	ConcurrentTasks    *uint32 `json:"concurrent_tasks,omitempty"`
}

// Capabilities declares what an agent can do
type Capabilities struct {
	Domains     []string              `json:"domains"`
	Operations  []Operation           `json:"operations"`
	Constraints CapabilityConstraints `json:"constraints"`
	Interfaces  []string              `json:"interfaces"`
}

// ExecutionEnvironment defines runtime constraints
type ExecutionEnvironment struct {
	MemoryLimitMB     uint32 `json:"memory_limit_mb"`
	CPUQuotaMs        uint32 `json:"cpu_quota_ms"`
	NetworkEnabled    bool   `json:"network_enabled"`
	FilesystemEnabled bool   `json:"filesystem_enabled"`
}

// Endpoint defines a network endpoint
type Endpoint struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	TLS      *bool  `json:"tls,omitempty"`
}

// RuntimeInfo describes the agent's runtime environment
type RuntimeInfo struct {
	Protocol             string               `json:"protocol"`
	Implementation       string               `json:"implementation"`
	Version              string               `json:"version"`
	WasmEngine           string               `json:"wasm_engine"`
	WasmVersion          string               `json:"wasm_version"`
	ModuleHash           string               `json:"module_hash"`
	ModuleURL            string               `json:"module_url,omitempty"`
	ExecutionEnvironment ExecutionEnvironment `json:"execution_environment"`
	Endpoints            []Endpoint           `json:"endpoints"`
}

// Badge represents a reputation badge
type Badge struct {
	Type      string    `json:"type"`
	Threshold string    `json:"threshold,omitempty"`
	IssuedBy  DID       `json:"issued_by"`
	IssuedAt  time.Time `json:"issued_at"`
}

// Reputation tracks agent trustworthiness
type Reputation struct {
	TrustScore             float64                  `json:"trust_score"`
	TotalTasks             uint64                   `json:"total_tasks"`
	SuccessfulTasks        uint64                   `json:"successful_tasks"`
	FailedTasks            uint64                   `json:"failed_tasks"`
	SuccessRate            float64                  `json:"success_rate"`
	AverageExecutionTimeMs uint64                   `json:"average_execution_time_ms"`
	UptimePercentage       float64                  `json:"uptime_percentage"`
	PeerEndorsements       uint32                   `json:"peer_endorsements"`
	Violations             uint32                   `json:"violations"`
	CreatedAt              time.Time                `json:"created_at"`
	LastActive             time.Time                `json:"last_active"`
	Badges                 []Badge                  `json:"badges"`
	SlashingHistory        []map[string]interface{} `json:"slashing_history"`
}

// NewReputation creates a default reputation for a new agent
func NewReputation() Reputation {
	now := time.Now()
	return Reputation{
		TrustScore:       50.0,
		TotalTasks:       0,
		SuccessfulTasks:  0,
		FailedTasks:      0,
		SuccessRate:      0.0,
		UptimePercentage: 100.0,
		CreatedAt:        now,
		LastActive:       now,
		Badges:           []Badge{},
		SlashingHistory:  []map[string]interface{}{},
	}
}

// Discount defines a pricing discount
type Discount struct {
	Type               string   `json:"type"`
	MinTasks           *uint64  `json:"min_tasks,omitempty"`
	MinTrustScore      *float64 `json:"min_trust_score,omitempty"`
	DiscountPercentage float64  `json:"discount_percentage"`
}

// SurgePricing defines dynamic pricing
type SurgePricing struct {
	Enabled         bool    `json:"enabled"`
	MultiplierMax   float64 `json:"multiplier_max"`
	DemandThreshold float64 `json:"demand_threshold"`
}

// Economic defines pricing and payment
type Economic struct {
	PricingModel    string        `json:"pricing_model"`
	BasePriceUAinur uint64        `json:"base_price_uainur"`
	SurgePricing    *SurgePricing `json:"surge_pricing,omitempty"`
	Discounts       []Discount    `json:"discounts"`
	PaymentMethods  []string      `json:"payment_methods"`
	EscrowRequired  bool          `json:"escrow_required"`
	RefundPolicy    string        `json:"refund_policy"`
}

// NewEconomic creates default economic parameters
func NewEconomic() Economic {
	return Economic{
		PricingModel:    "per_operation",
		BasePriceUAinur: 100,
		Discounts:       []Discount{},
		PaymentMethods:  []string{"ainur"},
		EscrowRequired:  false,
		RefundPolicy:    "full_refund_on_failure",
	}
}

// P2PConfig defines P2P network settings
type P2PConfig struct {
	PeerID            string   `json:"peer_id"`
	ListenAddresses   []string `json:"listen_addresses"`
	AnnounceAddresses []string `json:"announce_addresses"`
	Protocols         []string `json:"protocols"`
}

// Discovery defines discovery methods
type Discovery struct {
	Methods        []string `json:"methods"`
	BootstrapNodes []string `json:"bootstrap_nodes"`
}

// LatencyTargets defines expected latency
type LatencyTargets struct {
	P50Ms uint64 `json:"p50_ms"`
	P95Ms uint64 `json:"p95_ms"`
	P99Ms uint64 `json:"p99_ms"`
}

// Availability defines where the agent is available
type Availability struct {
	Regions        []string       `json:"regions"`
	LatencyTargets LatencyTargets `json:"latency_targets"`
}

// Network defines networking configuration
type Network struct {
	P2P          P2PConfig    `json:"p2p"`
	Discovery    Discovery    `json:"discovery"`
	Availability Availability `json:"availability"`
}

// CredentialSubject is the core agent data
type CredentialSubject struct {
	ID           DID          `json:"id"`
	Type         string       `json:"type"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Version      string       `json:"version"`
	Capabilities Capabilities `json:"capabilities"`
	Runtime      RuntimeInfo  `json:"runtime"`
	Reputation   Reputation   `json:"reputation"`
	Economic     Economic     `json:"economic"`
	Network      Network      `json:"network"`
}

// Proof is the cryptographic signature
type Proof struct {
	Type               string    `json:"type"`
	Created            time.Time `json:"created"`
	VerificationMethod string    `json:"verification_method"`
	ProofPurpose       string    `json:"proof_purpose"`
	ProofValue         string    `json:"proof_value"`
}

// AgentCard is a W3C Verifiable Credential for agents
type AgentCard struct {
	Context           []string          `json:"@context"`
	ID                string            `json:"id"`
	Type              []string          `json:"type"`
	Issuer            DID               `json:"issuer"`
	IssuanceDate      time.Time         `json:"issuanceDate"`
	ExpirationDate    time.Time         `json:"expirationDate"`
	CredentialSubject CredentialSubject `json:"credentialSubject"`
	Proof             *Proof            `json:"proof,omitempty"`
}

// AgentCardBuilder helps construct AgentCards
type AgentCardBuilder struct {
	agentDID       *DID
	name           string
	description    string
	version        string
	capabilities   *Capabilities
	runtime        *RuntimeInfo
	reputation     *Reputation
	economic       *Economic
	network        *Network
	issuer         *DID
	expirationDays uint64
}

// NewAgentCardBuilder creates a new builder
func NewAgentCardBuilder() *AgentCardBuilder {
	return &AgentCardBuilder{
		expirationDays: 365,
	}
}

// SetAgentDID sets the agent's DID
func (b *AgentCardBuilder) SetAgentDID(did DID) *AgentCardBuilder {
	b.agentDID = &did
	return b
}

// SetName sets the agent's name
func (b *AgentCardBuilder) SetName(name string) *AgentCardBuilder {
	b.name = name
	return b
}

// SetDescription sets the agent's description
func (b *AgentCardBuilder) SetDescription(desc string) *AgentCardBuilder {
	b.description = desc
	return b
}

// SetVersion sets the agent's version
func (b *AgentCardBuilder) SetVersion(version string) *AgentCardBuilder {
	b.version = version
	return b
}

// SetCapabilities sets the agent's capabilities
func (b *AgentCardBuilder) SetCapabilities(cap Capabilities) *AgentCardBuilder {
	b.capabilities = &cap
	return b
}

// SetRuntime sets the runtime information
func (b *AgentCardBuilder) SetRuntime(runtime RuntimeInfo) *AgentCardBuilder {
	b.runtime = &runtime
	return b
}

// SetReputation sets the reputation
func (b *AgentCardBuilder) SetReputation(rep Reputation) *AgentCardBuilder {
	b.reputation = &rep
	return b
}

// SetEconomic sets economic parameters
func (b *AgentCardBuilder) SetEconomic(econ Economic) *AgentCardBuilder {
	b.economic = &econ
	return b
}

// SetNetwork sets network configuration
func (b *AgentCardBuilder) SetNetwork(net Network) *AgentCardBuilder {
	b.network = &net
	return b
}

// SetIssuer sets the issuer DID
func (b *AgentCardBuilder) SetIssuer(issuer DID) *AgentCardBuilder {
	b.issuer = &issuer
	return b
}

// SetExpirationDays sets expiration in days
func (b *AgentCardBuilder) SetExpirationDays(days uint64) *AgentCardBuilder {
	b.expirationDays = days
	return b
}

// Build constructs the AgentCard
func (b *AgentCardBuilder) Build() (*AgentCard, error) {
	if b.agentDID == nil {
		return nil, fmt.Errorf("agent_did is required")
	}
	if b.name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if b.description == "" {
		return nil, fmt.Errorf("description is required")
	}
	if b.capabilities == nil {
		return nil, fmt.Errorf("capabilities are required")
	}
	if b.runtime == nil {
		return nil, fmt.Errorf("runtime is required")
	}
	if b.network == nil {
		return nil, fmt.Errorf("network is required")
	}

	version := b.version
	if version == "" {
		version = "1.0.0"
	}

	reputation := b.reputation
	if reputation == nil {
		rep := NewReputation()
		reputation = &rep
	}

	economic := b.economic
	if economic == nil {
		econ := NewEconomic()
		economic = &econ
	}

	issuer := b.issuer
	if issuer == nil {
		issuer = b.agentDID
	}

	now := time.Now()
	expiration := now.AddDate(0, 0, int(b.expirationDays))
	cardID := fmt.Sprintf("did:ainur:agentcard:%s", uuid.New().String())

	return &AgentCard{
		Context: []string{
			"https://www.w3.org/2018/credentials/v1",
			"https://ainur.network/contexts/agentcard/v1",
		},
		ID:             cardID,
		Type:           []string{"VerifiableCredential", "AgentCard"},
		Issuer:         *issuer,
		IssuanceDate:   now,
		ExpirationDate: expiration,
		CredentialSubject: CredentialSubject{
			ID:           *b.agentDID,
			Type:         "AutonomousAgent",
			Name:         b.name,
			Description:  b.description,
			Version:      version,
			Capabilities: *b.capabilities,
			Runtime:      *b.runtime,
			Reputation:   *reputation,
			Economic:     *economic,
			Network:      *b.network,
		},
		Proof: nil,
	}, nil
}

// ToJSON converts the AgentCard to JSON
func (ac *AgentCard) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ac, "", "  ")
}

// FromJSON parses an AgentCard from JSON
func FromJSON(data []byte) (*AgentCard, error) {
	var card AgentCard
	if err := json.Unmarshal(data, &card); err != nil {
		return nil, err
	}
	return &card, nil
}

// Sign signs the AgentCard with an Ed25519 private key
func (ac *AgentCard) Sign(privateKey ed25519.PrivateKey) error {
	// Remove any existing proof
	ac.Proof = nil

	// Create canonical JSON of credential subject
	subjectJSON, err := json.Marshal(ac.CredentialSubject)
	if err != nil {
		return fmt.Errorf("failed to marshal credential subject: %w", err)
	}

	// Sign the canonical JSON
	signature := ed25519.Sign(privateKey, subjectJSON)

	// Encode signature as base64
	proofValue := base64.StdEncoding.EncodeToString(signature)

	// Create verification method DID
	verificationMethod := fmt.Sprintf("%s#keys-1", ac.CredentialSubject.ID)

	// Add proof
	ac.Proof = &Proof{
		Type:               "Ed25519Signature2020",
		Created:            time.Now(),
		VerificationMethod: verificationMethod,
		ProofPurpose:       "assertionMethod",
		ProofValue:         proofValue,
	}

	return nil
}

// Verify verifies the AgentCard signature
func (ac *AgentCard) Verify(publicKey ed25519.PublicKey) (bool, error) {
	if ac.Proof == nil {
		return false, fmt.Errorf("agentcard has no proof")
	}

	// Decode signature from base64
	signature, err := base64.StdEncoding.DecodeString(ac.Proof.ProofValue)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// Recreate canonical JSON (without proof)
	proofBackup := ac.Proof
	ac.Proof = nil
	subjectJSON, err := json.Marshal(ac.CredentialSubject)
	ac.Proof = proofBackup
	if err != nil {
		return false, fmt.Errorf("failed to marshal credential subject: %w", err)
	}

	// Verify signature
	return ed25519.Verify(publicKey, subjectJSON, signature), nil
}

// Hash creates a SHA-256 hash of the AgentCard
func (ac *AgentCard) Hash() (string, error) {
	subjectJSON, err := json.Marshal(ac.CredentialSubject)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(subjectJSON)
	return fmt.Sprintf("sha256:%s", hex.EncodeToString(hash[:])), nil
}

// GenerateKeyPair generates a new Ed25519 keypair
func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}
