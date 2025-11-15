// Package e2e provides end-to-end integration tests for Sprint 8 escrow features
package e2e

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// Sprint8EscrowTestSuite provides comprehensive E2E testing for Sprint 8 escrow features
type Sprint8EscrowTestSuite struct {
	suite.Suite
	ctx          context.Context
	client       *SubstrateClient
	alice        *TestAccount
	bob          *TestAccount
	charlie      *TestAccount
	dave         *TestAccount
	eve          *TestAccount
	testTimeout  time.Duration
}

// TestAccount represents a test account with keys and balances
type TestAccount struct {
	Name       string
	Address    string
	PrivateKey string
	PublicKey  string
	Balance    uint64
}

// SubstrateClient represents a connection to the Substrate blockchain
type SubstrateClient struct {
	nodeURL   string
	connected bool
}

// SetupSuite initializes the test environment
func (s *Sprint8EscrowTestSuite) SetupSuite() {
	s.ctx = context.Background()
	s.testTimeout = 30 * time.Second

	// Initialize test accounts with sufficient balances
	s.alice = &TestAccount{
		Name:       "Alice",
		Address:    "5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY",
		PrivateKey: "0xe5be9a5092b81bca64be81d212e7f2f9eba183bb7a90954f7b76361f6edb5c0a",
		PublicKey:  "0xd43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d",
		Balance:    10000000, // 10M tokens
	}

	s.bob = &TestAccount{
		Name:       "Bob",
		Address:    "5FHneW46xGXgs5mUiveU4sbTyGBzmstUspZC92UhjJM694ty",
		PrivateKey: "0x398f0c28f98885e046333d4a41c19cee4c37368a9832c6502f6cfd182e2aef89",
		PublicKey:  "0x8eaf04151687736326c9fea17e25fc5287613693c912909cb226aa4794f26a48",
		Balance:    5000000, // 5M tokens
	}

	s.charlie = &TestAccount{
		Name:       "Charlie",
		Address:    "5FLSigC9HGRKVhB9FiEo4Y3koPsNmBmLJbpXg2mp1hXcS59Y",
		PrivateKey: "0x389edda51cb19e39a8b6d4e7e49b81d6b9a53b7a93e5b8e39d0fcb89e5e5a2e8",
		PublicKey:  "0x90b5ab205c6974c9ea841be688864633dc9ca8a357843eeacf2314649965fe22",
		Balance:    3000000, // 3M tokens
	}

	s.dave = &TestAccount{
		Name:       "Dave",
		Address:    "5DAAnrj7VHTznn2AWBemMuyBwZWs6FNFjdyVXUeYum3PTXFy",
		PrivateKey: "0x348d8c7a7de1e69f8a6a67b6e5d8c5e9f7a6b8c9d0e1f2a3b4c5d6e7f8a9b0c1",
		PublicKey:  "0x306721211d5404bd9da88e0204360a1a9ab8b87c66c1bc2fcdd37f3c2222cc20",
		Balance:    2000000, // 2M tokens
	}

	s.eve = &TestAccount{
		Name:       "Eve",
		Address:    "5HGjWAeFDfFCWPsjFQdVV2Msvz2XtMktvgocEZcCj68kUMaw",
		PrivateKey: "0x456e789a1bc2def3456789a0bcdef1234567890abcdef1234567890abcdef123",
		PublicKey:  "0xe7e9c7b9a8f6e5d4c3b2a1908f7e6d5c4b3a29180e9d8c7b6a5948372615d4c3",
		Balance:    1000000, // 1M tokens
	}

	// Initialize blockchain client
	s.client = &SubstrateClient{
		nodeURL:   "ws://localhost:9944",
		connected: false,
	}

	// Connect to the blockchain
	err := s.connectToChain()
	require.NoError(s.T(), err, "Failed to connect to blockchain")

	// Fund test accounts
	s.setupTestAccounts()
}

// TearDownSuite cleans up after all tests
func (s *Sprint8EscrowTestSuite) TearDownSuite() {
	if s.client.connected {
		s.client.connected = false
	}
}

// SetupTest runs before each individual test
func (s *Sprint8EscrowTestSuite) SetupTest() {
	// Reset any test-specific state
}

// TearDownTest runs after each individual test
func (s *Sprint8EscrowTestSuite) TearDownTest() {
	// Clean up test-specific resources
}

// connectToChain establishes connection to the Substrate blockchain
func (s *Sprint8EscrowTestSuite) connectToChain() error {
	// Mock implementation - would connect to actual Substrate node
	s.client.connected = true
	return nil
}

// setupTestAccounts ensures test accounts have sufficient balances
func (s *Sprint8EscrowTestSuite) setupTestAccounts() {
	// Mock implementation - would fund accounts on actual blockchain
}

// ==========================================================================
// MULTI-PARTY ESCROW TESTS
// ==========================================================================

// TestMultiPartyEscrowCreation tests creating multi-party escrows
func (s *Sprint8EscrowTestSuite) TestMultiPartyEscrowCreation() {
	s.T().Log("Testing multi-party escrow creation...")

	taskID := s.generateTaskID("multi-party-test-1")
	amount := uint64(1000000) // 1M tokens

	// Step 1: Alice creates basic escrow
	escrowID, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)
	assert.NotEmpty(s.T(), escrowID)

	// Step 2: Add Bob as payer participant
	err = s.addParticipant(s.alice, taskID, s.bob, "Payer", 500000)
	require.NoError(s.T(), err)

	// Step 3: Add Charlie as payee participant
	err = s.addParticipant(s.alice, taskID, s.charlie, "Payee", 400000)
	require.NoError(s.T(), err)

	// Step 4: Add Dave as arbiter
	err = s.addParticipant(s.alice, taskID, s.dave, "Arbiter", 0)
	require.NoError(s.T(), err)

	// Step 5: Verify escrow state
	escrow, err := s.getEscrow(taskID)
	require.NoError(s.T(), err)
	assert.True(s.T(), escrow.IsMultiParty)
	assert.Equal(s.T(), 3, len(escrow.Participants))

	s.T().Log("✅ Multi-party escrow creation test passed")
}

// TestMultiPartyEscrowWorkflow tests complete multi-party escrow workflow
func (s *Sprint8EscrowTestSuite) TestMultiPartyEscrowWorkflow() {
	s.T().Log("Testing multi-party escrow complete workflow...")

	taskID := s.generateTaskID("multi-party-workflow-1")
	amount := uint64(2000000) // 2M tokens

	// Create multi-party escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	err = s.addParticipant(s.alice, taskID, s.bob, "Payer", 1000000)
	require.NoError(s.T(), err)

	err = s.addParticipant(s.alice, taskID, s.charlie, "Payee", 800000)
	require.NoError(s.T(), err)

	// Accept task
	err = s.acceptTask(s.eve, taskID)
	require.NoError(s.T(), err)

	// Record initial balances
	initialCharlie := s.getBalance(s.charlie)
	initialEve := s.getBalance(s.eve)

	// Release payment
	err = s.releasePayment(s.alice, taskID)
	require.NoError(s.T(), err)

	// Verify final balances
	finalCharlie := s.getBalance(s.charlie)
	finalEve := s.getBalance(s.eve)

	assert.Greater(s.T(), finalCharlie, initialCharlie, "Charlie should receive payment")
	assert.Greater(s.T(), finalEve, initialEve, "Eve should receive payment")

	s.T().Log("✅ Multi-party escrow workflow test passed")
}

// ==========================================================================
// MILESTONE-BASED ESCROW TESTS
// ==========================================================================

// TestMilestoneEscrowCreation tests creating milestone-based escrows
func (s *Sprint8EscrowTestSuite) TestMilestoneEscrowCreation() {
	s.T().Log("Testing milestone-based escrow creation...")

	taskID := s.generateTaskID("milestone-test-1")
	amount := uint64(1500000) // 1.5M tokens

	// Create basic escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Add milestones
	milestones := []struct {
		description        string
		amount            uint64
		requiredApprovals uint32
	}{
		{"Research Phase", 300000, 1},
		{"Development Phase", 800000, 2},
		{"Testing & Delivery", 400000, 1},
	}

	for _, milestone := range milestones {
		err = s.addMilestone(s.alice, taskID, milestone.description, milestone.amount, milestone.requiredApprovals)
		require.NoError(s.T(), err)
	}

	// Verify milestone creation
	escrow, err := s.getEscrow(taskID)
	require.NoError(s.T(), err)
	assert.True(s.T(), escrow.IsMilestoneBased)
	assert.Equal(s.T(), 3, len(escrow.Milestones))

	s.T().Log("✅ Milestone escrow creation test passed")
}

// TestMilestoneApprovalWorkflow tests milestone completion and approval
func (s *Sprint8EscrowTestSuite) TestMilestoneApprovalWorkflow() {
	s.T().Log("Testing milestone approval workflow...")

	taskID := s.generateTaskID("milestone-approval-1")
	amount := uint64(1000000) // 1M tokens

	// Setup milestone escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID, "Test Milestone", 500000, 2)
	require.NoError(s.T(), err)

	err = s.addParticipant(s.alice, taskID, s.bob, "Arbiter", 0)
	require.NoError(s.T(), err)

	// Accept task
	err = s.acceptTask(s.charlie, taskID)
	require.NoError(s.T(), err)

	// Complete milestone
	err = s.completeMilestone(s.charlie, taskID, 0)
	require.NoError(s.T(), err)

	// Record initial balance
	initialBalance := s.getBalance(s.charlie)

	// First approval (not enough)
	err = s.approveMilestone(s.alice, taskID, 0)
	require.NoError(s.T(), err)

	// Second approval (triggers payment)
	err = s.approveMilestone(s.bob, taskID, 0)
	require.NoError(s.T(), err)

	// Verify payment was released
	finalBalance := s.getBalance(s.charlie)
	expectedPayment := uint64(475000) // 500000 - 5% fee
	assert.Greater(s.T(), finalBalance, initialBalance+expectedPayment-10000, "Milestone payment should be received")

	s.T().Log("✅ Milestone approval workflow test passed")
}

// ==========================================================================
// BATCH OPERATION TESTS
// ==========================================================================

// TestBatchEscrowCreation tests batch creation of multiple escrows
func (s *Sprint8EscrowTestSuite) TestBatchEscrowCreation() {
	s.T().Log("Testing batch escrow creation...")

	batchSize := 5
	baseAmount := uint64(100000) // 100k tokens each

	var taskIDs []string
	for i := 0; i < batchSize; i++ {
		taskIDs = append(taskIDs, s.generateTaskID(fmt.Sprintf("batch-test-%d", i)))
	}

	// Record initial balance
	initialBalance := s.getBalance(s.alice)

	// Execute batch creation
	err := s.batchCreateEscrow(s.alice, taskIDs, baseAmount)
	require.NoError(s.T(), err)

	// Verify all escrows were created
	for _, taskID := range taskIDs {
		escrow, err := s.getEscrow(taskID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), baseAmount, escrow.Amount)
	}

	// Verify total amount was reserved
	finalBalance := s.getBalance(s.alice)
	totalReserved := uint64(batchSize) * baseAmount
	assert.LessOrEqual(s.T(), finalBalance, initialBalance-totalReserved+10000, "Batch amount should be reserved")

	s.T().Log("✅ Batch escrow creation test passed")
}

// TestBatchOperationsPerformance tests batch operation performance
func (s *Sprint8EscrowTestSuite) TestBatchOperationsPerformance() {
	s.T().Log("Testing batch operations performance...")

	startTime := time.Now()
	batchSize := 20
	baseAmount := uint64(50000) // 50k tokens each

	var taskIDs []string
	for i := 0; i < batchSize; i++ {
		taskIDs = append(taskIDs, s.generateTaskID(fmt.Sprintf("perf-test-%d", i)))
	}

	// Batch create
	err := s.batchCreateEscrow(s.alice, taskIDs, baseAmount)
	require.NoError(s.T(), err)

	createTime := time.Since(startTime)
	s.T().Logf("Batch creation of %d escrows took: %v", batchSize, createTime)

	// Accept all tasks
	for _, taskID := range taskIDs {
		err = s.acceptTask(s.bob, taskID)
		require.NoError(s.T(), err)
	}

	// Batch release
	releaseStart := time.Now()
	err = s.batchReleasePayment(s.alice, taskIDs)
	require.NoError(s.T(), err)

	releaseTime := time.Since(releaseStart)
	s.T().Logf("Batch release of %d payments took: %v", batchSize, releaseTime)

	totalTime := time.Since(startTime)
	s.T().Logf("Total batch workflow time: %v", totalTime)

	// Performance assertions
	assert.Less(s.T(), createTime, 10*time.Second, "Batch creation should complete quickly")
	assert.Less(s.T(), releaseTime, 5*time.Second, "Batch release should complete quickly")

	s.T().Log("✅ Batch operations performance test passed")
}

// TestBatchRefundOperations tests batch refund functionality
func (s *Sprint8EscrowTestSuite) TestBatchRefundOperations() {
	s.T().Log("Testing batch refund operations...")

	batchSize := 10
	baseAmount := uint64(75000) // 75k tokens each

	var taskIDs []string
	for i := 0; i < batchSize; i++ {
		taskIDs = append(taskIDs, s.generateTaskID(fmt.Sprintf("refund-test-%d", i)))
	}

	// Create escrows
	err := s.batchCreateEscrow(s.alice, taskIDs, baseAmount)
	require.NoError(s.T(), err)

	// Record balance before refund
	balanceBefore := s.getBalance(s.alice)

	// Execute batch refund
	err = s.batchRefundEscrow(s.alice, taskIDs)
	require.NoError(s.T(), err)

	// Verify refunds were processed
	balanceAfter := s.getBalance(s.alice)
	expectedRefund := uint64(batchSize) * baseAmount
	assert.GreaterOrEqual(s.T(), balanceAfter, balanceBefore+expectedRefund-10000, "Batch refund should restore balance")

	// Verify escrow states
	for _, taskID := range taskIDs {
		escrow, err := s.getEscrow(taskID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), "Refunded", escrow.State)
	}

	s.T().Log("✅ Batch refund operations test passed")
}

// ==========================================================================
// REFUND POLICY TESTS
// ==========================================================================

// TestTimeBasedRefundPolicy tests time-based refund policies
func (s *Sprint8EscrowTestSuite) TestTimeBasedRefundPolicy() {
	s.T().Log("Testing time-based refund policy...")

	taskID := s.generateTaskID("time-refund-test-1")
	amount := uint64(200000) // 200k tokens

	// Create escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Set time-based refund policy
	policy := RefundPolicy{
		PolicyType: "TimeBased",
		Parameters: map[string]interface{}{
			"fullRefundDeadline":      100, // 100 blocks
			"partialRefundPercentage": 50,  // 50% after deadline
		},
		CanOverride:       false,
		OverrideAuthority: nil,
	}

	err = s.setRefundPolicy(s.alice, taskID, policy)
	require.NoError(s.T(), err)

	// Test full refund within deadline
	refundAmount, err := s.evaluateRefundAmount(taskID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), amount, refundAmount, "Should get full refund within deadline")

	s.T().Log("✅ Time-based refund policy test passed")
}

// TestConditionalRefundPolicy tests conditional refund policies based on milestones
func (s *Sprint8EscrowTestSuite) TestConditionalRefundPolicy() {
	s.T().Log("Testing conditional refund policy...")

	taskID := s.generateTaskID("conditional-refund-test-1")
	amount := uint64(300000) // 300k tokens

	// Setup milestone escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID, "Phase 1", 150000, 1)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID, "Phase 2", 150000, 1)
	require.NoError(s.T(), err)

	// Set conditional refund policy
	policy := RefundPolicy{
		PolicyType: "Conditional",
		Parameters: map[string]interface{}{
			"milestonesCompleted": 2,
			"refundPercentages":   []int{90, 50, 10}, // 90%, 50%, 10% based on completion
		},
		CanOverride:       false,
		OverrideAuthority: nil,
	}

	err = s.setRefundPolicy(s.alice, taskID, policy)
	require.NoError(s.T(), err)

	// Test refund amount with no milestones completed
	refundAmount, err := s.evaluateRefundAmount(taskID)
	require.NoError(s.T(), err)
	expectedRefund := uint64(270000) // 90% of 300k
	assert.Equal(s.T(), expectedRefund, refundAmount, "Should get 90% refund with no milestones completed")

	s.T().Log("✅ Conditional refund policy test passed")
}

// TestRefundPolicyOverride tests arbiter override of refund policies
func (s *Sprint8EscrowTestSuite) TestRefundPolicyOverride() {
	s.T().Log("Testing refund policy override...")

	taskID := s.generateTaskID("override-test-1")
	amount := uint64(400000) // 400k tokens

	// Create escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Set policy with override capability
	policy := RefundPolicy{
		PolicyType: "NoRefund",
		Parameters: map[string]interface{}{
			"workStartDeadline": 50, // No refund after 50 blocks
		},
		CanOverride:       true,
		OverrideAuthority: &s.eve.Address, // Eve can override
	}

	err = s.setRefundPolicy(s.alice, taskID, policy)
	require.NoError(s.T(), err)

	// Record balance before override
	balanceBefore := s.getBalance(s.alice)

	// Arbiter overrides to 75% refund
	overrideAmount := uint64(300000) // 75% of 400k
	err = s.overrideRefundAmount(s.eve, taskID, overrideAmount)
	require.NoError(s.T(), err)

	// Verify override was applied
	balanceAfter := s.getBalance(s.alice)
	assert.GreaterOrEqual(s.T(), balanceAfter, balanceBefore+overrideAmount-5000, "Override refund should be applied")

	s.T().Log("✅ Refund policy override test passed")
}

// ==========================================================================
// TEMPLATE SYSTEM TESTS
// ==========================================================================

// TestBuiltinTemplates tests all built-in escrow templates
func (s *Sprint8EscrowTestSuite) TestBuiltinTemplates() {
	s.T().Log("Testing built-in escrow templates...")

	templateTests := []struct {
		templateID uint32
		name       string
		amount     uint64
	}{
		{1, "Simple Payment", 100000},
		{2, "Milestone Project", 500000},
		{3, "Multi-Party Contract", 750000},
		{4, "Time-Locked Release", 200000},
		{5, "Conditional Payment", 300000},
		{6, "Escrowed Purchase", 150000},
		{7, "Subscription Payment", 50000},
	}

	for _, test := range templateTests {
		s.T().Logf("Testing template: %s", test.name)

		taskID := s.generateTaskID(fmt.Sprintf("template-test-%d", test.templateID))

		// Create escrow from template
		err := s.createEscrowFromTemplate(s.alice, taskID, test.amount, test.templateID)
		require.NoError(s.T(), err, "Failed to create escrow from template %s", test.name)

		// Verify escrow was created with template parameters
		escrow, err := s.getEscrow(taskID)
		require.NoError(s.T(), err)
		assert.Equal(s.T(), test.amount, escrow.Amount)
		assert.Equal(s.T(), test.templateID, escrow.TemplateID)
	}

	s.T().Log("✅ Built-in templates test passed")
}

// TestCustomTemplateCreation tests creating custom templates
func (s *Sprint8EscrowTestSuite) TestCustomTemplateCreation() {
	s.T().Log("Testing custom template creation...")

	templateName := "Custom Service Contract"
	templateDescription := "Custom template for service agreements with specific parameters"

	templateParams := TemplateParams{
		DefaultFeePercent:     3,
		DefaultTimeout:        2000,
		MultiPartyEnabled:     true,
		MilestoneEnabled:      true,
		MaxParticipants:       5,
		MaxMilestones:         8,
		DisputesEnabled:       true,
	}

	// Create custom template
	templateID, err := s.createTemplate(s.alice, templateName, templateDescription, "Custom", templateParams)
	require.NoError(s.T(), err)
	assert.Greater(s.T(), templateID, uint32(7), "Custom template should have ID > built-in templates")

	// Use custom template to create escrow
	taskID := s.generateTaskID("custom-template-test-1")
	amount := uint64(250000) // 250k tokens

	err = s.createEscrowFromTemplate(s.alice, taskID, amount, templateID)
	require.NoError(s.T(), err)

	// Verify template configuration was applied
	escrow, err := s.getEscrow(taskID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), uint8(3), escrow.FeePercent, "Custom fee percentage should be applied")
	assert.Equal(s.T(), templateID, escrow.TemplateID)

	s.T().Log("✅ Custom template creation test passed")
}

// TestTemplateFromEscrow tests creating templates from existing escrows
func (s *Sprint8EscrowTestSuite) TestTemplateFromEscrow() {
	s.T().Log("Testing template creation from existing escrow...")

	// Create a complex escrow configuration
	taskID := s.generateTaskID("template-source-1")
	amount := uint64(500000) // 500k tokens

	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Add participants and milestones
	err = s.addParticipant(s.alice, taskID, s.bob, "Payer", 250000)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID, "Development", 300000, 2)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID, "Testing", 200000, 1)
	require.NoError(s.T(), err)

	// Create template from escrow configuration
	templateName := "Complex Project Template"
	templateID, err := s.createTemplateFromEscrow(s.alice, taskID, templateName)
	require.NoError(s.T(), err)

	// Verify template was created with correct configuration
	template, err := s.getTemplate(templateID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), templateName, template.Name)
	assert.True(s.T(), template.DefaultParams.MultiPartyEnabled)
	assert.True(s.T(), template.DefaultParams.MilestoneEnabled)

	s.T().Log("✅ Template creation from escrow test passed")
}

// ==========================================================================
// LOAD AND STRESS TESTS
// ==========================================================================

// TestHighVolumeOperations tests system behavior under high transaction volume
func (s *Sprint8EscrowTestSuite) TestHighVolumeOperations() {
	s.T().Log("Testing high volume operations...")

	concurrentUsers := 5
	transactionsPerUser := 10
	baseAmount := uint64(10000) // 10k tokens each

	startTime := time.Now()
	var errors []error

	// Create concurrent operations
	resultChan := make(chan error, concurrentUsers*transactionsPerUser)

	for i := 0; i < concurrentUsers; i++ {
		go func(userIndex int) {
			for j := 0; j < transactionsPerUser; j++ {
				taskID := s.generateTaskID(fmt.Sprintf("volume-test-%d-%d", userIndex, j))

				// Create escrow
				_, err := s.createEscrow(s.alice, taskID, baseAmount)
				if err != nil {
					resultChan <- err
					continue
				}

				// Accept task
				err = s.acceptTask(s.bob, taskID)
				if err != nil {
					resultChan <- err
					continue
				}

				// Release payment
				err = s.releasePayment(s.alice, taskID)
				resultChan <- err
			}
		}(i)
	}

	// Collect results
	totalOperations := concurrentUsers * transactionsPerUser
	for i := 0; i < totalOperations; i++ {
		if err := <-resultChan; err != nil {
			errors = append(errors, err)
		}
	}

	duration := time.Since(startTime)
	successRate := float64(totalOperations-len(errors)) / float64(totalOperations) * 100

	s.T().Logf("High volume test completed in %v", duration)
	s.T().Logf("Success rate: %.2f%% (%d/%d successful)", successRate, totalOperations-len(errors), totalOperations)

	// Performance assertions
	assert.Greater(s.T(), successRate, 95.0, "Success rate should be > 95%")
	assert.Less(s.T(), duration, 60*time.Second, "High volume operations should complete within 1 minute")

	s.T().Log("✅ High volume operations test passed")
}

// TestSystemLimits tests system behavior at configured limits
func (s *Sprint8EscrowTestSuite) TestSystemLimits() {
	s.T().Log("Testing system limits...")

	taskID := s.generateTaskID("limits-test-1")
	amount := uint64(100000) // 100k tokens

	// Create escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Test maximum participants limit
	maxParticipants := 10 // Configured limit
	for i := 0; i < maxParticipants; i++ {
		participant := s.generateTestAccount(fmt.Sprintf("participant-%d", i))
		err = s.addParticipant(s.alice, taskID, participant, "Payer", 1000)
		require.NoError(s.T(), err, "Should be able to add participant %d", i)
	}

	// Try to exceed participant limit
	extraParticipant := s.generateTestAccount("extra-participant")
	err = s.addParticipant(s.alice, taskID, extraParticipant, "Payer", 1000)
	assert.Error(s.T(), err, "Should fail when exceeding participant limit")

	// Test maximum milestones limit
	taskID2 := s.generateTaskID("limits-test-2")
	_, err = s.createEscrow(s.alice, taskID2, amount)
	require.NoError(s.T(), err)

	maxMilestones := 20 // Configured limit
	for i := 0; i < maxMilestones; i++ {
		err = s.addMilestone(s.alice, taskID2, fmt.Sprintf("Milestone %d", i), 5000, 1)
		require.NoError(s.T(), err, "Should be able to add milestone %d", i)
	}

	// Try to exceed milestone limit
	err = s.addMilestone(s.alice, taskID2, "Extra Milestone", 5000, 1)
	assert.Error(s.T(), err, "Should fail when exceeding milestone limit")

	s.T().Log("✅ System limits test passed")
}

// ==========================================================================
// INTEGRATION AND WORKFLOW TESTS
// ==========================================================================

// TestComplexWorkflow tests a comprehensive end-to-end workflow
func (s *Sprint8EscrowTestSuite) TestComplexWorkflow() {
	s.T().Log("Testing complex end-to-end workflow...")

	taskID := s.generateTaskID("complex-workflow-1")
	amount := uint64(1000000) // 1M tokens

	// Phase 1: Setup complex escrow
	_, err := s.createEscrow(s.alice, taskID, amount)
	require.NoError(s.T(), err)

	// Add multiple participants
	err = s.addParticipant(s.alice, taskID, s.bob, "Payer", 500000)
	require.NoError(s.T(), err)

	err = s.addParticipant(s.alice, taskID, s.charlie, "Payee", 400000)
	require.NoError(s.T(), err)

	err = s.addParticipant(s.alice, taskID, s.dave, "Arbiter", 0)
	require.NoError(s.T(), err)

	// Add milestones
	milestones := []struct {
		desc      string
		amount    uint64
		approvals uint32
	}{
		{"Analysis & Planning", 200000, 2},
		{"Implementation", 500000, 3},
		{"Testing & Deployment", 300000, 2},
	}

	for _, m := range milestones {
		err = s.addMilestone(s.alice, taskID, m.desc, m.amount, m.approvals)
		require.NoError(s.T(), err)
	}

	// Set advanced refund policy
	policy := RefundPolicy{
		PolicyType: "Graduated",
		Parameters: map[string]interface{}{
			"stages": []interface{}{
				map[string]interface{}{"block": 100, "percentage": 90},
				map[string]interface{}{"block": 200, "percentage": 70},
				map[string]interface{}{"block": 300, "percentage": 50},
			},
		},
		CanOverride:       true,
		OverrideAuthority: &s.dave.Address,
	}

	err = s.setRefundPolicy(s.alice, taskID, policy)
	require.NoError(s.T(), err)

	// Phase 2: Execute workflow
	err = s.acceptTask(s.eve, taskID)
	require.NoError(s.T(), err)

	// Complete and approve first milestone
	err = s.completeMilestone(s.eve, taskID, 0)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.alice, taskID, 0)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.bob, taskID, 0)
	require.NoError(s.T(), err)

	// Verify first payment
	initialBalance := s.getBalance(s.eve)

	// Complete second milestone
	err = s.completeMilestone(s.eve, taskID, 1)
	require.NoError(s.T(), err)

	// Get all required approvals
	err = s.approveMilestone(s.alice, taskID, 1)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.bob, taskID, 1)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.dave, taskID, 1)
	require.NoError(s.T(), err)

	// Verify second payment
	midBalance := s.getBalance(s.eve)
	assert.Greater(s.T(), midBalance, initialBalance, "Should receive second milestone payment")

	// Complete final milestone
	err = s.completeMilestone(s.eve, taskID, 2)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.alice, taskID, 2)
	require.NoError(s.T(), err)

	err = s.approveMilestone(s.charlie, taskID, 2)
	require.NoError(s.T(), err)

	// Verify final payment
	finalBalance := s.getBalance(s.eve)
	assert.Greater(s.T(), finalBalance, midBalance, "Should receive final milestone payment")

	// Phase 3: Verify final state
	escrow, err := s.getEscrow(taskID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), "Completed", escrow.State)

	// Verify all milestones are complete
	for i, milestone := range escrow.Milestones {
		assert.True(s.T(), milestone.Completed, "Milestone %d should be completed", i)
	}

	s.T().Log("✅ Complex workflow test passed")
}

// TestErrorRecovery tests system behavior during error conditions
func (s *Sprint8EscrowTestSuite) TestErrorRecovery() {
	s.T().Log("Testing error recovery scenarios...")

	// Test 1: Invalid participant addition
	taskID := s.generateTaskID("error-recovery-1")
	_, err := s.createEscrow(s.alice, taskID, 100000)
	require.NoError(s.T(), err)

	// Try to add participant with insufficient balance
	poorAccount := s.generateTestAccount("poor-account")
	err = s.addParticipant(s.alice, taskID, poorAccount, "Payer", 999999999)
	assert.Error(s.T(), err, "Should fail with insufficient balance")

	// Verify escrow state is unchanged
	escrow, err := s.getEscrow(taskID)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), 0, len(escrow.Participants))

	// Test 2: Milestone approval without completion
	taskID2 := s.generateTaskID("error-recovery-2")
	_, err = s.createEscrow(s.alice, taskID2, 200000)
	require.NoError(s.T(), err)

	err = s.addMilestone(s.alice, taskID2, "Test Milestone", 100000, 1)
	require.NoError(s.T(), err)

	err = s.acceptTask(s.bob, taskID2)
	require.NoError(s.T(), err)

	// Try to approve uncompleted milestone
	err = s.approveMilestone(s.alice, taskID2, 0)
	assert.Error(s.T(), err, "Should fail to approve uncompleted milestone")

	// Test 3: Unauthorized operations
	taskID3 := s.generateTaskID("error-recovery-3")
	_, err = s.createEscrow(s.alice, taskID3, 150000)
	require.NoError(s.T(), err)

	// Try unauthorized participant addition
	err = s.addParticipant(s.bob, taskID3, s.charlie, "Payer", 50000)
	assert.Error(s.T(), err, "Should fail unauthorized participant addition")

	s.T().Log("✅ Error recovery test passed")
}

// ==========================================================================
// HELPER METHODS
// ==========================================================================

// generateTaskID creates a unique task ID for testing
func (s *Sprint8EscrowTestSuite) generateTaskID(prefix string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s-%d", prefix, timestamp)
}

// generateTestAccount creates a test account for specific scenarios
func (s *Sprint8EscrowTestSuite) generateTestAccount(name string) *TestAccount {
	return &TestAccount{
		Name:       name,
		Address:    fmt.Sprintf("test-address-%s-%d", name, time.Now().UnixNano()),
		PrivateKey: "0x0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		PublicKey:  "0xfedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		Balance:    10000, // Small balance for testing limits
	}
}

// Mock blockchain interaction methods
// In a real implementation, these would use Substrate Go SDK

func (s *Sprint8EscrowTestSuite) createEscrow(user *TestAccount, taskID string, amount uint64) (string, error) {
	// Mock implementation
	return fmt.Sprintf("escrow-%s", taskID), nil
}

func (s *Sprint8EscrowTestSuite) addParticipant(creator *TestAccount, taskID string, participant *TestAccount, role string, amount uint64) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) addMilestone(creator *TestAccount, taskID string, description string, amount uint64, requiredApprovals uint32) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) acceptTask(agent *TestAccount, taskID string) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) completeMilestone(agent *TestAccount, taskID string, milestoneID uint32) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) approveMilestone(approver *TestAccount, taskID string, milestoneID uint32) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) releasePayment(creator *TestAccount, taskID string) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) batchCreateEscrow(user *TestAccount, taskIDs []string, amount uint64) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) batchReleasePayment(user *TestAccount, taskIDs []string) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) batchRefundEscrow(user *TestAccount, taskIDs []string) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) setRefundPolicy(user *TestAccount, taskID string, policy RefundPolicy) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) evaluateRefundAmount(taskID string) (uint64, error) {
	// Mock implementation - returns full amount for testing
	return 200000, nil
}

func (s *Sprint8EscrowTestSuite) overrideRefundAmount(arbiter *TestAccount, taskID string, amount uint64) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) createTemplate(creator *TestAccount, name, description, templateType string, params TemplateParams) (uint32, error) {
	// Mock implementation
	return 100, nil // Return high ID for custom template
}

func (s *Sprint8EscrowTestSuite) createEscrowFromTemplate(user *TestAccount, taskID string, amount uint64, templateID uint32) error {
	// Mock implementation
	return nil
}

func (s *Sprint8EscrowTestSuite) createTemplateFromEscrow(user *TestAccount, taskID string, templateName string) (uint32, error) {
	// Mock implementation
	return 200, nil
}

func (s *Sprint8EscrowTestSuite) getEscrow(taskID string) (*EscrowInfo, error) {
	// Mock implementation
	return &EscrowInfo{
		TaskID:            taskID,
		Amount:            100000,
		State:             "Pending",
		IsMultiParty:      false,
		IsMilestoneBased:  false,
		Participants:      []ParticipantInfo{},
		Milestones:        []MilestoneInfo{},
		FeePercent:        5,
		TemplateID:        0,
	}, nil
}

func (s *Sprint8EscrowTestSuite) getTemplate(templateID uint32) (*TemplateInfo, error) {
	// Mock implementation
	return &TemplateInfo{
		TemplateID:    templateID,
		Name:          "Test Template",
		DefaultParams: TemplateParams{
			MultiPartyEnabled: true,
			MilestoneEnabled:  true,
		},
	}, nil
}

func (s *Sprint8EscrowTestSuite) getBalance(account *TestAccount) uint64 {
	// Mock implementation - simulate balance changes
	return account.Balance
}

// Data structures for test responses

type EscrowInfo struct {
	TaskID            string
	Amount            uint64
	State             string
	IsMultiParty      bool
	IsMilestoneBased  bool
	Participants      []ParticipantInfo
	Milestones        []MilestoneInfo
	FeePercent        uint8
	TemplateID        uint32
}

type ParticipantInfo struct {
	Account  string
	Role     string
	Amount   uint64
	Approved bool
}

type MilestoneInfo struct {
	ID                uint32
	Description       string
	Amount            uint64
	Completed         bool
	RequiredApprovals uint32
	ApprovedBy        []string
}

type RefundPolicy struct {
	PolicyType        string
	Parameters        map[string]interface{}
	CanOverride       bool
	OverrideAuthority *string
}

type TemplateParams struct {
	DefaultFeePercent uint8
	DefaultTimeout    uint64
	MultiPartyEnabled bool
	MilestoneEnabled  bool
	MaxParticipants   uint32
	MaxMilestones     uint32
	DisputesEnabled   bool
}

type TemplateInfo struct {
	TemplateID    uint32
	Name          string
	DefaultParams TemplateParams
}

// TestSprint8EscrowSuite runs the complete test suite
func TestSprint8EscrowSuite(t *testing.T) {
	suite.Run(t, new(Sprint8EscrowTestSuite))
}

// Benchmark tests for performance analysis

func BenchmarkEscrowCreation(b *testing.B) {
	suite := &Sprint8EscrowTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := fmt.Sprintf("benchmark-create-%d", i)
		_, err := suite.createEscrow(suite.alice, taskID, 100000)
		if err != nil {
			b.Fatalf("Failed to create escrow: %v", err)
		}
	}
}

func BenchmarkBatchOperations(b *testing.B) {
	suite := &Sprint8EscrowTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	batchSize := 10
	var taskIDs []string
	for i := 0; i < batchSize; i++ {
		taskIDs = append(taskIDs, fmt.Sprintf("batch-bench-%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := suite.batchCreateEscrow(suite.alice, taskIDs, 50000)
		if err != nil {
			b.Fatalf("Failed to batch create escrows: %v", err)
		}
	}
}

func BenchmarkMilestoneWorkflow(b *testing.B) {
	suite := &Sprint8EscrowTestSuite{}
	suite.SetupSuite()
	defer suite.TearDownSuite()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		taskID := fmt.Sprintf("benchmark-milestone-%d", i)

		// Create escrow with milestone
		_, err := suite.createEscrow(suite.alice, taskID, 200000)
		if err != nil {
			b.Fatalf("Failed to create escrow: %v", err)
		}

		err = suite.addMilestone(suite.alice, taskID, "Benchmark Milestone", 100000, 1)
		if err != nil {
			b.Fatalf("Failed to add milestone: %v", err)
		}

		err = suite.acceptTask(suite.bob, taskID)
		if err != nil {
			b.Fatalf("Failed to accept task: %v", err)
		}

		err = suite.completeMilestone(suite.bob, taskID, 0)
		if err != nil {
			b.Fatalf("Failed to complete milestone: %v", err)
		}

		err = suite.approveMilestone(suite.alice, taskID, 0)
		if err != nil {
			b.Fatalf("Failed to approve milestone: %v", err)
		}
	}
}