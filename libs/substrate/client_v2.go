// Package substrate - Chain V2 Client
// Modern Polkadot SDK solochain integration with custom pallets
package substrate

import (
	"context"
	"fmt"
	"time"

	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// ClientV2 represents a connection to the chain-v2 Substrate blockchain
// Uses modern Polkadot SDK solochain template with custom pallets:
// - Pallet-DID (Index 8): Decentralized identity management
// - Pallet-Registry (Index 9): Agent capability registry
// - Pallet-Escrow (Index 10): Trustless payment escrow
type ClientV2 struct {
	api      *gsrpc.SubstrateAPI
	metadata *types.Metadata
	genesis  types.Hash
	endpoint string
	timeout  time.Duration
}

// NewClientV2 creates a new chain-v2 Substrate RPC client
//
// Example:
//
//	client, err := substrate.NewClientV2("ws://127.0.0.1:41339")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
func NewClientV2(endpoint string) (*ClientV2, error) {
	// Connect to the node
	api, err := gsrpc.NewSubstrateAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to chain-v2: %w", err)
	}

	// Fetch metadata
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	// Get genesis hash
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		return nil, fmt.Errorf("failed to get genesis hash: %w", err)
	}

	return &ClientV2{
		api:      api,
		metadata: meta,
		genesis:  genesisHash,
		endpoint: endpoint,
		timeout:  30 * time.Second,
	}, nil
}

// Close closes the connection to the Substrate node
func (c *ClientV2) Close() {
	// The gsrpc library doesn't expose a Close method
	// Connections are managed internally
}

// GetMetadata returns the runtime metadata
func (c *ClientV2) GetMetadata() *types.Metadata {
	return c.metadata
}

// GetGenesisHash returns the genesis block hash
func (c *ClientV2) GetGenesisHash() types.Hash {
	return c.genesis
}

// GetLatestBlockNumber returns the current block number
func (c *ClientV2) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	header, err := c.api.RPC.Chain.GetHeaderLatest()
	if err != nil {
		return 0, fmt.Errorf("failed to get latest header: %w", err)
	}
	return uint64(header.Number), nil
}

// GetBlockHash returns the block hash for a given block number
func (c *ClientV2) GetBlockHash(ctx context.Context, blockNumber uint64) (types.Hash, error) {
	hash, err := c.api.RPC.Chain.GetBlockHash(blockNumber)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get block hash: %w", err)
	}
	return hash, nil
}

// GetChainInfo returns basic chain information
func (c *ClientV2) GetChainInfo(ctx context.Context) (*ChainInfo, error) {
	// Get chain name
	name, err := c.api.RPC.System.Chain()
	if err != nil {
		return nil, fmt.Errorf("failed to get chain name: %w", err)
	}

	// Get node version
	version, err := c.api.RPC.System.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get node version: %w", err)
	}

	// Get latest block
	blockNumber, err := c.GetLatestBlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block number: %w", err)
	}

	return &ChainInfo{
		Name:        string(name),
		Version:     string(version),
		GenesisHash: c.genesis.Hex(),
		BlockNumber: blockNumber,
		Endpoint:    c.endpoint,
	}, nil
}

// ChainInfo contains basic blockchain information
type ChainInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	GenesisHash string `json:"genesis_hash"`
	BlockNumber uint64 `json:"block_number"`
	Endpoint    string `json:"endpoint"`
}

// HealthCheck verifies the connection to the chain-v2 node
func (c *ClientV2) HealthCheck(ctx context.Context) error {
	_, err := c.api.RPC.Chain.GetHeaderLatest()
	if err != nil {
		return fmt.Errorf("chain-v2 health check failed: %w", err)
	}
	return nil
}

// WaitForBlock waits until the specified block number is reached
func (c *ClientV2) WaitForBlock(ctx context.Context, targetBlock uint64) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			current, err := c.GetLatestBlockNumber(ctx)
			if err != nil {
				return err
			}
			if current >= targetBlock {
				return nil
			}
		}
	}
}

// GetAccountInfo returns account information for the given address
func (c *ClientV2) GetAccountInfo(ctx context.Context, address string) (*AccountInfo, error) {
	// Parse SS58 address
	pubKey, err := types.NewMultiAddressFromHexAccountID(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address format: %w", err)
	}

	// Get account info from system.account storage
	var accountInfo types.AccountInfo
	key, err := types.CreateStorageKey(c.metadata, "System", "Account", pubKey.AsID[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	ok, err := c.api.RPC.State.GetStorageLatest(key, &accountInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to query account: %w", err)
	}
	if !ok {
		return &AccountInfo{
			Address: address,
			Balance: 0,
			Nonce:   0,
		}, nil
	}

	return &AccountInfo{
		Address: address,
		Balance: accountInfo.Data.Free.Int64(),
		Nonce:   uint64(accountInfo.Nonce),
	}, nil
}

// AccountInfo contains account information
type AccountInfo struct {
	Address string `json:"address"`
	Balance int64  `json:"balance"`
	Nonce   uint64 `json:"nonce"`
}
