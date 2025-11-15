package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/aidenlippert/zerostate/libs/orchestration"
	"github.com/aidenlippert/zerostate/libs/substrate"
)

// TestEscrowIntegration tests all escrow functionality end-to-end
func TestEscrowIntegration(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	// Mock escrow client for testing
	mockClient := &MockEscrowClient{}

	// Create orchestrator with mock blockchain
	mockBlockchain := &MockBlockchainService{rpcClient: &MockRPCClient{}}
	orchestrator := createTestOrchestrator(ctx, logger, mockBlockchain, mockClient)

	t.Run("MultiPartyEscrowIntegration", func(t *testing.T) {
		testMultiPartyEscrowIntegration(t, ctx, orchestrator)
	})

	t.Run("MilestoneEscrowIntegration", func(t *testing.T) {
		testMilestoneEscrowIntegration(t, ctx, orchestrator)
	})

	t.Run("BatchOperationsIntegration", func(t *testing.T) {
		testBatchOperationsIntegration(t, ctx, orchestrator)
	})

	t.Run("TemplateEscrowIntegration", func(t *testing.T) {
		testTemplateEscrowIntegration(t, ctx, orchestrator)
	})

	t.Run("RefundPolicyIntegration", func(t *testing.T) {
		testRefundPolicyIntegration(t, ctx, orchestrator)
	})
}

func testMultiPartyEscrowIntegration(t *testing.T, ctx context.Context, orchestrator *orchestration.Orchestrator) {
	participants := []string{"did:agent:alice", "did:agent:bob", "did:validator:charlie"}
	requiredVotes := 2
	budget := 100.0

	// Create multi-party task
	task, err := orchestrator.CreateMultiPartyTask(
		ctx,
		"user123",
		"collaborative-task",
		[]string{"data-processing", "validation"},
		map[string]interface{}{"data": "test-dataset"},
		participants,
		requiredVotes,
		budget,
	)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "multi_party", task.EscrowType)
	assert.Equal(t, participants, task.Participants)
	assert.Equal(t, requiredVotes, task.RequiredVotes)
	assert.Equal(t, budget, task.Budget)
	assert.NotEmpty(t, task.EscrowTxHash)

	t.Logf("Multi-party task created: ID=%s, TxHash=%s", task.ID, task.EscrowTxHash)

	// Test adding participants (would normally be called during task execution)
	// This tests the underlying escrow client functionality
	taskIDBytes, err := convertTaskIDForTest(task.ID)
	require.NoError(t, err)

	// Simulate adding a participant to the escrow
	err = orchestrator.GetEscrowClient().AddParticipant(
		ctx,
		taskIDBytes,
		substrate.AccountID{}, // Mock account ID
		substrate.ParticipantRoleValidator,
		5000000000, // 50 tokens in smallest units
	)
	assert.NoError(t, err)

	t.Logf("Participant added successfully to multi-party escrow")
}

func testMilestoneEscrowIntegration(t *testing.T, ctx context.Context, orchestrator *orchestration.Orchestrator) {
	milestones := []orchestration.TaskMilestone{
		{
			ID:                "milestone-1",
			Name:              "Data Collection",
			Description:       "Collect and validate input data",
			Amount:            30.0,
			RequiredApprovals: 1,
			Status:            "created",
			Order:             0,
		},
		{
			ID:                "milestone-2",
			Name:              "Processing",
			Description:       "Process data according to specifications",
			Amount:            50.0,
			RequiredApprovals: 2,
			Status:            "created",
			Order:             1,
		},
		{
			ID:                "milestone-3",
			Name:              "Validation",
			Description:       "Validate results and generate report",
			Amount:            20.0,
			RequiredApprovals: 1,
			Status:            "created",
			Order:             2,
		},
	}

	budget := 100.0

	// Create milestone task
	task, err := orchestrator.CreateMilestoneTask(
		ctx,
		"user456",
		"milestone-task",
		[]string{"data-processing", "validation"},
		map[string]interface{}{"requirements": "detailed-analysis"},
		milestones,
		budget,
	)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, "milestone", task.EscrowType)
	assert.Len(t, task.Milestones, 3)
	assert.Equal(t, 0, task.CurrentMilestone)
	assert.Equal(t, budget, task.Budget)
	assert.NotEmpty(t, task.EscrowTxHash)

	t.Logf("Milestone task created: ID=%s, TxHash=%s", task.ID, task.EscrowTxHash)

	// Test milestone approval
	err = orchestrator.ApproveMilestone(ctx, task.ID, 0, "did:validator:alice", "evidence-hash-123")
	assert.NoError(t, err)

	// Verify milestone was approved
	updatedTask, err := orchestrator.GetQueue().GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", updatedTask.Milestones[0].Status)
	assert.Len(t, updatedTask.Milestones[0].Approvals, 1)
	assert.Equal(t, "did:validator:alice", updatedTask.Milestones[0].Approvals[0].ApproverDID)

	t.Logf("Milestone 0 approved successfully")

	// Test second milestone requiring multiple approvals
	err = orchestrator.ApproveMilestone(ctx, task.ID, 1, "did:validator:bob", "evidence-hash-456")
	assert.NoError(t, err)

	// Milestone should not be fully approved yet (requires 2 approvals)
	updatedTask, err = orchestrator.GetQueue().GetTask(task.ID)
	require.NoError(t, err)
	assert.NotEqual(t, "approved", updatedTask.Milestones[1].Status)
	assert.Len(t, updatedTask.Milestones[1].Approvals, 1)

	// Add second approval
	err = orchestrator.ApproveMilestone(ctx, task.ID, 1, "did:validator:charlie", "evidence-hash-789")
	assert.NoError(t, err)

	// Now milestone should be fully approved
	updatedTask, err = orchestrator.GetQueue().GetTask(task.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", updatedTask.Milestones[1].Status)
	assert.Len(t, updatedTask.Milestones[1].Approvals, 2)

	t.Logf("Milestone 1 fully approved after 2 approvals")
}

func testBatchOperationsIntegration(t *testing.T, ctx context.Context, orchestrator *orchestration.Orchestrator) {
	batchTasks := []orchestration.BatchTaskRequest{
		{
			Type:         "image-classification",
			Capabilities: []string{"image-processing"},
			Input:        map[string]interface{}{"image_url": "https://example.com/image1.jpg"},
			Budget:       25.0,
		},
		{
			Type:         "sentiment-analysis",
			Capabilities: []string{"text-processing"},
			Input:        map[string]interface{}{"text": "Customer feedback text"},
			Budget:       15.0,
		},
		{
			Type:         "data-transformation",
			Capabilities: []string{"data-processing"},
			Input:        map[string]interface{}{"format": "json-to-csv"},
			Budget:       20.0,
		},
	}

	// Create batch tasks
	result, err := orchestrator.CreateBatchTasks(ctx, "user789", batchTasks)
	require.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, len(batchTasks), result.TotalRequested)
	assert.Equal(t, len(batchTasks), result.TotalSucceeded)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.SuccessfulTasks, len(batchTasks))
	assert.Empty(t, result.FailedTasks)
	assert.NotEmpty(t, result.BatchID)
	assert.NotEmpty(t, result.TransactionHash)

	t.Logf("Batch tasks created: BatchID=%s, TxHash=%s, Succeeded=%d",
		result.BatchID, result.TransactionHash, result.TotalSucceeded)

	// Verify all tasks have correct batch properties
	for i, task := range result.SuccessfulTasks {
		assert.Equal(t, result.BatchID, task.BatchID)
		assert.True(t, task.IsBatchTask)
		assert.Equal(t, "simple", task.EscrowType)
		assert.Equal(t, batchTasks[i].Budget, task.Budget)
		assert.NotEmpty(t, task.EscrowTxHash)
	}

	t.Logf("All batch tasks properly configured")
}

func testTemplateEscrowIntegration(t *testing.T, ctx context.Context, orchestrator *orchestration.Orchestrator) {
	// Note: In a real test, the template would be created on the blockchain first
	// For this test, we'll mock the template retrieval

	templateID := "template-simple-classification"

	// Create task from template
	task, err := orchestrator.CreateTaskFromTemplate(
		ctx,
		"user101112",
		templateID,
		map[string]interface{}{"input_data": "classification-dataset"},
		50.0,
	)

	require.NoError(t, err)
	assert.NotNil(t, task)
	assert.Equal(t, templateID, task.TemplateID)
	assert.Equal(t, 50.0, task.Budget)
	assert.NotEmpty(t, task.EscrowTxHash)

	t.Logf("Task created from template: ID=%s, TemplateID=%s, TxHash=%s",
		task.ID, task.TemplateID, task.EscrowTxHash)
}

func testRefundPolicyIntegration(t *testing.T, ctx context.Context, orchestrator *orchestration.Orchestrator) {
	// Create a simple task to test refund policy
	task := orchestration.NewTask(
		"user131415",
		"test-task",
		[]string{"test-capability"},
		map[string]interface{}{"test": "data"},
	)
	task.Budget = 100.0
	task.RefundPolicyType = "linear"

	taskIDBytes, err := convertTaskIDForTest(task.ID)
	require.NoError(t, err)

	escrowClient := orchestrator.GetEscrowClient()

	// Set refund policy
	policy := substrate.RefundPolicy{
		PolicyType:    substrate.RefundPolicyTypeLinear,
		InitialRefund: 90, // 90% initial refund
		FinalRefund:   10, // 10% final refund
		DecayBlocks:   100, // Decay over 100 blocks
	}

	err = escrowClient.SetRefundPolicy(ctx, taskIDBytes, policy)
	assert.NoError(t, err)

	t.Logf("Refund policy set for task: %s", task.ID)

	// Get refund policy
	retrievedPolicy, err := escrowClient.GetRefundPolicy(ctx, taskIDBytes)
	assert.NoError(t, err)
	assert.Equal(t, substrate.RefundPolicyTypeLinear, retrievedPolicy.PolicyType)
	assert.Equal(t, uint32(90), retrievedPolicy.InitialRefund)
	assert.Equal(t, uint32(10), retrievedPolicy.FinalRefund)

	t.Logf("Refund policy retrieved successfully")

	// Calculate refund at different time points
	currentBlock := substrate.BlockNumber(50)
	refundCalc, err := escrowClient.CalculateRefund(ctx, taskIDBytes, &currentBlock)
	assert.NoError(t, err)
	assert.NotNil(t, refundCalc)
	assert.Greater(t, refundCalc.RefundAmount.Uint64(), uint64(0))

	t.Logf("Refund calculation: Amount=%s, Percentage=%d%%",
		refundCalc.RefundAmount.String(), refundCalc.RefundPercentage)
}

// Mock implementations for testing

type MockEscrowClient struct {
	escrows      map[string]*substrate.EscrowDetails
	policies     map[string]*substrate.RefundPolicy
	templates    map[string]*substrate.EscrowTemplate
	milestones   map[string][]substrate.Milestone
	participants map[string][]substrate.EscrowParticipant
}

func (m *MockEscrowClient) CreateEscrow(ctx context.Context, taskID [32]byte, amount uint64, taskHash [32]byte, timeoutBlocks *uint32) (substrate.Hash, error) {
	return substrate.Hash{0x12, 0x34}, nil // Mock transaction hash
}

func (m *MockEscrowClient) AddParticipant(ctx context.Context, taskID [32]byte, participant substrate.AccountID, role substrate.ParticipantRole, amount uint64) error {
	return nil // Mock success
}

func (m *MockEscrowClient) AddMilestone(ctx context.Context, taskID [32]byte, milestone substrate.Milestone) error {
	return nil // Mock success
}

func (m *MockEscrowClient) ApproveMilestone(ctx context.Context, taskID [32]byte, milestoneIndex uint32, evidence string) error {
	return nil // Mock success
}

func (m *MockEscrowClient) SetRefundPolicy(ctx context.Context, taskID [32]byte, policy substrate.RefundPolicy) error {
	key := string(taskID[:])
	if m.policies == nil {
		m.policies = make(map[string]*substrate.RefundPolicy)
	}
	m.policies[key] = &policy
	return nil
}

func (m *MockEscrowClient) GetRefundPolicy(ctx context.Context, taskID [32]byte) (*substrate.RefundPolicy, error) {
	key := string(taskID[:])
	if m.policies == nil {
		m.policies = make(map[string]*substrate.RefundPolicy)
	}
	if policy, exists := m.policies[key]; exists {
		return policy, nil
	}
	// Return default policy
	return &substrate.RefundPolicy{
		PolicyType:    substrate.RefundPolicyTypeLinear,
		InitialRefund: 90,
		FinalRefund:   10,
		DecayBlocks:   100,
	}, nil
}

func (m *MockEscrowClient) CalculateRefund(ctx context.Context, taskID [32]byte, atTime *substrate.BlockNumber) (*substrate.RefundCalculation, error) {
	amount := substrate.NewBalance(5000000000) // 50 tokens
	return &substrate.RefundCalculation{
		RefundAmount:     amount,
		RefundPercentage: 50,
		CalculatedAt:     substrate.BlockNumber(100),
	}, nil
}

func (m *MockEscrowClient) GetTemplate(ctx context.Context, templateID [32]byte) (*substrate.EscrowTemplate, error) {
	return &substrate.EscrowTemplate{
		TemplateType: substrate.EscrowTemplateTypeSimple,
		Name:         "Simple Classification Template",
		Description:  "Template for simple classification tasks",
		TaskType:     substrate.Bytes("classification"),
	}, nil
}

func (m *MockEscrowClient) CreateEscrowFromTemplate(ctx context.Context, taskID, templateID [32]byte, amount uint64) (substrate.Hash, error) {
	return substrate.Hash{0x56, 0x78}, nil // Mock transaction hash
}

func (m *MockEscrowClient) BatchCreateEscrow(ctx context.Context, requests []substrate.BatchCreateEscrowRequest) (*substrate.BatchCreateEscrowResult, error) {
	return &substrate.BatchCreateEscrowResult{
		TotalProcessed:  uint32(len(requests)),
		TotalSucceeded:  uint32(len(requests)),
		TotalFailed:     0,
		TransactionHash: substrate.Hash{0x9A, 0xBC},
	}, nil
}

func (m *MockEscrowClient) ReleasePayment(ctx context.Context, taskID [32]byte) error {
	return nil // Mock success
}

func (m *MockEscrowClient) RefundEscrow(ctx context.Context, taskID [32]byte) error {
	return nil // Mock success
}

type MockBlockchainService struct {
	rpcClient substrate.RPCClient
}

func (m *MockBlockchainService) GetRPCClient() substrate.RPCClient {
	return m.rpcClient
}

func (m *MockBlockchainService) IsEnabled() bool {
	return true
}

type MockRPCClient struct{}

func (m *MockRPCClient) GetMetadata() *substrate.Metadata {
	return &substrate.Metadata{} // Mock metadata
}

type MockQueue struct {
	tasks map[string]*orchestration.Task
}

func (m *MockQueue) Enqueue(task *orchestration.Task) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*orchestration.Task)
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *MockQueue) GetTask(taskID string) (*orchestration.Task, error) {
	if m.tasks == nil {
		m.tasks = make(map[string]*orchestration.Task)
	}
	if task, exists := m.tasks[taskID]; exists {
		return task, nil
	}
	return nil, nil
}

func (m *MockQueue) Update(task *orchestration.Task) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*orchestration.Task)
	}
	m.tasks[task.ID] = task
	return nil
}

func (m *MockQueue) DequeueWait(ctx context.Context) (*orchestration.Task, error) {
	return nil, nil // No tasks for testing
}

// Test helper functions

func createTestOrchestrator(ctx context.Context, logger *zap.Logger, blockchain *MockBlockchainService, escrowClient *MockEscrowClient) *orchestration.Orchestrator {
	// This is a simplified test orchestrator
	// In real implementation, you'd need to inject the mock escrow client properly
	queue := &MockQueue{}

	config := orchestration.DefaultOrchestratorConfig()
	config.NumWorkers = 1 // Minimal workers for testing

	orchestrator := orchestration.NewOrchestratorWithBlockchain(
		ctx,
		queue,
		nil, // No agent selector needed for this test
		nil, // No task executor needed for this test
		config,
		logger,
		nil, // Will be mocked
	)

	// In a real implementation, you'd have a way to inject the mock escrow client
	// For now, we'll assume the orchestrator has a method to set it for testing

	return orchestrator
}

func convertTaskIDForTest(taskID string) ([32]byte, error) {
	var result [32]byte
	copy(result[:], []byte(taskID))
	return result, nil
}

// Benchmark tests for performance validation

func BenchmarkCreateMultiPartyTask(b *testing.B) {
	ctx := context.Background()
	logger := zaptest.NewLogger(b)
	mockClient := &MockEscrowClient{}
	mockBlockchain := &MockBlockchainService{rpcClient: &MockRPCClient{}}
	orchestrator := createTestOrchestrator(ctx, logger, mockBlockchain, mockClient)

	participants := []string{"did:agent:alice", "did:agent:bob"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orchestrator.CreateMultiPartyTask(
			ctx,
			"benchmark-user",
			"benchmark-task",
			[]string{"capability"},
			map[string]interface{}{"data": "test"},
			participants,
			2,
			100.0,
		)
		if err != nil {
			b.Fatalf("Failed to create task: %v", err)
		}
	}
}

func BenchmarkBatchCreateTasks(b *testing.B) {
	ctx := context.Background()
	logger := zaptest.NewLogger(b)
	mockClient := &MockEscrowClient{}
	mockBlockchain := &MockBlockchainService{rpcClient: &MockRPCClient{}}
	orchestrator := createTestOrchestrator(ctx, logger, mockBlockchain, mockClient)

	batchTasks := []orchestration.BatchTaskRequest{
		{
			Type:         "task1",
			Capabilities: []string{"cap1"},
			Input:        map[string]interface{}{"input": "data1"},
			Budget:       25.0,
		},
		{
			Type:         "task2",
			Capabilities: []string{"cap2"},
			Input:        map[string]interface{}{"input": "data2"},
			Budget:       30.0,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orchestrator.CreateBatchTasks(ctx, "benchmark-user", batchTasks)
		if err != nil {
			b.Fatalf("Failed to create batch tasks: %v", err)
		}
	}
}