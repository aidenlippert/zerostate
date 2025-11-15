// Package substrate - DID Client Operations
// Wraps pallet-did extrinsics (index 8) for decentralized identity management
package substrate

import (
	"context"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

// DIDClient handles interactions with pallet-did (index 8)
type DIDClient struct {
	client   *ClientV2
	keyring  *signature.KeyringPair
	palletID uint8
}

// NewDIDClient creates a new DID client
func NewDIDClient(client *ClientV2, keyring *signature.KeyringPair) *DIDClient {
	return &DIDClient{
		client:   client,
		keyring:  keyring,
		palletID: 8, // Pallet-DID index
	}
}

// CreateDID creates a new DID on-chain
//
// Extrinsic: palletDid.createDid(did, publicKey)
//
// Example:
//
//	err := didClient.CreateDID(ctx, "did:ainur:alice", alicePublicKey)
func (dc *DIDClient) CreateDID(ctx context.Context, did string, publicKey []byte) error {
	// Prepare call
	meta := dc.client.GetMetadata()

	call, err := types.NewCall(
		meta,
		"Did.create_did",
		types.NewBytes([]byte(did)),
		types.NewBytes(publicKey),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	// Submit transaction
	hash, err := dc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to create DID: %w", err)
	}

	fmt.Printf("DID created in block: %s\n", hash.Hex())
	return nil
}

// UpdateKey updates the public key for a DID
//
// Extrinsic: palletDid.updateKey(did, newPublicKey)
func (dc *DIDClient) UpdateKey(ctx context.Context, did string, newPublicKey []byte) error {
	meta := dc.client.GetMetadata()

	call, err := types.NewCall(
		meta,
		"Did.update_key",
		types.NewBytes([]byte(did)),
		types.NewBytes(newPublicKey),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := dc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to update key: %w", err)
	}

	fmt.Printf("DID key updated in block: %s\n", hash.Hex())
	return nil
}

// RevokeDID revokes (deactivates) a DID
//
// Extrinsic: palletDid.revokeDid(did)
func (dc *DIDClient) RevokeDID(ctx context.Context, did string) error {
	meta := dc.client.GetMetadata()

	call, err := types.NewCall(
		meta,
		"Did.revoke_did",
		types.NewBytes([]byte(did)),
	)
	if err != nil {
		return fmt.Errorf("failed to create call: %w", err)
	}

	hash, err := dc.submitTransaction(ctx, call)
	if err != nil {
		return fmt.Errorf("failed to revoke DID: %w", err)
	}

	fmt.Printf("DID revoked in block: %s\n", hash.Hex())
	return nil
}

// GetDIDDocument queries the DID document from storage
//
// Storage query: palletDid.didDocuments(did)
func (dc *DIDClient) GetDIDDocument(ctx context.Context, did string) (*DIDDocument, error) {
	meta := dc.client.GetMetadata()

	// Create storage key for DidDocuments map
	key, err := types.CreateStorageKey(meta, "Did", "DidDocuments", types.NewBytes([]byte(did)))
	if err != nil {
		return nil, fmt.Errorf("failed to create storage key: %w", err)
	}

	// Query storage
	var doc DIDDocumentRaw
	ok, err := dc.client.api.RPC.State.GetStorageLatest(key, &doc)
	if err != nil {
		return nil, fmt.Errorf("failed to query DID document: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("DID not found: %s", did)
	}

	// Convert PublicKey from []byte to [32]byte
	var pubKey [32]byte
	if len(doc.PublicKey) >= 32 {
		copy(pubKey[:], doc.PublicKey[:32])
	}

	return &DIDDocument{
		Controller: AccountID(doc.Controller),
		PublicKey:  pubKey,
		CreatedAt:  BlockNumber(doc.CreatedAt),
		UpdatedAt:  BlockNumber(doc.UpdatedAt),
		Active:     doc.Active,
	}, nil
}

// IsDIDActive checks if a DID is active
func (dc *DIDClient) IsDIDActive(ctx context.Context, did string) (bool, error) {
	doc, err := dc.GetDIDDocument(ctx, did)
	if err != nil {
		if err.Error() == fmt.Sprintf("DID not found: %s", did) {
			return false, nil
		}
		return false, err
	}
	return doc.Active, nil
}

// ResolvePublicKey gets the public key for a DID
func (dc *DIDClient) ResolvePublicKey(ctx context.Context, did string) ([32]byte, error) {
	doc, err := dc.GetDIDDocument(ctx, did)
	if err != nil {
		return [32]byte{}, err
	}
	return doc.PublicKey, nil
}

// DIDDocumentRaw is the raw storage format
type DIDDocumentRaw struct {
	Controller types.AccountID
	PublicKey  []byte
	CreatedAt  types.BlockNumber
	UpdatedAt  types.BlockNumber
	Active     bool
}

// submitTransaction submits a signed transaction to the blockchain
func (dc *DIDClient) submitTransaction(ctx context.Context, call types.Call) (types.Hash, error) {
	// Get runtime version
	rv, err := dc.client.api.RPC.State.GetRuntimeVersionLatest()
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to get runtime version: %w", err)
	}

	// Get nonce
	key, err := types.CreateStorageKey(dc.client.metadata, "System", "Account", dc.keyring.PublicKey)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to create storage key: %w", err)
	}

	var accountInfo types.AccountInfo
	ok, err := dc.client.api.RPC.State.GetStorageLatest(key, &accountInfo)
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
	genesisHash := dc.client.GetGenesisHash()

	// Get latest block hash
	blockHash, err := dc.client.api.RPC.Chain.GetBlockHashLatest()
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
	err = ext.Sign(*dc.keyring, o)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to sign extrinsic: %w", err)
	}

	// Submit the extrinsic
	hash, err := dc.client.api.RPC.Author.SubmitExtrinsic(ext)
	if err != nil {
		return types.Hash{}, fmt.Errorf("failed to submit extrinsic: %w", err)
	}

	return hash, nil
}

// CreateTestKeyring creates a test keyring for development
func CreateTestKeyring(seedPhrase string) (*signature.KeyringPair, error) {
	keyring, err := signature.KeyringPairFromSecret(seedPhrase, 42) // SS58 format 42 = Substrate
	if err != nil {
		return nil, fmt.Errorf("failed to create keyring: %w", err)
	}
	return &keyring, nil
}
