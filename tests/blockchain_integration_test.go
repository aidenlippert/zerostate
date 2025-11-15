package tests

import (
"context"
"crypto/sha256"
"fmt"
"testing"
"time"

"github.com/aidenlippert/zerostate/libs/substrate"
"github.com/centrifuge/go-substrate-rpc-client/v4/signature"
)

func TestBlockchainIntegration(t *testing.T) {
ctx := context.Background()

fmt.Println("\n🔗 Connecting to chain-v2...")
client, err := substrate.NewClientV2("ws://127.0.0.1:35651")
if err != nil {
nect: %v", err)
}
defer client.Close()

// Get chain info
info, err := client.GetChainInfo(ctx)
if err != nil {
 info: %v", err)
}
fmt.Printf("✅ Connected to %s v%s (block #%d)\n\n", info.Name, info.Version, info.BlockNumber)

// Create test keyring (Alice for dev)
keyring, err := signature.KeyringPairFromSecret("//Alice", 42)
if err != nil {
g: %v", err)
}
fmt.Printf("🔑 Using account: %s\n\n", keyring.Address)

// TEST 1: DID Creation
t.Run("DID Creation", func(t *testing.T) {
tln("════════════════════════════════════════════════════════════════")
tln("TEST 1: DID Creation")
tln("════════════════════════════════════════════════════════════════")

t := substrate.NewDIDClient(client, keyring)
ur:test-agent-001"

tf("Creating DID: %s\n", testDID)
t.CreateDID(ctx, testDID, keyring.PublicKey)
il {
tf("⚠️  DID creation failed (might already exist): %v\n", err)
tln("✅ DID created successfully!")
d)
uery DID
tln("\nQuerying DID document...")
t.GetDIDDocument(ctx, testDID)
il {
uery DID: %v", err)
tf("✅ DID found!\n")
tf("   Owner: %x\n", doc.Owner)
tf("   Public Key: %x\n", doc.PublicKey)
tf("   Active: %v\n", doc.Active)
tf("   Created at block: %d\n", doc.CreatedAt)
t Registration
t.Run("Agent Registration", func(t *testing.T) {
tln("\n════════════════════════════════════════════════════════════════")
tln("TEST 2: Agent Registration")
tln("════════════════════════════════════════════════════════════════")

t := substrate.NewRegistryClient(client, keyring)
ur:test-agent-001"

t-wasm-content")
tReg := substrate.AgentRegistration{
testDID,
ame:         "Test Math Agent",
g{"math", "calculation"},
tf("Registering agent: %s\n", agentReg.Name)
tf("  Capabilities: %v\n", agentReg.Capabilities)
tf("  WASM Hash: %x...\n", wasmHash[:8])

t.RegisterAgent(ctx, agentReg)
il {
tf("⚠️  Agent registration failed (might already exist): %v\n", err)
tln("✅ Agent registered successfully!")
d)
uery agent card
tln("\nQuerying agent card...")
t.GetAgentCard(ctx, testDID)
il {
uery agent: %v", err)
tf("✅ Agent found!\n")
tf("   Name: %s\n", card.Name)
tf("   DID: %s\n", card.DID)
tf("   Capabilities: %v\n", card.Capabilities)
tf("   Price: %s\n", card.PricePerTask)
tf("   Registered at block: %d\n", card.RegisteredAt)

t.Run("Escrow Creation", func(t *testing.T) {
tln("\n════════════════════════════════════════════════════════════════")
tln("TEST 3: Escrow Creation")
tln("════════════════════════════════════════════════════════════════")

t := substrate.NewEscrowClient(client, keyring)

te("test-task-001"))

tf("Creating escrow for task: %x...\n", taskID[:8])
tln("  Amount: 1000 tokens")
tln("  Timeout: 100 blocks")

t.CreateEscrow(ctx, taskID, 1000, 100)
il {
tf("⚠️  Escrow creation failed: %v\n", err)
tln("✅ Escrow created successfully!")
d)
tln("\n════════════════════════════════════════════════════════════════")
fmt.Println("🎉 BLOCKCHAIN INTEGRATION TEST COMPLETE!")
fmt.Println("════════════════════════════════════════════════════════════════")
}
