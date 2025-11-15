// Package substrate provides a Go client for interacting with the Ainur Substrate blockchain.
//
// This package enables the Orchestrator to:
// - Query on-chain state (escrows, DIDs, agent registry)
// - Submit transactions (create escrow, release payment)
// - Listen for blockchain events
//
// Architecture:
//
//	Orchestrator (Go) → substrate.Client → Substrate RPC → L1 Blockchain
package substrate

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a connection to the Substrate blockchain
type Client struct {
	endpoint string
	conn     *websocket.Conn
	timeout  time.Duration
}

// NewClient creates a new Substrate RPC client
//
// Example:
//
//	client, err := substrate.NewClient("ws://127.0.0.1:9944")
func NewClient(endpoint string) (*Client, error) {
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to substrate: %w", err)
	}

	return &Client{
		endpoint: endpoint,
		conn:     conn,
		timeout:  30 * time.Second,
	}, nil
}

// Close closes the WebSocket connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// ============================================================================
// RPC Request/Response Types
// ============================================================================

// RPCRequest represents a JSON-RPC 2.0 request
type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

// RPCResponse represents a JSON-RPC 2.0 response
type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      int             `json:"id"`
}

// RPCError represents a JSON-RPC error
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ============================================================================
// Substrate-Specific Types
// ============================================================================

// AccountID represents a Substrate account (32 bytes)
type AccountID [32]byte

// Balance represents AINU token balance (u128 in Rust)
type Balance string // Hex-encoded

// BlockNumber represents a block number
type BlockNumber uint32

// Hash represents a 32-byte hash
type Hash [32]byte

// DID represents a decentralized identifier
type DID string

// ============================================================================
// Escrow Types (matching pallet-escrow)
// ============================================================================

// EscrowState represents the state of an escrow
type EscrowState string

const (
	EscrowStatePending   EscrowState = "Pending"
	EscrowStateAccepted  EscrowState = "Accepted"
	EscrowStateCompleted EscrowState = "Completed"
	EscrowStateRefunded  EscrowState = "Refunded"
	EscrowStateDisputed  EscrowState = "Disputed"
)

// EscrowDetails represents on-chain escrow information
type EscrowDetails struct {
	TaskID       [32]byte    `json:"task_id"`
	User         AccountID   `json:"user"`
	AgentDID     *DID        `json:"agent_did"`
	AgentAccount *AccountID  `json:"agent_account"`
	Amount       Balance     `json:"amount"`
	FeePercent   uint8       `json:"fee_percent"`
	CreatedAt    BlockNumber `json:"created_at"`
	ExpiresAt    BlockNumber `json:"expires_at"`
	State        EscrowState `json:"state"`
	TaskHash     [32]byte    `json:"task_hash"`
}

// ============================================================================
// DID Types (matching pallet-did)
// ============================================================================

// DIDDocument represents on-chain DID information
type DIDDocument struct {
	Controller AccountID   `json:"controller"`
	PublicKey  [32]byte    `json:"public_key"`
	CreatedAt  BlockNumber `json:"created_at"`
	UpdatedAt  BlockNumber `json:"updated_at"`
	Active     bool        `json:"active"`
}

// ============================================================================
// Registry Types (matching pallet-registry)
// ============================================================================

// AgentCard represents on-chain agent registration
type AgentCard struct {
	DID          DID         `json:"did"`
	Name         string      `json:"name"`
	Capabilities []string    `json:"capabilities"`
	WASMHash     [32]byte    `json:"wasm_hash"`
	PricePerTask Balance     `json:"price_per_task"`
	RegisteredAt BlockNumber `json:"registered_at"`
	UpdatedAt    BlockNumber `json:"updated_at"`
	Active       bool        `json:"active"`
}

// ============================================================================
// Core RPC Methods
// ============================================================================

// Call executes a generic RPC call
func (c *Client) Call(ctx context.Context, method string, params ...interface{}) (json.RawMessage, error) {
	req := RPCRequest{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	var resp RPCResponse
	if err := c.conn.ReadJSON(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// GetBlockNumber returns the current block number
func (c *Client) GetBlockNumber(ctx context.Context) (BlockNumber, error) {
	result, err := c.Call(ctx, "chain_getHeader")
	if err != nil {
		return 0, err
	}

	var header struct {
		Number string `json:"number"`
	}
	if err := json.Unmarshal(result, &header); err != nil {
		return 0, err
	}

	// Parse hex string to uint32
	var blockNum uint64
	if _, err := fmt.Sscanf(header.Number, "0x%x", &blockNum); err != nil {
		return 0, err
	}

	return BlockNumber(blockNum), nil
}

// GetBlockHash returns the block hash at a given height
func (c *Client) GetBlockHash(ctx context.Context, blockNum BlockNumber) (Hash, error) {
	result, err := c.Call(ctx, "chain_getBlockHash", blockNum)
	if err != nil {
		return Hash{}, err
	}

	var hashStr string
	if err := json.Unmarshal(result, &hashStr); err != nil {
		return Hash{}, err
	}

	hashBytes, err := hex.DecodeString(hashStr[2:]) // Remove "0x" prefix
	if err != nil {
		return Hash{}, err
	}

	var hash Hash
	copy(hash[:], hashBytes)
	return hash, nil
}

// ============================================================================
// Escrow Query Methods
// ============================================================================

// GetEscrow queries the on-chain escrow for a task
//
// Example:
//
//	escrow, err := client.GetEscrow(ctx, taskID)
//	if err != nil {
//	    return err
//	}
//	fmt.Printf("Escrow state: %s, Amount: %s\n", escrow.State, escrow.Amount)
func (c *Client) GetEscrow(ctx context.Context, taskID [32]byte) (*EscrowDetails, error) {
	taskIDHex := "0x" + hex.EncodeToString(taskID[:])

	result, err := c.Call(ctx, "state_getStorage",
		"0x"+palletStorageKey("Escrow", "Escrows", taskIDHex))
	if err != nil {
		return nil, err
	}

	// If result is null, escrow doesn't exist
	if string(result) == "null" {
		return nil, fmt.Errorf("escrow not found for task %s", taskIDHex)
	}

	// Decode SCALE-encoded escrow details
	var escrow EscrowDetails
	if err := decodeScaleResult(result, &escrow); err != nil {
		return nil, err
	}

	return &escrow, nil
}

// GetUserEscrows returns all escrows created by a user
func (c *Client) GetUserEscrows(ctx context.Context, userAccount AccountID) ([]EscrowDetails, error) {
	accountHex := "0x" + hex.EncodeToString(userAccount[:])

	result, err := c.Call(ctx, "state_getStorage",
		"0x"+palletStorageKey("Escrow", "UserEscrows", accountHex))
	if err != nil {
		return nil, err
	}

	// Decode list of task IDs
	var taskIDs [][32]byte
	if err := decodeScaleResult(result, &taskIDs); err != nil {
		return nil, err
	}

	// Fetch each escrow
	escrows := make([]EscrowDetails, 0, len(taskIDs))
	for _, taskID := range taskIDs {
		escrow, err := c.GetEscrow(ctx, taskID)
		if err != nil {
			continue // Skip missing escrows
		}
		escrows = append(escrows, *escrow)
	}

	return escrows, nil
}

// ============================================================================
// DID Query Methods
// ============================================================================

// GetDID queries the on-chain DID document
func (c *Client) GetDID(ctx context.Context, did DID) (*DIDDocument, error) {
	didHex := "0x" + hex.EncodeToString([]byte(did))

	result, err := c.Call(ctx, "state_getStorage",
		"0x"+palletStorageKey("Did", "DidDocuments", didHex))
	if err != nil {
		return nil, err
	}

	if string(result) == "null" {
		return nil, fmt.Errorf("DID not found: %s", did)
	}

	var doc DIDDocument
	if err := decodeScaleResult(result, &doc); err != nil {
		return nil, err
	}

	return &doc, nil
}

// IsDIDActive checks if a DID is active
func (c *Client) IsDIDActive(ctx context.Context, did DID) (bool, error) {
	doc, err := c.GetDID(ctx, did)
	if err != nil {
		return false, err
	}
	return doc.Active, nil
}

// ============================================================================
// Registry Query Methods
// ============================================================================

// GetAgentCard queries the on-chain agent registry
func (c *Client) GetAgentCard(ctx context.Context, did DID) (*AgentCard, error) {
	didHex := "0x" + hex.EncodeToString([]byte(did))

	result, err := c.Call(ctx, "state_getStorage",
		"0x"+palletStorageKey("Registry", "AgentCards", didHex))
	if err != nil {
		return nil, err
	}

	if string(result) == "null" {
		return nil, fmt.Errorf("agent not found: %s", did)
	}

	var card AgentCard
	if err := decodeScaleResult(result, &card); err != nil {
		return nil, err
	}

	return &card, nil
}

// FindAgentsByCapability queries agents with a specific capability
func (c *Client) FindAgentsByCapability(ctx context.Context, capability string) ([]DID, error) {
	capHex := "0x" + hex.EncodeToString([]byte(capability))

	result, err := c.Call(ctx, "state_getStorage",
		"0x"+palletStorageKey("Registry", "CapabilityIndex", capHex))
	if err != nil {
		return nil, err
	}

	if string(result) == "null" {
		return []DID{}, nil
	}

	// Decode list of DIDs
	var dids []DID
	if err := decodeScaleResult(result, &dids); err != nil {
		return nil, err
	}

	return dids, nil
}

// ============================================================================
// Transaction Submission Methods
// ============================================================================

// CreateEscrowParams represents parameters for creating an escrow
type CreateEscrowParams struct {
	TaskID        [32]byte
	Amount        Balance
	TaskHash      [32]byte
	TimeoutBlocks *BlockNumber
}

// CreateEscrow submits a transaction to create an escrow
//
// This will be implemented when we have transaction signing
func (c *Client) CreateEscrow(ctx context.Context, params CreateEscrowParams) (Hash, error) {
	// TODO: Implement extrinsic submission with signature
	return Hash{}, fmt.Errorf("not implemented: requires transaction signing")
}

// ReleasePayment submits a transaction to release payment
func (c *Client) ReleasePayment(ctx context.Context, taskID [32]byte) (Hash, error) {
	// TODO: Implement extrinsic submission
	return Hash{}, fmt.Errorf("not implemented: requires transaction signing")
}

// RefundEscrow submits a transaction to refund an escrow
func (c *Client) RefundEscrow(ctx context.Context, taskID [32]byte) (Hash, error) {
	// TODO: Implement extrinsic submission
	return Hash{}, fmt.Errorf("not implemented: requires transaction signing")
}

// ============================================================================
// Event Listening
// ============================================================================

// Event represents a blockchain event
type Event struct {
	Phase     string          `json:"phase"`
	Event     json.RawMessage `json:"event"`
	Topics    []string        `json:"topics"`
	BlockNum  BlockNumber     `json:"-"`
	BlockHash Hash            `json:"-"`
}

// EscrowCreatedEvent represents the EscrowCreated event
type EscrowCreatedEvent struct {
	TaskID [32]byte  `json:"task_id"`
	User   AccountID `json:"user"`
	Amount Balance   `json:"amount"`
}

// PaymentReleasedEvent represents the PaymentReleased event
type PaymentReleasedEvent struct {
	TaskID [32]byte  `json:"task_id"`
	Agent  AccountID `json:"agent"`
	Amount Balance   `json:"amount"`
	Fee    Balance   `json:"fee"`
}

// SubscribeEvents subscribes to blockchain events
//
// Example:
//
//	events := make(chan Event)
//	err := client.SubscribeEvents(ctx, events)
//	for event := range events {
//	    // Handle event
//	}
func (c *Client) SubscribeEvents(ctx context.Context, events chan<- Event) error {
	// TODO: Implement event subscription via state_subscribeStorage
	return fmt.Errorf("not implemented: event subscription")
}

// ============================================================================
// Utility Functions
// ============================================================================

// palletStorageKey generates a storage key for a pallet
func palletStorageKey(pallet, storage, key string) string {
	// Substrate uses twox128(pallet) + twox128(storage) + twox128(key)
	// For now, return placeholder - proper implementation needs crypto
	return "placeholder_" + pallet + "_" + storage + "_" + key
}

// decodeScaleResult decodes a SCALE-encoded result
func decodeScaleResult(result json.RawMessage, v interface{}) error {
	// TODO: Implement proper SCALE codec decoding
	// For now, just unmarshal JSON (assumes RPC returns decoded data)
	return json.Unmarshal(result, v)
}

// ============================================================================
// Helper Functions for Orchestrator Integration
// ============================================================================

// TaskIDFromUUID converts a UUID to a 32-byte task ID
func TaskIDFromUUID(uuid string) ([32]byte, error) {
	// Remove hyphens from UUID
	uuidClean := ""
	for _, c := range uuid {
		if c != '-' {
			uuidClean += string(c)
		}
	}

	// Decode hex
	bytes, err := hex.DecodeString(uuidClean)
	if err != nil {
		return [32]byte{}, err
	}

	// Pad to 32 bytes if needed
	var taskID [32]byte
	copy(taskID[:], bytes)
	return taskID, nil
}

// BalanceFromAINU converts AINU tokens to blockchain balance
// AINU has 12 decimals: 1 AINU = 1_000_000_000_000 base units
func BalanceFromAINU(ainu float64) Balance {
	baseUnits := uint64(ainu * 1e12)
	return Balance(fmt.Sprintf("0x%x", baseUnits))
}

// BalanceToAINU converts blockchain balance to AINU tokens
func BalanceToAINU(balance Balance) (float64, error) {
	var baseUnits uint64
	if _, err := fmt.Sscanf(string(balance), "0x%x", &baseUnits); err != nil {
		return 0, err
	}
	return float64(baseUnits) / 1e12, nil
}
