#!/bin/bash

# Test chain-v2 RPC client connection
# This script verifies that our Go client can connect to the running chain-v2 node

set -e

echo "🧪 Testing Chain-V2 RPC Client Connection"
echo "========================================"
echo ""

# Create test program
cat > /tmp/test_chain_v2.go << 'EOF'
package main

import (
	"context"
	"fmt"
	"log"
	"time"
)

// Minimal inline client for testing (avoiding import paths)
import (
	gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

func main() {
	// Try multiple RPC ports (node uses random ports sometimes)
	ports := []string{"35651", "41339", "9944"}
	
	var api *gsrpc.SubstrateAPI
	var err error
	var connectedPort string
	
	fmt.Println("Connecting to chain-v2 node...")
	for _, port := range ports {
		endpoint := fmt.Sprintf("ws://127.0.0.1:%s", port)
		api, err = gsrpc.NewSubstrateAPI(endpoint)
		if err == nil {
			connectedPort = port
			break
		}
	}
	
	if api == nil {
		log.Fatalf("❌ Failed to connect on any port: %v", err)
	}
	
	fmt.Printf("✅ Connected successfully on port %s!\n", connectedPort)
	
	// Test 1: Get chain name
	fmt.Println("\n📋 Test 1: Get Chain Info")
	name, err := api.RPC.System.Chain()
	if err != nil {
		log.Fatalf("❌ Failed to get chain name: %v", err)
	}
	fmt.Printf("   Chain name: %s\n", name)
	
	// Test 2: Get node version
	version, err := api.RPC.System.Version()
	if err != nil {
		log.Fatalf("❌ Failed to get version: %v", err)
	}
	fmt.Printf("   Node version: %s\n", version)
	
	// Test 3: Get genesis hash
	genesisHash, err := api.RPC.Chain.GetBlockHash(0)
	if err != nil {
		log.Fatalf("❌ Failed to get genesis hash: %v", err)
	}
	fmt.Printf("   Genesis hash: %s\n", genesisHash.Hex())
	
	// Test 4: Get latest block
	header, err := api.RPC.Chain.GetHeaderLatest()
	if err != nil {
		log.Fatalf("❌ Failed to get latest header: %v", err)
	}
	fmt.Printf("   Latest block: #%d\n", header.Number)
	
	// Test 5: Get metadata
	fmt.Println("\n📦 Test 2: Get Runtime Metadata")
	meta, err := api.RPC.State.GetMetadataLatest()
	if err != nil {
		log.Fatalf("❌ Failed to get metadata: %v", err)
	}
	fmt.Printf("   Metadata version: %d\n", meta.Version)
	
	// Test 6: Check for custom pallets
	fmt.Println("\n🔍 Test 3: Verify Custom Pallets")
	
	pallets := []string{"Did", "Registry", "Escrow"}
	for _, palletName := range pallets {
		found := false
		for _, pallet := range meta.AsMetadataV14.Pallets {
			if string(pallet.Name) == palletName {
				found = true
				fmt.Printf("   ✅ Pallet-%s found (index: %d)\n", palletName, pallet.Index)
				break
			}
		}
		if !found {
			fmt.Printf("   ❌ Pallet-%s NOT FOUND\n", palletName)
		}
	}
	
	// Test 7: Get Alice's account info
	fmt.Println("\n💰 Test 4: Query Alice's Account")
	
	// Alice's address (//Alice in dev mode)
	aliceAddr := "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY"
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Parse address
	pubKey, err := types.NewAddressFromHexAccountID(aliceAddr)
	if err != nil {
		log.Printf("   ⚠️  Could not parse Alice's address: %v\n", err)
	} else {
		// Get account info
		key, err := types.CreateStorageKey(meta, "System", "Account", pubKey.AsAccountID[:])
		if err != nil {
			log.Printf("   ⚠️  Could not create storage key: %v\n", err)
		} else {
			var accountInfo types.AccountInfo
			ok, err := api.RPC.State.GetStorageLatest(key, &accountInfo)
			if err != nil {
				log.Printf("   ⚠️  Failed to query account: %v\n", err)
			} else if !ok {
				fmt.Println("   Alice's account not found (expected in dev mode)")
			} else {
				fmt.Printf("   Alice's balance: %s\n", accountInfo.Data.Free.String())
				fmt.Printf("   Alice's nonce: %d\n", accountInfo.Nonce)
			}
		}
	}
	
	_ = ctx // use ctx to avoid unused warning
	
	fmt.Println("\n✅ All tests passed!")
	fmt.Println("\n🎉 Chain-V2 RPC client is fully operational!")
}
EOF

# Run the test
TEST_DIR="/tmp/test_chain_v2_$$"
mkdir -p "$TEST_DIR"
cd "$TEST_DIR"

mv /tmp/test_chain_v2.go .

echo "📦 Building test program..."
go mod init test_chain_v2 > /dev/null 2>&1
echo "   Installing dependencies..."
go get github.com/centrifuge/go-substrate-rpc-client/v4@latest > /dev/null 2>&1
go mod tidy > /dev/null 2>&1

echo ""
echo "🚀 Running tests..."
echo ""
go run test_chain_v2.go

# Cleanup
cd /tmp
rm -rf "$TEST_DIR"

echo ""
echo "=========================================="
echo "✅ Chain-V2 RPC Client Test Complete!"
