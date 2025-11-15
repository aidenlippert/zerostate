package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/aidenlippert/zerostate/libs/substrate"
)

func TestEscrowClientMethods(t *testing.T) {
	logger := zaptest.NewLogger(t)
	mockRPCClient := &MockRPCClient{}

	// Create escrow client with mock RPC client
	escrowClient := substrate.NewEscrowClient(mockRPCClient, logger)
	require.NotNil(t, escrowClient)

	ctx := context.Background()
	testTaskID := [32]byte{0x01, 0x02, 0x03}

	t.Run("TestMultiPartyMethods", func(t *testing.T) {
		// Test AddParticipant
		err := escrowClient.AddParticipant(
			ctx,
			testTaskID,
			substrate.AccountID{0x11, 0x22, 0x33},
			substrate.ParticipantRoleAgent,
			1000000000, // 10 tokens
		)
		assert.NoError(t, err)

		// Test RemoveParticipant
		err = escrowClient.RemoveParticipant(
			ctx,
			testTaskID,
			substrate.AccountID{0x11, 0x22, 0x33},
		)
		assert.NoError(t, err)

		// Test ApproveMultiParty
		err = escrowClient.ApproveMultiParty(
			ctx,
			testTaskID,
			substrate.AccountID{0x44, 0x55, 0x66},
		)
		assert.NoError(t, err)
	})

	t.Run("TestMilestoneMethods", func(t *testing.T) {
		milestone := substrate.Milestone{
			Name:              "Test Milestone",
			Description:       "Testing milestone functionality",
			Amount:            substrate.Balance("500000000"), // 5 tokens
			RequiredApprovals: 2,
			Status:            substrate.MilestoneStatusCreated,
			CreatedAt:         substrate.BlockNumber(100),
		}

		// Test AddMilestone
		err := escrowClient.AddMilestone(ctx, testTaskID, milestone)
		assert.NoError(t, err)

		// Test CompleteMilestone
		err = escrowClient.CompleteMilestone(ctx, testTaskID, 0, "completion evidence")
		assert.NoError(t, err)

		// Test ApproveMilestone
		err = escrowClient.ApproveMilestone(ctx, testTaskID, 0, "approval evidence")
		assert.NoError(t, err)
	})

	t.Run("TestBatchMethods", func(t *testing.T) {
		// Test BatchCreateEscrow
		requests := []substrate.BatchCreateEscrowRequest{
			{
				TaskID:   [32]byte{0x10, 0x20, 0x30},
				Amount:   1000000000,
				TaskHash: [32]byte{0xaa, 0xbb, 0xcc},
			},
			{
				TaskID:   [32]byte{0x40, 0x50, 0x60},
				Amount:   2000000000,
				TaskHash: [32]byte{0xdd, 0xee, 0xff},
			},
		}

		result, err := escrowClient.BatchCreateEscrow(ctx, requests)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, uint32(len(requests)), result.TotalProcessed)

		// Test BatchReleasePayment
		taskIDs := [][32]byte{
			{0x10, 0x20, 0x30},
			{0x40, 0x50, 0x60},
		}

		err = escrowClient.BatchReleasePayment(ctx, taskIDs)
		assert.NoError(t, err)

		// Test BatchRefundEscrow
		err = escrowClient.BatchRefundEscrow(ctx, taskIDs)
		assert.NoError(t, err)

		// Test BatchDisputeEscrow
		err = escrowClient.BatchDisputeEscrow(ctx, taskIDs)
		assert.NoError(t, err)
	})

	t.Run("TestRefundPolicyMethods", func(t *testing.T) {
		// Test SetRefundPolicy
		policy := substrate.RefundPolicy{
			PolicyType:    substrate.RefundPolicyTypeExponential,
			InitialRefund: 95,
			FinalRefund:   5,
			DecayBlocks:   200,
			Steps: []substrate.RefundStep{
				{Threshold: 50, RefundPercentage: 75},
				{Threshold: 100, RefundPercentage: 50},
				{Threshold: 150, RefundPercentage: 25},
			},
		}

		err := escrowClient.SetRefundPolicy(ctx, testTaskID, policy)
		assert.NoError(t, err)

		// Test GetRefundPolicy
		retrievedPolicy, err := escrowClient.GetRefundPolicy(ctx, testTaskID)
		assert.NoError(t, err)
		assert.Equal(t, policy.PolicyType, retrievedPolicy.PolicyType)
		assert.Equal(t, policy.InitialRefund, retrievedPolicy.InitialRefund)

		// Test CalculateRefund
		atTime := substrate.BlockNumber(125)
		calculation, err := escrowClient.CalculateRefund(ctx, testTaskID, &atTime)
		assert.NoError(t, err)
		assert.NotNil(t, calculation)
		assert.Greater(t, calculation.RefundAmount.Uint64(), uint64(0))

		// Test ProcessRefundWithPolicy
		err = escrowClient.ProcessRefundWithPolicy(ctx, testTaskID, &atTime)
		assert.NoError(t, err)
	})

	t.Run("TestTemplateMethods", func(t *testing.T) {
		// Test CreateTemplate
		template := substrate.EscrowTemplate{
			TemplateType:        substrate.EscrowTemplateTypeMultiParty,
			Name:                "Multi-Party Classification Template",
			Description:         "Template for multi-party AI classification tasks",
			TaskType:            substrate.Bytes("ai-classification"),
			DefaultAmount:       substrate.Balance("5000000000"), // 50 tokens
			DefaultTimeout:      substrate.BlockNumber(1000),     // 1000 blocks
			RequiredVotes:       3,
			DefaultParticipants: []substrate.AccountID{{0x11}, {0x22}, {0x33}},
			Milestones: []substrate.Milestone{
				{
					Name:              "Data Preparation",
					Amount:            substrate.Balance("1500000000"),
					RequiredApprovals: 1,
					Status:            substrate.MilestoneStatusCreated,
				},
				{
					Name:              "Model Training",
					Amount:            substrate.Balance("2500000000"),
					RequiredApprovals: 2,
					Status:            substrate.MilestoneStatusCreated,
				},
				{
					Name:              "Results Validation",
					Amount:            substrate.Balance("1000000000"),
					RequiredApprovals: 3,
					Status:            substrate.MilestoneStatusCreated,
				},
			},
		}

		templateID := [32]byte{0xaa, 0xbb, 0xcc, 0xdd}
		err := escrowClient.CreateTemplate(ctx, templateID, template)
		assert.NoError(t, err)

		// Test GetTemplate
		retrievedTemplate, err := escrowClient.GetTemplate(ctx, templateID)
		assert.NoError(t, err)
		assert.Equal(t, template.Name, retrievedTemplate.Name)
		assert.Equal(t, template.TemplateType, retrievedTemplate.TemplateType)

		// Test ListTemplates
		templates, err := escrowClient.ListTemplates(ctx, 10, 0)
		assert.NoError(t, err)
		assert.NotNil(t, templates)

		// Test CreateEscrowFromTemplate
		newTaskID := [32]byte{0xee, 0xff, 0x00, 0x11}
		hash, err := escrowClient.CreateEscrowFromTemplate(ctx, newTaskID, templateID, 5000000000)
		assert.NoError(t, err)
		assert.NotEmpty(t, hash)
	})

	t.Run("TestExtendedQueryMethods", func(t *testing.T) {
		// Test GetExtendedEscrowDetails
		details, err := escrowClient.GetExtendedEscrowDetails(ctx, testTaskID)
		assert.NoError(t, err)
		assert.NotNil(t, details)

		// Test GetEscrowStats
		stats, err := escrowClient.GetEscrowStats(ctx, testTaskID)
		assert.NoError(t, err)
		assert.NotNil(t, stats)
	})
}

func TestEscrowTypes(t *testing.T) {
	t.Run("TestParticipantRole", func(t *testing.T) {
		role := substrate.ParticipantRoleAgent
		assert.Equal(t, "Agent", role.String())

		role = substrate.ParticipantRoleValidator
		assert.Equal(t, "Validator", role.String())
	})

	t.Run("TestMilestoneStatus", func(t *testing.T) {
		status := substrate.MilestoneStatusCreated
		assert.Equal(t, "Created", status.String())

		status = substrate.MilestoneStatusCompleted
		assert.Equal(t, "Completed", status.String())
	})

	t.Run("TestRefundPolicyType", func(t *testing.T) {
		policyType := substrate.RefundPolicyTypeLinear
		assert.Equal(t, "Linear", policyType.String())

		policyType = substrate.RefundPolicyTypeStepwise
		assert.Equal(t, "Stepwise", policyType.String())
	})

	t.Run("TestEscrowTemplateType", func(t *testing.T) {
		templateType := substrate.EscrowTemplateTypeSimple
		assert.Equal(t, "Simple", templateType.String())

		templateType = substrate.EscrowTemplateTypeHybrid
		assert.Equal(t, "Hybrid", templateType.String())
	})
}

func TestEscrowCalculations(t *testing.T) {
	t.Run("TestLinearRefundCalculation", func(t *testing.T) {
		// Simulate linear refund calculation
		initialAmount := uint64(10000000000) // 100 tokens
		elapsedBlocks := uint64(50)
		totalBlocks := uint64(100)
		initialRefund := uint32(90)
		finalRefund := uint32(10)

		// Linear calculation: refund = initialRefund - (elapsed/total) * (initialRefund - finalRefund)
		progress := float64(elapsedBlocks) / float64(totalBlocks)
		refundPercentage := float64(initialRefund) - progress*float64(initialRefund-finalRefund)
		expectedRefund := uint64(float64(initialAmount) * refundPercentage / 100.0)

		assert.Equal(t, uint64(5000000000), expectedRefund) // 50 tokens (50% refund)
	})

	t.Run("TestStepwiseRefundCalculation", func(t *testing.T) {
		steps := []substrate.RefundStep{
			{Threshold: 25, RefundPercentage: 80},
			{Threshold: 50, RefundPercentage: 60},
			{Threshold: 75, RefundPercentage: 40},
			{Threshold: 100, RefundPercentage: 20},
		}

		elapsedBlocks := uint64(60)
		expectedRefund := uint32(60) // Should match the 50-75 range

		var actualRefund uint32
		for _, step := range steps {
			if elapsedBlocks >= uint64(step.Threshold) {
				actualRefund = step.RefundPercentage
			}
		}

		assert.Equal(t, expectedRefund, actualRefund)
	})
}

func BenchmarkEscrowOperations(b *testing.B) {
	logger := zaptest.NewLogger(b)
	mockRPCClient := &MockRPCClient{}
	escrowClient := substrate.NewEscrowClient(mockRPCClient, logger)

	ctx := context.Background()
	testTaskID := [32]byte{0x01, 0x02, 0x03}

	b.Run("BenchmarkCreateEscrow", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := escrowClient.CreateEscrow(ctx, testTaskID, 1000000000, [32]byte{}, nil)
			if err != nil {
				b.Fatalf("CreateEscrow failed: %v", err)
			}
		}
	})

	b.Run("BenchmarkAddParticipant", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := escrowClient.AddParticipant(ctx, testTaskID, substrate.AccountID{}, substrate.ParticipantRoleAgent, 1000000000)
			if err != nil {
				b.Fatalf("AddParticipant failed: %v", err)
			}
		}
	})

	b.Run("BenchmarkBatchCreateEscrow", func(b *testing.B) {
		requests := make([]substrate.BatchCreateEscrowRequest, 10)
		for i := range requests {
			requests[i] = substrate.BatchCreateEscrowRequest{
				TaskID: [32]byte{byte(i)},
				Amount: 1000000000,
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := escrowClient.BatchCreateEscrow(ctx, requests)
			if err != nil {
				b.Fatalf("BatchCreateEscrow failed: %v", err)
			}
		}
	})
}

// Mock RPC Client for unit tests
type MockRPCClient struct{}

func (m *MockRPCClient) GetMetadata() *substrate.Metadata {
	return &substrate.Metadata{
		Version: 14,
		// Add mock metadata fields as needed
	}
}

func (m *MockRPCClient) Call(method string, args ...interface{}) (interface{}, error) {
	// Mock successful calls
	switch method {
	case "chain_getBlockHash":
		return substrate.Hash{0x12, 0x34, 0x56, 0x78}, nil
	case "state_getStorage":
		return []byte{0x01, 0x02, 0x03, 0x04}, nil
	default:
		return nil, nil
	}
}

func (m *MockRPCClient) Subscribe(method string, args ...interface{}) (<-chan interface{}, error) {
	ch := make(chan interface{}, 1)
	close(ch) // Close immediately for tests
	return ch, nil
}