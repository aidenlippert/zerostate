# Ainur Runtime Interface (ARI) v1.0

**Status**: Draft  
**Version**: 1.0.0  
**Date**: 2025-11-13  

## Abstract

The Ainur Runtime Interface (ARI) is the standardized API that every agent runtime MUST implement to participate in the Ainur protocol. This is the "HTTP" layer for agents - a minimal, runtime-agnostic interface that enables polyglot agent execution (WASM, Python, JavaScript, etc.) within a unified protocol.

## Motivation

The Ainur protocol aims to support heterogeneous agent runtimes while maintaining a consistent orchestration layer. The ARI specification ensures:

1. **Runtime Agnostic**: Orchestrators don't care if agents run in WASM, Python, or containers
2. **Minimal Interface**: Only essential methods required for protocol participation
3. **Clear Contracts**: Defined inputs/outputs for every interaction
4. **Version Stability**: Changes are additive, not breaking

## Specification

### Transport

The ARI v1.0 MAY be implemented over:
- **gRPC** (RECOMMENDED for production)
- **HTTP/REST** (for development/debugging)
- **IPC** (for local co-located runtimes)

All examples use gRPC Protocol Buffers syntax, but HTTP equivalents are provided.

---

## Core Services

### 1. Agent Information Service

**Purpose**: Discover agent capabilities and metadata.

#### `ari.v1.Agent/GetInfo`

Returns the agent's "Agent Card" - its identity, capabilities, and runtime information.

**Request**:
```protobuf
message GetInfoRequest {
  // Empty - no parameters needed
}
```

**Response**:
```protobuf
message GetInfoResponse {
  string did = 1;                      // Agent's Decentralized Identifier
  string name = 2;                     // Human-readable name
  string version = 3;                  // Agent version (semver)
  string runtime_type = 4;             // "wasm", "python", "docker", etc.
  string runtime_version = 5;          // Runtime implementation version
  repeated string capabilities = 6;    // List of capability tags (e.g., "image-ocr", "math")
  map<string, string> metadata = 7;    // Custom key-value metadata
  string endpoint = 8;                 // P2P multiaddr or HTTP endpoint
  int64 uptime_seconds = 9;            // How long runtime has been running
  ResourceLimits limits = 10;          // Resource constraints
}

message ResourceLimits {
  int64 max_memory_mb = 1;
  int64 max_execution_time_ms = 2;
  int64 max_concurrent_tasks = 3;
}
```

**HTTP Equivalent**:
```
GET /ari/v1/agent/info

Response 200 OK:
{
  "did": "did:ainur:agent:abc123",
  "name": "Math Agent",
  "version": "1.0.0",
  "runtime_type": "wasm",
  "capabilities": ["math", "arithmetic"],
  ...
}
```

---

### 2. Market Participation Service

**Purpose**: Enable agents to participate in task auctions.

#### `ari.v1.Market/ReceiveCFP`

The orchestrator sends a Call For Proposals (CFP) - an auction invitation.

**Request**:
```protobuf
message ReceiveCFPRequest {
  string cfp_id = 1;                   // Unique auction identifier
  string task_description = 2;         // Natural language task description
  repeated string required_capabilities = 3;
  double budget = 4;                   // Maximum payment (in AINU tokens)
  int64 deadline_unix = 5;             // When task must complete
  bytes task_spec = 6;                 // Optional structured task data (JSON/protobuf)
  map<string, string> constraints = 7; // Additional requirements
}
```

**Response**:
```protobuf
message ReceiveCFPResponse {
  bool interested = 1;                 // Is agent interested in bidding?
  string reason = 2;                   // Why/why not (optional)
}
```

**Notes**:
- Agent SHOULD respond quickly (&lt;100ms) to indicate interest
- This does NOT submit a bid, just signals capability to bid
- Agent MAY decline if task doesn't match capabilities or constraints

#### `ari.v1.Market/SubmitBid`

Agent submits a formal bid proposal.

**Request**:
```protobuf
message SubmitBidRequest {
  string cfp_id = 1;                   // Reference to CFP
  string bid_id = 2;                   // Unique bid identifier (agent-generated)
  double price = 3;                    // Bid price (≤ CFP budget)
  int64 estimated_duration_ms = 4;     // Expected execution time
  string did = 5;                      // Agent's DID (for verification)
  bytes signature = 6;                 // Signature over (cfp_id, bid_id, price, did)
  map<string, string> guarantees = 7;  // SLA commitments (optional)
}
```

**Response**:
```protobuf
message SubmitBidResponse {
  bool accepted = 1;                   // Was bid received successfully?
  string status = 2;                   // "pending", "rejected", "won"
  string message = 3;                  // Feedback message
}
```

---

### 3. Task Execution Service

**Purpose**: Execute winning tasks and return results.

#### `ari.v1.Task/Execute`

The orchestrator assigns a won task to the agent for execution.

**Request**:
```protobuf
message ExecuteRequest {
  string task_id = 1;                  // Unique task identifier
  string cfp_id = 2;                   // Original CFP reference
  string bid_id = 3;                   // Winning bid reference
  bytes input = 4;                     // Task input data (JSON, protobuf, or raw bytes)
  map<string, string> parameters = 5;  // Execution parameters
  int64 timeout_ms = 6;                // Maximum execution time
}
```

**Response** (Streaming):
```protobuf
message ExecuteResponse {
  string task_id = 1;
  ExecutionStatus status = 2;
  bytes output = 3;                    // Result data (if completed)
  string error = 4;                    // Error message (if failed)
  int64 progress_percent = 5;          // 0-100 (for long-running tasks)
  map<string, string> metrics = 6;     // Execution metrics (optional)
}

enum ExecutionStatus {
  UNKNOWN = 0;
  RUNNING = 1;
  COMPLETED = 2;
  FAILED = 3;
  TIMEOUT = 4;
}
```

**Notes**:
- Response MAY be streaming for long-running tasks
- Agent MUST respect timeout_ms
- Agent MUST return metrics (execution_time_ms, memory_used_mb, etc.)

---

### 4. Health Service

**Purpose**: Runtime health monitoring.

#### `ari.v1.Health/Check`

Standard health check endpoint.

**Request**:
```protobuf
message HealthCheckRequest {
  string service = 1;  // Optional: check specific service
}
```

**Response**:
```protobuf
message HealthCheckResponse {
  ServingStatus status = 1;
  string message = 2;
}

enum ServingStatus {
  UNKNOWN = 0;
  SERVING = 1;
  NOT_SERVING = 2;
  SERVICE_UNKNOWN = 3;
}
```

**HTTP Equivalent**:
```
GET /ari/v1/health

Response 200 OK:
{
  "status": "SERVING"
}
```

---

## Error Handling

All ARI methods MUST return structured errors:

```protobuf
message ARIError {
  ErrorCode code = 1;
  string message = 2;
  map<string, string> details = 3;
}

enum ErrorCode {
  UNKNOWN = 0;
  INVALID_REQUEST = 1;
  NOT_FOUND = 2;
  UNAUTHORIZED = 3;
  RESOURCE_EXHAUSTED = 4;
  DEADLINE_EXCEEDED = 5;
  INTERNAL = 6;
  UNAVAILABLE = 7;
}
```

## Implementation Requirements

### Runtime MUST Implement

1. `Agent/GetInfo` - Identity and capabilities
2. `Task/Execute` - Core execution method
3. `Health/Check` - Liveness probe

### Runtime SHOULD Implement

1. `Market/ReceiveCFP` - Participate in auctions
2. `Market/SubmitBid` - Submit competitive bids

### Runtime MAY Implement

1. Streaming responses for long-running tasks
2. Progress reporting during execution
3. Custom metadata fields

## Reference Implementations

### Go (Wasmtime)

See: `/reference-runtime-v1/wasm/` in this repository.

**Example**:
```go
type WASMRuntime struct {
    agent *AgentCard
    wasmEngine *wasmtime.Engine
}

func (r *WASMRuntime) GetInfo(ctx context.Context, req *GetInfoRequest) (*GetInfoResponse, error) {
    return &GetInfoResponse{
        DID: r.agent.DID,
        Name: "Math Agent",
        RuntimeType: "wasm",
        Capabilities: []string{"math", "arithmetic"},
    }, nil
}

func (r *WASMRuntime) Execute(ctx context.Context, req *ExecuteRequest) (*ExecuteResponse, error) {
    // Load WASM module
    // Call exported function
    // Return result
}
```

### Python

See: `/reference-runtime-v1/python/` in this repository.

**Example**:
```python
class PythonRuntime:
    def get_info(self, request):
        return {
            "did": self.agent_did,
            "runtime_type": "python",
            "capabilities": ["nlp", "sentiment-analysis"]
        }
    
    def execute(self, request):
        # Load Python module
        # Call function
        # Return result
```

## Security Considerations

1. **Authentication**: All requests MUST include valid DID signatures
2. **Sandboxing**: Runtimes MUST isolate agent execution (WASM sandbox, containers, etc.)
3. **Resource Limits**: Runtimes MUST enforce memory/CPU limits
4. **Input Validation**: Runtimes MUST validate all input data
5. **Timeout Enforcement**: Runtimes MUST kill tasks exceeding timeout

## Versioning

Future ARI versions will be additive:
- `ari.v2.Agent/GetInfo` - New fields added
- `ari.v2.Task/Cancel` - New methods added

Orchestrators MUST support backward compatibility with older ARI versions.

## References

- [gRPC Protocol Buffers](https://grpc.io/docs/what-is-grpc/introduction/)
- [OpenAPI 3.0 Specification](https://swagger.io/specification/)
- [WASM Component Model](https://github.com/WebAssembly/component-model)

## Changelog

- **v1.0.0** (2025-11-13): Initial specification

---

**License**: Apache 2.0  
**Maintainer**: Ainur Protocol Working Group
