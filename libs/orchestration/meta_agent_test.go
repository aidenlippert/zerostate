package orchestration

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/aidenlippert/zerostate/libs/database"
	"github.com/aidenlippert/zerostate/libs/search"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// ============================================================================
// MOCKS
// ============================================================================

// MockSearchIndex mocks the HNSW semantic search index
type MockSearchIndex struct {
	mock.Mock
}

func (m *MockSearchIndex) SearchByCapabilities(ctx context.Context, capabilities []string, limit int) ([]*search.AgentCard, error) {
	args := m.Called(ctx, capabilities, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*search.AgentCard), args.Error(1)
}

func (m *MockSearchIndex) Search(ctx context.Context, query string, limit int) ([]*search.AgentCard, error) {
	args := m.Called(ctx, query, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*search.AgentCard), args.Error(1)
}

func (m *MockSearchIndex) IndexCard(ctx context.Context, cardJSON []byte) error {
	args := m.Called(ctx, cardJSON)
	return args.Error(0)
}

func (m *MockSearchIndex) GetCard(did string) (*search.AgentCard, bool) {
	args := m.Called(did)
	if args.Get(0) == nil {
		return nil, args.Bool(1)
	}
	return args.Get(0).(*search.AgentCard), args.Bool(1)
}

func (m *MockSearchIndex) RemoveCard(did string) {
	m.Called(did)
}

func (m *MockSearchIndex) Size() int {
	args := m.Called()
	return args.Int(0)
}

func (m *MockSearchIndex) ListAll() []*search.AgentCard {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*search.AgentCard)
}

func (m *MockSearchIndex) Stats() map[string]interface{} {
	args := m.Called()
	return args.Get(0).(map[string]interface{})
}

// MockDatabase mocks the database for testing
type MockDatabase struct {
	mock.Mock
}

func (m *MockDatabase) GetAgentByDID(did string) (*database.Agent, error) {
	args := m.Called(did)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*database.Agent), args.Error(1)
}

func (m *MockDatabase) SearchAgents(query string) ([]*database.Agent, error) {
	args := m.Called(query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*database.Agent), args.Error(1)
}

// ============================================================================
// TEST FIXTURES
// ============================================================================

func createMockAgentCard(did string, capabilities []string, similarity float64) *search.AgentCard {
	return &search.AgentCard{
		DID:          did,
		Capabilities: capabilities,
		Metadata: map[string]string{
			"name":        did + "-agent",
			"description": "Test agent for " + capabilities[0],
		},
		Timestamp: time.Now(),
	}
}

func createMockDatabaseAgent(did string, capabilities []string, price float64, rating float64, tasksCompleted int) *database.Agent {
	capsJSON, _ := json.Marshal(capabilities)

	return &database.Agent{
		ID:             uuid.New(),
		DID:            did,
		Name:           did + "-agent",
		Description:    sql.NullString{String: "Test agent for " + capabilities[0], Valid: true},
		Capabilities:   capsJSON,
		Status:         database.AgentStatusOnline,
		Price:          price,
		Rating:         rating,
		TasksCompleted: tasksCompleted,
		CreatedAt:      time.Now().Add(-30 * 24 * time.Hour), // 30 days old
		UpdatedAt:      time.Now(),
	}
}

// ============================================================================
// INTEGRATION TESTS
// ============================================================================

func TestMetaAgent_SelectAgent_WithHNSWSemanticSearch(t *testing.T) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "task-123",
		Description:  "I need an agent for medical imaging analysis",
		Capabilities: []string{"medical-imaging", "diagnostics"},
		Budget:       10.0,
	}

	// Mock HNSW search returns 3 agent cards (semantic similarity sorted)
	agentCards := []*search.AgentCard{
		createMockAgentCard("did:ainur:medical-ai", []string{"medical-imaging", "diagnostics", "radiology"}, 0.95),
		createMockAgentCard("did:ainur:health-scan", []string{"medical-imaging", "health-analysis"}, 0.87),
		createMockAgentCard("did:ainur:image-proc", []string{"image-processing", "diagnostics"}, 0.72),
	}

	mockSearch.On("SearchByCapabilities", ctx, task.Capabilities, config.MaxAgentsForAuction*2).
		Return(agentCards, nil)

	// Mock database returns full agent details for each DID
	// Agent 1: Highest rating, medium price (should win!)
	mockDB.On("GetAgentByDID", "did:ainur:medical-ai").
		Return(createMockDatabaseAgent("did:ainur:medical-ai",
			[]string{"medical-imaging", "diagnostics", "radiology"},
			5.0,  // price
			4.8,  // rating (highest!)
			150), // tasks completed
			nil)

	// Agent 2: Lower rating, cheap price
	mockDB.On("GetAgentByDID", "did:ainur:health-scan").
		Return(createMockDatabaseAgent("did:ainur:health-scan",
			[]string{"medical-imaging", "health-analysis"},
			2.0, // price (cheapest!)
			3.5, // rating
			50), // tasks completed
			nil)

	// Agent 3: Medium rating, expensive (should lose)
	mockDB.On("GetAgentByDID", "did:ainur:image-proc").
		Return(createMockDatabaseAgent("did:ainur:image-proc",
			[]string{"image-processing", "diagnostics"},
			8.0, // price (most expensive!)
			4.2, // rating
			80), // tasks completed
			nil)

	// Execute
	selectedAgent, err := metaAgent.SelectAgent(ctx, task)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, selectedAgent)

	// The winner should be "medical-ai" because:
	// - Highest semantic similarity (0.95)
	// - Highest rating (4.8)
	// - Most tasks completed (150)
	// - Reasonable price (5.0, within budget)
	// Multi-criteria scoring: (0.3*price + 0.3*quality + 0.2*speed + 0.2*reputation)
	assert.Equal(t, "did:ainur:medical-ai", selectedAgent.DID)
	assert.Equal(t, "did:ainur:medical-ai-agent", selectedAgent.Name)

	// Verify mocks were called
	mockSearch.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestMetaAgent_SelectAgent_FallbackToDatabase(t *testing.T) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "task-456",
		Description:  "Math calculation agent needed",
		Capabilities: []string{"math", "calculation"},
		Budget:       5.0,
	}

	// Mock HNSW search FAILS (e.g., index not ready)
	mockSearch.On("SearchByCapabilities", ctx, task.Capabilities, config.MaxAgentsForAuction*2).
		Return(nil, assert.AnError)

	// Should fallback to database search
	mathAgent := createMockDatabaseAgent("did:ainur:math-agent",
		[]string{"math", "calculation"},
		3.0, 4.0, 100)

	mockDB.On("SearchAgents", "math").
		Return([]*database.Agent{mathAgent}, nil)

	// Execute
	selectedAgent, err := metaAgent.SelectAgent(ctx, task)

	// Assert - should succeed via fallback
	assert.NoError(t, err)
	assert.NotNil(t, selectedAgent)
	assert.Equal(t, "did:ainur:math-agent", selectedAgent.DID)

	// Verify fallback was used
	mockSearch.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestMetaAgent_SelectAgent_NoAgentsAvailable(t *testing.T) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "task-789",
		Description:  "Ultra rare quantum agent",
		Capabilities: []string{"quantum-computing", "superconductor"},
		Budget:       100.0,
	}

	// Mock HNSW search returns empty (no agents match)
	mockSearch.On("SearchByCapabilities", ctx, task.Capabilities, config.MaxAgentsForAuction*2).
		Return([]*search.AgentCard{}, nil)

	// Execute
	selectedAgent, err := metaAgent.SelectAgent(ctx, task)

	// Assert - should return error
	assert.Error(t, err)
	assert.Nil(t, selectedAgent)
	assert.Equal(t, ErrNoAgentsAvailable, err)

	mockSearch.AssertExpectations(t)
}

func TestMetaAgent_SelectAgent_BudgetConstraint(t *testing.T) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "task-budget",
		Description:  "Cheap agent needed",
		Capabilities: []string{"simple-task"},
		Budget:       1.0, // Very low budget!
	}

	// Mock HNSW returns agents
	agentCards := []*search.AgentCard{
		createMockAgentCard("did:ainur:expensive", []string{"simple-task"}, 0.9),
		createMockAgentCard("did:ainur:cheap", []string{"simple-task"}, 0.8),
	}

	mockSearch.On("SearchByCapabilities", ctx, task.Capabilities, config.MaxAgentsForAuction*2).
		Return(agentCards, nil)

	// Expensive agent (price exceeds budget, should be filtered)
	mockDB.On("GetAgentByDID", "did:ainur:expensive").
		Return(createMockDatabaseAgent("did:ainur:expensive",
			[]string{"simple-task"},
			5.0, // price > budget (1.0)
			4.5, 100), nil)

	// Cheap agent (within budget)
	mockDB.On("GetAgentByDID", "did:ainur:cheap").
		Return(createMockDatabaseAgent("did:ainur:cheap",
			[]string{"simple-task"},
			0.5, // price < budget!
			3.8, 50), nil)

	// Execute
	selectedAgent, err := metaAgent.SelectAgent(ctx, task)

	// Assert - should select cheap agent
	assert.NoError(t, err)
	assert.NotNil(t, selectedAgent)
	assert.Equal(t, "did:ainur:cheap", selectedAgent.DID)

	mockSearch.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

func TestMetaAgent_SelectAgent_StatusFiltering(t *testing.T) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "task-status",
		Description:  "Need online agent",
		Capabilities: []string{"test"},
		Budget:       10.0,
	}

	agentCards := []*search.AgentCard{
		createMockAgentCard("did:ainur:offline", []string{"test"}, 0.9),
		createMockAgentCard("did:ainur:online", []string{"test"}, 0.8),
	}

	mockSearch.On("SearchByCapabilities", ctx, task.Capabilities, config.MaxAgentsForAuction*2).
		Return(agentCards, nil)

	// Offline agent (should be filtered out)
	offlineAgent := createMockDatabaseAgent("did:ainur:offline", []string{"test"}, 2.0, 4.5, 100)
	offlineAgent.Status = database.AgentStatusOffline // OFFLINE!
	mockDB.On("GetAgentByDID", "did:ainur:offline").Return(offlineAgent, nil)

	// Online agent (should be selected)
	onlineAgent := createMockDatabaseAgent("did:ainur:online", []string{"test"}, 2.0, 4.0, 80)
	onlineAgent.Status = database.AgentStatusOnline // ONLINE!
	mockDB.On("GetAgentByDID", "did:ainur:online").Return(onlineAgent, nil)

	// Execute
	selectedAgent, err := metaAgent.SelectAgent(ctx, task)

	// Assert - should select online agent
	assert.NoError(t, err)
	assert.NotNil(t, selectedAgent)
	assert.Equal(t, "did:ainur:online", selectedAgent.DID)
	assert.Equal(t, database.AgentStatusOnline, selectedAgent.Status)

	mockSearch.AssertExpectations(t)
	mockDB.AssertExpectations(t)
}

// ============================================================================
// PERFORMANCE BENCHMARKS
// ============================================================================

func BenchmarkMetaAgent_SelectAgent_HNSWSearch(b *testing.B) {
	// Setup
	mockSearch := new(MockSearchIndex)
	mockDB := new(MockDatabase)
	logger := zap.NewNop()
	config := DefaultMetaAgentConfig()

	metaAgent := NewMetaAgent(mockDB, mockSearch, config, logger)

	ctx := context.Background()
	task := &Task{
		ID:           "bench-task",
		Description:  "Benchmark test",
		Capabilities: []string{"benchmark"},
		Budget:       10.0,
	}

	// Setup mocks
	agentCards := []*search.AgentCard{
		createMockAgentCard("did:ainur:bench1", []string{"benchmark"}, 0.9),
		createMockAgentCard("did:ainur:bench2", []string{"benchmark"}, 0.85),
		createMockAgentCard("did:ainur:bench3", []string{"benchmark"}, 0.8),
	}

	mockSearch.On("SearchByCapabilities", mock.Anything, mock.Anything, mock.Anything).
		Return(agentCards, nil)

	for i := 1; i <= 3; i++ {
		did := "did:ainur:bench" + string(rune('0'+i))
		agent := createMockDatabaseAgent(did, []string{"benchmark"}, 5.0, 4.0, 100)
		mockDB.On("GetAgentByDID", did).Return(agent, nil)
	}

	// Benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := metaAgent.SelectAgent(ctx, task)
		if err != nil {
			b.Fatal(err)
		}
	}
}
