# Sprint 10: Build Fixes & End-to-End Integration Testing

**Sprint Goal**: Fix compilation issues, enable full system testing, and verify end-to-end agent marketplace flow

**Status**: Planning → In Progress
**Start Date**: January 2025
**Target Completion**: 3-5 days
**Previous Sprint**: [Sprint 9 - Agent Marketplace & Database Integration](./SPRINT_9_COMPLETE.md)

---

## Sprint Objectives

### 1. Fix Build & Compilation Issues (Priority: P0 - Critical) 🔴
**Goal**: Resolve libp2p dependency conflicts and enable successful compilation

**Tasks**:
- [ ] Diagnose libp2p dependency conflicts in detail
- [ ] Pin compatible libp2p versions across all modules
- [ ] Update go.mod/go.work with conflict-free dependencies
- [ ] Verify clean build with `go build ./...`
- [ ] Run all existing tests to ensure no regressions

**Acceptance Criteria**:
- ✅ `go build ./cmd/api` completes successfully
- ✅ All 254 existing tests still pass
- ✅ No ambiguous import errors
- ✅ Server starts without dependency errors

**Estimated Effort**: 4-8 hours

---

### 2. Local Development Setup (Priority: P0 - Critical) 🔴
**Goal**: Enable local testing with minimal external dependencies

**Tasks**:
- [ ] Document required environment variables
- [ ] Set up local PostgreSQL or SQLite for testing
- [ ] Configure mock S3 storage (or use LocalStack)
- [ ] Create development configuration file
- [ ] Test server startup in development mode

**Acceptance Criteria**:
- ✅ Server starts with default dev configuration
- ✅ Database initializes with schema
- ✅ Mock agents seeded successfully
- ✅ All API endpoints respond (even if with mock data)

**Estimated Effort**: 2-4 hours

---

### 3. End-to-End Integration Testing (Priority: P1 - High) 🟡
**Goal**: Verify complete agent marketplace flow works end-to-end

**Test Scenarios**:

#### 3.1 Agent Upload Flow
- [ ] Upload WASM agent binary via API
- [ ] Verify database persistence
- [ ] Verify S3/storage upload (or mock)
- [ ] Check hash calculation and validation
- [ ] Confirm agent appears in listings

#### 3.2 Agent Discovery Flow
- [ ] List all agents
- [ ] Search agents by keyword
- [ ] Get single agent details
- [ ] Verify mock agent data seeding

#### 3.3 Task Execution Flow
- [ ] Submit task for execution
- [ ] Verify WASM runner executes
- [ ] Check result storage in task_results table
- [ ] Retrieve result via API
- [ ] List task results with filtering

#### 3.4 Complete User Journey
- [ ] Register/login user
- [ ] Upload agent as authenticated user
- [ ] Search for uploaded agent
- [ ] Execute task on agent
- [ ] View task results
- [ ] List agent execution history

**Acceptance Criteria**:
- ✅ All test scenarios pass
- ✅ No runtime errors or panics
- ✅ Database transactions commit successfully
- ✅ API responses match expected format
- ✅ Execution results stored correctly

**Estimated Effort**: 6-10 hours

---

### 4. API Endpoint Testing (Priority: P1 - High) 🟡
**Goal**: Test all Sprint 9 endpoints with real requests

**Endpoints to Test**:

#### Task Execution Endpoints
```bash
# Execute task directly
POST /api/v1/tasks/execute
{
  "agent_id": "agent_001",
  "input": "test input"
}

# Get task result
GET /api/v1/tasks/:id/results

# List task results
GET /api/v1/tasks/results?agent_id=xxx&limit=10
```

#### Agent Discovery Endpoints
```bash
# List agents
GET /api/v1/agents?limit=20&offset=0

# Search agents
GET /api/v1/agents/search?q=data

# Get agent details
GET /api/v1/agents/:id

# Get agent stats
GET /api/v1/agents/:id/stats
```

**Test Cases**:
- [ ] Valid requests return 200 OK
- [ ] Invalid agent_id returns 404
- [ ] Missing required fields returns 400
- [ ] Unauthorized requests return 401
- [ ] Database errors handled gracefully
- [ ] Response format matches documentation

**Estimated Effort**: 4-6 hours

---

### 5. Performance & Load Testing (Priority: P2 - Medium) 🟢
**Goal**: Validate system performance under realistic load

**Test Scenarios**:
- [ ] Single agent execution: <100ms response time
- [ ] Concurrent executions: 10 simultaneous tasks
- [ ] Agent search performance: <50ms query time
- [ ] Database connection pooling works correctly
- [ ] Memory usage stays bounded

**Tools**:
- `ab` (Apache Bench) for simple load testing
- Custom Go benchmark tests
- Database query profiling

**Acceptance Criteria**:
- ✅ Task execution: <100ms average (excluding WASM runtime)
- ✅ Agent search: <50ms query time
- ✅ Concurrent executions: 10+ without errors
- ✅ Memory usage: <500MB under load

**Estimated Effort**: 3-5 hours

---

### 6. Documentation & Developer Experience (Priority: P2 - Medium) 🟢
**Goal**: Make it easy for developers to get started

**Deliverables**:
- [ ] Create QUICKSTART.md with setup instructions
- [ ] Document all API endpoints with examples
- [ ] Add environment variable reference
- [ ] Create example curl commands
- [ ] Write troubleshooting guide

**Estimated Effort**: 2-3 hours

---

## Technical Approach

### 1. libp2p Dependency Resolution Strategy

**Analysis Phase**:
```bash
# Check current conflicts
go mod graph | grep libp2p

# Identify version mismatches
go list -m all | grep libp2p
```

**Resolution Options**:

**Option A: Pin to Single Version** (Recommended)
```go
// In go.mod
require (
    github.com/libp2p/go-libp2p v0.33.0
)

replace (
    github.com/libp2p/go-libp2p/core => github.com/libp2p/go-libp2p v0.33.0
)
```

**Option B: Update All Modules**
```bash
go get -u github.com/libp2p/go-libp2p@latest
go mod tidy
```

**Option C: Separate Module** (if A & B fail)
- Isolate libp2p dependencies in separate module
- Use interface adapters to decouple

---

### 2. Testing Infrastructure

**Test Database Setup**:
```bash
# Option 1: PostgreSQL in Docker
docker run --name postgres-test -e POSTGRES_PASSWORD=test -p 5432:5432 -d postgres:15

# Option 2: SQLite (simpler for dev)
export DATABASE_URL="sqlite://./test.db"
```

**Mock S3 Setup**:
```bash
# Option 1: LocalStack
docker run -p 4566:4566 localstack/localstack

# Option 2: In-memory mock (already implemented)
# Use nil s3Storage - system handles gracefully
```

**Environment Configuration**:
```bash
# Create .env.test file
DATABASE_URL=postgres://localhost/zerostate_test
JWT_SECRET=test-secret-key-do-not-use-in-production
S3_BUCKET=test-bucket
S3_ENDPOINT=http://localhost:4566  # For LocalStack
```

---

### 3. Integration Test Structure

**Test File**: `tests/integration/sprint10_e2e_test.go`

```go
func TestCompleteAgentMarketplaceFlow(t *testing.T) {
    // Setup
    db := setupTestDatabase(t)
    defer db.Close()

    server := setupTestServer(t, db)
    defer server.Stop()

    // Test: Upload Agent
    t.Run("UploadAgent", func(t *testing.T) {
        // Upload WASM binary
        // Verify database persistence
        // Check response
    })

    // Test: Search Agent
    t.Run("SearchAgent", func(t *testing.T) {
        // Search for uploaded agent
        // Verify appears in results
    })

    // Test: Execute Task
    t.Run("ExecuteTask", func(t *testing.T) {
        // Execute task on agent
        // Verify result storage
        // Check response format
    })

    // Test: Get Results
    t.Run("GetTaskResults", func(t *testing.T) {
        // Retrieve result by ID
        // List all results
        // Filter by agent_id
    })
}
```

---

## Success Criteria

### Must Have (P0)
- ✅ Project compiles successfully with `go build ./...`
- ✅ All 254 existing tests pass
- ✅ Server starts and responds to health checks
- ✅ Database schema initializes correctly
- ✅ Basic API endpoints return valid responses

### Should Have (P1)
- ✅ Complete end-to-end flow works (upload → execute → results)
- ✅ All Sprint 9 endpoints tested and verified
- ✅ Agent discovery works with database
- ✅ Task execution stores results correctly
- ✅ Error handling works as expected

### Nice to Have (P2)
- ✅ Performance benchmarks documented
- ✅ Load testing shows acceptable performance
- ✅ QUICKSTART.md created
- ✅ API documentation complete
- ✅ Troubleshooting guide written

---

## Risk Assessment

### High Risk 🔴
**Risk**: libp2p dependencies cannot be easily resolved
- **Mitigation**: Have fallback plan to isolate in separate module
- **Contingency**: Worst case - disable p2p features temporarily

**Risk**: WASM execution fails in integration tests
- **Mitigation**: Use working wasm-demo as reference
- **Contingency**: Start with simple WASM binaries, increase complexity

### Medium Risk 🟡
**Risk**: Database setup too complex for developers
- **Mitigation**: Provide Docker Compose setup
- **Contingency**: Default to SQLite for simplicity

**Risk**: S3 integration issues
- **Mitigation**: System already handles nil s3Storage gracefully
- **Contingency**: Use mock/placeholder URLs for testing

### Low Risk 🟢
**Risk**: Performance doesn't meet targets
- **Mitigation**: Targets are already conservative
- **Contingency**: Document actual performance, adjust targets

---

## Milestones

### Milestone 1: Clean Build (Day 1-2)
- Fix libp2p dependencies
- Successful compilation
- All tests pass

### Milestone 2: Local Development (Day 2-3)
- Database setup working
- Server starts successfully
- Mock agents seeded

### Milestone 3: Integration Testing (Day 3-4)
- Upload flow tested
- Execution flow tested
- Discovery flow tested

### Milestone 4: Polish & Documentation (Day 4-5)
- Performance testing complete
- Documentation written
- Sprint 10 completion report

---

## Dependencies & Prerequisites

### Required
- Go 1.24+ installed
- PostgreSQL 15+ OR SQLite3
- Git for version control

### Optional
- Docker for containerized testing
- LocalStack for S3 mocking
- Apache Bench for load testing

### From Previous Sprints
- ✅ Sprint 8: WASM execution engine (verified working)
- ✅ Sprint 9: Database schema and API endpoints
- ✅ 254 existing tests passing

---

## Next Sprint Preview (Sprint 11)

Potential focus areas after Sprint 10:

1. **Frontend Integration** - Connect web UI to new API endpoints
2. **Authentication & Authorization** - JWT, user permissions, rate limiting
3. **Payment Integration** - Stripe/payment processor for agent usage
4. **Advanced Features** - Agent versioning, deployment management
5. **Production Deployment** - Fly.io deployment with real infrastructure

---

## Related Documentation

- [Sprint 9 Completion Report](./SPRINT_9_COMPLETE.md)
- [Sprint 8 Completion Report](./SPRINT_8_COMPLETE.md)
- [Project Status](./PROJECT_STATUS.md)
- [Test Matrix](./TEST_MATRIX.md)

---

## Notes

**Key Insight**: Sprint 9 delivered all features but compilation is blocked. Sprint 10 focuses on making it actually work and testable.

**Philosophy**: "Working software over comprehensive documentation" - Get it running first, then optimize.

**Testing Strategy**: Start simple (health checks) → Medium complexity (API endpoints) → Full integration (complete user journeys)

---

**Sprint 10 Status**: Ready to Begin 🚀
