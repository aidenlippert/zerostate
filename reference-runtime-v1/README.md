# Reference Runtime v1

A minimal reference implementation of the **ARI-v1 (Ainur Runtime Interface)** specification that can execute WASM agents.

## Overview

This runtime implements the complete ARI-v1 protocol, enabling orchestrators to:
- Discover the runtime's capabilities via `Agent/GetInfo`
- Execute tasks via `Task/Execute` (with streaming support)
- Monitor health via `Health/Check`

## Architecture

```
reference-runtime-v1/
├── pkg/ari/v1/          # Protocol Buffer definitions
│   ├── agent.proto      # Agent service
│   ├── task.proto       # Task execution service
│   ├── health.proto     # Health check service
│   └── market.proto     # Market services (CFP/bidding)
├── internal/
│   ├── agent/           # Agent info service implementation
│   ├── task/            # Task execution with WASM runner
│   ├── health/          # Health monitoring
│   └── server/          # gRPC server
├── cmd/runtime/         # Main entry point
└── testdata/            # Example configurations
```

## Quick Start

### 1. Install Dependencies

```bash
# Install Protocol Buffer compiler (if not already installed)
# Ubuntu/Debian:
sudo apt-get install -y protobuf-compiler

# macOS:
brew install protobuf

# Install Go protoc plugins
make install-proto

# Download Go dependencies
make deps
```

### 2. Generate Protocol Buffers

```bash
make proto
```

This generates Go code from the `.proto` files in `pkg/ari/v1/`.

### 3. Build the Runtime

```bash
make build
```

This creates `bin/runtime` binary.

### 4. Run the Runtime

```bash
./bin/runtime --agent-config testdata/math-agent.yaml
```

The runtime will:
- Load the math WASM agent
- Start gRPC server on `localhost:9000`
- Wait for task execution requests

## Testing with grpcurl

### Install grpcurl

```bash
# macOS
brew install grpcurl

# Ubuntu/Debian
sudo apt-get install grpcurl

# Or using Go
go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
```

### Test Agent/GetInfo

```bash
grpcurl -plaintext localhost:9000 ari.v1.Agent/GetInfo
```

Expected output:
```json
{
  "did": "did:ainur:agent:math-001",
  "name": "Math Agent",
  "version": "1.0.0",
  "capabilities": ["math", "arithmetic", "algebra"],
  "runtimeInfo": {
    "type": "wasm",
    "version": "1.0.0"
  },
  "limits": {
    "maxMemoryMb": 128,
    "maxExecutionTimeMs": 5000,
    "maxConcurrentTasks": 10
  }
}
```

### Test Health/Check

```bash
grpcurl -plaintext localhost:9000 ari.v1.Health/Check
```

Expected output:
```json
{
  "status": "HEALTH_STATUS_SERVING",
  "message": "Runtime is healthy",
  "activeTasks": 0,
  "totalTasksProcessed": "0",
  "uptimeSeconds": "42"
}
```

### Test Task/Execute

```bash
grpcurl -plaintext -d '{
  "task_id": "test-001",
  "input": "{\"function\":\"add\",\"args\":[5,7]}"
}' localhost:9000 ari.v1.Task/Execute
```

Expected output (streaming):
```json
{
  "taskId": "test-001",
  "status": "TASK_STATUS_RUNNING",
  "progress": 0,
  "progressMessage": "Task started"
}
{
  "taskId": "test-001",
  "status": "TASK_STATUS_COMPLETED",
  "result": "12",
  "executionMs": "2",
  "progress": 1,
  "progressMessage": "Task completed successfully"
}
```

## Configuration

Example agent configuration (`testdata/math-agent.yaml`):

```yaml
agent:
  did: did:ainur:agent:math-001
  name: Math Agent
  version: 1.0.0
  
  runtime:
    type: wasm
    path: ../../agents/math-agent-rust/target/wasm32-unknown-unknown/release/math_agent.wasm
  
  capabilities:
    - math
    - arithmetic
    - algebra
  
  limits:
    max_memory_mb: 128
    max_execution_time_ms: 5000
    max_concurrent_tasks: 10

server:
  host: "0.0.0.0"
  port: 9000

p2p:
  enabled: true
  presence_topic: ainur/v1/global/l3_aether/presence/did:ainur:agent:math-001
  heartbeat_interval: 30

logging:
  level: info
  format: json
```

## Development

### Run in Development Mode

```bash
make dev
```

This uses [air](https://github.com/cosmtrek/air) to automatically rebuild on file changes.

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

### Lint Code

```bash
make lint
```

## ARI-v1 Protocol

This runtime implements the full ARI-v1 specification. See `/specs/L5-ARI-v1.md` for details.

### Supported Services

| Service | Methods | Status |
|---------|---------|--------|
| `ari.v1.Agent` | `GetInfo` | ✅ Implemented |
| `ari.v1.Task` | `Execute` | ✅ Implemented |
| `ari.v1.Health` | `Check` | ✅ Implemented |
| `ari.v1.Market` | `ReceiveCFP`, `SubmitBid` | ⏳ Planned for Sprint 3 |

### Task Input Format

Tasks expect JSON input with:
- `function`: Name of the WASM function to call (e.g., `"add"`, `"multiply"`)
- `args`: Array of arguments (e.g., `[5, 7]`)

Example:
```json
{
  "function": "add",
  "args": [5, 7]
}
```

### Task Output Format

Successful execution returns JSON with the result:
```json
{
  "taskId": "test-001",
  "status": "TASK_STATUS_COMPLETED",
  "result": "12",
  "executionMs": "2"
}
```

Failed execution returns error:
```json
{
  "taskId": "test-001",
  "status": "TASK_STATUS_FAILED",
  "error": "function not found: invalid_func",
  "executionMs": "1"
}
```

## Integration with Orchestrator

The orchestrator discovers this runtime via L3 Aether topics (Sprint 1 Phase 3).

**Discovery flow**:
1. Runtime publishes presence to `ainur/v1/global/l3_aether/presence/{did}`
2. Orchestrator subscribes to `ainur/v1/global/l3_aether/presence/*`
3. Orchestrator receives agent card from presence message
4. Orchestrator creates gRPC client to runtime endpoint
5. Orchestrator calls `GetInfo()` to verify capabilities
6. Orchestrator routes tasks to runtime via `Execute()`

## Roadmap

### Sprint 1 Phase 2 (Current)
- [x] Protocol Buffer definitions
- [x] Agent service (GetInfo)
- [x] Task service (Execute with WASM)
- [x] Health service (Check)
- [x] gRPC server
- [x] YAML configuration
- [ ] P2P presence publishing (Phase 3)

### Sprint 1 Phase 3
- [ ] libp2p integration for L3 Aether
- [ ] Publish presence heartbeats
- [ ] Subscribe to CFP topics
- [ ] Integration test with orchestrator

### Sprint 3
- [ ] Market service implementation
- [ ] CFP handling and bidding logic
- [ ] Reputation tracking

## License

MIT

## Contributing

See `/CONTRIBUTING.md` for guidelines.

---

**Built with ❤️ for the Ainur protocol**
