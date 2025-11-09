# ZeroState Agent SDK

Build intelligent agents for the ZeroState decentralized agent network.

## Features

- **Simple API** - Easy-to-use base classes and interfaces
- **WASM Support** - Compile agents to WebAssembly for network deployment
- **Task Management** - Built-in task execution, tracking, and error handling
- **Communication** - Agent-to-agent messaging and collaboration
- **Monitoring** - Health checks, metrics, and logging

## Quick Start

### 1. Create Your Agent

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/aidenlippert/zerostate/libs/agentsdk"
    "go.uber.org/zap"
)

// MyAgent implements the Agent interface
type MyAgent struct {
    *agentsdk.BaseAgent
}

// HandleTask implements custom task logic
func (a *MyAgent) HandleTask(ctx context.Context, task *agentsdk.Task) (*agentsdk.TaskResult, error) {
    // Your business logic here
    var input map[string]interface{}
    json.Unmarshal(task.Input, &input)

    result := map[string]interface{}{
        "message": "Task completed!",
        "input":   input,
    }

    resultJSON, _ := json.Marshal(result)

    return &agentsdk.TaskResult{
        TaskID: task.ID,
        Status: agentsdk.TaskStatusCompleted,
        Result: resultJSON,
        Cost:   1.0,
    }, nil
}

func main() {
    logger, _ := zap.NewDevelopment()

    config := &agentsdk.Config{
        Name:        "MyAgent",
        Description: "My first ZeroState agent",
        Version:     "1.0.0",
        Capabilities: []agentsdk.Capability{
            {
                Name:    "data-processing",
                Version: "1.0.0",
                Cost:    &agentsdk.Cost{Unit: "task", Price: 1.0},
            },
        },
        MaxConcurrentTasks: 5,
    }

    // Create agent
    baseAgent := agentsdk.NewBaseAgent(config, logger)
    myAgent := &MyAgent{BaseAgent: baseAgent}

    // Initialize and start
    ctx := context.Background()
    myAgent.Initialize(ctx, config)
    myAgent.Start(ctx)

    // For WASM deployment
    wasmAgent := agentsdk.NewWASMAgent(myAgent, logger)
    wasmAgent.Run(ctx)
}
```

### 2. Build for WASM

```bash
GOOS=wasip1 GOARCH=wasm go build -o agent.wasm main.go
```

### 3. Register on Network

```bash
curl -X POST http://localhost:8080/api/v1/agents/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "wasm_binary=@agent.wasm" \
  -F "name=MyAgent" \
  -F "description=My first agent" \
  -F "version=1.0.0" \
  -F "capabilities=data-processing" \
  -F "price=1.00"
```

## API Reference

### Agent Interface

```go
type Agent interface {
    // Identity
    GetDID() string
    GetName() string
    GetCapabilities() []Capability
    GetVersion() string

    // Lifecycle
    Initialize(ctx context.Context, config *Config) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Health() HealthStatus

    // Task Execution
    HandleTask(ctx context.Context, task *Task) (*TaskResult, error)
    CanHandle(task *Task) bool

    // Communication
    SendMessage(ctx context.Context, targetDID string, msg *Message) error
    BroadcastMessage(ctx context.Context, msg *Message) error

    // Collaboration
    JoinWorkflow(ctx context.Context, workflowID string) error
    LeaveWorkflow(ctx context.Context, workflowID string) error
}
```

### BaseAgent

Provides common functionality for all agents:
- Task tracking and execution
- Health monitoring
- Heartbeat management
- Message handling (when bus configured)
- Workflow management

### Task Types

```go
type Task struct {
    ID           string          // Unique task ID
    Type         string          // Task type identifier
    Capabilities []string        // Required capabilities
    Input        json.RawMessage // Task input data
    Budget       float64         // Maximum cost
    Priority     int             // Priority (0-3)
    Deadline     time.Time       // Execution deadline
    Metadata     map[string]interface{}
}

type TaskResult struct {
    TaskID       string          // Task ID
    Status       TaskStatus      // PENDING, RUNNING, COMPLETED, FAILED
    Result       json.RawMessage // Result data
    Error        string          // Error message if failed
    Cost         float64         // Actual cost
    StartedAt    time.Time
    CompletedAt  time.Time
    ProofOfWork  string          // Optional PoW
}
```

### Configuration

```go
type Config struct {
    // Identity
    DID         string       // Agent DID (auto-generated if empty)
    Name        string       // Agent name
    Description string       // Agent description
    Version     string       // Agent version
    Capabilities []Capability // Agent capabilities

    // Network
    NetworkEndpoint string   // Network endpoint
    Region          string   // Geographic region
    BootstrapPeers  []string // Bootstrap peers

    // Pricing
    DefaultPrice float64 // Default price per task
    MinBudget    float64 // Minimum budget to accept

    // Performance
    MaxConcurrentTasks int           // Max concurrent tasks
    TaskTimeout        time.Duration // Task timeout
    HeartbeatInterval  time.Duration // Heartbeat interval

    // Logging
    LogLevel string // Log level (debug, info, warn, error)

    // Custom settings
    Custom map[string]interface{}
}
```

## Examples

See [examples/agents/](../../examples/agents/) for complete examples:

- **echo-agent/** - Simple echo agent for testing
- **image-processor/** - Image processing agent
- **ml-inference/** - Machine learning inference agent
- **data-processor/** - Data processing pipeline agent

## Building Agents

### For Local Testing

```bash
go build -o agent ./main.go
./agent
```

### For WASM Deployment

```bash
# Install WASM target
go install golang.org/dl/go1.24@latest
go1.24 download

# Build WASM binary
GOOS=wasip1 GOARCH=wasm go build -o agent.wasm main.go

# Verify WASM binary
file agent.wasm  # Should show "WebAssembly (wasm) binary module"
```

## Testing

```go
func TestMyAgent(t *testing.T) {
    logger, _ := zap.NewDevelopment()
    config := &agentsdk.Config{
        Name:    "TestAgent",
        Version: "1.0.0",
        Capabilities: []agentsdk.Capability{
            {Name: "test", Version: "1.0.0"},
        },
    }

    agent := &MyAgent{BaseAgent: agentsdk.NewBaseAgent(config, logger)}
    ctx := context.Background()

    // Initialize
    err := agent.Initialize(ctx, config)
    assert.NoError(t, err)

    // Test task execution
    task := &agentsdk.Task{
        ID:           "test-task",
        Type:         "test",
        Capabilities: []string{"test"},
        Input:        json.RawMessage(`{"test": "data"}`),
        Budget:       10.0,
    }

    result, err := agent.HandleTask(ctx, task)
    assert.NoError(t, err)
    assert.Equal(t, agentsdk.TaskStatusCompleted, result.Status)
}
```

## Advanced Features

### Custom Message Handlers

```go
// Implement MessageBus interface for custom communication
type MyMessageBus struct {
    // Your implementation
}

func (b *MyMessageBus) Send(ctx context.Context, msg *agentsdk.Message) error {
    // Custom send logic
}

// Set on agent
agent.SetMessageBus(myMessageBus)
```

### Workflow Participation

```go
// Join collaborative workflow
err := agent.JoinWorkflow(ctx, "workflow-123")

// Coordinate with other agents
msg := &agentsdk.Message{
    Type:    agentsdk.MessageTypeCoordination,
    Payload: json.RawMessage(`{"status": "ready"}`),
}
agent.BroadcastMessage(ctx, msg)

// Leave workflow
agent.LeaveWorkflow(ctx, "workflow-123")
```

### Task Execution Wrapper

```go
// Use ExecuteTask for automatic tracking and error handling
result, err := agent.ExecuteTask(ctx, task, func(ctx context.Context, task *agentsdk.Task) (*agentsdk.TaskResult, error) {
    // Your task logic here
    return &agentsdk.TaskResult{
        Status: agentsdk.TaskStatusCompleted,
        Result: json.RawMessage(`{"success": true}`),
    }, nil
})
```

## Best Practices

1. **Error Handling** - Always return descriptive errors in TaskResult
2. **Timeouts** - Respect task deadlines and configure reasonable TaskTimeout
3. **Resource Limits** - Set MaxConcurrentTasks based on your resources
4. **Logging** - Use structured logging with zap for debugging
5. **Testing** - Test agents locally before deploying to network
6. **Versioning** - Use semantic versioning for agents
7. **Capabilities** - Be specific about what your agent can do
8. **Pricing** - Set fair prices based on computational cost

## Troubleshooting

### WASM Build Issues

```bash
# Ensure correct Go version
go version  # Should be 1.21+

# Clean build cache
go clean -cache

# Rebuild with verbose output
GOOS=wasip1 GOARCH=wasm go build -v -o agent.wasm main.go
```

### Agent Not Receiving Tasks

1. Check agent is registered: `curl http://localhost:8080/api/v1/agents`
2. Verify capabilities match task requirements
3. Check budget is sufficient
4. Review agent logs for errors

### Performance Issues

1. Increase `MaxConcurrentTasks` for higher throughput
2. Optimize task handler logic
3. Use goroutines for parallel processing
4. Profile with `go tool pprof`

## Support

- Documentation: [docs/](../../docs/)
- Examples: [examples/agents/](../../examples/agents/)
- Issues: https://github.com/aidenlippert/zerostate/issues

## License

See [LICENSE](../../LICENSE)
