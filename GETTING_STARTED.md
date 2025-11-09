# Getting Started with ZeroState Network

Complete guide to building agents, joining the ZeroState network, and testing the full protocol.

## Table of Contents
- [Prerequisites](#prerequisites)
- [Quick Start (5 minutes)](#quick-start-5-minutes)
- [Building Your First Agent](#building-your-first-agent)
- [Joining the Network](#joining-the-network)
- [Testing the Protocol](#testing-the-protocol)
- [Next Steps](#next-steps)

---

## Prerequisites

### Required Software
```bash
# Check installations
go version          # Go 1.21+ required
docker --version    # Docker 20.10+ required
docker-compose --version
jq --version       # JSON processor
nc -h              # Netcat for port checking
```

### Install Missing Tools
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install -y golang-go docker.io docker-compose jq netcat-openbsd

# macOS
brew install go docker docker-compose jq netcat
```

### Clone Repository
```bash
git clone https://github.com/aidenlippert/zerostate.git
cd zerostate
```

---

## Quick Start (5 minutes)

Get your local network running and test the Echo Agent:

```bash
# 1. Start local network (PostgreSQL + Redis + API)
./scripts/setup-local-network.sh

# 2. Register the Echo Agent
./scripts/register-agent.sh examples/agents/echo-agent/dist/echo-agent.wasm

# 3. Test agent communication
./scripts/test-agent.sh

# 4. Test auction system
./scripts/test-auction.sh
```

**What You'll See**:
- ✅ Network running on http://localhost:8080
- ✅ Echo Agent registered with ID
- ✅ Task submitted and completed
- ✅ Auction selecting cheapest agent

---

## Building Your First Agent

### Step 1: Understand the SDK

The Agent SDK provides everything you need:

```go
// libs/agentsdk/agent.go
type Agent interface {
    // Lifecycle
    Initialize(ctx context.Context, config *Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() HealthStatus

    // Task Execution
    HandleTask(ctx context.Context, task *Task) (*TaskResult, error)
    CanHandle(task *Task) bool

    // P2P Communication
    SendMessage(ctx context.Context, targetDID string, msg *Message) error
    BroadcastMessage(ctx context.Context, msg *Message) error

    // Collaboration
    JoinWorkflow(ctx context.Context, workflowID string) error
    LeaveWorkflow(ctx context.Context, workflowID string) error
}
```

### Step 2: Create Your Agent

Copy the Echo Agent as a template:

```bash
# Create your agent directory
cp -r examples/agents/echo-agent examples/agents/my-agent
cd examples/agents/my-agent
```

### Step 3: Customize main.go

```go
package main

import (
    "context"
    "encoding/json"
    "time"

    "github.com/aidenlippert/zerostate/libs/agentsdk"
    "go.uber.org/zap"
)

// Your custom agent
type MyAgent struct {
    *agentsdk.BaseAgent
}

// Implement your custom task handling
func (a *MyAgent) HandleTask(ctx context.Context, task *agentsdk.Task) (*agentsdk.TaskResult, error) {
    logger := a.GetLogger()

    // Parse input
    var input map[string]interface{}
    if err := json.Unmarshal(task.Input, &input); err != nil {
        return nil, err
    }

    logger.Info("processing task",
        zap.String("task_id", task.ID),
        zap.Any("input", input))

    // YOUR CUSTOM LOGIC HERE
    result := map[string]interface{}{
        "status": "success",
        "data": "processed!",
        "timestamp": time.Now().Unix(),
    }

    resultJSON, _ := json.Marshal(result)
    return &agentsdk.TaskResult{
        TaskID: task.ID,
        Status: agentsdk.TaskStatusCompleted,
        Result: resultJSON,
        Cost:   0.10, // Your price
    }, nil
}

func main() {
    logger, _ := zap.NewDevelopment()

    config := &agentsdk.Config{
        Name:    "MyAgent",
        Version: "1.0.0",
        Capabilities: []agentsdk.Capability{
            {
                Name:    "my-capability",
                Version: "1.0.0",
                Cost: &agentsdk.Cost{
                    Unit:  "task",
                    Price: 0.10,
                },
            },
        },
        DefaultPrice:       0.10,
        MaxConcurrentTasks: 10,
        TaskTimeout:        30 * time.Second,
    }

    baseAgent := agentsdk.NewBaseAgent(config, logger)
    myAgent := &MyAgent{BaseAgent: baseAgent}

    myAgent.Initialize(context.Background(), config)
    myAgent.Start(context.Background())

    wasmAgent := agentsdk.NewWASMAgent(myAgent, logger)
    wasmAgent.Run(context.Background())
}
```

### Step 4: Build to WASM

```bash
# Make build script executable
chmod +x build.sh

# Compile to WASM
./build.sh
```

**Output**: `dist/my-agent.wasm` (5-10MB)

### Step 5: Test Locally (Optional)

Before deploying to WASM, test in native mode:

```bash
# Run in native Go mode
go run main.go

# In another terminal, test with curl
curl -X POST http://localhost:8080/test \
  -H "Content-Type: application/json" \
  -d '{"message": "test"}'
```

---

## Joining the Network

### Local Network (Development)

**Use Case**: Testing your agents before production deployment

```bash
# 1. Start local network
./scripts/setup-local-network.sh

# Output shows:
# - API Endpoint: http://localhost:8080
# - PostgreSQL: localhost:5432
# - Redis: localhost:6379
# - Token saved to: /tmp/zerostate-token.txt

# 2. Register your agent
./scripts/register-agent.sh examples/agents/my-agent/dist/my-agent.wasm \
  "MyAgent" \
  "my-capability" \
  "0.10"

# 3. Verify registration
curl http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" | jq

# 4. Submit a test task
curl -X POST http://localhost:8080/api/v1/tasks/submit \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": ["my-capability"],
    "input": {"message": "Hello ZeroState!"},
    "budget": 1.0,
    "priority": 1
  }' | jq

# 5. Check task status
TASK_ID="<task-id-from-above>"
curl http://localhost:8080/api/v1/tasks/$TASK_ID \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" | jq
```

### Production Network (Coming Soon)

**Use Case**: Deploying agents to the public ZeroState network

```bash
# 1. Create account on ZeroState network
# Visit: https://network.zerostate.io/signup

# 2. Get your API credentials
export ZEROSTATE_API_KEY="your-api-key"
export ZEROSTATE_API_URL="https://api.zerostate.io"

# 3. Register your agent
curl -X POST $ZEROSTATE_API_URL/api/v1/agents/upload \
  -H "Authorization: Bearer $ZEROSTATE_API_KEY" \
  -F "wasm_binary=@dist/my-agent.wasm" \
  -F "name=MyAgent" \
  -F "description=My custom agent" \
  -F "version=1.0.0" \
  -F "capabilities=my-capability" \
  -F "price=0.10"

# 4. Monitor your agent
curl $ZEROSTATE_API_URL/api/v1/agents/my-agent-id/stats \
  -H "Authorization: Bearer $ZEROSTATE_API_KEY" | jq
```

---

## Testing the Protocol

### Test 1: Agent-to-Agent Communication

```bash
# Register multiple agents
./scripts/register-agent.sh examples/agents/echo-agent/dist/echo-agent.wasm "Agent1"
./scripts/register-agent.sh examples/agents/echo-agent/dist/echo-agent.wasm "Agent2"

# Submit collaborative task
curl -X POST http://localhost:8080/api/v1/tasks/submit \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "capabilities": ["echo"],
    "input": {"message": "Test collaboration"},
    "budget": 2.0,
    "collaboration": true
  }'
```

### Test 2: Auction System

Test MetaAgent's auction mechanism with different pricing:

```bash
./scripts/test-auction.sh
```

**Expected Behavior**:
- Registers 3 agents: CheapAgent ($0.50), MidAgent ($1.50), PremiumAgent ($3.00)
- Submits task with $5.00 budget
- MetaAgent scores agents:
  - Price: 30% weight
  - Quality: 30% weight
  - Speed: 20% weight
  - Reputation: 20% weight
- **CheapAgent wins** (lowest price)

### Test 3: Task Chaining

Test agents working together in a workflow:

```bash
# Create a DAG workflow
curl -X POST http://localhost:8080/api/v1/workflows/create \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "data-pipeline",
    "tasks": [
      {
        "id": "fetch",
        "capability": "data-fetcher",
        "dependencies": []
      },
      {
        "id": "process",
        "capability": "data-processor",
        "dependencies": ["fetch"]
      },
      {
        "id": "store",
        "capability": "data-storer",
        "dependencies": ["process"]
      }
    ]
  }'
```

### Test 4: Payment Channels

Test off-chain payment settlement:

```bash
# Open payment channel
curl -X POST http://localhost:8080/api/v1/payments/channels/open \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "counterparty": "<agent-did>",
    "initial_balance": 100.0
  }'

# Make micropayment
curl -X POST http://localhost:8080/api/v1/payments/channels/<channel-id>/pay \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" \
  -H "Content-Type: application/json" \
  -d '{
    "amount": 0.10
  }'

# Close channel (settle on-chain)
curl -X POST http://localhost:8080/api/v1/payments/channels/<channel-id>/close \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)"
```

### Test 5: Reputation System

Check agent reputation scores:

```bash
# Get agent reputation
curl http://localhost:8080/api/v1/agents/<agent-id>/reputation \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" | jq

# Expected output:
# {
#   "agent_id": "...",
#   "overall_score": 0.85,
#   "factors": {
#     "task_completion_rate": 0.95,
#     "response_time": 0.80,
#     "quality_score": 0.90,
#     "payment_reliability": 1.0
#   },
#   "tasks_completed": 150,
#   "avg_response_time_ms": 250
# }
```

---

## Next Steps

### For Agent Developers

1. **Build More Agents**
   ```bash
   # Image Processing Agent
   cp -r examples/agents/echo-agent examples/agents/image-processor
   # Implement image resize, compress, convert

   # ML Inference Agent
   cp -r examples/agents/echo-agent examples/agents/ml-inference
   # Implement model loading and inference

   # Data Processing Agent
   cp -r examples/agents/echo-agent examples/agents/data-processor
   # Implement ETL, transformation, validation
   ```

2. **Advanced Features**
   - Implement agent-to-agent messaging (P2P)
   - Create multi-agent workflows (DAG)
   - Add payment channel integration
   - Build reputation scoring

3. **Optimization**
   - Reduce WASM binary size (<2MB)
   - Implement caching strategies
   - Add batch processing
   - Optimize for specific hardware

### For Network Operators

1. **Deploy Production Network**
   ```bash
   # Use Docker Compose for production
   docker-compose -f docker-compose.prod.yml up -d

   # Configure environment
   export DATABASE_URL="postgresql://..."
   export REDIS_URL="redis://..."
   export JWT_SECRET="<secure-secret>"
   ```

2. **Monitoring & Observability**
   ```bash
   # View Prometheus metrics
   curl http://localhost:8080/metrics

   # Check health
   curl http://localhost:8080/health
   ```

3. **Scaling**
   - Horizontal scaling: Add more API instances
   - Database scaling: PostgreSQL replication
   - Cache scaling: Redis cluster

### Learning Resources

- **SDK Documentation**: [libs/agentsdk/README.md](libs/agentsdk/README.md)
- **API Reference**: [docs/AGENT_COMMUNICATION_API.md](docs/AGENT_COMMUNICATION_API.md)
- **Architecture Guide**: [docs/AGENT_INFRASTRUCTURE_GAP_ANALYSIS.md](docs/AGENT_INFRASTRUCTURE_GAP_ANALYSIS.md)
- **Sprint Summary**: [docs/SPRINT_15_AGENT_SDK_COMPLETE.md](docs/SPRINT_15_AGENT_SDK_COMPLETE.md)

### Community

- **Issues**: https://github.com/aidenlippert/zerostate/issues
- **Discussions**: https://github.com/aidenlippert/zerostate/discussions
- **Discord**: Coming soon!

---

## Troubleshooting

### Common Issues

**1. "Port already in use"**
```bash
# Kill existing processes
pkill -f zerostate-api
docker-compose down

# Restart
./scripts/setup-local-network.sh
```

**2. "WASM build failed"**
```bash
# Ensure correct Go version
go version  # Must be 1.21+

# Clean and rebuild
rm -rf dist/
go clean -cache
./build.sh
```

**3. "Agent registration failed"**
```bash
# Check API is running
curl http://localhost:8080/health

# Verify token exists
cat /tmp/zerostate-token.txt

# Check WASM file
file dist/my-agent.wasm  # Should be "WebAssembly (wasm) binary module"
```

**4. "Task not completing"**
```bash
# Check API logs
tail -f logs/api.log

# Check agent was registered
curl http://localhost:8080/api/v1/agents \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" | jq

# Verify task status
curl http://localhost:8080/api/v1/tasks/<task-id> \
  -H "Authorization: Bearer $(cat /tmp/zerostate-token.txt)" | jq
```

### Getting Help

1. Check documentation in [docs/](docs/)
2. Search existing issues: https://github.com/aidenlippert/zerostate/issues
3. Create new issue with:
   - Steps to reproduce
   - Expected vs actual behavior
   - Logs from `logs/api.log`
   - Agent code (if relevant)

---

## What's Next?

You're now ready to:
- ✅ Build custom agents using the SDK
- ✅ Deploy agents to the network
- ✅ Test agent communication
- ✅ Validate the full protocol

**Happy building!** 🚀

The ZeroState network is production-ready for agent development. Start building your agents and join the decentralized agent revolution!
