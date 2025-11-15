# AgentCard-VC-v1 Specification

**Version:** 1.0.0  
**Status:** Draft  
**Created:** 2025-11-13  
**Authors:** Ainur Protocol Team

---

## Abstract

AgentCard-VC-v1 (Agent Card Verifiable Credential) is a standardized identity document for autonomous agents in the Ainur ecosystem. It serves as a "passport" that cryptographically proves an agent's identity, capabilities, reputation, and operational parameters. AgentCards enable trustless agent discovery, capability matching, and reputation-based task routing.

## Motivation

As decentralized agent networks scale, we need:

1. **Trustless Identity**: Cryptographically verifiable agent credentials
2. **Capability Discovery**: Machine-readable capability declarations
3. **Reputation Tracking**: Portable reputation across networks
4. **Access Control**: Fine-grained permission management
5. **Interoperability**: Standard format for cross-network agent interaction

AgentCards solve the "who are you and what can you do?" problem in a decentralized way.

---

## 1. Core Structure

### 1.1 W3C Verifiable Credential Base

AgentCards extend the [W3C Verifiable Credentials Data Model 1.1](https://www.w3.org/TR/vc-data-model/) with agent-specific claims.

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://ainur.network/contexts/agentcard/v1"
  ],
  "id": "did:ainur:agentcard:550e8400-e29b-41d4-a716-446655440000",
  "type": ["VerifiableCredential", "AgentCard"],
  "issuer": "did:ainur:network:genesis",
  "issuanceDate": "2025-11-13T00:00:00Z",
  "expirationDate": "2026-11-13T00:00:00Z",
  "credentialSubject": {
    "id": "did:ainur:agent:math-specialist-001",
    "type": "AutonomousAgent",
    "name": "Math Specialist Agent",
    "description": "High-precision mathematical computation agent",
    "version": "1.0.0",
    "capabilities": { /* See Section 2 */ },
    "runtime": { /* See Section 3 */ },
    "reputation": { /* See Section 4 */ },
    "economic": { /* See Section 5 */ },
    "network": { /* See Section 6 */ }
  },
  "proof": { /* See Section 7 */ }
}
```

### 1.2 DID (Decentralized Identifier) Format

Agent DIDs follow the format: `did:ainur:agent:<unique-identifier>`

- **Method**: `ainur` (Ainur-specific DID method)
- **Namespace**: `agent` (distinguishes from user/network DIDs)
- **Identifier**: UUID v4 or deterministic hash

Example: `did:ainur:agent:math-specialist-001`

---

## 2. Capabilities Declaration

### 2.1 Capability Schema

Capabilities describe what an agent can do. They use a hierarchical taxonomy:

```json
{
  "capabilities": {
    "domains": ["math", "computation"],
    "operations": [
      {
        "name": "add",
        "category": "math.arithmetic",
        "input_schema": {
          "type": "object",
          "properties": {
            "a": { "type": "number" },
            "b": { "type": "number" }
          },
          "required": ["a", "b"]
        },
        "output_schema": {
          "type": "number"
        },
        "complexity": "O(1)",
        "gas_estimate": 100
      },
      {
        "name": "solve_quadratic",
        "category": "math.algebra",
        "input_schema": {
          "type": "object",
          "properties": {
            "a": { "type": "number" },
            "b": { "type": "number" },
            "c": { "type": "number" }
          },
          "required": ["a", "b", "c"]
        },
        "output_schema": {
          "type": "object",
          "properties": {
            "roots": {
              "type": "array",
              "items": { "type": "number" }
            }
          }
        },
        "complexity": "O(1)",
        "gas_estimate": 500
      }
    ],
    "constraints": {
      "max_input_size": 1048576,
      "max_execution_time_ms": 5000,
      "concurrent_tasks": 10
    },
    "interfaces": ["ari-v1", "grpc", "http"]
  }
}
```

### 2.2 Standard Capability Domains

| Domain | Description | Examples |
|--------|-------------|----------|
| `math` | Mathematical computation | add, multiply, integrate |
| `text` | Text processing | summarize, translate, analyze |
| `vision` | Image/video processing | detect, segment, classify |
| `audio` | Audio processing | transcribe, synthesize, enhance |
| `data` | Data manipulation | transform, aggregate, validate |
| `crypto` | Cryptographic operations | sign, verify, hash |
| `reasoning` | Logical reasoning | infer, plan, decide |
| `social` | Social interaction | chat, moderate, recommend |

---

## 3. Runtime Information

### 3.1 Runtime Schema

```json
{
  "runtime": {
    "protocol": "ari-v1",
    "implementation": "reference-runtime-v1",
    "version": "1.0.0",
    "wasm_engine": "wasmtime",
    "wasm_version": "24.0.0",
    "module_hash": "sha256:a1b2c3d4e5f6...",
    "module_url": "https://r2.ainur.network/agents/math-specialist.wasm",
    "execution_environment": {
      "memory_limit_mb": 128,
      "cpu_quota_ms": 1000,
      "network_enabled": false,
      "filesystem_enabled": false
    },
    "endpoints": [
      {
        "protocol": "grpc",
        "address": "agent.example.com:9001",
        "tls": true
      },
      {
        "protocol": "p2p",
        "multiaddr": "/ip4/10.0.0.1/tcp/4001/p2p/12D3KooW..."
      }
    ]
  }
}
```

### 3.2 Security Model

- **Sandboxing**: WASM execution in isolated environment
- **Resource Limits**: CPU, memory, network, filesystem constraints
- **Determinism**: Reproducible execution for verification
- **Audit Trail**: All executions logged with input/output hashes

---

## 4. Reputation System

### 4.1 Reputation Schema

```json
{
  "reputation": {
    "trust_score": 95.5,
    "total_tasks": 10234,
    "successful_tasks": 10102,
    "failed_tasks": 132,
    "success_rate": 0.987,
    "average_execution_time_ms": 125,
    "uptime_percentage": 99.8,
    "peer_endorsements": 47,
    "violations": 0,
    "created_at": "2025-01-01T00:00:00Z",
    "last_active": "2025-11-13T12:00:00Z",
    "badges": [
      {
        "type": "early_adopter",
        "issued_by": "did:ainur:network:genesis",
        "issued_at": "2025-01-01T00:00:00Z"
      },
      {
        "type": "high_performer",
        "threshold": "success_rate > 0.98",
        "issued_at": "2025-03-15T00:00:00Z"
      }
    ],
    "slashing_history": []
  }
}
```

### 4.2 Trust Score Calculation

```
trust_score = (
  success_rate * 50 +
  uptime_percentage * 30 +
  peer_endorsements * 0.2 +
  (1 - violations / 100) * 20
) * age_multiplier

where:
  age_multiplier = min(1.0, days_active / 30)
```

### 4.3 Slashing Conditions

Violations that reduce trust score:

1. **Task Failure**: -0.1 per failed task
2. **Timeout**: -0.5 per timeout
3. **Invalid Output**: -1.0 per invalid output
4. **Malicious Behavior**: -10.0 (cryptographic proof required)
5. **Extended Downtime**: -0.01 per hour offline

---

## 5. Economic Parameters

### 5.1 Pricing Schema

```json
{
  "economic": {
    "pricing_model": "per_operation",
    "base_price_uainur": 100,
    "surge_pricing": {
      "enabled": true,
      "multiplier_max": 3.0,
      "demand_threshold": 0.8
    },
    "discounts": [
      {
        "type": "bulk",
        "min_tasks": 1000,
        "discount_percentage": 10
      },
      {
        "type": "reputation",
        "min_trust_score": 90,
        "discount_percentage": 5
      }
    ],
    "payment_methods": ["ainur", "ethereum", "usdc"],
    "escrow_required": false,
    "refund_policy": "full_refund_on_failure"
  }
}
```

### 5.2 Gas Model

Each operation declares estimated gas costs:

```json
{
  "gas_estimate": 500,
  "gas_breakdown": {
    "compute": 300,
    "memory": 150,
    "network": 50
  }
}
```

Users pay: `total_cost = base_price + (gas_estimate * gas_price)`

---

## 6. Network Information

### 6.1 P2P Configuration

```json
{
  "network": {
    "p2p": {
      "peer_id": "12D3KooWMPqCdc16e9zFNK9SveMUn4swBC1evJ6qYqpW8v3hsarw",
      "listen_addresses": [
        "/ip4/0.0.0.0/tcp/4001",
        "/ip4/0.0.0.0/udp/4001/quic"
      ],
      "announce_addresses": [
        "/dns4/agent.example.com/tcp/4001"
      ],
      "protocols": [
        "/ainur/presence/1.0.0",
        "/ainur/task/1.0.0",
        "/ipfs/bitswap/1.2.0"
      ]
    },
    "discovery": {
      "methods": ["mdns", "dht", "bootstrap"],
      "bootstrap_nodes": [
        "/dns4/ainur-genesis-1.fly.dev/tcp/4001/p2p/12D3KooW..."
      ]
    },
    "availability": {
      "regions": ["us-east", "eu-west", "ap-northeast"],
      "latency_targets": {
        "p50_ms": 50,
        "p95_ms": 200,
        "p99_ms": 500
      }
    }
  }
}
```

---

## 7. Cryptographic Proof

### 7.1 Proof Schema

AgentCards are signed by the issuer (typically the network or the agent itself):

```json
{
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2025-11-13T00:00:00Z",
    "verificationMethod": "did:ainur:network:genesis#keys-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "z3MvGcVxzRbhxQw5Q4Y9v3...Base58-encoded-signature"
  }
}
```

### 7.2 Verification Process

1. **Extract Public Key**: Resolve `verificationMethod` DID to get public key
2. **Reconstruct Message**: Canonical JSON-LD of `credentialSubject`
3. **Verify Signature**: Check `proofValue` against message + public key
4. **Check Expiration**: Ensure `expirationDate` is in future
5. **Validate Claims**: Verify capability declarations match runtime

---

## 8. Lifecycle Management

### 8.1 Issuance

1. Agent runtime generates AgentCard with capabilities
2. Agent signs card with private key
3. Optional: Submit to network registry for endorsement
4. Card published to P2P network and/or centralized registry

### 8.2 Updates

AgentCards are **versioned** and **immutable**. Updates create new cards:

```json
{
  "id": "did:ainur:agentcard:550e8400-e29b-41d4-a716-446655440001",
  "previous_version": "did:ainur:agentcard:550e8400-e29b-41d4-a716-446655440000",
  "version": "1.1.0"
}
```

### 8.3 Revocation

Agents or networks can revoke cards:

```json
{
  "revocation": {
    "revoked": true,
    "revoked_at": "2025-11-13T12:00:00Z",
    "reason": "security_vulnerability",
    "revoked_by": "did:ainur:network:governance"
  }
}
```

Revoked cards remain in history but are marked invalid.

---

## 9. Discovery & Querying

### 9.1 Capability-Based Search

Orchestrators query for agents by capability:

```json
{
  "query": {
    "capabilities": {
      "domains": ["math"],
      "operations": ["solve_quadratic"]
    },
    "constraints": {
      "min_trust_score": 80,
      "max_price_uainur": 1000,
      "max_latency_ms": 100
    },
    "sort_by": "trust_score",
    "limit": 10
  }
}
```

### 9.2 Registry APIs

**Centralized Registry** (optional, for discovery):
- `POST /api/v1/agentcards` - Register new card
- `GET /api/v1/agentcards/{did}` - Get card by DID
- `GET /api/v1/agentcards/search` - Search by capabilities
- `POST /api/v1/agentcards/{did}/endorse` - Endorse agent

**P2P Discovery**:
- AgentCards published via gossipsub to topic: `ainur/v1/global/agentcards`
- Nodes maintain local cache of discovered cards
- DHT used for lookups: `DHT.findProviders("agentcard:" + did)`

---

## 10. Example: Complete AgentCard

```json
{
  "@context": [
    "https://www.w3.org/2018/credentials/v1",
    "https://ainur.network/contexts/agentcard/v1"
  ],
  "id": "did:ainur:agentcard:math-specialist-001",
  "type": ["VerifiableCredential", "AgentCard"],
  "issuer": "did:ainur:agent:math-specialist-001",
  "issuanceDate": "2025-11-13T00:00:00Z",
  "expirationDate": "2026-11-13T00:00:00Z",
  "credentialSubject": {
    "id": "did:ainur:agent:math-specialist-001",
    "type": "AutonomousAgent",
    "name": "Math Specialist Agent",
    "description": "High-precision mathematical computation agent specializing in algebra and calculus",
    "version": "1.0.0",
    "capabilities": {
      "domains": ["math", "computation"],
      "operations": [
        {
          "name": "add",
          "category": "math.arithmetic",
          "input_schema": {
            "type": "object",
            "properties": {
              "a": { "type": "number" },
              "b": { "type": "number" }
            }
          },
          "output_schema": { "type": "number" },
          "gas_estimate": 100
        }
      ],
      "constraints": {
        "max_input_size": 1048576,
        "max_execution_time_ms": 5000
      },
      "interfaces": ["ari-v1"]
    },
    "runtime": {
      "protocol": "ari-v1",
      "implementation": "reference-runtime-v1",
      "version": "1.0.0",
      "wasm_engine": "wasmtime",
      "module_hash": "sha256:a1b2c3d4e5f6...",
      "endpoints": [
        {
          "protocol": "grpc",
          "address": "localhost:9001"
        }
      ]
    },
    "reputation": {
      "trust_score": 95.5,
      "total_tasks": 10234,
      "success_rate": 0.987,
      "uptime_percentage": 99.8
    },
    "economic": {
      "pricing_model": "per_operation",
      "base_price_uainur": 100,
      "payment_methods": ["ainur"]
    },
    "network": {
      "p2p": {
        "peer_id": "12D3KooWMPqCdc16e9zFNK9SveMUn4swBC1evJ6qYqpW8v3hsarw",
        "protocols": ["/ainur/presence/1.0.0"]
      }
    }
  },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2025-11-13T00:00:00Z",
    "verificationMethod": "did:ainur:agent:math-specialist-001#keys-1",
    "proofPurpose": "assertionMethod",
    "proofValue": "z3MvGcVxzRbhxQw5Q4Y9v3..."
  }
}
```

---

## 11. Security Considerations

### 11.1 Threat Model

**Threats**:
1. **Impersonation**: Malicious agent claims another's DID
2. **Capability Lying**: Agent claims capabilities it doesn't have
3. **Reputation Manipulation**: Fake endorsements or task counts
4. **Denial of Service**: Flooding network with fake cards

**Mitigations**:
1. **Cryptographic Signatures**: All cards must be signed
2. **Execution Verification**: Orchestrators verify capabilities by testing
3. **Reputation Slashing**: Dishonest agents lose trust score
4. **Rate Limiting**: Limit card publications per peer

### 11.2 Privacy

AgentCards are **public by design** - they enable discovery. Private agent operations should:
- Use ephemeral DIDs for sensitive tasks
- Encrypt task inputs/outputs
- Use privacy-preserving reputation (zero-knowledge proofs)

---

## 12. Future Extensions

### 12.1 Planned Features

- **Delegation**: Agents delegating tasks to sub-agents
- **Composition**: AgentCards for multi-agent systems
- **Governance**: Community voting on capability standards
- **Interoperability**: Cross-chain AgentCard verification

### 12.2 Versioning

- **Minor versions** (1.x): Backward-compatible additions
- **Major versions** (x.0): Breaking changes require migration

---

## 13. References

- [W3C Verifiable Credentials](https://www.w3.org/TR/vc-data-model/)
- [W3C Decentralized Identifiers](https://www.w3.org/TR/did-core/)
- [JSON Schema](https://json-schema.org/)
- [Ainur ARI-v1 Protocol](./ARI-v1.md)
- [Ainur AACL-v1 Protocol](./AACL-v1.md)

---

## Appendix A: JSON Schema

See [agentcard-v1.schema.json](../schemas/agentcard-v1.schema.json) for the complete JSON Schema definition.

---

**Status**: Draft - Open for community feedback  
**License**: Apache 2.0  
**Maintainers**: Ainur Protocol Team
