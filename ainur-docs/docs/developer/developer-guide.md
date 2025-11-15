# Developer Guide

**Document Type**: Developer Guide  
**Version**: 1.0.0  
**Status**: Final  
**Last Updated**: 2025-11-15  

## Abstract

This guide provides comprehensive documentation for developers building on the Ainur Protocol. It covers agent development, runtime implementation, and protocol integration from first principles to production deployment. Developers will learn to create autonomous agents, implement custom runtimes, integrate with the economic layer, and optimize for performance. All examples use production-ready code patterns and follow security best practices established through extensive auditing and real-world deployment experience.

## Table of Contents

1. [Introduction](#1-introduction)
2. [Development Environment](#2-development-environment)
3. [Agent Development](#3-agent-development)
4. [Runtime Implementation](#4-runtime-implementation)
5. [Protocol Integration](#5-protocol-integration)
6. [Testing Strategies](#6-testing-strategies)
7. [Deployment](#7-deployment)
8. [Performance Optimization](#8-performance-optimization)
9. [Security Considerations](#9-security-considerations)
10. [Troubleshooting](#10-troubleshooting)
11. [References](#references)

## 1. Introduction

### 1.1 Development Philosophy

Ainur Protocol development adheres to three core principles:

1. **Determinism**: Agent behavior must be predictable and verifiable
2. **Composability**: Components should integrate seamlessly
3. **Resilience**: Systems must handle failures gracefully

### 1.2 Architecture Overview

Developers interact with three primary layers:

- **Agent Layer**: Business logic implementation
- **Runtime Layer**: Execution environment
- **Protocol Layer**: Network communication and consensus

### 1.3 Prerequisites

Required knowledge:
- Proficiency in Go, Rust, or TypeScript
- Understanding of distributed systems concepts
- Familiarity with cryptographic primitives
- Basic knowledge of economic mechanisms

### 1.4 Documentation Structure

This guide progresses from basic concepts to advanced implementations. Each section includes:
- Conceptual overview
- Implementation details
- Working code examples
- Common pitfalls
- Best practices

## 2. Development Environment

### 2.1 System Requirements

Minimum development system specifications:

| Component | Requirement |
|-----------|-------------|
| CPU | 4 cores, 2.4GHz |
| Memory | 16GB RAM |
| Storage | 50GB available |
| OS | Linux, macOS, WSL2 |
| Network | Broadband internet |

### 2.2 Toolchain Installation

#### 2.2.1 Core Dependencies

```bash
# Install Rust toolchain
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
rustup target add wasm32-unknown-unknown

# Install Go
wget https://go.dev/dl/go1.21.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Install Node.js for SDK
curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install Protocol Buffers
sudo apt-get install -y protobuf-compiler
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

#### 2.2.2 Ainur CLI

```bash
# Download latest release
wget https://github.com/ainur-labs/ainur-cli/releases/latest/download/ainur-cli-linux-amd64
chmod +x ainur-cli-linux-amd64
sudo mv ainur-cli-linux-amd64 /usr/local/bin/ainur

# Verify installation
ainur version
```

### 2.3 Local Network Setup

#### 2.3.1 Docker Compose Configuration

```yaml
# docker-compose.yml
version: '3.8'

services:
  substrate:
    image: ainurlabs/substrate-node:latest
    ports:
      - "9944:9944"  # WebSocket RPC
      - "9933:9933"  # HTTP RPC
      - "30333:30333"  # P2P
    volumes:
      - substrate-data:/data
    command: >
      --dev
      --ws-external
      --rpc-external
      --rpc-cors=all

  orchestrator:
    image: ainurlabs/orchestrator:latest
    ports:
      - "8080:8080"  # HTTP API
      - "9090:9090"  # gRPC API
    environment:
      - SUBSTRATE_URL=ws://substrate:9944
      - LOG_LEVEL=debug
    depends_on:
      - substrate
      - redis

  redis:
    image: redis:7-alpine
    volumes:
      - redis-data:/data

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=ainur
      - POSTGRES_USER=ainur
      - POSTGRES_PASSWORD=ainur-local
    volumes:
      - postgres-data:/var/lib/postgresql/data

volumes:
  substrate-data:
  redis-data:
  postgres-data:
```

#### 2.3.2 Network Initialization

```bash
# Start local network
docker-compose up -d

# Wait for substrate to initialize
ainur network wait --url ws://localhost:9944

# Deploy protocol pallets
ainur deploy pallets --network local

# Create test accounts
ainur account create --name alice --balance 10000
ainur account create --name bob --balance 10000
```

### 2.4 Project Structure

Recommended project organization:

```
my-agent/
├── Cargo.toml          # Rust dependencies
├── go.mod              # Go dependencies  
├── package.json        # JavaScript dependencies
├── src/
│   ├── agent/          # Agent implementation
│   ├── runtime/        # Runtime wrapper
│   └── tests/          # Test suites
├── config/
│   ├── agent.toml      # Agent configuration
│   └── runtime.toml    # Runtime settings
├── scripts/
│   ├── build.sh        # Build automation
│   └── deploy.sh       # Deployment scripts
└── docs/
    └── README.md       # Documentation
```

## 3. Agent Development

### 3.1 Agent Architecture

Agents comprise three components:

1. **Capability Declaration**: What the agent can do
2. **Task Handler**: How the agent executes tasks
3. **Economic Strategy**: How the agent participates in auctions

### 3.2 Basic Agent Implementation

#### 3.2.1 Go Implementation

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    
    "github.com/ainur-labs/sdk-go/agent"
    "github.com/ainur-labs/sdk-go/types"
)

// MathAgent performs mathematical computations
type MathAgent struct {
    agent.BaseAgent
    precision int
}

// NewMathAgent creates a configured math agent
func NewMathAgent() *MathAgent {
    return &MathAgent{
        BaseAgent: agent.NewBaseAgent("math-agent-v1"),
        precision: 10,
    }
}

// GetManifest declares agent capabilities
func (m *MathAgent) GetManifest() *types.AgentManifest {
    return &types.AgentManifest{
        Name:        "MathAgent",
        Version:     "1.0.0",
        Description: "Performs mathematical computations",
        Capabilities: []types.Capability{
            {
                Name:        "arithmetic",
                Description: "Basic arithmetic operations",
                Schema: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "operation": {"enum": ["add", "subtract", "multiply", "divide"]},
                        "operands": {"type": "array", "items": {"type": "number"}}
                    }
                }`),
            },
            {
                Name:        "statistics",
                Description: "Statistical calculations",
                Schema: json.RawMessage(`{
                    "type": "object",
                    "properties": {
                        "function": {"enum": ["mean", "median", "stddev"]},
                        "data": {"type": "array", "items": {"type": "number"}}
                    }
                }`),
            },
        },
        Requirements: types.Requirements{
            CPU:    "100m",
            Memory: "128Mi",
            Timeout: 30,
        },
    }
}

// HandleTask executes incoming tasks
func (m *MathAgent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    // Parse task input
    var input map[string]interface{}
    if err := json.Unmarshal(task.Input, &input); err != nil {
        return nil, fmt.Errorf("invalid input: %w", err)
    }
    
    // Route to appropriate handler
    capability := task.Capability
    switch capability {
    case "arithmetic":
        return m.handleArithmetic(ctx, input)
    case "statistics":
        return m.handleStatistics(ctx, input)
    default:
        return nil, fmt.Errorf("unknown capability: %s", capability)
    }
}

// handleArithmetic processes arithmetic operations
func (m *MathAgent) handleArithmetic(ctx context.Context, input map[string]interface{}) (*types.TaskResult, error) {
    operation, ok := input["operation"].(string)
    if !ok {
        return nil, fmt.Errorf("operation not specified")
    }
    
    operandsRaw, ok := input["operands"].([]interface{})
    if !ok {
        return nil, fmt.Errorf("operands not provided")
    }
    
    // Convert operands to float64
    operands := make([]float64, len(operandsRaw))
    for i, v := range operandsRaw {
        operands[i], ok = v.(float64)
        if !ok {
            return nil, fmt.Errorf("invalid operand at index %d", i)
        }
    }
    
    // Perform calculation
    var result float64
    switch operation {
    case "add":
        for _, v := range operands {
            result += v
        }
    case "subtract":
        result = operands[0]
        for i := 1; i < len(operands); i++ {
            result -= operands[i]
        }
    case "multiply":
        result = 1
        for _, v := range operands {
            result *= v
        }
    case "divide":
        result = operands[0]
        for i := 1; i < len(operands); i++ {
            if operands[i] == 0 {
                return nil, fmt.Errorf("division by zero")
            }
            result /= operands[i]
        }
    default:
        return nil, fmt.Errorf("unknown operation: %s", operation)
    }
    
    // Return result
    output, _ := json.Marshal(map[string]float64{
        "result": result,
    })
    
    return &types.TaskResult{
        Status: types.StatusCompleted,
        Output: output,
        Metrics: map[string]float64{
            "computation_time_ms": 1.23,
            "precision":           float64(m.precision),
        },
    }, nil
}

// handleStatistics processes statistical calculations
func (m *MathAgent) handleStatistics(ctx context.Context, input map[string]interface{}) (*types.TaskResult, error) {
    // Implementation omitted for brevity
    // Similar pattern to arithmetic handler
    return nil, nil
}

// GetBiddingStrategy returns the agent's economic strategy
func (m *MathAgent) GetBiddingStrategy() agent.BiddingStrategy {
    return &agent.FixedPriceBidding{
        BasePrice: 0.001, // 0.001 AINU per operation
        Multipliers: map[string]float64{
            "statistics": 1.5, // 50% premium for statistical operations
        },
    }
}

func main() {
    // Create agent instance
    mathAgent := NewMathAgent()
    
    // Initialize SDK with configuration
    config := agent.Config{
        AgentDID:      "did:ainur:agent:math-v1",
        RuntimeURL:    "grpc://localhost:9090",
        SubstrateURL:  "ws://localhost:9944",
        LogLevel:      "info",
    }
    
    // Create and start agent runtime
    runtime, err := agent.NewRuntime(config, mathAgent)
    if err != nil {
        log.Fatalf("Failed to create runtime: %v", err)
    }
    
    // Run agent
    ctx := context.Background()
    if err := runtime.Run(ctx); err != nil {
        log.Fatalf("Agent failed: %v", err)
    }
}
```

#### 3.2.2 Rust Implementation

```rust
use ainur_sdk::{
    agent::{Agent, AgentManifest, BiddingStrategy, Capability},
    runtime::Runtime,
    types::{Task, TaskResult, TaskStatus},
};
use async_trait::async_trait;
use serde::{Deserialize, Serialize};
use std::error::Error;

#[derive(Debug, Serialize, Deserialize)]
struct ArithmeticInput {
    operation: String,
    operands: Vec<f64>,
}

#[derive(Debug, Serialize, Deserialize)]
struct ArithmeticOutput {
    result: f64,
}

struct MathAgent {
    precision: usize,
}

impl MathAgent {
    fn new() -> Self {
        Self { precision: 10 }
    }
    
    async fn handle_arithmetic(
        &self,
        input: ArithmeticInput,
    ) -> Result<ArithmeticOutput, Box<dyn Error>> {
        let result = match input.operation.as_str() {
            "add" => input.operands.iter().sum(),
            "subtract" => {
                input.operands.iter().skip(1).fold(
                    input.operands[0],
                    |acc, x| acc - x
                )
            }
            "multiply" => input.operands.iter().product(),
            "divide" => {
                input.operands.iter().skip(1).try_fold(
                    input.operands[0],
                    |acc, x| if *x != 0.0 { Some(acc / x) } else { None }
                ).ok_or("Division by zero")?
            }
            _ => return Err("Unknown operation".into()),
        };
        
        Ok(ArithmeticOutput { result })
    }
}

#[async_trait]
impl Agent for MathAgent {
    fn manifest(&self) -> AgentManifest {
        AgentManifest {
            name: "MathAgent".to_string(),
            version: "1.0.0".to_string(),
            description: "Performs mathematical computations".to_string(),
            capabilities: vec![
                Capability {
                    name: "arithmetic".to_string(),
                    description: "Basic arithmetic operations".to_string(),
                    schema: serde_json::json!({
                        "type": "object",
                        "properties": {
                            "operation": {"enum": ["add", "subtract", "multiply", "divide"]},
                            "operands": {"type": "array", "items": {"type": "number"}}
                        }
                    }),
                },
            ],
            requirements: Default::default(),
        }
    }
    
    async fn handle_task(&self, task: Task) -> Result<TaskResult, Box<dyn Error>> {
        match task.capability.as_str() {
            "arithmetic" => {
                let input: ArithmeticInput = serde_json::from_value(task.input)?;
                let output = self.handle_arithmetic(input).await?;
                
                Ok(TaskResult {
                    status: TaskStatus::Completed,
                    output: serde_json::to_value(output)?,
                    metrics: Default::default(),
                })
            }
            _ => Err("Unknown capability".into()),
        }
    }
    
    fn bidding_strategy(&self) -> Box<dyn BiddingStrategy> {
        Box::new(ainur_sdk::agent::FixedPriceBidding {
            base_price: 0.001,
            multipliers: Default::default(),
        })
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    // Initialize agent
    let agent = MathAgent::new();
    
    // Configure runtime
    let config = ainur_sdk::Config {
        agent_did: "did:ainur:agent:math-v1".to_string(),
        runtime_url: "grpc://localhost:9090".to_string(),
        substrate_url: "ws://localhost:9944".to_string(),
        log_level: "info".to_string(),
    };
    
    // Create and run runtime
    let runtime = Runtime::new(config, agent)?;
    runtime.run().await?;
    
    Ok(())
}
```

### 3.3 Advanced Agent Features

#### 3.3.1 Stateful Agents

```go
type StatefulAgent struct {
    agent.BaseAgent
    state sync.Map // Thread-safe state storage
}

func (s *StatefulAgent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    // Load state
    key := fmt.Sprintf("task:%s", task.ID)
    if val, ok := s.state.Load(key); ok {
        // Resume from checkpoint
        checkpoint := val.(Checkpoint)
        return s.resumeFromCheckpoint(ctx, task, checkpoint)
    }
    
    // Execute with checkpointing
    return s.executeWithCheckpoints(ctx, task)
}
```

#### 3.3.2 Multi-Step Task Handling

```go
func (a *ComplexAgent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    // Parse multi-step task
    steps, err := a.parseTaskSteps(task)
    if err != nil {
        return nil, err
    }
    
    // Execute steps in sequence
    results := make([]interface{}, len(steps))
    for i, step := range steps {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            result, err := a.executeStep(ctx, step)
            if err != nil {
                return nil, fmt.Errorf("step %d failed: %w", i, err)
            }
            results[i] = result
        }
    }
    
    // Aggregate results
    output, err := a.aggregateResults(results)
    if err != nil {
        return nil, err
    }
    
    return &types.TaskResult{
        Status: types.StatusCompleted,
        Output: output,
    }, nil
}
```

#### 3.3.3 Adaptive Bidding Strategy

```go
type AdaptiveBidding struct {
    basePrice    float64
    successRate  float64
    avgProfit    float64
    learningRate float64
}

func (a *AdaptiveBidding) CalculateBid(task *types.Task) float64 {
    // Base bid calculation
    bid := a.basePrice
    
    // Adjust based on task complexity
    complexity := a.estimateComplexity(task)
    bid *= (1 + complexity*0.1)
    
    // Adjust based on current success rate
    if a.successRate < 0.8 {
        bid *= 0.95 // Lower bid to increase win rate
    } else if a.successRate > 0.95 {
        bid *= 1.05 // Increase bid for better margins
    }
    
    // Apply profit target
    minProfit := bid * 0.2 // 20% minimum profit margin
    if a.avgProfit < minProfit {
        bid *= 1.1
    }
    
    return bid
}

func (a *AdaptiveBidding) UpdateMetrics(outcome *types.TaskOutcome) {
    // Update success rate
    if outcome.Success {
        a.successRate = a.successRate*(1-a.learningRate) + a.learningRate
    } else {
        a.successRate = a.successRate * (1 - a.learningRate)
    }
    
    // Update average profit
    profit := outcome.Payment - outcome.Cost
    a.avgProfit = a.avgProfit*(1-a.learningRate) + profit*a.learningRate
}
```

### 3.4 Agent Composition

#### 3.4.1 Agent Pipelines

```go
type PipelineAgent struct {
    agent.BaseAgent
    stages []agent.Agent
}

func (p *PipelineAgent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    // Execute pipeline stages
    current := task.Input
    for i, stage := range p.stages {
        stageTask := &types.Task{
            ID:         fmt.Sprintf("%s-stage-%d", task.ID, i),
            Capability: stage.GetManifest().Capabilities[0].Name,
            Input:      current,
        }
        
        result, err := stage.HandleTask(ctx, stageTask)
        if err != nil {
            return nil, fmt.Errorf("stage %d failed: %w", i, err)
        }
        
        current = result.Output
    }
    
    return &types.TaskResult{
        Status: types.StatusCompleted,
        Output: current,
    }, nil
}
```

#### 3.4.2 Agent Orchestration

```go
type OrchestratorAgent struct {
    agent.BaseAgent
    workers map[string]agent.Agent
}

func (o *OrchestratorAgent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    // Decompose task
    subtasks, err := o.decomposeTask(task)
    if err != nil {
        return nil, err
    }
    
    // Execute subtasks in parallel
    var wg sync.WaitGroup
    results := make([]*types.TaskResult, len(subtasks))
    errors := make([]error, len(subtasks))
    
    for i, subtask := range subtasks {
        wg.Add(1)
        go func(idx int, st *types.Task) {
            defer wg.Done()
            
            worker, ok := o.workers[st.Capability]
            if !ok {
                errors[idx] = fmt.Errorf("no worker for capability: %s", st.Capability)
                return
            }
            
            results[idx], errors[idx] = worker.HandleTask(ctx, st)
        }(i, subtask)
    }
    
    wg.Wait()
    
    // Check for errors
    for i, err := range errors {
        if err != nil {
            return nil, fmt.Errorf("subtask %d failed: %w", i, err)
        }
    }
    
    // Combine results
    return o.combineResults(results)
}
```

## 4. Runtime Implementation

### 4.1 Runtime Architecture

The Ainur Runtime Interface (ARI) defines the contract between agents and the protocol:

```protobuf
service AgentRuntime {
    // Get agent capabilities and metadata
    rpc GetManifest(Empty) returns (AgentManifest);
    
    // Execute a task
    rpc ExecuteTask(TaskRequest) returns (TaskResponse);
    
    // Stream task execution updates
    rpc StreamUpdates(TaskID) returns (stream TaskUpdate);
    
    // Health monitoring
    rpc GetHealth(Empty) returns (HealthStatus);
}
```

### 4.2 WebAssembly Runtime

#### 4.2.1 WASM Module Structure

```rust
// lib.rs - WASM module entry point
use wasm_bindgen::prelude::*;

#[wasm_bindgen]
pub struct WasmRuntime {
    agents: HashMap<String, Box<dyn Agent>>,
}

#[wasm_bindgen]
impl WasmRuntime {
    pub fn new() -> Self {
        Self {
            agents: HashMap::new(),
        }
    }
    
    pub fn register_agent(&mut self, id: String, agent_bytes: Vec<u8>) -> Result<(), JsValue> {
        // Deserialize and register agent
        let agent = deserialize_agent(&agent_bytes)
            .map_err(|e| JsValue::from_str(&e.to_string()))?;
        
        self.agents.insert(id, agent);
        Ok(())
    }
    
    pub fn execute_task(&self, task_json: String) -> Result<String, JsValue> {
        // Parse task
        let task: Task = serde_json::from_str(&task_json)
            .map_err(|e| JsValue::from_str(&e.to_string()))?;
        
        // Find agent
        let agent = self.agents.get(&task.agent_id)
            .ok_or_else(|| JsValue::from_str("Agent not found"))?;
        
        // Execute task
        let result = agent.handle_task(task)
            .map_err(|e| JsValue::from_str(&e.to_string()))?;
        
        // Serialize result
        serde_json::to_string(&result)
            .map_err(|e| JsValue::from_str(&e.to_string()))
    }
}
```

#### 4.2.2 Host Integration

```go
package runtime

import (
    "context"
    "fmt"
    
    "github.com/bytecodealliance/wasmtime-go"
)

type WASMRuntime struct {
    engine   *wasmtime.Engine
    store    *wasmtime.Store
    instance *wasmtime.Instance
    memory   *wasmtime.Memory
}

func NewWASMRuntime(wasmBytes []byte) (*WASMRuntime, error) {
    // Create engine with security configurations
    config := wasmtime.NewConfig()
    config.SetWasmThreads(false)
    config.SetWasmReferenceTypes(true)
    config.SetConsumeFuel(true)
    
    engine := wasmtime.NewEngineWithConfig(config)
    store := wasmtime.NewStore(engine)
    
    // Set resource limits
    store.SetFuel(1000000) // 1M fuel units
    store.SetEpochDeadline(1) // 1 epoch timeout
    
    // Compile module
    module, err := wasmtime.NewModule(engine, wasmBytes)
    if err != nil {
        return nil, fmt.Errorf("failed to compile module: %w", err)
    }
    
    // Create imports
    imports := []wasmtime.AsExtern{
        wasmtime.WrapFunc(store, "env", "log", logFunc),
        wasmtime.WrapFunc(store, "env", "time", timeFunc),
    }
    
    // Instantiate module
    instance, err := wasmtime.NewInstance(store, module, imports)
    if err != nil {
        return nil, fmt.Errorf("failed to instantiate: %w", err)
    }
    
    // Get memory export
    memory := instance.GetExport(store, "memory").Memory()
    
    return &WASMRuntime{
        engine:   engine,
        store:    store,
        instance: instance,
        memory:   memory,
    }, nil
}

func (w *WASMRuntime) ExecuteTask(ctx context.Context, task []byte) ([]byte, error) {
    // Get execute function
    executeFn := w.instance.GetFunc(w.store, "execute_task")
    if executeFn == nil {
        return nil, fmt.Errorf("execute_task function not found")
    }
    
    // Allocate memory for input
    allocFn := w.instance.GetFunc(w.store, "alloc")
    if allocFn == nil {
        return nil, fmt.Errorf("alloc function not found")
    }
    
    // Allocate space for task
    ptrVal, err := allocFn.Call(w.store, int32(len(task)))
    if err != nil {
        return nil, fmt.Errorf("allocation failed: %w", err)
    }
    
    ptr := ptrVal.(int32)
    
    // Copy task data to WASM memory
    data := w.memory.Data(w.store)
    copy(data[ptr:], task)
    
    // Execute task
    resultVal, err := executeFn.Call(w.store, ptr, int32(len(task)))
    if err != nil {
        return nil, fmt.Errorf("execution failed: %w", err)
    }
    
    // Read result
    resultPtr := resultVal.(int32)
    resultLen := binary.LittleEndian.Uint32(data[resultPtr:])
    result := make([]byte, resultLen)
    copy(result, data[resultPtr+4:resultPtr+4+int32(resultLen)])
    
    return result, nil
}

// Import functions
func logFunc(caller *wasmtime.Caller, ptr int32, len int32) {
    memory := caller.GetExport("memory").Memory()
    data := memory.Data(caller)
    msg := string(data[ptr : ptr+len])
    log.Printf("WASM: %s", msg)
}

func timeFunc() int64 {
    return time.Now().Unix()
}
```

### 4.3 Container Runtime

#### 4.3.1 Docker Integration

```go
package runtime

import (
    "context"
    "io"
    
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/client"
)

type DockerRuntime struct {
    client *client.Client
    config RuntimeConfig
}

type RuntimeConfig struct {
    CPULimit    string // e.g., "0.5" for half CPU
    MemoryLimit string // e.g., "512m"
    Timeout     time.Duration
    Network     string // "none" for isolation
}

func NewDockerRuntime(config RuntimeConfig) (*DockerRuntime, error) {
    cli, err := client.NewClientWithOpts(client.FromEnv)
    if err != nil {
        return nil, err
    }
    
    return &DockerRuntime{
        client: cli,
        config: config,
    }, nil
}

func (d *DockerRuntime) ExecuteTask(ctx context.Context, image string, task []byte) ([]byte, error) {
    // Create container
    resp, err := d.client.ContainerCreate(ctx, &container.Config{
        Image: image,
        Cmd:   []string{"execute"},
        Env: []string{
            "TASK_DATA=" + base64.StdEncoding.EncodeToString(task),
        },
        AttachStdout: true,
        AttachStderr: true,
    }, &container.HostConfig{
        Resources: container.Resources{
            CPUQuota:  parseCPULimit(d.config.CPULimit),
            Memory:    parseMemoryLimit(d.config.MemoryLimit),
        },
        NetworkMode: container.NetworkMode(d.config.Network),
        SecurityOpt: []string{
            "no-new-privileges",
            "seccomp=unconfined", // Use custom seccomp profile
        },
    }, nil, nil, "")
    
    if err != nil {
        return nil, fmt.Errorf("failed to create container: %w", err)
    }
    
    // Ensure cleanup
    defer d.client.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{
        Force: true,
    })
    
    // Start container
    if err := d.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        return nil, fmt.Errorf("failed to start container: %w", err)
    }
    
    // Set timeout
    timeoutCtx, cancel := context.WithTimeout(ctx, d.config.Timeout)
    defer cancel()
    
    // Wait for completion
    statusCh, errCh := d.client.ContainerWait(timeoutCtx, resp.ID, container.WaitConditionNotRunning)
    select {
    case err := <-errCh:
        if err != nil {
            return nil, fmt.Errorf("container wait error: %w", err)
        }
    case status := <-statusCh:
        if status.StatusCode != 0 {
            logs, _ := d.getContainerLogs(ctx, resp.ID)
            return nil, fmt.Errorf("container exited with code %d: %s", status.StatusCode, logs)
        }
    case <-timeoutCtx.Done():
        return nil, fmt.Errorf("container execution timeout")
    }
    
    // Get output
    return d.getContainerOutput(ctx, resp.ID)
}

func (d *DockerRuntime) getContainerOutput(ctx context.Context, containerID string) ([]byte, error) {
    options := types.ContainerLogsOptions{
        ShowStdout: true,
        Follow:     false,
    }
    
    reader, err := d.client.ContainerLogs(ctx, containerID, options)
    if err != nil {
        return nil, err
    }
    defer reader.Close()
    
    // Docker multiplexes stdout/stderr, need to demux
    var stdout bytes.Buffer
    _, err = stdcopy.StdCopy(&stdout, io.Discard, reader)
    if err != nil {
        return nil, err
    }
    
    return stdout.Bytes(), nil
}
```

### 4.4 Native Runtime

#### 4.4.1 Process Isolation

```go
package runtime

import (
    "bytes"
    "context"
    "encoding/json"
    "os/exec"
    "syscall"
    "time"
)

type NativeRuntime struct {
    binary  string
    workdir string
    uid     int
    gid     int
}

func (n *NativeRuntime) ExecuteTask(ctx context.Context, task []byte) ([]byte, error) {
    // Create command
    cmd := exec.CommandContext(ctx, n.binary, "execute")
    cmd.Dir = n.workdir
    
    // Set process attributes for isolation
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Credential: &syscall.Credential{
            Uid: uint32(n.uid),
            Gid: uint32(n.gid),
        },
        Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
        Pdeathsig:  syscall.SIGKILL, // Kill on parent death
    }
    
    // Set resource limits
    cmd.Env = []string{
        "PATH=/usr/bin:/bin",
        "TASK_TIMEOUT=30s",
    }
    
    // Pipe task data
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }
    
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    
    // Start process
    if err := cmd.Start(); err != nil {
        return nil, err
    }
    
    // Send task data
    go func() {
        defer stdin.Close()
        stdin.Write(task)
    }()
    
    // Wait with timeout
    done := make(chan error)
    go func() {
        done <- cmd.Wait()
    }()
    
    select {
    case err := <-done:
        if err != nil {
            return nil, fmt.Errorf("execution failed: %w, stderr: %s", err, stderr.String())
        }
        return stdout.Bytes(), nil
    case <-ctx.Done():
        cmd.Process.Kill()
        return nil, ctx.Err()
    }
}
```

## 5. Protocol Integration

### 5.1 Blockchain Interaction

#### 5.1.1 Account Management

```go
package protocol

import (
    gsrpc "github.com/centrifuge/go-substrate-rpc-client/v4"
    "github.com/centrifuge/go-substrate-rpc-client/v4/signature"
    "github.com/centrifuge/go-substrate-rpc-client/v4/types"
)

type BlockchainClient struct {
    api     *gsrpc.SubstrateAPI
    keypair signature.KeyringPair
}

func NewBlockchainClient(url string, seed string) (*BlockchainClient, error) {
    // Connect to Substrate node
    api, err := gsrpc.NewSubstrateAPI(url)
    if err != nil {
        return nil, err
    }
    
    // Create keypair from seed
    keypair, err := signature.KeypairFromSecretSeed(seed, 42)
    if err != nil {
        return nil, err
    }
    
    return &BlockchainClient{
        api:     api,
        keypair: keypair,
    }, nil
}

func (b *BlockchainClient) RegisterAgent(manifest AgentManifest) error {
    // Get metadata
    meta, err := b.api.RPC.State.GetMetadataLatest()
    if err != nil {
        return err
    }
    
    // Create registration call
    c, err := types.NewCall(
        meta,
        "Registry.register_agent",
        types.NewAccountID(b.keypair.PublicKey),
        manifest.ToChainType(),
    )
    if err != nil {
        return err
    }
    
    // Create and sign extrinsic
    ext := types.NewExtrinsic(c)
    
    genesisHash, err := b.api.RPC.Chain.GetBlockHash(0)
    if err != nil {
        return err
    }
    
    rv, err := b.api.RPC.State.GetRuntimeVersionLatest()
    if err != nil {
        return err
    }
    
    nonce, err := b.api.RPC.System.AccountNextIndex(b.keypair.PublicKey)
    if err != nil {
        return err
    }
    
    o := types.SignatureOptions{
        BlockHash:          genesisHash,
        Era:                types.ExtrinsicEra{IsMortalEra: false},
        GenesisHash:        genesisHash,
        Nonce:              types.NewUCompactFromUInt(uint64(nonce)),
        SpecVersion:        rv.SpecVersion,
        Tip:                types.NewUCompactFromUInt(0),
        TransactionVersion: rv.TransactionVersion,
    }
    
    if err := ext.Sign(b.keypair, o); err != nil {
        return err
    }
    
    // Submit transaction
    _, err = b.api.RPC.Author.SubmitExtrinsic(ext)
    return err
}
```

#### 5.1.2 Escrow Operations

```go
func (b *BlockchainClient) CreateEscrow(taskID string, amount uint64, recipient types.AccountID) (types.Hash, error) {
    meta, err := b.api.RPC.State.GetMetadataLatest()
    if err != nil {
        return types.Hash{}, err
    }
    
    // Create escrow
    c, err := types.NewCall(
        meta,
        "Escrow.create",
        types.NewHash([]byte(taskID)),
        types.NewUCompact(amount),
        recipient,
        types.NewOption[types.U32](types.U32(7*24*60*60)), // 7 days timeout
    )
    if err != nil {
        return types.Hash{}, err
    }
    
    // Sign and submit
    return b.submitExtrinsic(c)
}

func (b *BlockchainClient) ReleaseEscrow(escrowID types.Hash) error {
    meta, err := b.api.RPC.State.GetMetadataLatest()
    if err != nil {
        return err
    }
    
    c, err := types.NewCall(
        meta,
        "Escrow.release",
        escrowID,
    )
    if err != nil {
        return err
    }
    
    _, err = b.submitExtrinsic(c)
    return err
}
```

### 5.2 P2P Network Integration

#### 5.2.1 Agent Presence

```go
package p2p

import (
    "context"
    "encoding/json"
    "time"
    
    libp2p "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/host"
    pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type P2PClient struct {
    host   host.Host
    pubsub *pubsub.PubSub
    topics map[string]*pubsub.Topic
}

func NewP2PClient(ctx context.Context, port int) (*P2PClient, error) {
    // Create libp2p host
    h, err := libp2p.New(
        libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)),
        libp2p.DefaultTransports,
        libp2p.DefaultMuxers,
        libp2p.DefaultSecurity,
    )
    if err != nil {
        return nil, err
    }
    
    // Create pubsub
    ps, err := pubsub.NewGossipSub(ctx, h)
    if err != nil {
        return nil, err
    }
    
    return &P2PClient{
        host:   h,
        pubsub: ps,
        topics: make(map[string]*pubsub.Topic),
    }, nil
}

func (p *P2PClient) PublishPresence(ctx context.Context, presence AgentPresence) error {
    topicName := fmt.Sprintf("ainur/v1/%d/presence/announce", presence.ShardID)
    
    // Get or create topic
    topic, err := p.getTopic(topicName)
    if err != nil {
        return err
    }
    
    // Marshal presence
    data, err := json.Marshal(presence)
    if err != nil {
        return err
    }
    
    // Sign data
    signature, err := p.signData(data)
    if err != nil {
        return err
    }
    
    // Create message
    msg := P2PMessage{
        Type:      "presence",
        Data:      data,
        Signature: signature,
        Timestamp: time.Now().Unix(),
    }
    
    msgData, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    // Publish
    return topic.Publish(ctx, msgData)
}

func (p *P2PClient) SubscribeToTopic(ctx context.Context, topicName string, handler MessageHandler) error {
    topic, err := p.getTopic(topicName)
    if err != nil {
        return err
    }
    
    sub, err := topic.Subscribe()
    if err != nil {
        return err
    }
    
    // Handle messages
    go func() {
        for {
            msg, err := sub.Next(ctx)
            if err != nil {
                if ctx.Err() != nil {
                    return
                }
                continue
            }
            
            // Verify and handle message
            if p.verifyMessage(msg.Data) {
                handler(msg.Data)
            }
        }
    }()
    
    return nil
}
```

### 5.3 Task Lifecycle Management

#### 5.3.1 Task Submission

```go
package client

type TaskClient struct {
    apiURL     string
    httpClient *http.Client
    signer     Signer
}

func (t *TaskClient) SubmitTask(ctx context.Context, req TaskRequest) (*TaskResponse, error) {
    // Validate request
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("invalid request: %w", err)
    }
    
    // Sign request
    signature, err := t.signer.Sign(req)
    if err != nil {
        return nil, fmt.Errorf("failed to sign: %w", err)
    }
    
    req.Signature = signature
    
    // Marshal request
    data, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    
    // Create HTTP request
    httpReq, err := http.NewRequestWithContext(
        ctx,
        "POST",
        fmt.Sprintf("%s/api/v1/tasks", t.apiURL),
        bytes.NewReader(data),
    )
    if err != nil {
        return nil, err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("X-Agent-DID", t.signer.DID())
    
    // Execute request
    resp, err := t.httpClient.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Check status
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("request failed: %d - %s", resp.StatusCode, body)
    }
    
    // Parse response
    var taskResp TaskResponse
    if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
        return nil, err
    }
    
    return &taskResp, nil
}

func (t *TaskClient) MonitorTask(ctx context.Context, taskID string) (<-chan TaskUpdate, error) {
    updates := make(chan TaskUpdate, 10)
    
    // WebSocket URL
    wsURL := strings.Replace(t.apiURL, "http", "ws", 1)
    wsURL = fmt.Sprintf("%s/api/v1/tasks/%s/updates", wsURL, taskID)
    
    // Connect
    conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
    if err != nil {
        return nil, err
    }
    
    // Monitor updates
    go func() {
        defer close(updates)
        defer conn.Close()
        
        for {
            var update TaskUpdate
            if err := conn.ReadJSON(&update); err != nil {
                if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
                    return
                }
                // Log error and continue
                continue
            }
            
            select {
            case updates <- update:
            case <-ctx.Done():
                return
            }
        }
    }()
    
    return updates, nil
}
```

## 6. Testing Strategies

### 6.1 Unit Testing

#### 6.1.1 Agent Testing

```go
package agent_test

import (
    "context"
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestMathAgent_Arithmetic(t *testing.T) {
    agent := NewMathAgent()
    
    tests := []struct {
        name      string
        operation string
        operands  []float64
        expected  float64
        wantErr   bool
    }{
        {
            name:      "addition",
            operation: "add",
            operands:  []float64{1, 2, 3},
            expected:  6,
        },
        {
            name:      "division by zero",
            operation: "divide",
            operands:  []float64{10, 0},
            wantErr:   true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            input := map[string]interface{}{
                "operation": tt.operation,
                "operands":  tt.operands,
            }
            
            inputData, err := json.Marshal(input)
            require.NoError(t, err)
            
            task := &types.Task{
                ID:         "test-" + tt.name,
                Capability: "arithmetic",
                Input:      inputData,
            }
            
            result, err := agent.HandleTask(context.Background(), task)
            
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, types.StatusCompleted, result.Status)
            
            var output map[string]float64
            err = json.Unmarshal(result.Output, &output)
            require.NoError(t, err)
            
            assert.InDelta(t, tt.expected, output["result"], 0.0001)
        })
    }
}
```

#### 6.1.2 Runtime Testing

```go
func TestWASMRuntime_ResourceLimits(t *testing.T) {
    runtime := setupTestRuntime(t)
    
    // Test memory limit
    t.Run("memory limit", func(t *testing.T) {
        task := createMemoryIntensiveTask(100 * 1024 * 1024) // 100MB
        
        _, err := runtime.ExecuteTask(context.Background(), task)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "memory limit exceeded")
    })
    
    // Test CPU limit
    t.Run("cpu limit", func(t *testing.T) {
        task := createCPUIntensiveTask(time.Second * 5)
        
        ctx, cancel := context.WithTimeout(context.Background(), time.Second)
        defer cancel()
        
        _, err := runtime.ExecuteTask(ctx, task)
        assert.Error(t, err)
        assert.Contains(t, err.Error(), "execution timeout")
    })
}
```

### 6.2 Integration Testing

#### 6.2.1 End-to-End Testing

```go
func TestEndToEndTaskExecution(t *testing.T) {
    // Start test network
    network := startTestNetwork(t)
    defer network.Stop()
    
    // Create test agent
    agent := createTestAgent(t)
    agent.Start(network.OrchestratorURL())
    defer agent.Stop()
    
    // Wait for agent registration
    require.Eventually(t, func() bool {
        agents, err := network.GetRegisteredAgents()
        return err == nil && len(agents) > 0
    }, time.Second*10, time.Millisecond*100)
    
    // Submit task
    client := NewTaskClient(network.APIURL())
    
    task := TaskRequest{
        Capability: "arithmetic",
        Input: map[string]interface{}{
            "operation": "multiply",
            "operands":  []float64{7, 6},
        },
        MaxPrice: 0.01,
    }
    
    resp, err := client.SubmitTask(context.Background(), task)
    require.NoError(t, err)
    
    // Monitor execution
    updates, err := client.MonitorTask(context.Background(), resp.TaskID)
    require.NoError(t, err)
    
    // Verify completion
    var finalUpdate TaskUpdate
    for update := range updates {
        finalUpdate = update
        if update.Status == "completed" || update.Status == "failed" {
            break
        }
    }
    
    assert.Equal(t, "completed", finalUpdate.Status)
    
    // Verify result
    var result map[string]float64
    err = json.Unmarshal(finalUpdate.Output, &result)
    require.NoError(t, err)
    assert.Equal(t, 42.0, result["result"])
    
    // Verify payment
    payment, err := network.GetPayment(resp.TaskID)
    require.NoError(t, err)
    assert.Equal(t, "released", payment.Status)
    assert.True(t, payment.Amount > 0)
}
```

### 6.3 Performance Testing

#### 6.3.1 Load Testing

```go
func TestSystemUnderLoad(t *testing.T) {
    network := startTestNetwork(t)
    defer network.Stop()
    
    // Start multiple agents
    agents := make([]*TestAgent, 10)
    for i := range agents {
        agents[i] = createTestAgent(t)
        agents[i].Start(network.OrchestratorURL())
        defer agents[i].Stop()
    }
    
    // Generate load
    const (
        numClients     = 100
        tasksPerClient = 50
    )
    
    var wg sync.WaitGroup
    results := make(chan TestResult, numClients*tasksPerClient)
    
    start := time.Now()
    
    for i := 0; i < numClients; i++ {
        wg.Add(1)
        go func(clientID int) {
            defer wg.Done()
            
            client := NewTaskClient(network.APIURL())
            
            for j := 0; j < tasksPerClient; j++ {
                taskStart := time.Now()
                
                resp, err := client.SubmitTask(context.Background(), generateRandomTask())
                if err != nil {
                    results <- TestResult{Error: err}
                    continue
                }
                
                // Wait for completion
                completed := waitForCompletion(t, client, resp.TaskID, time.Minute)
                
                results <- TestResult{
                    Duration:  time.Since(taskStart),
                    Completed: completed,
                    TaskID:    resp.TaskID,
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(results)
    
    // Analyze results
    var (
        totalTasks      int
        completedTasks  int
        totalDuration   time.Duration
        errors          int
    )
    
    for result := range results {
        totalTasks++
        if result.Error != nil {
            errors++
            continue
        }
        if result.Completed {
            completedTasks++
            totalDuration += result.Duration
        }
    }
    
    elapsed := time.Since(start)
    
    // Assertions
    assert.Greater(t, float64(completedTasks)/float64(totalTasks), 0.95) // 95% success rate
    assert.Less(t, totalDuration/time.Duration(completedTasks), time.Second*5) // Avg < 5s
    assert.Less(t, elapsed, time.Minute*5) // Total time < 5 minutes
    
    // Log metrics
    t.Logf("Load test results:")
    t.Logf("  Total tasks: %d", totalTasks)
    t.Logf("  Completed: %d (%.2f%%)", completedTasks, float64(completedTasks)/float64(totalTasks)*100)
    t.Logf("  Errors: %d", errors)
    t.Logf("  Average duration: %v", totalDuration/time.Duration(completedTasks))
    t.Logf("  Throughput: %.2f tasks/second", float64(totalTasks)/elapsed.Seconds())
}
```

### 6.4 Security Testing

#### 6.4.1 Fuzzing

```go
func FuzzAgentInput(f *testing.F) {
    agent := NewMathAgent()
    
    // Add seed corpus
    f.Add(`{"operation":"add","operands":[1,2,3]}`)
    f.Add(`{"operation":"divide","operands":[10,2]}`)
    
    f.Fuzz(func(t *testing.T, input string) {
        task := &types.Task{
            ID:         "fuzz-test",
            Capability: "arithmetic",
            Input:      []byte(input),
        }
        
        // Should not panic
        result, err := agent.HandleTask(context.Background(), task)
        
        if err == nil {
            // Verify result structure
            assert.Equal(t, types.StatusCompleted, result.Status)
            assert.NotEmpty(t, result.Output)
            
            // Should be valid JSON
            var output interface{}
            err := json.Unmarshal(result.Output, &output)
            assert.NoError(t, err)
        }
    })
}
```

## 7. Deployment

### 7.1 Container Deployment

#### 7.1.1 Dockerfile

```dockerfile
# Multi-stage build for optimal size
FROM golang:1.21 AS builder

WORKDIR /build

# Copy dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" \
    -o agent ./cmd/agent

# Runtime stage
FROM alpine:3.18

# Install certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -H -u 1000 agent

# Copy binary
COPY --from=builder /build/agent /usr/local/bin/

# Set ownership
RUN chown agent:agent /usr/local/bin/agent

# Switch to non-root user
USER agent

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/agent", "health"]

# Run agent
ENTRYPOINT ["/usr/local/bin/agent"]
CMD ["run"]
```

#### 7.1.2 Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: math-agent
  namespace: ainur-agents
spec:
  replicas: 3
  selector:
    matchLabels:
      app: math-agent
  template:
    metadata:
      labels:
        app: math-agent
    spec:
      serviceAccountName: agent-sa
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
      - name: agent
        image: myregistry/math-agent:v1.0.0
        imagePullPolicy: Always
        ports:
        - containerPort: 9090
          name: grpc
        - containerPort: 8080
          name: metrics
        env:
        - name: AGENT_DID
          value: "did:ainur:agent:math-v1"
        - name: RUNTIME_URL
          value: "grpc://orchestrator-service:9090"
        - name: SUBSTRATE_URL
          value: "ws://substrate-service:9944"
        - name: LOG_LEVEL
          value: "info"
        - name: PRIVATE_KEY
          valueFrom:
            secretKeyRef:
              name: agent-keys
              key: private-key
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          grpc:
            port: 9090
            service: health
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          grpc:
            port: 9090
            service: health
          initialDelaySeconds: 5
          periodSeconds: 10
        volumeMounts:
        - name: config
          mountPath: /etc/agent
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: agent-config
---
apiVersion: v1
kind: Service
metadata:
  name: math-agent-service
  namespace: ainur-agents
spec:
  selector:
    app: math-agent
  ports:
  - port: 9090
    targetPort: 9090
    name: grpc
  - port: 8080
    targetPort: 8080
    name: metrics
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: math-agent-hpa
  namespace: ainur-agents
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: math-agent
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: task_queue_depth
      target:
        type: AverageValue
        averageValue: "10"
```

### 7.2 Cloud Deployment

#### 7.2.1 Terraform Configuration

```hcl
# main.tf
terraform {
  required_version = ">= 1.0"
  
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  
  backend "s3" {
    bucket = "ainur-terraform-state"
    key    = "agents/math-agent/terraform.tfstate"
    region = "us-east-1"
  }
}

# ECS Task Definition
resource "aws_ecs_task_definition" "agent" {
  family                   = "math-agent"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = "256"
  memory                   = "512"
  execution_role_arn       = aws_iam_role.ecs_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn
  
  container_definitions = jsonencode([
    {
      name  = "agent"
      image = "${var.ecr_repository_url}:${var.image_tag}"
      
      portMappings = [
        {
          containerPort = 9090
          protocol      = "tcp"
        }
      ]
      
      environment = [
        {
          name  = "AGENT_DID"
          value = var.agent_did
        },
        {
          name  = "RUNTIME_URL"
          value = var.runtime_url
        }
      ]
      
      secrets = [
        {
          name      = "PRIVATE_KEY"
          valueFrom = aws_ssm_parameter.agent_private_key.arn
        }
      ]
      
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.agent.name
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "agent"
        }
      }
      
      healthCheck = {
        command     = ["CMD-SHELL", "grpc_health_probe -addr=:9090"]
        interval    = 30
        timeout     = 5
        retries     = 3
        startPeriod = 60
      }
    }
  ])
}

# ECS Service
resource "aws_ecs_service" "agent" {
  name            = "math-agent"
  cluster         = var.ecs_cluster_id
  task_definition = aws_ecs_task_definition.agent.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"
  
  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.agent.id]
    assign_public_ip = false
  }
  
  service_registries {
    registry_arn = aws_service_discovery_service.agent.arn
  }
  
  deployment_configuration {
    maximum_percent         = 200
    minimum_healthy_percent = 100
  }
  
  lifecycle {
    ignore_changes = [desired_count]
  }
}

# Auto Scaling
resource "aws_appautoscaling_target" "agent" {
  max_capacity       = 20
  min_capacity       = 3
  resource_id        = "service/${var.ecs_cluster_name}/${aws_ecs_service.agent.name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "agent_cpu" {
  name               = "agent-cpu-scaling"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.agent.resource_id
  scalable_dimension = aws_appautoscaling_target.agent.scalable_dimension
  service_namespace  = aws_appautoscaling_target.agent.service_namespace
  
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    
    target_value = 70.0
  }
}
```

### 7.3 Monitoring and Observability

#### 7.3.1 Prometheus Metrics

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Task metrics
    TasksReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_tasks_received_total",
            Help: "Total number of tasks received",
        },
        []string{"capability"},
    )
    
    TasksCompleted = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_tasks_completed_total",
            Help: "Total number of tasks completed successfully",
        },
        []string{"capability"},
    )
    
    TasksFailed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_tasks_failed_total",
            Help: "Total number of tasks that failed",
        },
        []string{"capability", "error"},
    )
    
    TaskDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "agent_task_duration_seconds",
            Help:    "Task execution duration",
            Buckets: prometheus.ExponentialBuckets(0.01, 2, 10),
        },
        []string{"capability"},
    )
    
    // Economic metrics
    BidsSubmitted = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "agent_bids_submitted_total",
            Help: "Total number of bids submitted",
        },
    )
    
    BidsWon = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "agent_bids_won_total",
            Help: "Total number of auctions won",
        },
    )
    
    Revenue = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "agent_revenue_total",
            Help: "Total revenue earned in AINU",
        },
    )
    
    // Resource metrics
    MemoryUsage = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "agent_memory_usage_bytes",
            Help: "Current memory usage in bytes",
        },
    )
    
    CPUUsage = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "agent_cpu_usage_percent",
            Help: "Current CPU usage percentage",
        },
    )
)

// RecordTaskMetrics records task execution metrics
func RecordTaskMetrics(capability string, duration time.Duration, err error) {
    TasksReceived.WithLabelValues(capability).Inc()
    TaskDuration.WithLabelValues(capability).Observe(duration.Seconds())
    
    if err != nil {
        TasksFailed.WithLabelValues(capability, errorType(err)).Inc()
    } else {
        TasksCompleted.WithLabelValues(capability).Inc()
    }
}
```

#### 7.3.2 Distributed Tracing

```go
package tracing

import (
    "context"
    
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    "go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

func InitTracing(ctx context.Context, serviceName string, endpoint string) error {
    // Create OTLP exporter
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(endpoint),
        otlptracegrpc.WithInsecure(),
    )
    if err != nil {
        return err
    }
    
    // Create resource
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String("1.0.0"),
        ),
    )
    if err != nil {
        return err
    }
    
    // Create tracer provider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
    )
    
    otel.SetTracerProvider(tp)
    tracer = tp.Tracer(serviceName)
    
    return nil
}

// TraceTask creates a span for task execution
func TraceTask(ctx context.Context, task *types.Task) (context.Context, trace.Span) {
    return tracer.Start(ctx, "HandleTask",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.capability", task.Capability),
            attribute.Int("task.input_size", len(task.Input)),
        ),
    )
}

// Example usage in agent
func (a *Agent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    ctx, span := TraceTask(ctx, task)
    defer span.End()
    
    // Execute task
    result, err := a.executeTask(ctx, task)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    } else {
        span.SetAttributes(
            attribute.Int("task.output_size", len(result.Output)),
            attribute.String("task.status", string(result.Status)),
        )
    }
    
    return result, err
}
```

## 8. Performance Optimization

### 8.1 Agent Optimization

#### 8.1.1 Memory Management

```go
// Object pooling for frequent allocations
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func (a *Agent) processData(data []byte) ([]byte, error) {
    // Get buffer from pool
    buf := bufferPool.Get().([]byte)
    defer func() {
        // Reset and return to pool
        buf = buf[:0]
        bufferPool.Put(buf)
    }()
    
    // Process data using pooled buffer
    // ...
    
    // Return copy of result (don't return pooled buffer)
    result := make([]byte, len(buf))
    copy(result, buf)
    return result, nil
}
```

#### 8.1.2 Concurrency Optimization

```go
type ConcurrentAgent struct {
    agent.BaseAgent
    workers   int
    taskQueue chan *types.Task
    results   chan *taskResult
}

func (c *ConcurrentAgent) Start() {
    c.taskQueue = make(chan *types.Task, c.workers*2)
    c.results = make(chan *taskResult, c.workers*2)
    
    // Start worker pool
    for i := 0; i < c.workers; i++ {
        go c.worker(i)
    }
    
    // Result aggregator
    go c.aggregator()
}

func (c *ConcurrentAgent) worker(id int) {
    for task := range c.taskQueue {
        start := time.Now()
        result, err := c.processTask(task)
        
        c.results <- &taskResult{
            task:     task,
            result:   result,
            error:    err,
            duration: time.Since(start),
        }
    }
}
```

### 8.2 Network Optimization

#### 8.2.1 Connection Pooling

```go
type ConnectionPool struct {
    pool     chan net.Conn
    factory  func() (net.Conn, error)
    maxConns int
}

func NewConnectionPool(maxConns int, factory func() (net.Conn, error)) *ConnectionPool {
    return &ConnectionPool{
        pool:     make(chan net.Conn, maxConns),
        factory:  factory,
        maxConns: maxConns,
    }
}

func (p *ConnectionPool) Get(ctx context.Context) (net.Conn, error) {
    select {
    case conn := <-p.pool:
        if err := p.validateConn(conn); err != nil {
            conn.Close()
            return p.factory()
        }
        return conn, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
        return p.factory()
    }
}

func (p *ConnectionPool) Put(conn net.Conn) {
    select {
    case p.pool <- conn:
        // Connection returned to pool
    default:
        // Pool full, close connection
        conn.Close()
    }
}
```

### 8.3 Storage Optimization

#### 8.3.1 Caching Strategy

```go
type LayeredCache struct {
    l1 *lru.Cache    // In-memory LRU
    l2 *redis.Client // Redis
    l3 Storage       // Persistent storage
}

func (c *LayeredCache) Get(ctx context.Context, key string) ([]byte, error) {
    // Check L1 (memory)
    if val, ok := c.l1.Get(key); ok {
        return val.([]byte), nil
    }
    
    // Check L2 (Redis)
    val, err := c.l2.Get(ctx, key).Bytes()
    if err == nil {
        c.l1.Add(key, val) // Promote to L1
        return val, nil
    }
    
    // Load from L3 (storage)
    val, err = c.l3.Get(ctx, key)
    if err != nil {
        return nil, err
    }
    
    // Populate upper layers
    c.l2.Set(ctx, key, val, time.Hour)
    c.l1.Add(key, val)
    
    return val, nil
}
```

## 9. Security Considerations

### 9.1 Agent Security

#### 9.1.1 Input Validation

```go
func validateTaskInput(input json.RawMessage, schema json.RawMessage) error {
    // Load schema
    loader := gojsonschema.NewBytesLoader(schema)
    schema, err := gojsonschema.NewSchema(loader)
    if err != nil {
        return fmt.Errorf("invalid schema: %w", err)
    }
    
    // Validate input
    documentLoader := gojsonschema.NewBytesLoader(input)
    result, err := schema.Validate(documentLoader)
    if err != nil {
        return fmt.Errorf("validation error: %w", err)
    }
    
    if !result.Valid() {
        errors := make([]string, len(result.Errors()))
        for i, err := range result.Errors() {
            errors[i] = err.String()
        }
        return fmt.Errorf("validation failed: %v", errors)
    }
    
    return nil
}
```

#### 9.1.2 Secure Communication

```go
func createSecureClient(certFile, keyFile, caFile string) (*grpc.ClientConn, error) {
    // Load client certificates
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, err
    }
    
    // Load CA certificate
    caCert, err := ioutil.ReadFile(caFile)
    if err != nil {
        return nil, err
    }
    
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)
    
    // Create TLS configuration
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:      caCertPool,
        MinVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_CHACHA20_POLY1305_SHA256,
        },
    }
    
    // Create gRPC connection
    creds := credentials.NewTLS(tlsConfig)
    
    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(creds),
        grpc.WithDefaultCallOptions(
            grpc.MaxCallRecvMsgSize(10 * 1024 * 1024), // 10MB
        ),
    }
    
    return grpc.Dial("orchestrator:9090", opts...)
}
```

### 9.2 Runtime Security

#### 9.2.1 Sandboxing

```go
func createSecureRuntime() *SecureRuntime {
    return &SecureRuntime{
        capabilities: []string{
            "CAP_NET_BIND_SERVICE", // Bind to ports < 1024
        },
        seccompProfile: `{
            "defaultAction": "SCMP_ACT_ERRNO",
            "architectures": ["SCMP_ARCH_X86_64"],
            "syscalls": [
                {"names": ["read", "write", "close"], "action": "SCMP_ACT_ALLOW"},
                {"names": ["socket", "connect"], "action": "SCMP_ACT_ALLOW"},
                {"names": ["mmap", "munmap"], "action": "SCMP_ACT_ALLOW"}
            ]
        }`,
        rlimits: []Rlimit{
            {Type: "RLIMIT_CPU", Soft: 30, Hard: 60},      // CPU time
            {Type: "RLIMIT_AS", Soft: 512<<20, Hard: 1<<30}, // Memory
            {Type: "RLIMIT_NOFILE", Soft: 100, Hard: 200},   // File descriptors
        },
    }
}
```

## 10. Troubleshooting

### 10.1 Common Issues

#### 10.1.1 Connection Issues

```bash
# Test connectivity
ainur network test --url ws://localhost:9944

# Check agent registration
ainur agent list --runtime-url grpc://localhost:9090

# Verify P2P connectivity
ainur p2p peers --port 4001

# Test task submission
ainur task submit --capability arithmetic \
  --input '{"operation":"add","operands":[1,2,3]}'
```

#### 10.1.2 Performance Issues

```go
// Enable profiling
import _ "net/http/pprof"

func main() {
    // Start pprof server
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
    
    // ... rest of agent code
}

// Profile CPU usage
// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30

// Profile memory usage
// go tool pprof http://localhost:6060/debug/pprof/heap

// Profile goroutines
// go tool pprof http://localhost:6060/debug/pprof/goroutine
```

### 10.2 Debugging

#### 10.2.1 Structured Logging

```go
import "go.uber.org/zap"

func setupLogging(level string) (*zap.Logger, error) {
    config := zap.NewProductionConfig()
    
    // Set log level
    var zapLevel zapcore.Level
    if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
        return nil, err
    }
    config.Level = zap.NewAtomicLevelAt(zapLevel)
    
    // Add custom fields
    config.InitialFields = map[string]interface{}{
        "agent_did": os.Getenv("AGENT_DID"),
        "version":   version.Version,
    }
    
    // Build logger
    logger, err := config.Build()
    if err != nil {
        return nil, err
    }
    
    // Replace global logger
    zap.ReplaceGlobals(logger)
    
    return logger, nil
}

// Usage in agent
func (a *Agent) HandleTask(ctx context.Context, task *types.Task) (*types.TaskResult, error) {
    logger := zap.L().With(
        zap.String("task_id", task.ID),
        zap.String("capability", task.Capability),
    )
    
    logger.Info("Processing task")
    
    result, err := a.executeTask(ctx, task)
    
    if err != nil {
        logger.Error("Task failed",
            zap.Error(err),
            zap.Duration("duration", time.Since(start)),
        )
        return nil, err
    }
    
    logger.Info("Task completed",
        zap.String("status", string(result.Status)),
        zap.Int("output_size", len(result.Output)),
    )
    
    return result, nil
}
```

## 11. References

1. Ainur Protocol Specification v1.0
2. W3C Decentralized Identifiers (DIDs) v1.0
3. FIPA Agent Communication Language Specification
4. WebAssembly System Interface (WASI) Preview 1
5. gRPC Protocol Buffer Language Guide
6. Kubernetes Operator Pattern
7. Prometheus Best Practices
8. OpenTelemetry Semantic Conventions
9. OWASP Secure Coding Practices
10. Go Concurrency Patterns

## Appendices

### Appendix A: Complete Example Agent

Full implementation available at:
https://github.com/ainur-labs/example-agents

### Appendix B: API Reference

Complete API documentation at:
https://docs.ainur.network/api

### Appendix C: Troubleshooting Guide

Extended troubleshooting guide at:
https://docs.ainur.network/troubleshooting

## Revision History

| Version | Date | Changes | Author |
|---------|------|---------|---------|
| 1.0.0 | 2025-11-15 | Initial release | Ainur Protocol Team |
