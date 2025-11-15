// Package substrate provides integration examples for using the Substrate client
// with the Ainur Orchestrator for on-chain escrow and discovery.
//
// These examples demonstrate real-world usage patterns.
package substrate

import (
	"context"
	"fmt"
	"log"
	"time"
)

// ============================================================================
// Example 1: Query On-Chain Escrow
// ============================================================================

func ExampleQueryEscrow() {
	// Connect to local Substrate node
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Get task ID from PostgreSQL
	taskUUID := "550e8400-e29b-41d4-a716-446655440000"
	taskID, err := TaskIDFromUUID(taskUUID)
	if err != nil {
		log.Fatal(err)
	}

	// Query on-chain escrow
	escrow, err := client.GetEscrow(ctx, taskID)
	if err != nil {
		log.Printf("Escrow not found (not yet on-chain): %v", err)
		return
	}

	// Display escrow details
	fmt.Printf("Escrow State: %s\n", escrow.State)
	fmt.Printf("Amount: %s\n", escrow.Amount)
	fmt.Printf("Created At Block: %d\n", escrow.CreatedAt)
	fmt.Printf("Expires At Block: %d\n", escrow.ExpiresAt)

	if escrow.AgentDID != nil {
		fmt.Printf("Agent DID: %s\n", *escrow.AgentDID)
	}
}

// ============================================================================
// Example 2: Check Escrow Timeout
// ============================================================================

func ExampleCheckTimeout() {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	taskUUID := "550e8400-e29b-41d4-a716-446655440000"
	taskID, _ := TaskIDFromUUID(taskUUID)

	// Get current block number
	currentBlock, err := client.GetBlockNumber(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Get escrow
	escrow, err := client.GetEscrow(ctx, taskID)
	if err != nil {
		log.Fatal(err)
	}

	// Check if expired
	if currentBlock >= escrow.ExpiresAt {
		fmt.Printf("⚠️  Escrow EXPIRED! Current: %d, Expires: %d\n",
			currentBlock, escrow.ExpiresAt)

		// Trigger refund
		// hash, err := client.RefundEscrow(ctx, taskID)
		// if err != nil {
		//     log.Fatal(err)
		// }
		// fmt.Printf("Refund transaction: %x\n", hash)
	} else {
		remainingBlocks := escrow.ExpiresAt - currentBlock
		remainingTime := time.Duration(remainingBlocks) * 6 * time.Second
		fmt.Printf("✅ Escrow active. Expires in %d blocks (~%v)\n",
			remainingBlocks, remainingTime)
	}
}

// ============================================================================
// Example 3: Agent Discovery via On-Chain Registry
// ============================================================================

func ExampleDiscoverAgents() {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Find agents with "math" capability
	dids, err := client.FindAgentsByCapability(ctx, "math")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d agents with 'math' capability:\n", len(dids))

	// Get details for each agent
	for _, did := range dids {
		card, err := client.GetAgentCard(ctx, did)
		if err != nil {
			log.Printf("Failed to get card for %s: %v", did, err)
			continue
		}

		if !card.Active {
			continue // Skip inactive agents
		}

		priceAINU, _ := BalanceToAINU(card.PricePerTask)

		fmt.Printf("\n  Agent: %s\n", card.Name)
		fmt.Printf("  DID: %s\n", card.DID)
		fmt.Printf("  Capabilities: %v\n", card.Capabilities)
		fmt.Printf("  Price: %.2f AINU\n", priceAINU)
		fmt.Printf("  WASM Hash: %x\n", card.WASMHash[:8])
	}
}

// ============================================================================
// Example 4: Hybrid Discovery (Transition Period)
// ============================================================================

// During Phase 4 "Great Rip-Out", we'll have a hybrid period where:
// - Some agents are in PostgreSQL only
// - Some agents are on-chain only
// - Eventually: ALL agents on-chain

func ExampleHybridDiscovery() {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	// Step 1: Query PostgreSQL (current system)
	dbAgents := queryPostgreSQLAgents("math") // Existing function
	fmt.Printf("PostgreSQL agents: %d\n", len(dbAgents))

	// Step 2: Query on-chain registry
	chainDIDs, err := client.FindAgentsByCapability(ctx, "math")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("On-chain agents: %d\n", len(chainDIDs))

	// Step 3: Merge results (prefer on-chain)
	uniqueAgents := make(map[string]bool)
	for _, did := range chainDIDs {
		uniqueAgents[string(did)] = true
	}
	for _, agent := range dbAgents {
		if agent.DID != "" {
			uniqueAgents[agent.DID] = true
		}
	}

	fmt.Printf("Total unique agents: %d\n", len(uniqueAgents))

	// Step 4: Fetch full details from on-chain
	for did := range uniqueAgents {
		card, err := client.GetAgentCard(ctx, DID(did))
		if err != nil {
			// Fallback to PostgreSQL
			continue
		}

		// Use on-chain data
		_ = card
	}
}

// Mock function - replace with actual DB query
func queryPostgreSQLAgents(capability string) []struct{ DID string } {
	return []struct{ DID string }{}
}

// ============================================================================
// Example 5: Listen for Payment Events (Future)
// ============================================================================

func ExampleListenForPayments() {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	events := make(chan Event)

	// Subscribe to events
	go func() {
		if err := client.SubscribeEvents(ctx, events); err != nil {
			log.Printf("Event subscription error: %v", err)
		}
	}()

	// Process events
	for range events {
		// Decode event type
		var paymentEvent PaymentReleasedEvent
		// if err := json.Unmarshal(event.Event, &paymentEvent); err != nil {
		//     continue
		// }

		fmt.Printf("💰 Payment Released!\n")
		fmt.Printf("   Task: %x\n", paymentEvent.TaskID[:8])
		fmt.Printf("   Agent: %x\n", paymentEvent.Agent[:8])
		amount, _ := BalanceToAINU(paymentEvent.Amount)
		fee, _ := BalanceToAINU(paymentEvent.Fee)

		fmt.Printf("   Amount: %.2f AINU\n", amount)
		fmt.Printf("   Fee: %.2f AINU\n", fee) // Update PostgreSQL task status
		// db.UpdateTaskStatus(paymentEvent.TaskID, "completed")
	}
}

// ============================================================================
// Example 6: Orchestrator Integration Point
// ============================================================================

// This is where the Orchestrator will create escrows after auction wins

type AuctionWinner struct {
	TaskID    string
	AgentDID  string
	BidAmount float64 // in AINU
}

func HandleAuctionWin(winner AuctionWinner) error {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		return err
	}
	defer client.Close()

	ctx := context.Background()

	// Convert task UUID to blockchain task ID
	taskID, err := TaskIDFromUUID(winner.TaskID)
	if err != nil {
		return err
	}

	// Convert AINU to balance
	amount := BalanceFromAINU(winner.BidAmount)

	// Create escrow on-chain
	params := CreateEscrowParams{
		TaskID:        taskID,
		Amount:        amount,
		TaskHash:      [32]byte{}, // Hash of task description
		TimeoutBlocks: nil,        // Use default timeout
	}

	// Submit transaction
	txHash, err := client.CreateEscrow(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create escrow: %w", err)
	}

	log.Printf("✅ Escrow created on-chain: %x", txHash)

	// Update PostgreSQL with on-chain escrow reference
	// db.UpdateTaskEscrow(winner.TaskID, txHash)

	return nil
}

// ============================================================================
// Example 7: Monitor and Auto-Refund Expired Escrows
// ============================================================================

func MonitorExpiredEscrows() {
	client, err := NewClient("ws://127.0.0.1:9944")
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Get current block
		_, err := client.GetBlockNumber(ctx)
		if err != nil {
			log.Printf("Error getting block number: %v", err)
			continue
		}

		// Query all active tasks from PostgreSQL
		// tasks := db.GetActiveTasks()

		// Check each task's escrow
		// for _, task := range tasks {
		//     taskID, _ := TaskIDFromUUID(task.ID)
		//
		//     escrow, err := client.GetEscrow(ctx, taskID)
		//     if err != nil {
		//         continue
		//     }
		//
		//     // Check if expired
		//     if currentBlock >= escrow.ExpiresAt &&
		//        (escrow.State == EscrowStatePending ||
		//         escrow.State == EscrowStateAccepted) {
		//
		//         // Trigger refund
		//         log.Printf("🔄 Refunding expired escrow: %s", task.ID)
		//         client.RefundEscrow(ctx, taskID)
		//     }
		// }
	}
}

func main() {
	fmt.Println("Substrate Integration Examples")
	fmt.Println("===============================")

	// Run examples
	// ExampleQueryEscrow()
	// ExampleCheckTimeout()
	// ExampleDiscoverAgents()
	// ExampleHybridDiscovery()
	// ExampleListenForPayments()
}
