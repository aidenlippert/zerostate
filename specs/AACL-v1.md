# AACL-v1 Specification

**Version:** 1.0.0  
**Status:** Draft  
**Created:** 2025-11-13  
**Authors:** Ainur Protocol Team

---

## Abstract

AACL-v1 (Ainur Agent Communication Language) is a semantic protocol for structured communication between autonomous agents, orchestrators, and users in the Ainur ecosystem. AACL defines message formats, conversation patterns, intent resolution, and semantic grounding that enable agents to understand requests, negotiate capabilities, and coordinate complex multi-agent workflows.

## Motivation

Current agent communication suffers from:

1. **Ambiguity**: Natural language is imprecise for machine interpretation
2. **Incompatibility**: Each agent speaks a different API dialect
3. **Context Loss**: No standard for maintaining conversation state
4. **Composability**: Hard to chain multiple agents without custom glue code
5. **Verification**: Difficult to prove what was requested vs. delivered

AACL solves these problems with a **semantic-first, JSON-LD based** communication protocol that balances human readability with machine precision.

---

## 1. Core Principles

### 1.1 Design Philosophy

1. **Semantic Clarity**: Every message has unambiguous meaning
2. **Progressive Disclosure**: Simple for basic use, powerful for advanced scenarios
3. **Human-in-the-Loop**: Natural language coexists with structured data
4. **Provenance**: All messages are signed and traceable
5. **Composability**: Messages can be chained into workflows

### 1.2 Protocol Layers

```
┌─────────────────────────────────────────┐
│  Layer 4: Workflow Orchestration       │  Multi-agent coordination
├─────────────────────────────────────────┤
│  Layer 3: Conversation Management      │  Session state, context
├─────────────────────────────────────────┤
│  Layer 2: Intent Resolution            │  Parse goals into actions
├─────────────────────────────────────────┤
│  Layer 1: Message Format               │  JSON-LD structure
└─────────────────────────────────────────┘
```

---

## 2. Message Format (Layer 1)

### 2.1 Base Message Structure

All AACL messages follow this JSON-LD structure:

```json
{
  "@context": "https://ainur.network/contexts/aacl/v1",
  "@type": "AACLMessage",
  "id": "msg:550e8400-e29b-41d4-a716-446655440000",
  "conversation_id": "conv:abc123",
  "timestamp": "2025-11-13T12:00:00Z",
  "from": "did:ainur:user:alice",
  "to": "did:ainur:agent:math-001",
  "intent": { /* See Section 3 */ },
  "payload": { /* Message-specific data */ },
  "metadata": {
    "priority": "normal",
    "timeout_ms": 5000,
    "language": "en",
    "user_agent": "ainur-sdk/1.0.0"
  },
  "signature": { /* Cryptographic proof */ }
}
```

### 2.2 Message Types

| Type | Description | Direction |
|------|-------------|-----------|
| `Request` | User/agent requests action | User → Agent |
| `Response` | Agent returns result | Agent → User |
| `Query` | Information request | Any → Any |
| `Notification` | Event broadcast | Agent → Subscribers |
| `Negotiation` | Capability/price negotiation | Bidirectional |
| `Error` | Failure notification | Any → Any |
| `Acknowledgment` | Receipt confirmation | Any → Any |

---

## 3. Intent Resolution (Layer 2)

### 3.1 Intent Schema

Intents describe **what** the user wants, not **how** to achieve it:

```json
{
  "intent": {
    "action": "compute",
    "goal": "Calculate the square root of 144",
    "capabilities_required": ["math.arithmetic"],
    "constraints": {
      "max_execution_time_ms": 1000,
      "precision": "float64"
    },
    "parameters": {
      "operation": "sqrt",
      "input": 144
    }
  }
}
```

### 3.2 Intent Action Vocabulary

Standard actions (extensible):

| Action | Description | Example |
|--------|-------------|---------|
| `compute` | Perform calculation | "Calculate 5 + 7" |
| `transform` | Convert data format | "Convert JSON to YAML" |
| `analyze` | Extract insights | "Summarize this document" |
| `generate` | Create new content | "Write a poem about AI" |
| `search` | Find information | "Search for agents with math capability" |
| `execute` | Run a program | "Execute this WASM module" |
| `coordinate` | Multi-agent workflow | "Build a web app (design + code + test)" |

### 3.3 Natural Language to Intent

AACL supports **hybrid** mode: natural language + structured fallback.

**User says**: "Calculate the square root of 144"

**AACL message**:
```json
{
  "intent": {
    "action": "compute",
    "goal": "Calculate the square root of 144",
    "natural_language": "Calculate the square root of 144",
    "parsed": {
      "operation": "sqrt",
      "arguments": [144]
    },
    "confidence": 0.95
  }
}
```

If `confidence < 0.8`, agent responds with **clarification request**:

```json
{
  "@type": "ClarificationRequest",
  "question": "Did you mean: sqrt(144) = 12?",
  "suggestions": [
    { "interpretation": "sqrt(144)", "confidence": 0.95 },
    { "interpretation": "144^(1/2)", "confidence": 0.92 }
  ]
}
```

---

## 4. Conversation Management (Layer 3)

### 4.1 Conversation Context

AACL maintains **stateful conversations**:

```json
{
  "conversation_id": "conv:abc123",
  "participants": [
    "did:ainur:user:alice",
    "did:ainur:agent:math-001"
  ],
  "created_at": "2025-11-13T12:00:00Z",
  "context": {
    "topic": "mathematical_computation",
    "previous_results": [
      {
        "message_id": "msg:xyz789",
        "result": 12,
        "timestamp": "2025-11-13T12:01:00Z"
      }
    ],
    "shared_state": {
      "precision": "float64",
      "unit_system": "metric"
    }
  },
  "status": "active"
}
```

### 4.2 Context References

Messages can reference previous context:

```json
{
  "intent": {
    "action": "compute",
    "goal": "Multiply the previous result by 2",
    "context_reference": {
      "message_id": "msg:xyz789",
      "field": "result"
    },
    "parameters": {
      "operation": "multiply",
      "input": "$context.previous_results[0].result",
      "multiplier": 2
    }
  }
}
```

**Resolution**: Agent fetches `result = 12` from context, computes `12 * 2 = 24`.

### 4.3 Session Management

```json
{
  "@type": "SessionControl",
  "action": "start | pause | resume | end",
  "conversation_id": "conv:abc123",
  "reason": "User requested pause"
}
```

---

## 5. Workflow Orchestration (Layer 4)

### 5.1 Multi-Agent Workflows

AACL supports **declarative workflows** where multiple agents coordinate:

```json
{
  "@type": "WorkflowRequest",
  "workflow_id": "wf:build-website",
  "goal": "Build a landing page for my startup",
  "steps": [
    {
      "step_id": "design",
      "agent_capability": "design.ui",
      "intent": {
        "action": "generate",
        "goal": "Design a modern landing page",
        "parameters": {
          "style": "minimalist",
          "colors": ["#1a1a1a", "#ffffff"]
        }
      },
      "outputs": ["design_mockup"]
    },
    {
      "step_id": "code",
      "agent_capability": "code.frontend",
      "intent": {
        "action": "generate",
        "goal": "Convert design to HTML/CSS",
        "inputs": ["$steps.design.outputs.design_mockup"]
      },
      "outputs": ["html_code", "css_code"]
    },
    {
      "step_id": "test",
      "agent_capability": "qa.accessibility",
      "intent": {
        "action": "analyze",
        "goal": "Check accessibility compliance",
        "inputs": ["$steps.code.outputs.html_code"]
      },
      "outputs": ["test_report"]
    }
  ],
  "dependencies": {
    "code": ["design"],
    "test": ["code"]
  }
}
```

### 5.2 Workflow Execution

Orchestrator manages workflow:

1. **Parse**: Validate workflow structure
2. **Plan**: Resolve dependencies into execution DAG
3. **Discover**: Find agents for each step
4. **Execute**: Run steps in dependency order
5. **Monitor**: Track progress, handle failures
6. **Complete**: Return final outputs

**Status updates**:
```json
{
  "@type": "WorkflowStatus",
  "workflow_id": "wf:build-website",
  "status": "in_progress",
  "completed_steps": ["design", "code"],
  "current_step": "test",
  "progress": 0.66
}
```

### 5.3 Error Handling

Workflows handle failures gracefully:

```json
{
  "@type": "WorkflowError",
  "workflow_id": "wf:build-website",
  "failed_step": "code",
  "error": {
    "code": "CAPABILITY_NOT_FOUND",
    "message": "No agents available with capability: code.frontend"
  },
  "recovery_options": [
    {
      "action": "retry",
      "description": "Wait for agent to become available"
    },
    {
      "action": "substitute",
      "description": "Use alternative capability: code.fullstack"
    },
    {
      "action": "abort",
      "description": "Cancel workflow and refund budget"
    }
  ]
}
```

---

## 6. Request-Response Patterns

### 6.1 Synchronous Request

**Request**:
```json
{
  "@type": "Request",
  "id": "msg:001",
  "from": "did:ainur:user:alice",
  "to": "did:ainur:agent:math-001",
  "intent": {
    "action": "compute",
    "parameters": {
      "operation": "add",
      "a": 5,
      "b": 7
    }
  }
}
```

**Response**:
```json
{
  "@type": "Response",
  "id": "msg:002",
  "in_reply_to": "msg:001",
  "from": "did:ainur:agent:math-001",
  "to": "did:ainur:user:alice",
  "status": "success",
  "result": {
    "value": 12,
    "unit": null,
    "confidence": 1.0
  },
  "execution_metadata": {
    "duration_ms": 125,
    "gas_used": 100,
    "cost_uainur": 10
  }
}
```

### 6.2 Asynchronous Request

For long-running tasks:

**Request**:
```json
{
  "@type": "AsyncRequest",
  "id": "msg:003",
  "intent": {
    "action": "analyze",
    "goal": "Analyze this 100-page document"
  },
  "callback": "https://user.example.com/webhook"
}
```

**Acknowledgment**:
```json
{
  "@type": "Acknowledgment",
  "id": "msg:004",
  "in_reply_to": "msg:003",
  "status": "accepted",
  "task_id": "task:xyz789",
  "estimated_completion": "2025-11-13T12:30:00Z"
}
```

**Completion** (sent to callback):
```json
{
  "@type": "AsyncResponse",
  "task_id": "task:xyz789",
  "status": "completed",
  "result": { /* Analysis results */ }
}
```

### 6.3 Streaming Response

For real-time results:

```json
{
  "@type": "StreamingResponse",
  "id": "msg:005",
  "in_reply_to": "msg:001",
  "stream_id": "stream:abc",
  "chunk_index": 0,
  "chunk_data": "The result is ",
  "is_final": false
}
```

```json
{
  "@type": "StreamingResponse",
  "stream_id": "stream:abc",
  "chunk_index": 1,
  "chunk_data": "12",
  "is_final": true
}
```

---

## 7. Capability Negotiation

### 7.1 Discovery Request

**User**: "Which agents can solve quadratic equations?"

```json
{
  "@type": "CapabilityQuery",
  "capabilities": {
    "domains": ["math"],
    "operations": ["solve_quadratic"]
  },
  "constraints": {
    "max_price_uainur": 1000,
    "min_trust_score": 80
  }
}
```

**Response**:
```json
{
  "@type": "CapabilityQueryResponse",
  "matches": [
    {
      "agent_did": "did:ainur:agent:math-001",
      "agent_name": "Math Specialist",
      "trust_score": 95,
      "price_uainur": 500,
      "estimated_time_ms": 200
    },
    {
      "agent_did": "did:ainur:agent:algebra-pro",
      "agent_name": "Algebra Pro",
      "trust_score": 88,
      "price_uainur": 300,
      "estimated_time_ms": 150
    }
  ]
}
```

### 7.2 Price Negotiation

```json
{
  "@type": "PriceNegotiation",
  "action": "offer",
  "agent_did": "did:ainur:agent:math-001",
  "task_description": "Solve 1000 quadratic equations",
  "user_offer_uainur": 400000,
  "agent_asking_price_uainur": 500000
}
```

**Agent response**:
```json
{
  "@type": "PriceNegotiation",
  "action": "counter_offer",
  "counter_price_uainur": 450000,
  "reasoning": "Bulk discount applied (10% off)"
}
```

---

## 8. Error Handling

### 8.1 Error Message Format

```json
{
  "@type": "Error",
  "id": "msg:err001",
  "in_reply_to": "msg:001",
  "error": {
    "code": "INVALID_INPUT",
    "message": "Parameter 'a' must be a number, got string",
    "details": {
      "field": "parameters.a",
      "expected_type": "number",
      "actual_type": "string",
      "actual_value": "hello"
    },
    "recovery": {
      "action": "fix_input",
      "suggestion": "Convert 'hello' to number or provide valid numeric input"
    }
  }
}
```

### 8.2 Standard Error Codes

| Code | Description | Recovery |
|------|-------------|----------|
| `INVALID_INPUT` | Malformed request | Fix input, retry |
| `CAPABILITY_NOT_FOUND` | No matching agent | Broaden search, wait |
| `INSUFFICIENT_BUDGET` | Not enough tokens | Add budget, reduce scope |
| `TIMEOUT` | Execution exceeded limit | Increase timeout, simplify |
| `AGENT_UNAVAILABLE` | Agent offline | Retry later, find alternative |
| `EXECUTION_FAILED` | Internal agent error | Report bug, use backup |
| `UNAUTHORIZED` | Access denied | Authenticate, check permissions |

---

## 9. Semantic Grounding

### 9.1 Ontology References

AACL messages can reference shared ontologies:

```json
{
  "intent": {
    "action": "compute",
    "parameters": {
      "operation": "https://schema.org/MathematicalOperation#Addition",
      "inputs": [
        {
          "@type": "https://schema.org/Number",
          "value": 5
        },
        {
          "@type": "https://schema.org/Number",
          "value": 7
        }
      ]
    }
  }
}
```

### 9.2 Unit Awareness

AACL supports unit-aware computation:

```json
{
  "intent": {
    "action": "compute",
    "goal": "Convert 100 kilometers to miles",
    "parameters": {
      "operation": "convert",
      "input": {
        "value": 100,
        "unit": "http://qudt.org/vocab/unit/KiloM"
      },
      "target_unit": "http://qudt.org/vocab/unit/Mile"
    }
  }
}
```

**Response**:
```json
{
  "result": {
    "value": 62.137,
    "unit": "http://qudt.org/vocab/unit/Mile"
  }
}
```

---

## 10. Security & Privacy

### 10.1 Message Signing

All AACL messages MUST be signed:

```json
{
  "signature": {
    "type": "Ed25519Signature2020",
    "created": "2025-11-13T12:00:00Z",
    "verificationMethod": "did:ainur:user:alice#keys-1",
    "proofPurpose": "authentication",
    "proofValue": "z3MvGc..."
  }
}
```

### 10.2 Encryption

Sensitive payloads can be encrypted:

```json
{
  "payload": {
    "@encrypted": true,
    "algorithm": "ECIES-secp256k1",
    "recipient": "did:ainur:agent:math-001#keys-1",
    "ciphertext": "base64-encoded-encrypted-data",
    "nonce": "base64-nonce"
  }
}
```

### 10.3 Access Control

Messages can specify authorization:

```json
{
  "authorization": {
    "required_role": "premium_user",
    "required_reputation": 50,
    "allowed_dids": ["did:ainur:agent:math-001"]
  }
}
```

---

## 11. SDK Examples

### 11.1 Python SDK

```python
from ainur_sdk import AACLClient, Intent

client = AACLClient(did="did:ainur:user:alice")

# Simple request
response = client.send(
    to="did:ainur:agent:math-001",
    intent=Intent(
        action="compute",
        parameters={
            "operation": "add",
            "a": 5,
            "b": 7
        }
    )
)

print(response.result.value)  # 12
```

### 11.2 JavaScript SDK

```javascript
import { AACLClient, Intent } from 'ainur-sdk';

const client = new AACLClient({ did: 'did:ainur:user:alice' });

const response = await client.send({
  to: 'did:ainur:agent:math-001',
  intent: new Intent({
    action: 'compute',
    parameters: {
      operation: 'add',
      a: 5,
      b: 7
    }
  })
});

console.log(response.result.value); // 12
```

### 11.3 Rust SDK

```rust
use ainur_sdk::{AACLClient, Intent};

let client = AACLClient::new("did:ainur:user:alice")?;

let response = client.send(
    "did:ainur:agent:math-001",
    Intent::new("compute")
        .with_param("operation", "add")
        .with_param("a", 5)
        .with_param("b", 7)
).await?;

println!("{}", response.result.value); // 12
```

---

## 12. Interoperability

### 12.1 HTTP/REST Binding

AACL messages can be sent over HTTP:

```http
POST /aacl/v1/messages HTTP/1.1
Host: agent.example.com
Content-Type: application/ld+json
Authorization: Bearer <did-token>

{
  "@context": "https://ainur.network/contexts/aacl/v1",
  "@type": "Request",
  "intent": { ... }
}
```

### 12.2 gRPC Binding

```protobuf
service AACL {
  rpc Send(AACLMessage) returns (AACLMessage);
  rpc Stream(AACLMessage) returns (stream AACLMessage);
}
```

### 12.3 WebSocket Binding

Real-time bidirectional communication:

```javascript
const ws = new WebSocket('wss://agent.example.com/aacl/v1/stream');

ws.send(JSON.stringify({
  "@type": "Request",
  "intent": { ... }
}));

ws.onmessage = (event) => {
  const response = JSON.parse(event.data);
  console.log(response.result);
};
```

---

## 13. Performance Optimization

### 13.1 Message Compression

Large payloads can be compressed:

```json
{
  "payload": {
    "@compressed": true,
    "algorithm": "gzip",
    "data": "base64-compressed-data",
    "original_size_bytes": 10485760
  }
}
```

### 13.2 Batching

Multiple requests in one message:

```json
{
  "@type": "BatchRequest",
  "requests": [
    { "id": "req1", "intent": { "action": "compute", ... } },
    { "id": "req2", "intent": { "action": "compute", ... } },
    { "id": "req3", "intent": { "action": "compute", ... } }
  ]
}
```

---

## 14. Versioning & Evolution

### 14.1 Backward Compatibility

AACL uses semantic versioning:
- **Minor**: Add optional fields (backward compatible)
- **Major**: Breaking changes (require migration)

### 14.2 Feature Detection

Agents advertise AACL features:

```json
{
  "aacl_version": "1.0.0",
  "supported_features": [
    "streaming",
    "workflows",
    "negotiation",
    "encryption"
  ]
}
```

---

## 15. Complete Example: Multi-Agent Workflow

**User**: "Build me a landing page with a signup form"

**AACL Workflow**:

```json
{
  "@context": "https://ainur.network/contexts/aacl/v1",
  "@type": "WorkflowRequest",
  "id": "msg:workflow001",
  "from": "did:ainur:user:alice",
  "conversation_id": "conv:landing-page",
  "intent": {
    "action": "coordinate",
    "goal": "Build a landing page with signup form"
  },
  "workflow": {
    "steps": [
      {
        "step_id": "design",
        "agent_capability": "design.ui",
        "intent": {
          "action": "generate",
          "goal": "Design modern landing page",
          "parameters": {
            "components": ["hero", "signup_form", "footer"],
            "style": "minimalist"
          }
        }
      },
      {
        "step_id": "code",
        "agent_capability": "code.frontend",
        "intent": {
          "action": "generate",
          "goal": "Convert design to React components",
          "inputs": ["$steps.design.outputs.mockup"]
        }
      },
      {
        "step_id": "backend",
        "agent_capability": "code.backend",
        "intent": {
          "action": "generate",
          "goal": "Create signup API endpoint",
          "parameters": {
            "database": "postgresql",
            "validation": ["email", "password_strength"]
          }
        }
      },
      {
        "step_id": "test",
        "agent_capability": "qa.e2e",
        "intent": {
          "action": "execute",
          "goal": "Run end-to-end tests",
          "inputs": [
            "$steps.code.outputs.components",
            "$steps.backend.outputs.api"
          ]
        }
      }
    ]
  },
  "budget_uainur": 50000,
  "deadline": "2025-11-13T18:00:00Z"
}
```

**Orchestrator executes workflow, returns**:

```json
{
  "@type": "WorkflowResponse",
  "in_reply_to": "msg:workflow001",
  "status": "completed",
  "outputs": {
    "design": {
      "mockup_url": "https://r2.ainur.network/mockups/abc123.png",
      "agent": "did:ainur:agent:design-ai"
    },
    "code": {
      "github_repo": "https://github.com/user/landing-page",
      "preview_url": "https://preview.ainur.network/abc123",
      "agent": "did:ainur:agent:coder-pro"
    },
    "backend": {
      "api_endpoint": "https://api.example.com/signup",
      "agent": "did:ainur:agent:backend-wizard"
    },
    "test": {
      "test_report": "All 15 tests passed ✅",
      "coverage": "98%",
      "agent": "did:ainur:agent:qa-bot"
    }
  },
  "total_cost_uainur": 35000,
  "execution_time_ms": 45000
}
```

---

## 16. References

- [JSON-LD 1.1](https://www.w3.org/TR/json-ld11/)
- [W3C Verifiable Credentials](https://www.w3.org/TR/vc-data-model/)
- [Schema.org](https://schema.org/)
- [QUDT Units Ontology](http://qudt.org/)
- [Ainur AgentCard-VC-v1](./AgentCard-VC-v1.md)
- [Ainur ARI-v1 Protocol](./ARI-v1.md)

---

## Appendix A: JSON Schema

See [aacl-v1.schema.json](../schemas/aacl-v1.schema.json) for complete validation schema.

## Appendix B: AACL Context Definition

```json
{
  "@context": {
    "aacl": "https://ainur.network/vocab/aacl#",
    "intent": "aacl:intent",
    "action": "aacl:action",
    "goal": "aacl:goal",
    "parameters": "aacl:parameters",
    "result": "aacl:result",
    "workflow": "aacl:workflow"
  }
}
```

---

**Status**: Draft - Open for community feedback  
**License**: Apache 2.0  
**Maintainers**: Ainur Protocol Team
