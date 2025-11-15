# 🌉 Substrate Go Client: Bridge to L1

**Package:** `libs/substrate`  
**Status:** ✅ **INTERFACE DESIGNED** - Ready for Implementation  
**Lines of Code:** ~600 (client.go) + ~400 (examples)  
**Date:** November 13, 2025

---

## 📋 OVERVIEW

The `substrate` package is the **bridge between the Go-based Orchestrator and the Substrate L1 blockchain**. It provides a clean, idiomatic Go API for interacting with on-chain state and submitting transactions.

### Architecture

```
┌─────────────────┐
│  Orchestrator   │ (Go)
│   (cmd/api)     │
└────────┬────────┘
         │
         │ import "libs/substrate"
         ▼
┌─────────────────┐
│ substrate.Client│
│                 │
│  • Query state  │
│  • Submit tx    │
│  • Listen events│
└────────┬────────┘
         │
         │ WebSocket JSON-RPC
         ▼
┌─────────────────┐
│  Substrate RPC  │ (ws://127.0.0.1:9944)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ L1 Blockchain   │
│                 │
│  • pallet-did   │
│  • pallet-registry │
│  • pallet-escrow│
└─────────────────┘
```

---

## 🔧 API REFERENCE

### Initialization

```go
import "github.com/aidenlippert/zerostate/libs/substrate"

// Connect to local node
client, err := substrate.NewClient("ws://127.0.0.1:9944")
if err != nil {
    log.Fatal(err)
}
defer client.Close()

ctx := context.Background()
```

---

### Escrow Queries

#### GetEscrow

Query on-chain escrow for a task.

```go
taskID, _ := substrate.TaskIDFromUUID("550e8400-e29b-41d4-a716-446655440000")

escrow, err := client.GetEscrow(ctx, taskID)
if err != nil {
    log.Printf("Escrow not found: %v", err)
    return
}

fmt.Printf("State: %s\n", escrow.State)
fmt.Printf("Amount: %s\n", escrow.Amount)
fmt.Printf("Expires at block: %d\n", escrow.ExpiresAt)
```

**Returns:** `*EscrowDetails`

**Fields:**
- `TaskID`: 32-byte task identifier
- `User`: Account that created escrow
- `AgentDID`: Agent's DID (if accepted)
- `Amount`: Locked AINU balance
- `State`: Pending/Accepted/Completed/Refunded/Disputed
- `CreatedAt`, `ExpiresAt`: Block numbers

---

#### GetUserEscrows

Get all escrows created by a user.

```go
var userAccount substrate.AccountID
// ... populate userAccount ...

escrows, err := client.GetUserEscrows(ctx, userAccount)
fmt.Printf("User has %d escrows\n", len(escrows))

for _, escrow := range escrows {
    fmt.Printf("  Task: %x, State: %s\n", escrow.TaskID[:8], escrow.State)
}
```

---

### DID Queries

#### GetDID

Query on-chain DID document.

```go
did := substrate.DID("did:ainur:alice")

doc, err := client.GetDID(ctx, did)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Controller: %x\n", doc.Controller)
fmt.Printf("Public Key: %x\n", doc.PublicKey)
fmt.Printf("Active: %t\n", doc.Active)
```

---

#### IsDIDActive

Check if a DID is active.

```go
active, err := client.IsDIDActive(ctx, "did:ainur:alice")
if !active {
    return errors.New("DID is inactive or not found")
}
```

---

### Registry Queries

#### GetAgentCard

Query on-chain agent registration.

```go
card, err := client.GetAgentCard(ctx, "did:ainur:alice")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Name: %s\n", card.Name)
fmt.Printf("Capabilities: %v\n", card.Capabilities)

priceAINU, _ := substrate.BalanceToAINU(card.PricePerTask)
fmt.Printf("Price: %.2f AINU\n", priceAINU)
```

**Returns:** `*AgentCard`

**Fields:**
- `DID`: Agent's decentralized identifier
- `Name`: Human-readable name
- `Capabilities`: List of capabilities (e.g., ["math", "text"])
- `WASMHash`: Hash of WASM module
- `PricePerTask`: Price in AINU
- `Active`: Status flag

---

#### FindAgentsByCapability

Search for agents with specific capability.

```go
dids, err := client.FindAgentsByCapability(ctx, "math")
fmt.Printf("Found %d agents with 'math' capability\n", len(dids))

for _, did := range dids {
    card, _ := client.GetAgentCard(ctx, did)
    fmt.Printf("  - %s (%.2f AINU)\n", card.Name, ...)
}
```

**This is the on-chain replacement for:**
```sql
SELECT * FROM agents WHERE capabilities @> '["math"]'
```

---

### Transaction Submission (Future)

#### CreateEscrow

Create on-chain escrow for a task.

```go
params := substrate.CreateEscrowParams{
    TaskID:        taskID,
    Amount:        substrate.BalanceFromAINU(100.0), // 100 AINU
    TaskHash:      sha256(taskDescription),
    TimeoutBlocks: nil, // Use default (24 hours)
}

txHash, err := client.CreateEscrow(ctx, params)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Escrow created: %x\n", txHash)
```

**Note:** Requires transaction signing implementation (Phase 4).

---

#### ReleasePayment

Release payment to agent upon task completion.

```go
txHash, err := client.ReleasePayment(ctx, taskID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Payment released: %x\n", txHash)
// Agent receives 95 AINU, protocol receives 5 AINU
```

---

#### RefundEscrow

Refund escrow to user (cancellation or timeout).

```go
txHash, err := client.RefundEscrow(ctx, taskID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Escrow refunded: %x\n", txHash)
// User receives 100% of locked funds back
```

---

### Event Listening (Future)

#### SubscribeEvents

Subscribe to blockchain events.

```go
events := make(chan substrate.Event)

go func() {
    client.SubscribeEvents(ctx, events)
}()

for event := range events {
    switch e := event.Event.(type) {
    case substrate.PaymentReleasedEvent:
        fmt.Printf("💰 Payment: %.2f AINU to agent\n", ...)
        // Update PostgreSQL task status
        
    case substrate.EscrowCreatedEvent:
        fmt.Printf("🔒 Escrow created: %.2f AINU\n", ...)
    }
}
```

---

## 🎯 INTEGRATION PATTERNS

### Pattern 1: Auction Winner → On-Chain Escrow

**Where:** Auctioneer (after auction completes)

```go
func (a *Auctioneer) HandleAuctionWin(winner AuctionWinner) error {
    // 1. Convert UUID to blockchain task ID
    taskID, err := substrate.TaskIDFromUUID(winner.TaskID)
    
    // 2. Create on-chain escrow
    params := substrate.CreateEscrowParams{
        TaskID: taskID,
        Amount: substrate.BalanceFromAINU(winner.BidAmount),
        TaskHash: hashTaskDescription(winner.TaskDesc),
    }
    
    txHash, err := a.substrateClient.CreateEscrow(ctx, params)
    
    // 3. Store transaction hash in PostgreSQL
    a.db.UpdateTaskEscrow(winner.TaskID, txHash)
    
    return nil
}
```

---

### Pattern 2: Task Completion → Release Payment

**Where:** Orchestrator (after validation)

```go
func (o *Orchestrator) CompleteTask(taskID string, result []byte) error {
    // 1. Validate result
    if !o.validateResult(result) {
        return errors.New("validation failed")
    }
    
    // 2. Release payment on-chain
    chainTaskID, _ := substrate.TaskIDFromUUID(taskID)
    txHash, err := o.substrateClient.ReleasePayment(ctx, chainTaskID)
    
    // 3. Update PostgreSQL
    o.db.UpdateTaskStatus(taskID, "completed", txHash)
    
    return nil
}
```

---

### Pattern 3: Hybrid Discovery (Transition Period)

**Where:** Agent Selector

```go
func (s *AgentSelector) FindAgents(capability string) []Agent {
    // During Phase 4 transition, query BOTH sources
    
    // Query PostgreSQL (legacy)
    dbAgents := s.db.FindAgentsByCapability(capability)
    
    // Query blockchain (new)
    chainDIDs, err := s.substrateClient.FindAgentsByCapability(ctx, capability)
    if err != nil {
        // Fallback to PostgreSQL only
        return dbAgents
    }
    
    // Merge results, prefer on-chain
    agents := mergeAgentSources(dbAgents, chainDIDs)
    
    return agents
}
```

**Eventually (Phase 4 complete):**
```go
func (s *AgentSelector) FindAgents(capability string) []Agent {
    // PostgreSQL DELETED - query blockchain only
    chainDIDs, err := s.substrateClient.FindAgentsByCapability(ctx, capability)
    
    agents := make([]Agent, 0, len(chainDIDs))
    for _, did := range chainDIDs {
        card, _ := s.substrateClient.GetAgentCard(ctx, did)
        agents = append(agents, agentFromCard(card))
    }
    
    return agents
}
```

---

### Pattern 4: Monitor Expired Escrows

**Where:** Background worker

```go
func MonitorEscrows(client *substrate.Client) {
    ticker := time.NewTicker(30 * time.Second)
    
    for range ticker.C {
        currentBlock, _ := client.GetBlockNumber(ctx)
        
        // Get all pending/accepted tasks
        tasks := db.GetActiveTasks()
        
        for _, task := range tasks {
            taskID, _ := substrate.TaskIDFromUUID(task.ID)
            escrow, err := client.GetEscrow(ctx, taskID)
            
            if err != nil {
                continue // Not on-chain yet
            }
            
            // Check expiration
            if currentBlock >= escrow.ExpiresAt {
                log.Printf("⚠️  Refunding expired escrow: %s", task.ID)
                client.RefundEscrow(ctx, taskID)
            }
        }
    }
}
```

---

## 🔄 MIGRATION STRATEGY (Phase 4)

### Step 1: Dual-Write (Week 1)

Write to BOTH PostgreSQL and blockchain:

```go
// Create task
task := db.CreateTask(...)

// ALSO create on-chain escrow
client.CreateEscrow(ctx, params)
```

### Step 2: Dual-Read (Week 2)

Read from BOTH, verify consistency:

```go
// Query PostgreSQL
dbAgents := db.FindAgents("math")

// Query blockchain
chainAgents := client.FindAgentsByCapability(ctx, "math")

// Compare and log discrepancies
if len(dbAgents) != len(chainAgents) {
    log.Warn("Inconsistency detected!")
}
```

### Step 3: Blockchain-Primary (Week 3)

Read from blockchain, fallback to PostgreSQL:

```go
agents, err := client.FindAgentsByCapability(ctx, "math")
if err != nil {
    // Fallback to PostgreSQL
    agents = db.FindAgents("math")
}
```

### Step 4: PostgreSQL Deletion (Week 4)

Delete centralized tables:

```sql
DROP TABLE agents;
DROP TABLE escrows;
-- Keep only user auth and task history
```

---

## 📊 COMPARISON: Before vs After

### Agent Discovery

**Before (PostgreSQL):**
```go
agents := db.Query("SELECT * FROM agents WHERE capabilities @> ?", '["math"]')
// Centralized, requires trust in database admin
```

**After (Blockchain):**
```go
dids := client.FindAgentsByCapability(ctx, "math")
cards := client.GetAgentCards(ctx, dids)
// Decentralized, trustless, auditable
```

---

### Escrow Management

**Before (PostgreSQL):**
```go
db.CreateEscrow(taskID, userID, amount)
// Trust admin to release payment
db.ReleasePayment(taskID)
```

**After (Blockchain):**
```go
client.CreateEscrow(ctx, params)
// Smart contract guarantees payment
client.ReleasePayment(ctx, taskID)
// Automatic 95/5 split, no admin needed
```

---

## 🧪 TESTING STRATEGY

### Unit Tests

```go
func TestGetEscrow(t *testing.T) {
    client, _ := substrate.NewClient("ws://127.0.0.1:9944")
    
    taskID := [32]byte{...}
    escrow, err := client.GetEscrow(ctx, taskID)
    
    assert.NoError(t, err)
    assert.Equal(t, substrate.EscrowStatePending, escrow.State)
}
```

### Integration Tests

```go
func TestEndToEndEscrow(t *testing.T) {
    // 1. Create escrow
    params := substrate.CreateEscrowParams{...}
    txHash, _ := client.CreateEscrow(ctx, params)
    
    // 2. Wait for block finalization
    time.Sleep(6 * time.Second)
    
    // 3. Query escrow
    escrow, _ := client.GetEscrow(ctx, params.TaskID)
    assert.Equal(t, substrate.EscrowStatePending, escrow.State)
    
    // 4. Accept task (as agent)
    client.AcceptTask(ctx, params.TaskID, "did:ainur:agent")
    
    // 5. Release payment
    client.ReleasePayment(ctx, params.TaskID)
    
    // 6. Verify completion
    escrow, _ = client.GetEscrow(ctx, params.TaskID)
    assert.Equal(t, substrate.EscrowStateCompleted, escrow.State)
}
```

---

## 🚀 IMPLEMENTATION ROADMAP

### Phase 3 (Current - Design Complete) ✅

- [x] Design client API
- [x] Define types (EscrowDetails, AgentCard, etc.)
- [x] Create integration examples
- [x] Document migration patterns

### Phase 4 (Next - Implementation)

- [ ] Implement SCALE codec decoding
- [ ] Implement transaction signing (sr25519)
- [ ] Add proper storage key generation (twox128)
- [ ] Implement event subscription
- [ ] Add connection pooling and retry logic
- [ ] Write comprehensive tests
- [ ] Deploy to testnet

### Phase 5 (Future - Production)

- [ ] Connection health monitoring
- [ ] Metrics and observability
- [ ] Rate limiting
- [ ] Caching layer for frequent queries
- [ ] Batch query optimization
- [ ] Production deployment

---

## 🎉 VICTORY SUMMARY

**We just designed the complete Go ↔ Substrate integration layer!**

- ✅ Clean, idiomatic Go API (~600 lines)
- ✅ Full escrow lifecycle support
- ✅ Agent discovery via on-chain registry
- ✅ DID verification
- ✅ Integration patterns documented
- ✅ Migration strategy defined
- ✅ 7 complete usage examples

**This is the bridge that makes decentralization real.**

Next: Phase 4 - The "Great Rip-Out" (DELETE PostgreSQL agents table!)

🚀 **The economy is going on-chain!**
