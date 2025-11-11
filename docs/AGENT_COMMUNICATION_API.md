# Agent Communication API Guide

**Version**: 1.0
**Date**: January 2025
**Status**: Production Ready

---

## Table of Contents

1. [Overview](#overview)
2. [Agent Messaging](#agent-messaging)
3. [Task Chaining](#task-chaining)
4. [DAG Workflows](#dag-workflows)
5. [Coordination Primitives](#coordination-primitives)
6. [Complete Examples](#complete-examples)
7. [Best Practices](#best-practices)
8. [Troubleshooting](#troubleshooting)

---

## Overview

The ZeroState Agent Communication API enables **decentralized multi-agent collaboration** through:
- **P2P Messaging**: Direct agent-to-agent communication via libp2p
- **Task Chaining**: Sequential multi-agent workflows
- **DAG Workflows**: Parallel execution with dependency management
- **Coordination**: Distributed locks, shared state, and barriers

### Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Agent A   │◄───────►│ MessageBus  │◄───────►│   Agent B   │
└─────────────┘         │  (GossipSub)│         └─────────────┘
                        └─────────────┘
                              │
                              ▼
                     ┌─────────────────┐
                     │   P2P Network   │
                     │  (libp2p DHT)   │
                     └─────────────────┘
```

---

## Agent Messaging

### Message Types

```go
const (
    MessageTypeRequest      = "REQUEST"      // Request task execution
    MessageTypeResponse     = "RESPONSE"     // Response to request
    MessageTypeBroadcast    = "BROADCAST"    // Broadcast to all agents
    MessageTypeNegotiation  = "NEGOTIATION"  // Auction/bidding
    MessageTypeCoordination = "COORDINATION" // Workflow coordination
    MessageTypeHeartbeat    = "HEARTBEAT"    // Health check
    MessageTypeAck          = "ACK"          // Acknowledgment
)
```

### Delivery Guarantees

```go
const (
    DeliveryBestEffort   = "BEST_EFFORT"    // No guarantee (fastest)
    DeliveryAtLeastOnce  = "AT_LEAST_ONCE"  // Requires ACK
    DeliveryExactlyOnce  = "EXACTLY_ONCE"   // Deduplication + ACK
)
```

### Creating a MessageBus

```go
import (
    "github.com/aidenlippert/zerostate/libs/p2p"
    "go.uber.org/zap"
)

// Create message bus
messageBus := p2p.NewMessageBus(gossipService, "agent_123_did", logger)

// Register handler for incoming requests
messageBus.RegisterHandler(p2p.MessageTypeRequest, func(msg *p2p.AgentMessage) error {
    // Handle incoming task request
    var taskReq p2p.TaskRequest
    json.Unmarshal(msg.Payload, &taskReq)

    // Execute task...
    result := executeTask(taskReq)

    // Send response
    response := &p2p.TaskResponse{
        TaskID: taskReq.TaskID,
        Status: "COMPLETED",
        Result: mustMarshal(result),
    }

    return messageBus.SendResponse(ctx, msg, response)
})
```

### Sending Requests

```go
// Create task request
taskReq := &p2p.TaskRequest{
    TaskID:   "task_123",
    AgentID:  "target_agent_did",
    Input:    json.RawMessage(`{"data": "..."}`),
    Deadline: time.Now().Add(30 * time.Second),
    Budget:   10.0,
    Priority: 1,
}

// Send request and wait for response (with timeout)
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

response, err := messageBus.SendRequest(ctx, "target_agent_did", taskReq, 30*time.Second)
if err != nil {
    log.Fatal(err)
}

if response.Status == "COMPLETED" {
    fmt.Println("Task completed:", string(response.Result))
}
```

### Broadcasting Messages

```go
// Broadcast to all agents in the network
announcement := map[string]interface{}{
    "type":    "new_capability",
    "agent":   "agent_123",
    "feature": "image-processing-v2",
}

err := messageBus.Broadcast(ctx, mustMarshal(announcement), "capability_announcement")
```

### Message Priorities

```go
// Priority levels (higher = more urgent)
const (
    PriorityLow      = 0
    PriorityNormal   = 1
    PriorityHigh     = 2
    PriorityCritical = 3
)

// High-priority request
taskReq.Priority = PriorityHigh
```

---

## Task Chaining

Sequential multi-agent workflows where output of one agent becomes input to the next.

### Creating a Task Chain

```go
import "github.com/aidenlippert/zerostate/libs/orchestration"

// Create chain
chain := orchestration.NewTaskChain("user_123", "image-pipeline")
chain.TotalBudget = 10.0

// Step 1: Upload image
chain.AddStep(&orchestration.TaskChainStep{
    Name:         "upload",
    Capabilities: []string{"file-upload"},
    TaskType:     "upload-image",
    Input: map[string]interface{}{
        "source": "https://example.com/image.jpg",
    },
    Timeout: 10 * time.Second,
    Budget:  1.0,
})

// Step 2: Resize (depends on upload output)
chain.AddStep(&orchestration.TaskChainStep{
    Name:         "resize",
    Capabilities: []string{"image-processing"},
    TaskType:     "resize-image",
    InputMapping: map[string]string{
        "file_url": "image_url",  // Map upload output to resize input
    },
    Input: map[string]interface{}{
        "width":  800,
        "height": 600,
    },
    Timeout: 15 * time.Second,
    Budget:  2.0,
})

// Step 3: Compress
chain.AddStep(&orchestration.TaskChainStep{
    Name:         "compress",
    Capabilities: []string{"image-processing"},
    TaskType:     "compress-image",
    InputMapping: map[string]string{
        "image_url": "image_url",
    },
    Input: map[string]interface{}{
        "quality": 85,
    },
    Timeout: 10 * time.Second,
    Budget:  1.5,
})
```

### Executing a Chain

```go
// Create executor
executor := orchestration.NewChainExecutor(messageBus, agentSelector, logger)

// Execute chain
ctx := context.Background()
err := executor.ExecuteChain(ctx, chain)

if err != nil {
    log.Fatal("Chain failed:", err)
}

// Check results
fmt.Printf("Chain completed! Total cost: $%.2f\n", chain.TotalCost)
for i, step := range chain.Steps {
    fmt.Printf("Step %d (%s): %s\n", i+1, step.Name, step.Status)
    fmt.Printf("  Result: %+v\n", step.Result)
}
```

### Conditional Branching

```go
// Step with conditional execution
chain.AddStep(&orchestration.TaskChainStep{
    Name:         "send-notification",
    Capabilities: []string{"notification"},
    TaskType:     "send-email",
    Condition:    orchestration.BranchOnSuccess,  // Only if previous step succeeded
    Timeout:      5 * time.Second,
    Budget:       0.1,
})

chain.AddStep(&orchestration.TaskChainStep{
    Name:         "handle-error",
    Capabilities: []string{"error-handling"},
    TaskType:     "log-error",
    Condition:    orchestration.BranchOnFailure,  // Only if previous step failed
    Timeout:      5 * time.Second,
    Budget:       0.1,
})
```

### Monitoring Chain Progress

```go
// Get chain status (for long-running chains)
status, err := executor.GetChainStatus(chain.ID)
if err == nil {
    fmt.Printf("Current step: %d/%d\n", status.CurrentStep+1, len(status.Steps))
    fmt.Printf("Status: %s\n", status.Status)
}

// Cancel chain
err = executor.CancelChain(chain.ID)
```

---

## DAG Workflows

Parallel execution of independent tasks with dependency management.

### Creating a DAG Workflow

```go
// Create workflow
workflow := orchestration.NewDAGWorkflow("user_123", "data-processing")
workflow.TotalBudget = 20.0
workflow.MaxParallelism = 5  // Limit concurrent execution
workflow.Timeout = 5 * time.Minute

// Node 1: Fetch data (no dependencies)
workflow.AddNode(&orchestration.DAGNode{
    ID:           "fetch",
    Name:         "Fetch Data",
    Capabilities: []string{"data-fetch"},
    TaskType:     "fetch-data",
    Input: map[string]interface{}{
        "source": "database",
    },
    Dependencies: []string{}, // No dependencies
    Timeout:      30 * time.Second,
    Budget:       1.0,
})

// Node 2: Process CSV (depends on fetch)
workflow.AddNode(&orchestration.DAGNode{
    ID:           "process_csv",
    Name:         "Process CSV",
    Capabilities: []string{"csv-processing"},
    TaskType:     "parse-csv",
    Dependencies: []string{"fetch"},
    InputMapping: map[string]string{
        "fetch.csv_data": "input_data",  // Map fetch output to this input
    },
    Timeout: 60 * time.Second,
    Budget:  3.0,
})

// Node 3: Process JSON (depends on fetch, parallel with CSV)
workflow.AddNode(&orchestration.DAGNode{
    ID:           "process_json",
    Name:         "Process JSON",
    Capabilities: []string{"json-processing"},
    TaskType:     "parse-json",
    Dependencies: []string{"fetch"},
    InputMapping: map[string]string{
        "fetch.json_data": "input_data",
    },
    Timeout: 60 * time.Second,
    Budget:  3.0,
})

// Node 4: Aggregate (depends on both CSV and JSON)
workflow.AddNode(&orchestration.DAGNode{
    ID:           "aggregate",
    Name:         "Aggregate Results",
    Capabilities: []string{"data-aggregation"},
    TaskType:     "aggregate-data",
    Dependencies: []string{"process_csv", "process_json"},
    InputMapping: map[string]string{
        "process_csv.result":  "csv_result",
        "process_json.result": "json_result",
    },
    Timeout: 30 * time.Second,
    Budget:  2.0,
})
```

### Executing a DAG Workflow

```go
// Create executor
executor := orchestration.NewDAGExecutor(messageBus, agentSelector, logger)

// Execute workflow (parallel execution automatically managed)
ctx := context.Background()
err := executor.ExecuteDAG(ctx, workflow)

if err != nil {
    log.Fatal("Workflow failed:", err)
}

// Check results
fmt.Printf("Workflow completed! Total cost: $%.2f\n", workflow.TotalCost)
for id, node := range workflow.Nodes {
    fmt.Printf("Node %s (%s): %s\n", id, node.Name, node.Status)
}
```

### Complex DAG Example

```go
// Diamond DAG:
//      A
//     / \
//    B   C
//     \ /
//      D

workflow := orchestration.NewDAGWorkflow("user_123", "diamond")

workflow.AddNode(&orchestration.DAGNode{
    ID:           "A",
    Name:         "Fetch Source",
    Capabilities: []string{"fetch"},
    TaskType:     "fetch",
    Dependencies: []string{},
})

workflow.AddNode(&orchestration.DAGNode{
    ID:           "B",
    Name:         "Process Left",
    Capabilities: []string{"process"},
    TaskType:     "process",
    Dependencies: []string{"A"},
    InputMapping: map[string]string{"A.data": "input"},
})

workflow.AddNode(&orchestration.DAGNode{
    ID:           "C",
    Name:         "Process Right",
    Capabilities: []string{"process"},
    TaskType:     "process",
    Dependencies: []string{"A"},
    InputMapping: map[string]string{"A.data": "input"},
})

workflow.AddNode(&orchestration.DAGNode{
    ID:           "D",
    Name:         "Combine",
    Capabilities: []string{"combine"},
    TaskType:     "combine",
    Dependencies: []string{"B", "C"},
    InputMapping: map[string]string{
        "B.result": "left",
        "C.result": "right",
    },
})

// B and C will execute in parallel after A completes
// D will execute after both B and C complete
```

---

## Coordination Primitives

### Distributed Locks

```go
import "github.com/aidenlippert/zerostate/libs/orchestration"

// Create coordination service
coordService := orchestration.NewCoordinationService(messageBus, "agent_123", logger)
defer coordService.Stop()

// Acquire exclusive lock
lock, err := coordService.AcquireLock(
    ctx,
    "resource_database_write",
    orchestration.LockTypeExclusive,
    30 * time.Second,  // TTL
)
if err != nil {
    log.Fatal("Failed to acquire lock:", err)
}

// Critical section
// ... perform work on locked resource ...

// Release lock
err = coordService.ReleaseLock(lock.Token)

// Or renew lock for long-running operations
err = coordService.RenewLock(lock.Token, 30*time.Second)
```

### Shared State

```go
// Create shared state
initialState := map[string]interface{}{
    "counter":    0,
    "status":     "initialized",
    "started_at": time.Now().Unix(),
}

state, err := coordService.SetState(ctx, "workflow_state", initialState, 0)
if err != nil {
    log.Fatal(err)
}

// Update state (with optimistic locking)
updatedValues := map[string]interface{}{
    "counter": 1,
    "status":  "running",
}

state, err = coordService.SetState(ctx, "workflow_state", updatedValues, state.Version)
if err == orchestration.ErrStateConflict {
    // Another agent updated the state, retry
    currentState, _ := coordService.GetState("workflow_state")
    // ... retry with current version ...
}

// Atomic field update (with automatic retry)
_, err = coordService.UpdateState(ctx, "workflow_state", "counter", 5)
// This will retry automatically if there's a version conflict
```

### Barriers

```go
// Wait for 3 agents to reach barrier
err := coordService.WaitAtBarrier(
    ctx,
    "sync_point_1",
    3,                // Required agent count
    60 * time.Second, // Timeout
)

if err != nil {
    log.Fatal("Barrier timeout:", err)
}

// All 3 agents have reached the barrier, continue execution
fmt.Println("Barrier passed, continuing...")
```

---

## Complete Examples

### Example 1: Image Processing Pipeline

```go
func ProcessImagePipeline(imageURL string) error {
    logger, _ := zap.NewDevelopment()
    messageBus := setupMessageBus()
    agentSelector := setupAgentSelector()

    // Create chain
    chain := orchestration.NewTaskChain("user_123", "image-pipeline")

    // Upload
    chain.AddStep(&orchestration.TaskChainStep{
        Name:         "download",
        Capabilities: []string{"http-fetch"},
        TaskType:     "download-file",
        Input:        map[string]interface{}{"url": imageURL},
        Timeout:      30 * time.Second,
        Budget:       0.5,
    })

    // Resize
    chain.AddStep(&orchestration.TaskChainStep{
        Name:         "resize",
        Capabilities: []string{"image-processing"},
        TaskType:     "resize-image",
        InputMapping: map[string]string{"file_path": "image_path"},
        Input:        map[string]interface{}{"width": 1920, "height": 1080},
        Timeout:      45 * time.Second,
        Budget:       2.0,
    })

    // Compress
    chain.AddStep(&orchestration.TaskChainStep{
        Name:         "compress",
        Capabilities: []string{"image-processing"},
        TaskType:     "compress-image",
        InputMapping: map[string]string{"image_path": "image_path"},
        Input:        map[string]interface{}{"quality": 85},
        Timeout:      30 * time.Second,
        Budget:       1.5,
    })

    // Upload to CDN
    chain.AddStep(&orchestration.TaskChainStep{
        Name:         "upload_cdn",
        Capabilities: []string{"cdn-upload"},
        TaskType:     "upload-to-cdn",
        InputMapping: map[string]string{"image_path": "file_path"},
        Timeout:      60 * time.Second,
        Budget:       1.0,
    })

    // Execute
    executor := orchestration.NewChainExecutor(messageBus, agentSelector, logger)
    return executor.ExecuteChain(context.Background(), chain)
}
```

### Example 2: Parallel Data Processing

```go
func ProcessDataInParallel(dataID string) error {
    logger, _ := zap.NewDevelopment()
    messageBus := setupMessageBus()
    agentSelector := setupAgentSelector()

    workflow := orchestration.NewDAGWorkflow("user_123", "parallel-processing")
    workflow.MaxParallelism = 10

    // Fetch data
    workflow.AddNode(&orchestration.DAGNode{
        ID:           "fetch",
        Capabilities: []string{"database"},
        TaskType:     "fetch-records",
        Input:        map[string]interface{}{"dataset_id": dataID},
        Dependencies: []string{},
        Timeout:      60 * time.Second,
        Budget:       2.0,
    })

    // Process chunks in parallel (10 workers)
    for i := 0; i < 10; i++ {
        nodeID := fmt.Sprintf("process_%d", i)
        workflow.AddNode(&orchestration.DAGNode{
            ID:           nodeID,
            Capabilities: []string{"data-processing"},
            TaskType:     "process-chunk",
            Dependencies: []string{"fetch"},
            InputMapping: map[string]string{
                "fetch.records": "records",
            },
            Input: map[string]interface{}{
                "chunk_id": i,
                "chunk_size": 1000,
            },
            Timeout: 120 * time.Second,
            Budget:  5.0,
        })
    }

    // Aggregate results
    processDeps := make([]string, 10)
    for i := 0; i < 10; i++ {
        processDeps[i] = fmt.Sprintf("process_%d", i)
    }

    workflow.AddNode(&orchestration.DAGNode{
        ID:           "aggregate",
        Capabilities: []string{"aggregation"},
        TaskType:     "aggregate-results",
        Dependencies: processDeps,
        Timeout:      60 * time.Second,
        Budget:       3.0,
    })

    // Execute
    executor := orchestration.NewDAGExecutor(messageBus, agentSelector, logger)
    return executor.ExecuteDAG(context.Background(), workflow)
}
```

### Example 3: Coordinated Workflow

```go
func CoordinatedWorkflow() error {
    logger, _ := zap.NewDevelopment()
    messageBus := setupMessageBus()
    coordService := orchestration.NewCoordinationService(messageBus, "agent_main", logger)
    defer coordService.Stop()

    ctx := context.Background()

    // Create shared state for workflow
    initialState := map[string]interface{}{
        "phase":     "initializing",
        "completed": 0,
        "total":     100,
    }
    _, err := coordService.SetState(ctx, "workflow_progress", initialState, 0)
    if err != nil {
        return err
    }

    // Wait for all workers to be ready (barrier)
    err = coordService.WaitAtBarrier(ctx, "workers_ready", 5, 60*time.Second)
    if err != nil {
        return err
    }

    // Acquire lock for critical section
    lock, err := coordService.AcquireLock(ctx, "output_file", orchestration.LockTypeExclusive, 120*time.Second)
    if err != nil {
        return err
    }
    defer coordService.ReleaseLock(lock.Token)

    // Update progress atomically
    for i := 0; i < 100; i++ {
        _, err = coordService.UpdateState(ctx, "workflow_progress", "completed", i+1)
        if err != nil {
            return err
        }
        time.Sleep(100 * time.Millisecond)
    }

    // Signal completion
    _, err = coordService.UpdateState(ctx, "workflow_progress", "phase", "completed")
    return err
}
```

---

## Best Practices

### 1. Error Handling

```go
// Always handle errors and provide context
err := executor.ExecuteChain(ctx, chain)
if err != nil {
    log.Printf("Chain %s failed: %v", chain.ID, err)

    // Check which step failed
    for i, step := range chain.Steps {
        if step.Status == orchestration.StepStatusFailed {
            log.Printf("  Failed at step %d (%s): %s", i, step.Name, step.Error)
        }
    }

    return fmt.Errorf("pipeline failed: %w", err)
}
```

### 2. Timeouts

```go
// Always set appropriate timeouts
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

// Set per-step timeouts
step.Timeout = 30 * time.Second

// Set workflow-level timeout
workflow.Timeout = 10 * time.Minute
```

### 3. Budget Management

```go
// Set budgets at all levels
chain.TotalBudget = 10.0  // Total for entire chain
step.Budget = 2.0         // Per-step budget

// Monitor costs
if chain.TotalCost > chain.TotalBudget {
    log.Warn("Chain exceeded budget")
}
```

### 4. Retries

```go
// Configure retries for individual steps
step.MaxRetries = 3

// Configure chain-level retries
chain.MaxRetries = 1
```

### 5. Lock Safety

```go
// Always release locks
lock, err := coordService.AcquireLock(ctx, resource, orchestration.LockTypeExclusive, 30*time.Second)
if err != nil {
    return err
}
defer coordService.ReleaseLock(lock.Token)  // Ensure release even if panic

// For long operations, renew locks
ticker := time.NewTicker(20 * time.Second)
defer ticker.Stop()

go func() {
    for range ticker.C {
        coordService.RenewLock(lock.Token, 30*time.Second)
    }
}()
```

---

## Troubleshooting

### Chain Execution Failures

**Problem**: Chain fails at random steps
**Solution**: Check agent availability and timeout settings
```go
// Increase timeouts
step.Timeout = 60 * time.Second

// Add retry logic
step.MaxRetries = 3
```

### DAG Deadlocks

**Problem**: DAG workflow hangs indefinitely
**Cause**: Cycle in dependency graph
**Solution**: Validate DAG before execution
```go
// DAG executor automatically validates and rejects cycles
err := executor.ExecuteDAG(ctx, workflow)
if errors.Is(err, orchestration.ErrDAGCycleDetected) {
    log.Fatal("Cycle detected in workflow dependencies")
}
```

### Lock Contention

**Problem**: Cannot acquire lock
**Solution**: Use appropriate lock types and TTL
```go
// Use shared locks for read-only access
lock, err := coordService.AcquireLock(ctx, resource, orchestration.LockTypeShared, 30*time.Second)

// Set reasonable TTL
lock, err := coordService.AcquireLock(ctx, resource, orchestration.LockTypeExclusive, 60*time.Second)
```

### State Conflicts

**Problem**: Frequent version conflicts in shared state
**Solution**: Use atomic updates or reduce update frequency
```go
// Use atomic field updates (with automatic retry)
_, err := coordService.UpdateState(ctx, key, field, value)

// Or batch updates
newState := map[string]interface{}{
    "field1": value1,
    "field2": value2,
    "field3": value3,
}
_, err = coordService.SetState(ctx, key, newState, currentVersion)
```

---

## Metrics

All components expose Prometheus metrics:

### Messaging Metrics
- `zerostate_agent_messages_sent_total`
- `zerostate_agent_messages_received_total`
- `zerostate_agent_request_duration_seconds`

### Chain Metrics
- `zerostate_chain_executions_total`
- `zerostate_chain_execution_duration_seconds`
- `zerostate_chain_steps_count`

### DAG Metrics
- `zerostate_dag_executions_total`
- `zerostate_dag_parallelism`
- `zerostate_dag_node_duration_seconds`

### Coordination Metrics
- `zerostate_coordination_locks_acquired_total`
- `zerostate_coordination_lock_wait_seconds`
- `zerostate_coordination_state_updates_total`

---

## Support

For questions and support:
- GitHub Issues: https://github.com/aidenlippert/zerostate/issues
- Documentation: [docs/](../docs/)
- Examples: [tests/integration/](../tests/integration/)

---

**Next**: See [MULTI_AGENT_EXAMPLES.md](MULTI_AGENT_EXAMPLES.md) for more real-world examples.
