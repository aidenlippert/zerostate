// Package substrate - Registry Client Operations
// Wraps pallet-registry extrinsics (index 9) for agent capability registry
package substrate

import (
	"context"
	"fmt"
	"math/big"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// RegistryClient handles interactions with pallet-registry (index 9)
type RegistryClient struct {
	client   *ClientV2
	keyring  *signature.KeyringPair
	palletID uint8
}

// NewRegistryClient creates a new Registry client
func NewRegistryClient(client *ClientV2, keyring *signature.KeyringPair) *RegistryClient {
	return &RegistryClient{
		client:   client,
		keyring:  keyring,
		palletID: 9, // Pallet-Registry index
	}
}

// AgentRegistration contains agent registration data
type AgentRegistration struct {
	DID          string   `json:"did"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	WASMHash     []byte   `json:"wasm_hash"` // 32 bytes
	PricePerTask uint64   `json:"price_per_task"`
}

// RegisterAgent registers a new agent on-chain
//
// Extrinsic: palletRegistry.registerAgent(did, name, capabilities, wasmHash, pricePerTask)
//
// Example:
//
//	reg := &AgentRegistration{
//	    DID: "did:ainur:math-agent",
//	    Name: "Math Calculator",
//	    Capabilities: []string{"math", "arithmetic"},
//	    WASMHash: wasmHash, // 32-byte hash
//	    PricePerTask: 1000,
//	}
//	err := registryClient.RegisterAgent(ctx, reg)
func (rc *RegistryClient) RegisterAgent(ctx context.Context, agent *AgentRegistration) error {
	if len(agent.WASMHash) != 32 {
		return fmt.Errorf("WASM hash must be exactly 32 bytes, got %d", len(agent.WASMHash))
	}

	meta := rc.client.GetMetadata()

	// Convert capabilities to bounded vectors
	capabilities := make([]types.Bytes, len(agent.Capabilities))
	for i, cap := range agent.Capabilities {
		capabilities[i] = types.NewBytes([]byte(cap))
	}

	// Create fixed-size array for WASM hash
	var wasmHashArray [32]byte
	copy(wasmHashArray[:], agent.WASMHash)

	call, err := types.NewCall(
		meta,
		"Registry.register_agent",
		types.NewBytes([]byte(agent.DID)),
		types.NewBytes([]byte(agent.Name)),
		capabilities,
		wasmHashArray,
		types.NewU128(*new(big.Int).SetUint64(agent.PricePerTask)),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := rc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to register agent: %w", err)
	}

	fmt.Printf("Agent registered in block: %s\n", hash.Hex())
	return nil
}

// UpdateAgent updates an existing agent's information
//
// Extrinsic: palletRegistry.updateAgent(did, name, capabilities, wasmHash, pricePerTask)
func (rc *RegistryClient) UpdateAgent(ctx context.Context, agent *AgentRegistration) error {
	if len(agent.WASMHash) != 32 {
		return fmt.Errorf("WASM hash must be exactly 32 bytes, got %d", len(agent.WASMHash))
	}

	meta := rc.client.GetMetadata()

	capabilities := make([]types.Bytes, len(agent.Capabilities))
	for i, cap := range agent.Capabilities {
		capabilities[i] = types.NewBytes([]byte(cap))
	}

	var wasmHashArray [32]byte
	copy(wasmHashArray[:], agent.WASMHash)

	call, err := types.NewCall(
		meta,
		"Registry.update_agent",
		types.NewBytes([]byte(agent.DID)),
		types.NewBytes([]byte(agent.Name)),
		capabilities,
		wasmHashArray,
		types.NewU128(*new(big.Int).SetUint64(agent.PricePerTask)),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := rc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	fmt.Printf("Agent updated in block: %s\n", hash.Hex())
	return nil
}

// DeregisterAgent deactivates an agent
//
// Extrinsic: palletRegistry.deregisterAgent(did)
func (rc *RegistryClient) DeregisterAgent(ctx context.Context, did string) error {
	meta := rc.client.GetMetadata()

	call, err := types.NewCall(
		meta,
		"Registry.deregister_agent",
		types.NewBytes([]byte(did)),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := rc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to deregister agent: %w", err)
	}

	fmt.Printf("Agent deregistered in block: %s\n", hash.Hex())
	return nil
}

// GetAgentCard queries an agent's card from storage
//
// Storage query: palletRegistry.agentCards(did)
func (rc *RegistryClient) GetAgentCard(ctx context.Context, did string) (*AgentCard, error) {
	meta := rc.client.GetMetadata()

	key, err := types.CreateStorageKey(meta, "Registry", "AgentCards", types.NewBytes([]byte(did)))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var card AgentCardRaw
	ok, err := rc.client.api.RPC.State.GetStorageLatest(key, &card)
	if err != nil {
		return nil, fmt.Errorf("failed to query agent card: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", did)
	}

	// Convert capabilities from bytes to strings
	capabilities := make([]string, len(card.Capabilities))
	for i, cap := range card.Capabilities {
		capabilities[i] = string(cap)
	}

	return &AgentCard{
		DID:          DID(card.DID),
		Name:         string(card.Name),
		Capabilities: capabilities,
		WASMHash:     card.WASMHash,
		PricePerTask: Balance(card.PricePerTask.String()),
		RegisteredAt: BlockNumber(card.RegisteredAt),
		UpdatedAt:    BlockNumber(card.LastUpdated),
		Active:       card.Active,
	}, nil
}

// FindAgentsByCapability finds all agents with a specific capability
//
// Storage query: palletRegistry.capabilityIndex(capability)
func (rc *RegistryClient) FindAgentsByCapability(ctx context.Context, capability string) ([]string, error) {
	meta := rc.client.GetMetadata()

	key, err := types.CreateStorageKey(meta, "Registry", "CapabilityIndex", types.NewBytes([]byte(capability)))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	var dids []types.Bytes
	ok, err := rc.client.api.RPC.State.GetStorageLatest(key, &dids)
	if err != nil {
		return nil, fmt.Errorf("failed to query capability index: %w", err)
	}
	if !ok {
		return []string{}, nil // No agents with this capability
	}

	// Convert from []types.Bytes to []string
	result := make([]string, len(dids))
	for i, did := range dids {
		result[i] = string(did)
	}

	return result, nil
}

// IsAgentActive checks if an agent is active
func (rc *RegistryClient) IsAgentActive(ctx context.Context, did string) (bool, error) {
	card, err := rc.GetAgentCard(ctx, did)
	if err != nil {
		if err.Error() == fmt.Sprintf("agent not found: %s", did) {
			return false, nil
		}
		return false, err
	}
	return card.Active, nil
}

// ListAllAgents returns all registered agents (for admin/debugging)
// Note: This is a utility function, not a pallet extrinsic
func (rc *RegistryClient) ListAllAgents(ctx context.Context) ([]*AgentCard, error) {
	// In production, you'd use pagination or a separate index
	// For now, this is a placeholder that would need to iterate storage
	return nil, fmt.Errorf("not implemented: use capability-based search instead")
}

// AgentCardRaw is the raw storage format
type AgentCardRaw struct {
	DID                 types.Bytes
	Name                types.Bytes
	Capabilities        []types.Bytes
	WASMHash            [32]byte
	PricePerTask        types.U128
	TotalTasksCompleted types.U32
	Active              bool
	RegisteredAt        types.BlockNumber
	LastUpdated         types.BlockNumber
}

// submitTransaction submits a signed transaction to the blockchain
func (rc *RegistryClient) submitTransaction(ctx context.Context, call types.Call) (types.Hash, error) {
	// Get runtime version
	rv, err := rc.client.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get runtime version: %w", err)
	}

	// Get nonce
	key, err := types.CreateStorageKey(rc.client.metadata, "System", "Account", rc.keyring.PublicKey)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to create storage key: %w", err)
	}

	var accountInfo types.AccountInfo
	ok, err := rc.client.api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get account info: %w", err)
	}

	nonce := types.NewUCompactFromUInt(0)
	if ok {
		nonce = types.NewUCompactFromUInt(uint64(accountInfo.Nonce))
	}

	// Create extrinsic
	ext := types.NewExtrinsic(call)

	// Get genesis hash
	genesisHash := rc.client.GetGenesisHash()

	// Get latest block hash
	blockHash, err := rc.client.api.RPC.Chain.GetBlockHashLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get block hash: %w", err)
	}

	// Sign options
	o := types.SignatureOptions{
		BlockHash:          blockHash,
		Era:                types.ExtrinsicEra{IsMortalEra: false},
		GenesisHash:        genesisHash,
		Nonce:              nonce,
		SpecVersion:        rv.SpecVersion,
		Tip:                types.NewUCompactFromUInt(0),
		TransactionVersion: rv.TransactionVersion,
	}

	// Sign the extrinsic
	err = ext.Sign(*rc.keyring, o)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign extrinsic: %w", err)
	}

	// Submit the extrinsic
	hash, err := rc.client.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to submit extrinsic: %w", err)
	}

	return hash, nil
}
