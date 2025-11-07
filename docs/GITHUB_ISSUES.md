# ZeroState - GitHub Issues Export

**Generated:** November 7, 2025
**Purpose:** Pre-formatted issues for GitHub import

Copy these into GitHub Issues to track work across the team.

---

## 🔴 P0 - CRITICAL (Blocks Launch)

### Issue #1: Agent Registration API
```markdown
**Title:** [API] Implement Agent Registration Endpoint

**Labels:** `P0-critical`, `application-layer`, `api`, `sprint-7`

**Assignee:** _Unassigned_

**Description:**

### Problem
No way for agent providers to upload their agents to the platform.

### Solution
Implement POST /api/agents/register endpoint with:
- WASM binary upload (multipart form or base64)
- Agent metadata validation
- Automatic Agent Card generation
- DHT publication
- HNSW index update
- WASM binary storage (S3 or filesystem)

### API Design
```go
POST /api/agents/register
Content-Type: multipart/form-data

{
  "name": "image-classifier-v1",
  "description": "CNN-based image classification",
  "capabilities": ["image-processing", "ml-inference"],
  "wasm_binary": <file upload>,
  "manifest": {
    "cpu_limit": "500m",
    "memory_limit": "256Mi",
    "pricing": {
      "per_execution": 0.01,
      "per_second": 0.001
    }
  }
}

Response 201:
{
  "agent_id": "agent-abc123",
  "agent_card_cid": "QmXxxx...",
  "status": "registered"
}
```

### Acceptance Criteria
- [ ] Endpoint accepts WASM binary upload
- [ ] Validates WASM binary (size, format)
- [ ] Generates Agent Card automatically
- [ ] Publishes to DHT
- [ ] Updates HNSW index
- [ ] Stores WASM binary persistently
- [ ] Returns agent_id and card CID
- [ ] Metrics: `agent_registrations_total`
- [ ] Tracing: full registration flow
- [ ] Logging: structured with agent_id
- [ ] Tests: unit + integration
- [ ] Documentation: API reference

### Technical Notes
- File upload limit: 50MB
- Timeout: 30s
- Storage: libs/api/handlers/agent_register.go
- Dependencies: libs/p2p (DHT), libs/search (HNSW)

### Estimated Effort
**L** (1-2 days)
```

---

### Issue #2: Task Submission API
```markdown
**Title:** [API] Implement Task Submission Endpoint

**Labels:** `P0-critical`, `application-layer`, `api`, `sprint-7`

**Assignee:** _Unassigned_

**Description:**

### Problem
No way for users to submit tasks for execution.

### Solution
Implement POST /api/tasks/submit endpoint with:
- Task requirements specification
- Input data (IPFS CIDs or inline)
- Budget/pricing constraints
- Callback webhook URL
- Task queuing
- Status tracking

### API Design
```go
POST /api/tasks/submit
Content-Type: application/json

{
  "task_type": "classify-images",
  "requirements": {
    "capabilities": ["image-processing", "ml-inference"],
    "max_latency_ms": 1000,
    "budget": 0.05,
    "min_reputation": 0.7
  },
  "inputs": {
    "images": ["ipfs://Qm...", "ipfs://Qm..."]
  },
  "callback_url": "https://myapp.com/webhook/task-complete"
}

Response 202:
{
  "task_id": "task-xyz789",
  "status": "queued",
  "estimated_start": "2025-11-07T12:00:00Z"
}
```

### Acceptance Criteria
- [ ] Endpoint accepts task submissions
- [ ] Validates requirements
- [ ] Queues task for processing
- [ ] Returns task_id immediately
- [ ] Task status API (GET /api/tasks/:id)
- [ ] Task result retrieval (GET /api/tasks/:id/result)
- [ ] Webhook callbacks on completion
- [ ] Metrics: `tasks_submitted_total`, `tasks_queued`
- [ ] Tests: unit + integration

### Technical Notes
- Storage: libs/api/handlers/task_submit.go
- Queue: libs/queue/ (Redis or in-memory)
- Dependencies: libs/execution

### Estimated Effort
**M** (4-8 hours)
```

---

### Issue #3: Meta-Agent Orchestrator
```markdown
**Title:** [Orchestration] Implement Meta-Agent Selection Logic

**Labels:** `P0-critical`, `orchestration`, `sprint-7`

**Assignee:** _Unassigned_

**Description:**

### Problem
No logic to match tasks to appropriate agents. System can't decide which agent should execute which task.

### Solution
Implement meta-agent orchestrator with:
- Agent selection algorithm
- Capability matching
- Multi-criteria scoring (price + quality + speed)
- Auction mechanism
- Failover logic

### Algorithm Design
```go
type MetaAgent struct {
    hnsw       *HNSWClient
    matcher    *CapabilityMatcher
    auctioneer *Auctioneer
    router     *Router
}

func (m *MetaAgent) SelectAgent(task *Task) (*Agent, error) {
    // 1. Find candidates via HNSW (semantic search)
    candidates := m.hnsw.Search(task.Requirements.Capabilities, k=20)

    // 2. Filter by hard requirements
    qualified := m.matcher.Filter(candidates, task.Requirements)

    // 3. Run auction
    bids := m.collectBids(qualified, task)
    winner := m.auctioneer.SelectWinner(bids, task.Preferences)

    // 4. Verify reputation
    if winner.Reputation < task.MinReputation {
        return m.SelectAgent(task) // Try next candidate
    }

    return winner, nil
}
```

### Acceptance Criteria
- [ ] Finds top K candidates via HNSW
- [ ] Filters by capabilities
- [ ] Filters by reputation threshold
- [ ] Runs auction (sealed-bid)
- [ ] Scores bids (multi-criteria)
- [ ] Selects winner
- [ ] Handles no-match case
- [ ] Metrics: `agent_selections_total`, `selection_duration`
- [ ] Tests: unit + integration

### Technical Notes
- Storage: libs/orchestration/meta_agent.go
- Dependencies: libs/search (HNSW), libs/reputation

### Estimated Effort
**L** (1-2 days)
```

---

### Issue #4: User Authentication System
```markdown
**Title:** [Auth] Implement User Authentication

**Labels:** `P0-critical`, `security`, `auth`, `sprint-8`

**Assignee:** _Unassigned_

**Description:**

### Problem
No user authentication. Anyone can access any endpoint.

### Solution
Implement JWT-based authentication with:
- User registration
- User login
- API key generation
- Rate limiting per user
- Session management

### API Design
```go
POST /api/auth/register
{
  "email": "user@example.com",
  "password": "secure_password",
  "role": "agent_provider"  // or "task_creator"
}

POST /api/auth/login
{
  "email": "user@example.com",
  "password": "secure_password"
}

Response:
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 3600
}

POST /api/auth/api-keys
Authorization: Bearer <access_token>

Response:
{
  "api_key": "zs_live_abc123...",
  "created_at": "2025-11-07T12:00:00Z"
}
```

### Acceptance Criteria
- [ ] User registration with email validation
- [ ] Password hashing (bcrypt)
- [ ] JWT token generation
- [ ] Token refresh mechanism
- [ ] API key generation
- [ ] API key authentication middleware
- [ ] Rate limiting (100 req/min per user)
- [ ] Session storage (Redis or in-memory)
- [ ] Tests: auth flows

### Technical Notes
- Storage: libs/auth/
- Dependencies: golang-jwt/jwt, bcrypt

### Estimated Effort
**M** (4-8 hours)
```

---

### Issue #5: Basic Web UI
```markdown
**Title:** [UI] Build Basic Web Dashboard

**Labels:** `P0-critical`, `frontend`, `ui`, `sprint-7`

**Assignee:** _Unassigned_

**Description:**

### Problem
No user interface. Everything is CLI/API only.

### Solution
Build minimal web dashboard with:
- Agent upload form
- Task submission form
- Task status viewer
- Agent list/search

### Pages
1. **Upload Agent** (`/agents/new`)
   - Form: name, description, capabilities
   - File upload: WASM binary
   - Submit button

2. **Submit Task** (`/tasks/new`)
   - Form: task type, requirements, inputs
   - Budget slider
   - Submit button

3. **My Tasks** (`/tasks`)
   - Table: task_id, status, agent, cost
   - Real-time updates (WebSocket or polling)

4. **Agent Marketplace** (`/agents`)
   - Search bar
   - Filter: capability, price, reputation
   - Agent cards with ratings

### Tech Stack
- React + TypeScript
- Tailwind CSS
- React Query (data fetching)
- React Hook Form (forms)

### Acceptance Criteria
- [ ] Agent upload form works
- [ ] Task submission form works
- [ ] Task status updates in real-time
- [ ] Agent search/filter works
- [ ] Responsive design (mobile-friendly)
- [ ] Deployed to /web directory

### Technical Notes
- Storage: web/
- Backend: libs/api/

### Estimated Effort
**L** (1-2 days)
```

---

### Issue #6: Database Integration
```markdown
**Title:** [Infrastructure] Add PostgreSQL Database

**Labels:** `P0-critical`, `infrastructure`, `database`, `sprint-8`

**Assignee:** _Unassigned_

**Description:**

### Problem
All data is in-memory. Lost on restart.

### Solution
Integrate PostgreSQL for persistent storage of:
- Users
- Agents
- Tasks
- Payments
- Reputation scores

### Schema Design
```sql
-- Users
CREATE TABLE users (
  id UUID PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  role VARCHAR(50) NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Agents
CREATE TABLE agents (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  name VARCHAR(255) NOT NULL,
  description TEXT,
  capabilities JSONB,
  wasm_cid VARCHAR(255),
  manifest JSONB,
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- Tasks
CREATE TABLE tasks (
  id UUID PRIMARY KEY,
  user_id UUID REFERENCES users(id),
  agent_id UUID REFERENCES agents(id),
  status VARCHAR(50),
  requirements JSONB,
  inputs JSONB,
  result JSONB,
  cost DECIMAL(10,6),
  created_at TIMESTAMP DEFAULT NOW(),
  completed_at TIMESTAMP
);
```

### Acceptance Criteria
- [ ] PostgreSQL container in docker-compose
- [ ] Database migrations (golang-migrate)
- [ ] Connection pooling
- [ ] CRUD repositories for all entities
- [ ] Transaction support
- [ ] Database backups configured
- [ ] Tests: repository layer

### Technical Notes
- Storage: libs/db/
- Dependencies: pgx (PostgreSQL driver), golang-migrate

### Estimated Effort
**L** (1-2 days)
```

---

### Issue #7: Payment Integration
```markdown
**Title:** [Payments] Integrate Stripe for Fiat Payments

**Labels:** `P0-critical`, `payments`, `sprint-8`

**Assignee:** _Unassigned_

**Description:**

### Problem
Payment channels exist but no real money flow.

### Solution
Integrate Stripe for fiat payments:
- User deposits (credit card)
- Automated channel funding
- Agent payouts
- Invoice generation

### Features
- Stripe Checkout for deposits
- Automatic payment channel creation
- Balance tracking
- Payout to bank accounts

### Acceptance Criteria
- [ ] Stripe Checkout integration
- [ ] User deposits create/fund payment channels
- [ ] Task execution deducts from channel
- [ ] Agent revenue tracking
- [ ] Payout API (weekly batches)
- [ ] Invoice generation
- [ ] Tests: payment flows

### Technical Notes
- Storage: libs/payments/stripe.go
- Dependencies: stripe-go

### Estimated Effort
**L** (1-2 days)
```

---

### Issue #8: Auction Mechanism
```markdown
**Title:** [Orchestration] Implement Sealed-Bid Auction

**Labels:** `P0-critical`, `orchestration`, `auction`, `sprint-7`

**Assignee:** _Unassigned_

**Description:**

### Problem
No price discovery. Don't know how to pick between multiple agents.

### Solution
Implement sealed-bid auction (Vickrey auction):
- Agents submit bids
- Lowest bid wins
- Winner pays 2nd-lowest price

### Algorithm
```go
type Bid struct {
    AgentID           string
    Price             float64
    EstimatedDuration time.Duration
    Reputation        float64
}

func (a *Auctioneer) RunAuction(agents []*Agent, task *Task) *Agent {
    // Collect bids
    bids := a.collectBids(agents, task)

    // Score bids (multi-criteria)
    scored := a.scoreBids(bids, task.Preferences)

    // Select winner (lowest score)
    winner := scored[0]

    // Vickrey pricing (pay 2nd price)
    if len(scored) > 1 {
        winner.FinalPrice = scored[1].Price
    }

    return winner.Agent
}

func (a *Auctioneer) scoreBids(bids []*Bid, prefs Preferences) []*ScoredBid {
    // Multi-criteria scoring:
    // - Price (40%)
    // - Reputation (30%)
    // - Speed (20%)
    // - Success rate (10%)
}
```

### Acceptance Criteria
- [ ] Agents can submit bids
- [ ] Auction runs automatically
- [ ] Winner selected
- [ ] Vickrey pricing applied
- [ ] Metrics: `auctions_total`, `auction_duration`
- [ ] Tests: auction scenarios

### Technical Notes
- Storage: libs/orchestration/auction.go

### Estimated Effort
**M** (4-8 hours)
```

---

## 🟡 P1 - HIGH PRIORITY

### Issue #9: Agent Versioning
```markdown
**Title:** [API] Implement Agent Versioning

**Labels:** `P1-high`, `api`, `versioning`, `sprint-8`

**Assignee:** _Unassigned_

**Description:**

### Problem
Agents can't be updated after registration.

### Solution
Implement agent versioning:
- PATCH /api/agents/:id endpoint
- Version tracking (v1, v2, v3)
- Old tasks use old versions
- New tasks use latest version

### Acceptance Criteria
- [ ] PATCH endpoint for updates
- [ ] Version incrementing
- [ ] Version-specific WASM storage
- [ ] Task-to-version mapping
- [ ] Tests

### Estimated Effort
**M** (4-8 hours)
```

---

### Issue #10: Task Result Storage
```markdown
**Title:** [Storage] Implement Task Result Storage

**Labels:** `P1-high`, `storage`, `sprint-8`

**Assignee:** _Unassigned_

**Description:**

### Problem
Task results not persisted.

### Solution
- Store results in S3/IPFS
- Return result URLs
- TTL-based cleanup

### Estimated Effort
**S** (2-4 hours)
```

---

_[Continue with remaining issues...]_

---

## Creating Issues in Bulk

### Option 1: GitHub CLI
```bash
# Install GitHub CLI
brew install gh

# Authenticate
gh auth login

# Create issues from this file
gh issue create --title "[API] Implement Agent Registration Endpoint" \
  --body "$(cat issue_template_1.md)" \
  --label "P0-critical,application-layer,api,sprint-7"
```

### Option 2: GitHub API Script
```bash
# See scripts/create_github_issues.sh
```

### Option 3: Manual Creation
Copy each issue block into GitHub Issues UI.

---

**Total Issues to Create: 50+**

Prioritize P0 (8 issues) first, then P1 (10-15 issues).
