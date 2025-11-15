package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aidenlippert/zerostate/libs/orchestration"
)

// MockBlockchain implements the BlockchainInterface for testing
type MockBlockchain struct {
	enabled bool
}

func NewMockBlockchain(enabled bool) *MockBlockchain {
	return &MockBlockchain{enabled: enabled}
}

func (mb *MockBlockchain) ReleasePayment(ctx context.Context, taskID string) (txHash string, err error) {
	fmt.Printf("🔓 MockBlockchain: Releasing payment for task %s\n", taskID)
	return fmt.Sprintf("0x%s", taskID[:8]), nil
}

func (mb *MockBlockchain) RefundEscrow(ctx context.Context, taskID string) (txHash string, err error) {
	fmt.Printf("💸 MockBlockchain: Refunding escrow for task %s\n", taskID)
	return fmt.Sprintf("0x%s", taskID[:8]), nil
}

func (mb *MockBlockchain) DisputeEscrow(ctx context.Context, taskID string, reason string) (txHash string, err error) {
	fmt.Printf("⚠️  MockBlockchain: Disputing escrow for task %s, reason: %s\n", taskID, reason)
	return fmt.Sprintf("0x%s", taskID[:8]), nil
}

func (mb *MockBlockchain) IsEnabled() bool {
	return mb.enabled
}

func (mb *MockBlockchain) GetEscrowStatus(ctx context.Context, taskID string) (orchestration.PaymentStatus, error) {
	return orchestration.PaymentStatusCreated, nil
}

func testPaymentLifecycle() {
	fmt.Println("🚀 Testing Payment Lifecycle Integration")
	fmt.Println("=========================================")

	// Setup
	ctx := context.Background()
	mockBlockchain := NewMockBlockchain(true)
	config := orchestration.DefaultPaymentConfig()
	config.RetryMaxAttempts = 2
	config.RetryBaseDelay = 50 * time.Millisecond

	pm := orchestration.NewPaymentLifecycleManager(mockBlockchain, config, nil)

	taskID := "task-12345678-test"
	userID := "user-987654321"
	agentID := "agent-123456789"
	amount := 10.75

	fmt.Printf("📋 Test Parameters:\n")
	fmt.Printf("   Task ID: %s\n", taskID)
	fmt.Printf("   User ID: %s\n", userID)
	fmt.Printf("   Agent ID: %s\n", agentID)
	fmt.Printf("   Amount: %.2f\n", amount)
	fmt.Println()

	// Test 1: Create Payment
	fmt.Println("🏗️  Test 1: Creating Payment")
	fmt.Println("---------------------------")
	payment := pm.CreatePayment(taskID, userID, amount)
	fmt.Printf("✅ Payment created with status: %s\n", payment.Status)
	fmt.Printf("   Created at: %s\n", payment.CreatedAt.Format(time.RFC3339))
	fmt.Printf("   Events: %d\n", len(payment.Events))
	fmt.Println()

	// Test 2: Update to Pending
	fmt.Println("⏳ Test 2: Updating to Pending Status")
	fmt.Println("------------------------------------")
	err := pm.UpdatePaymentStatus(taskID, orchestration.PaymentStatusPending, "waiting for agent", "")
	if err != nil {
		log.Fatalf("Failed to update to pending: %v", err)
	}
	fmt.Printf("✅ Status updated to: %s\n", orchestration.PaymentStatusPending)
	fmt.Println()

	// Test 3: Update to Accepted
	fmt.Println("🎯 Test 3: Updating to Accepted Status")
	fmt.Println("-------------------------------------")
	err = pm.UpdatePaymentStatus(taskID, orchestration.PaymentStatusAccepted, "agent selected", "")
	if err != nil {
		log.Fatalf("Failed to update to accepted: %v", err)
	}
	fmt.Printf("✅ Status updated to: %s\n", orchestration.PaymentStatusAccepted)
	fmt.Println()

	// Test 4: Release Payment (Success Case)
	fmt.Println("💰 Test 4: Releasing Payment")
	fmt.Println("---------------------------")
	err = pm.ReleasePayment(ctx, taskID, agentID)
	if err != nil {
		log.Fatalf("Failed to release payment: %v", err)
	}

	finalPayment, _ := pm.GetPaymentInfo(taskID)
	fmt.Printf("✅ Payment released successfully\n")
	fmt.Printf("   Final status: %s\n", finalPayment.Status)
	fmt.Printf("   Payment TX Hash: %s\n", finalPayment.PaymentTxHash)
	fmt.Printf("   Agent ID: %s\n", finalPayment.AgentID)
	fmt.Printf("   Completed at: %s\n", finalPayment.CompletedAt.Format(time.RFC3339))
	fmt.Printf("   Total events: %d\n", len(finalPayment.Events))
	fmt.Println()

	// Test 5: Payment State Machine Validation
	fmt.Println("🔄 Test 5: Payment State Machine Validation")
	fmt.Println("-----------------------------------------")

	// Create another payment for state machine testing
	taskID2 := "task-87654321-test"
	pm.CreatePayment(taskID2, userID, amount)

	// Test invalid transition
	err = pm.UpdatePaymentStatus(taskID2, orchestration.PaymentStatusReleased, "invalid transition", "")
	if err != nil {
		fmt.Printf("✅ Invalid transition properly rejected: %v\n", err)
	} else {
		fmt.Printf("❌ Invalid transition was allowed (this is bad!)\n")
	}

	// Test valid transitions
	validTransitions := []orchestration.PaymentStatus{
		orchestration.PaymentStatusPending,
		orchestration.PaymentStatusAccepted,
	}

	for _, status := range validTransitions {
		err = pm.UpdatePaymentStatus(taskID2, status, "valid transition", "")
		if err != nil {
			fmt.Printf("❌ Valid transition to %s failed: %v\n", status, err)
		} else {
			fmt.Printf("✅ Valid transition to %s succeeded\n", status)
		}
	}
	fmt.Println()

	// Test 6: Refund Payment
	fmt.Println("💸 Test 6: Testing Payment Refund")
	fmt.Println("--------------------------------")
	taskID3 := "task-refund-test"
	pm.CreatePayment(taskID3, userID, amount)
	pm.UpdatePaymentStatus(taskID3, orchestration.PaymentStatusPending, "waiting for agent", "")

	err = pm.RefundPayment(ctx, taskID3, "task failed due to timeout")
	if err != nil {
		log.Fatalf("Failed to refund payment: %v", err)
	}

	refundPayment, _ := pm.GetPaymentInfo(taskID3)
	fmt.Printf("✅ Payment refunded successfully\n")
	fmt.Printf("   Final status: %s\n", refundPayment.Status)
	fmt.Printf("   Payment TX Hash: %s\n", refundPayment.PaymentTxHash)
	fmt.Printf("   Total events: %d\n", len(refundPayment.Events))
	fmt.Println()

	// Test 7: Dispute Payment
	fmt.Println("⚠️  Test 7: Testing Payment Dispute")
	fmt.Println("----------------------------------")
	taskID4 := "task-dispute-test"
	pm.CreatePayment(taskID4, userID, amount)
	pm.UpdatePaymentStatus(taskID4, orchestration.PaymentStatusAccepted, "agent selected", "")

	err = pm.DisputePayment(ctx, taskID4, "poor quality work", userID)
	if err != nil {
		log.Fatalf("Failed to dispute payment: %v", err)
	}

	disputePayment, _ := pm.GetPaymentInfo(taskID4)
	fmt.Printf("✅ Payment disputed successfully\n")
	fmt.Printf("   Final status: %s\n", disputePayment.Status)
	fmt.Printf("   Total events: %d\n", len(disputePayment.Events))
	fmt.Println()

	// Test 8: Metrics
	fmt.Println("📊 Test 8: Payment Metrics")
	fmt.Println("-------------------------")
	metrics := pm.GetPaymentMetrics()
	fmt.Printf("✅ Payment metrics retrieved:\n")
	for key, value := range metrics {
		fmt.Printf("   %s: %v\n", key, value)
	}
	fmt.Println()

	fmt.Println("🎉 All payment lifecycle tests completed successfully!")
	fmt.Println("=====================================================")
}

func main() {
	testPaymentLifecycle()
}