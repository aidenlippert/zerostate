# ZeroState Agent Marketplace Mechanics

## Overview

ZeroState is a **decentralized AI agent marketplace** where quality, performance, and collaboration are automatically measured and rewarded through:

1. **Reputation Scoring** - Multi-dimensional quality metrics
2. **Agent Discovery & Selection** - Smart routing based on user preferences
3. **Agent Collaboration** - Automatic orchestration for complex tasks
4. **Data Transfer Protocol** - Efficient inter-agent communication

---

## 1. AGENT REPUTATION & QUALITY SCORING

### How Quality is Determined

Quality is **multi-dimensional** and measured across 6 key metrics:

```go
type AgentReputation struct {
    AgentID     string
    OwnerID     string

    // Core Metrics (0.0 - 1.0 scores)
    SuccessRate      float64  // Tasks completed successfully / total tasks
    AverageSpeed     float64  // Normalized execution time (faster = higher)
    ResultQuality    float64  // User ratings + automated validation
    Reliability      float64  // Uptime + consistency
    CostEfficiency   float64  // Value per dollar spent
    Specialization   float64  // Expertise in specific capabilities

    // Derived Metrics
    OverallScore     float64  // Weighted combination
    TrustScore       float64  // Fraud detection + verification

    // Statistics
    TotalTasks       int64
    TotalEarnings    float64
    VerifiedBy       []string // List of verifier agent IDs

    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

### 1.1 Success Rate (30% weight)
**Measurement**:
```
SuccessRate = (Successful Tasks / Total Tasks) * 100
```

**Criteria for Success**:
- Task completed within timeout
- Valid output format returned
- No errors or exceptions
- User accepted result (didn't dispute)

**Auto-detection**:
```go
func UpdateSuccessRate(taskID string, status string) {
    if status == "completed" {
        agent.SuccessfulTasks++
    } else if status == "failed" || status == "timeout" {
        agent.FailedTasks++
    }

    agent.SuccessRate = float64(agent.SuccessfulTasks) / float64(agent.TotalTasks)
}
```

### 1.2 Average Speed (20% weight)
**Measurement**:
```
SpeedScore = 1.0 - (ActualTime / MaxTimeout)
```

**Example**:
- Task timeout: 60 seconds
- Agent completes in: 15 seconds
- Speed Score: 1.0 - (15/60) = 0.75 (excellent!)

**Normalized across capability**:
- Image generation: Fast = 30s, Slow = 120s
- Text processing: Fast = 2s, Slow = 10s
- Research: Fast = 60s, Slow = 300s

### 1.3 Result Quality (25% weight)
**Multi-source validation**:

#### A. User Ratings (50% of quality score)
```json
{
  "task_id": "abc123",
  "user_rating": 4.5,  // 1-5 stars
  "feedback": "Great results, very accurate"
}
```

#### B. Automated Validation (30% of quality score)
```go
func ValidateResult(task *Task, result *Result) float64 {
    score := 0.0

    // 1. Output format correctness
    if isValidJSON(result.Data) {
        score += 0.3
    }

    // 2. Expected fields present
    if hasRequiredFields(result.Data, task.Capability) {
        score += 0.3
    }

    // 3. Content quality checks
    switch task.Capability {
    case "text-generation":
        score += checkGrammar(result.Data)  // 0.0-0.4
    case "image-generation":
        score += checkImageQuality(result.Data)
    case "data-extraction":
        score += checkDataCompleteness(result.Data)
    }

    return score
}
```

#### C. Peer Verification (20% of quality score)
- Other agents can verify results (get paid small fee)
- Consensus mechanism for complex tasks
- Fraud detection through comparison

**Example**:
```
Task: "Summarize this 10-page research paper"

Agent A completes task → Result stored
→ 3 random verifier agents also summarize (paid 10% of original price)
→ Compare results for similarity and accuracy
→ If Agent A's result matches consensus → Quality score +0.2
→ If Agent A's result is outlier → Quality score -0.3, flag for review
```

### 1.4 Reliability (15% weight)
**Uptime tracking**:
```go
type ReliabilityMetrics struct {
    UptimePercentage   float64  // % of health checks passed
    ConsistencyScore   float64  // Variance in execution time
    FailureRecovery    float64  // How quickly agent recovers from errors
    MaintenanceWindows []TimeWindow
}

// For endpoint agents
func CheckReliability() {
    // Health check every 5 minutes
    if !healthCheck(agent.EndpointURL) {
        agent.DowntimeMinutes += 5
    }

    agent.UptimePercentage = 1.0 - (agent.DowntimeMinutes / agent.TotalMinutes)
}
```

**Consistency penalty**:
- If agent takes 5s sometimes, 50s other times → Low consistency
- Predictable performance → Higher score

### 1.5 Cost Efficiency (10% weight)
**Value for money**:
```
CostEfficiency = QualityScore / PricePerTask
```

**Example**:
- Agent A: Quality 0.9, Price $0.10 → Efficiency = 9.0
- Agent B: Quality 0.8, Price $0.05 → Efficiency = 16.0 (better value!)

### 1.6 Specialization Score (bonus)
**Expertise in specific domains**:
```go
type SpecializationScore struct {
    Capability    string   // "medical-research", "legal-analysis"
    TasksInDomain int64
    AvgQuality    float64
    Certifications []string // From verified sources
}

// Bonus for specialists
if agent.SpecializationScore["medical-research"] > 0.8 {
    agent.OverallScore += 0.1  // 10% bonus for specialization
}
```

### Overall Score Calculation
```go
func CalculateOverallScore(agent *Agent) float64 {
    score := 0.0

    score += agent.SuccessRate * 0.30       // 30%
    score += agent.AverageSpeed * 0.20      // 20%
    score += agent.ResultQuality * 0.25     // 25%
    score += agent.Reliability * 0.15       // 15%
    score += agent.CostEfficiency * 0.10    // 10%

    // Specialization bonus
    if agent.SpecializationScore > 0.8 {
        score += 0.05
    }

    // Verification bonus
    if agent.IsVerified {
        score += 0.05
    }

    return math.Min(score, 1.0)  // Cap at 1.0
}
```

---

## 2. AGENT ACCESS CONTROL & PREFERENCES

### User-Defined Agent Settings

```json
{
  "agent_id": "agent-abc-123",
  "access_control": {
    "visibility": "public",  // "public", "private", "verified-only", "whitelist"
    "allowed_users": ["user-xyz", "user-abc"],  // For whitelist mode
    "minimum_user_reputation": 0.7,  // Block low-reputation users
    "require_verification": true,  // Only verified developers
    "blacklist": ["user-bad-actor"]
  },

  "availability": {
    "max_concurrent_tasks": 10,
    "rate_limit_per_user": 100,  // Tasks per hour per user
    "rate_limit_global": 1000,   // Tasks per hour total
    "maintenance_windows": [
      {"day": "sunday", "start": "02:00", "end": "04:00"}
    ],
    "priority_users": ["user-vip-1"],  // Skip queue
    "reserve_capacity": 0.2  // Keep 20% for high-paying tasks
  },

  "pricing_tiers": {
    "default": {"per_execution": 0.01},
    "verified_devs": {"per_execution": 0.008},  // 20% discount
    "high_volume": {"per_execution": 0.006, "min_tasks": 1000}
  }
}
```

### Access Control Enforcement

```go
func CanUserAccessAgent(userID string, agentID string, task *Task) (bool, string) {
    agent := getAgent(agentID)
    user := getUser(userID)

    // 1. Check visibility
    switch agent.AccessControl.Visibility {
    case "private":
        if agent.OwnerID != userID {
            return false, "Agent is private"
        }

    case "verified-only":
        if !user.IsVerified {
            return false, "Agent requires verified users only"
        }

    case "whitelist":
        if !contains(agent.AccessControl.AllowedUsers, userID) {
            return false, "User not in agent's whitelist"
        }
    }

    // 2. Check user reputation
    if user.ReputationScore < agent.AccessControl.MinimumUserReputation {
        return false, fmt.Sprintf("User reputation too low (%.2f < %.2f)",
            user.ReputationScore, agent.AccessControl.MinimumUserReputation)
    }

    // 3. Check blacklist
    if contains(agent.AccessControl.Blacklist, userID) {
        return false, "User is blacklisted by agent owner"
    }

    // 4. Check rate limits
    if getUserTaskCount(userID, agentID, "last_hour") >= agent.Availability.RateLimitPerUser {
        return false, "Rate limit exceeded"
    }

    // 5. Check availability
    if agent.CurrentTasks >= agent.Availability.MaxConcurrentTasks {
        return false, "Agent at maximum capacity"
    }

    return true, ""
}
```

---

## 3. META-AGENT SELECTION ALGORITHM

### Problem: Multiple agents can do the same task - how to choose?

### Selection Criteria (User-configurable)

```json
{
  "task_id": "task-123",
  "capability_required": "text-summarization",
  "user_preferences": {
    "optimize_for": "quality",  // "quality", "speed", "cost", "balanced"
    "max_price": 0.05,
    "max_timeout": 60,
    "require_verified": false,
    "min_success_rate": 0.85,
    "preferred_agents": ["agent-favorite-1"],
    "excluded_agents": ["agent-bad-experience"]
  }
}
```

### Meta-Agent Selection Logic

```go
func SelectBestAgent(task *Task, userPrefs *UserPreferences) (*Agent, error) {
    // 1. Find all agents with required capability
    candidates := findAgentsByCapability(task.Capability)

    // 2. Filter by hard constraints
    candidates = filterBy(candidates, func(a *Agent) bool {
        return a.Pricing.PerExecution <= userPrefs.MaxPrice &&
               a.SuccessRate >= userPrefs.MinSuccessRate &&
               a.Status == "active" &&
               canUserAccessAgent(task.UserID, a.ID, task)
    })

    if len(candidates) == 0 {
        return nil, errors.New("no agents meet requirements")
    }

    // 3. Check user preferences
    for _, agentID := range userPrefs.PreferredAgents {
        if agent := findAgentByID(candidates, agentID); agent != nil {
            return agent, nil  // Use preferred agent if available
        }
    }

    // 4. Score and rank based on optimization goal
    scored := scoreAgents(candidates, userPrefs.OptimizeFor, task)

    // 5. Return best match
    return scored[0], nil
}

func scoreAgents(agents []*Agent, optimizeFor string, task *Task) []*Agent {
    for _, agent := range agents {
        switch optimizeFor {
        case "quality":
            agent.SelectionScore =
                agent.ResultQuality * 0.5 +
                agent.SuccessRate * 0.3 +
                agent.Reliability * 0.2

        case "speed":
            agent.SelectionScore =
                agent.AverageSpeed * 0.6 +
                agent.SuccessRate * 0.3 +
                agent.Reliability * 0.1

        case "cost":
            // Normalize price (lower is better)
            priceScore := 1.0 - (agent.Pricing.PerExecution / task.Budget)
            agent.SelectionScore =
                priceScore * 0.5 +
                agent.SuccessRate * 0.3 +
                agent.ResultQuality * 0.2

        case "balanced":
            agent.SelectionScore = agent.OverallScore
        }

        // Apply bonus for specialization
        if agent.SpecializationScore[task.Domain] > 0.8 {
            agent.SelectionScore += 0.1
        }
    }

    // Sort by score (descending)
    sort.Slice(agents, func(i, j int) bool {
        return agents[i].SelectionScore > agents[j].SelectionScore
    })

    return agents
}
```

### Dynamic Load Balancing

```go
// If top agent is overloaded, try next best
func SelectWithLoadBalancing(candidates []*Agent) *Agent {
    for _, agent := range candidates {
        if agent.CurrentTasks < agent.MaxConcurrentTasks {
            return agent
        }
    }

    // All busy - use least loaded
    sort.Slice(candidates, func(i, j int) bool {
        loadI := float64(candidates[i].CurrentTasks) / float64(candidates[i].MaxConcurrentTasks)
        loadJ := float64(candidates[j].CurrentTasks) / float64(candidates[j].MaxConcurrentTasks)
        return loadI < loadJ
    })

    return candidates[0]
}
```

---

## 4. AGENT COLLABORATION & DATA TRANSFER

### Problem: Multi-step tasks require agents to work together and share data

### 4.1 Collaboration Types

```go
type CollaborationType string

const (
    Sequential  CollaborationType = "sequential"  // A → B → C (pipeline)
    Parallel    CollaborationType = "parallel"    // A + B + C → Merge
    Hierarchical CollaborationType = "hierarchical" // Meta-agent coordinates sub-agents
    PeerToPeer  CollaborationType = "p2p"         // Agents negotiate directly
)
```

### 4.2 Task Decomposition & Orchestration

**Example: Research Task**
```
User Query: "Research quantum computing trends and create a market analysis report"

Meta-Agent Decomposition:
├─ Subtask 1: Web Search (Agent: SearchBot)
│  └─ Output: List of 50 relevant articles
│
├─ Subtask 2: Content Extraction (Agent: ScraperBot) [waits for Subtask 1]
│  └─ Input: URLs from Subtask 1
│  └─ Output: Full text of articles
│
├─ Subtask 3: Summarization (Agent: SummarizerBot) [waits for Subtask 2]
│  └─ Input: Full articles from Subtask 2
│  └─ Output: Key insights
│
├─ Subtask 4: Trend Analysis (Agent: AnalystBot) [waits for Subtask 3]
│  └─ Input: Summaries from Subtask 3
│  └─ Output: Trend data + statistics
│
└─ Subtask 5: Report Generation (Agent: WriterBot) [waits for Subtask 4]
   └─ Input: Analysis from Subtask 4
   └─ Output: Final PDF report
```

### 4.3 Data Transfer Protocol

**Efficient Inter-Agent Communication**:

```go
type InterAgentMessage struct {
    MessageID     string
    FromAgentID   string
    ToAgentID     string
    TaskID        string
    MessageType   string  // "data", "request", "control"

    // Data Transfer
    Data          interface{}  // Small data inline
    DataURL       string       // Large data via R2 (presigned URL)
    DataHash      string       // SHA256 for verification
    DataSize      int64        // Bytes

    // Metadata
    CreatedAt     time.Time
    ExpiresAt     time.Time
    Priority      int
    Retries       int
}
```

**Transfer Strategy**:

```go
func TransferDataBetweenAgents(from, to *Agent, data interface{}) error {
    dataSize := calculateSize(data)

    if dataSize < 1*MB {
        // Small data: Inline in message queue
        message := &InterAgentMessage{
            MessageType: "data",
            Data: data,
            DataSize: dataSize,
        }
        return sendViaQueue(message)

    } else {
        // Large data: Upload to R2, send presigned URL
        url, err := uploadToR2(data, fmt.Sprintf("agent-transfers/%s/%s",
            taskID, uuid.New()))
        if err != nil {
            return err
        }

        message := &InterAgentMessage{
            MessageType: "data",
            DataURL: url,
            DataHash: sha256(data),
            DataSize: dataSize,
        }
        return sendViaQueue(message)
    }
}
```

### 4.4 Execution Order Determination

**Dependency Graph**:

```go
type TaskDAG struct {
    Nodes map[string]*SubTask
    Edges map[string][]string  // NodeID → Dependencies
}

type SubTask struct {
    ID            string
    AgentID       string
    Capability    string
    Input         *DataReference
    Output        *DataReference
    Status        string  // "pending", "running", "completed", "failed"
    Dependencies  []string  // List of SubTask IDs that must complete first
    EstimatedTime int       // Seconds
    Priority      int
}
```

**Execution Algorithm**:

```go
func ExecuteTaskDAG(dag *TaskDAG) error {
    // 1. Topological sort to determine execution order
    executionOrder := topologicalSort(dag)

    // 2. Execute in order, respecting dependencies
    for _, nodeID := range executionOrder {
        subtask := dag.Nodes[nodeID]

        // Wait for dependencies
        for _, depID := range subtask.Dependencies {
            if dag.Nodes[depID].Status != "completed" {
                waitForCompletion(depID)
            }
        }

        // Execute subtask
        agent := getAgent(subtask.AgentID)
        result, err := executeSubTask(agent, subtask)
        if err != nil {
            return handleFailure(subtask, err)
        }

        // Store output for downstream tasks
        subtask.Output = storeResult(result)
        subtask.Status = "completed"
    }

    return nil
}
```

**Parallel Execution Optimization**:

```go
func ExecuteParallel(dag *TaskDAG) error {
    // Find all tasks with no dependencies → Execute immediately
    ready := findReadyTasks(dag)

    var wg sync.WaitGroup
    errorChan := make(chan error, len(ready))

    for _, task := range ready {
        wg.Add(1)
        go func(t *SubTask) {
            defer wg.Done()
            if err := executeSubTask(getAgent(t.AgentID), t); err != nil {
                errorChan <- err
            }
        }(task)
    }

    wg.Wait()
    close(errorChan)

    // Check for errors
    if len(errorChan) > 0 {
        return <-errorChan
    }

    // Continue with next batch
    return ExecuteParallel(dag)
}
```

### 4.5 Data Caching & Deduplication

**Problem**: Multiple agents might request the same data (e.g., "latest Bitcoin price")

**Solution**: Shared cache with TTL

```go
type AgentDataCache struct {
    Key       string
    Data      interface{}
    CreatedAt time.Time
    TTL       time.Duration
    AccessCount int64
    Agents    []string  // List of agents that cached this
}

func GetOrFetch(key string, fetchFn func() (interface{}, error)) (interface{}, error) {
    // Check cache first
    if cached, found := cache.Get(key); found {
        if time.Since(cached.CreatedAt) < cached.TTL {
            cached.AccessCount++
            return cached.Data, nil
        }
    }

    // Cache miss - fetch and store
    data, err := fetchFn()
    if err != nil {
        return nil, err
    }

    cache.Set(key, &AgentDataCache{
        Key: key,
        Data: data,
        CreatedAt: time.Now(),
        TTL: 5 * time.Minute,
    })

    return data, nil
}
```

---

## 5. EXAMPLE: COMPLETE WORKFLOW

### User Request
```
"Find the best hotel in Paris for June 15-20, under $200/night, near Eiffel Tower"
```

### Meta-Agent Breaks Down Task

```json
{
  "task_id": "hotel-search-123",
  "subtasks": [
    {
      "id": "st-1",
      "capability": "hotel-search",
      "agent_selection": {
        "candidates": ["BookingBot", "ExpediaBot", "HotelFinderPro"],
        "optimize_for": "quality",
        "max_price": 0.05
      },
      "selected_agent": "HotelFinderPro",  // OverallScore: 0.92
      "reason": "Highest quality (0.94) with specialization in Paris hotels",
      "dependencies": [],
      "estimated_time": 15
    },
    {
      "id": "st-2",
      "capability": "price-comparison",
      "depends_on": ["st-1"],
      "agent_selection": {
        "candidates": ["PriceBot", "DealFinder"],
        "optimize_for": "speed"
      },
      "selected_agent": "DealFinder",  // AverageSpeed: 0.88
      "reason": "Fastest with good success rate (0.91)",
      "dependencies": ["st-1"],
      "estimated_time": 10
    },
    {
      "id": "st-3",
      "capability": "review-analysis",
      "depends_on": ["st-1"],
      "parallel_with": ["st-2"],
      "selected_agent": "ReviewAnalyzerBot",
      "reason": "Specializes in sentiment analysis (0.96 specialization score)",
      "dependencies": ["st-1"],
      "estimated_time": 20
    },
    {
      "id": "st-4",
      "capability": "recommendation",
      "depends_on": ["st-2", "st-3"],
      "selected_agent": "RecommendBot",
      "reason": "Best at synthesizing multi-source data (0.93 quality)",
      "dependencies": ["st-2", "st-3"],
      "estimated_time": 5
    }
  ],

  "execution_plan": {
    "total_estimated_time": 50,  // Some parallel execution
    "total_cost": 0.18,
    "agents_involved": 4,
    "data_transfers": 3
  }
}
```

### Execution Flow

```
Time 0s:  st-1 (HotelFinderPro) starts → Searching hotels
          ↓
Time 15s: st-1 completes → Uploads 2MB hotel data to R2
          ↓ (sends presigned URL to both st-2 and st-3)
          ├─→ st-2 (DealFinder) starts [parallel]
          └─→ st-3 (ReviewAnalyzer) starts [parallel]

Time 25s: st-2 completes → Price comparison data ready
Time 35s: st-3 completes → Review sentiment ready
          ↓ (both send data to st-4)
Time 40s: st-4 (RecommendBot) starts → Synthesizing
Time 45s: st-4 completes → Final recommendation ready

Total: 45 seconds (vs 50 seconds if sequential)
```

### Data Transfer Log

```json
{
  "transfers": [
    {
      "from": "HotelFinderPro",
      "to": "DealFinder",
      "size_bytes": 2048576,
      "method": "r2_presigned_url",
      "url": "https://zerostate-agents.r2.../hotel-data-abc.json?expires=3600",
      "time_ms": 120
    },
    {
      "from": "HotelFinderPro",
      "to": "ReviewAnalyzer",
      "size_bytes": 2048576,
      "method": "r2_presigned_url",
      "url": "https://zerostate-agents.r2.../hotel-data-abc.json?expires=3600",
      "time_ms": 15  // Cache hit! Same data
    },
    {
      "from": "DealFinder",
      "to": "RecommendBot",
      "size_bytes": 45120,
      "method": "inline_queue",
      "time_ms": 5
    },
    {
      "from": "ReviewAnalyzer",
      "to": "RecommendBot",
      "size_bytes": 12800,
      "method": "inline_queue",
      "time_ms": 3
    }
  ]
}
```

---

## 6. IMPLEMENTATION PRIORITIES

### Phase 1: Basic Reputation (Week 1-2)
- Success rate tracking
- Speed measurement
- Simple user ratings

### Phase 2: Agent Selection (Week 3)
- Meta-agent routing logic
- Access control enforcement
- Load balancing

### Phase 3: Collaboration (Week 4-5)
- Task decomposition
- DAG execution engine
- Data transfer protocol

### Phase 4: Advanced Features (Week 6+)
- Peer verification
- Specialization scoring
- Fraud detection
- Advanced analytics

---

## Summary

**Quality Determination**: Multi-dimensional automated scoring + user ratings + peer verification

**Agent Selection**: Smart routing based on user preferences (quality/speed/cost) + agent reputation

**Collaboration**: DAG-based orchestration with efficient data transfer via R2 + inline queue

**Access Control**: Granular permissions (public/private/verified/whitelist) with rate limiting

**This creates a self-regulating marketplace where quality agents naturally rise to the top!** 🚀
