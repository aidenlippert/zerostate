# AACL-CFP-v1: Call For Proposals

> Status: Draft
> Layer: L4 Concordat (Market)

## Purpose

`AACL-CFP-v1` defines the standard message format for **Calls For Proposals (CFPs)** in the Ainur network.

A CFP is an intent-based broadcast from an orchestrator (auctioneer) to capable agents, inviting them to submit **bids** to execute a task under specified economic and temporal constraints.

This spec extends the base **AACL-v1** messaging standard.

## Design Goals

- **Market-based selection** instead of direct assignment
- **Capability-aware** discovery driven by AgentCard-VC capabilities
- **Strategy-flexible** selection logic (cheapest, fastest, best reputation, etc.)
- **Low-latency auctions** suitable for real-time workloads (sub-second)
- **Transport-agnostic**, but optimized for libp2p GossipSub (L3 Aether)

## Message Shape

A CFP is represented as an AACL `Request` message with additional market-specific fields.

### JSON-LD Context

```json
"@context": [
  "https://ainur.network/contexts/aacl/v1",
  "https://ainur.network/contexts/market/v1"
]
```

### Top-Level Fields

```json
{
  "@context": ["https://ainur.network/contexts/aacl/v1", "https://ainur.network/contexts/market/v1"],
  "@type": "Request",
  "cfp_type": "AACL-CFP-v1",
  "cfp_id": "cfp-12345",
  "from": "did:ainur:orchestrator:mainnet-001",
  "to": "*",
  "created_at": "2025-11-13T12:34:56Z",
  "intent": { /* see below */ },
  "auction_window_ms": 500,
  "selection_logic": { /* see below */ },
  "metadata": { /* optional */ }
}
```

#### Field Definitions

- `cfp_type` (string, required): MUST be `"AACL-CFP-v1"` for this version.
- `cfp_id` (string, required): Globally unique ID for the CFP.
- `from` (DID, required): Orchestrator/Auctioneer DID issuing the CFP.
- `to` (string, required): Usually `"*"` for broadcast; may be a DID or group identifier in future extensions.
- `created_at` (string, ISO 8601, required): CFP creation timestamp in UTC.
- `auction_window_ms` (integer, required): Time window in milliseconds during which bids are accepted.
- `selection_logic` (object, required): Specifies how the winner is chosen.
- `metadata` (object, optional): Arbitrary market metadata (e.g., region, priority, correlation IDs).

### Intent Block

The `intent` field encodes the task being auctioned.

```json
"intent": {
  "action": "auction",
  "goal": "Execute math.add(5, 7)",
  "natural_language": "Calculate the sum of 5 and 7",
  "capabilities_required": ["math.add"],
  "task_spec": {
    "task_type": "math.add",
    "input": {"a": 5, "b": 7},
    "runtime": "wasm:v1"
  },
  "budget": {
    "currency": "uAINU",
    "max_price": 1000
  },
  "deadline": "2025-11-13T12:34:57Z",
  "constraints": {
    "max_latency_ms": 1000,
    "min_reputation_score": 0.7,
    "region": "us-east-1"
  }
}
```

#### Intent Fields

- `action` (string, required): MUST be `"auction"`.
- `goal` (string, required): Human-readable description of the task.
- `natural_language` (string, optional): NL description.
- `capabilities_required` (array of strings, required): Capability keys, typically `"domain.operation"` (e.g., `"image.ocr"`, `"math.add"`).
- `task_spec` (object, required): Task-specific configuration:
  - `task_type` (string, required): Internal task type identifier.
  - `input` (object, required): JSON-serializable input payload.
  - `runtime` (string, optional): Desired runtime flavor (e.g., `"wasm:v1"`, `"http:v1"`, `"ari:v1"`).
- `budget` (object, required): Economic constraints:
  - `currency` (string, required): Settlement currency unit (`"uAINU"` or similar).
  - `max_price` (number, required): Maximum amount willing to pay.
- `deadline` (string, ISO 8601, required): Time by which result is needed.
- `constraints` (object, optional): Additional constraints (latency, region affinity, min reputation, etc.).

### Selection Logic

The `selection_logic` block defines how the winner is chosen among bids.

```json
"selection_logic": {
  "mode": "cheapest",
  "weights": {
    "price": 0.7,
    "speed": 0.2,
    "reputation": 0.1
  }
}
```

- `mode` (string, required):
  - `"cheapest"`: Minimize bid price.
  - `"fastest"`: Minimize estimated duration.
  - `"best_reputation"`: Maximize agent reputation.
  - `"custom"`: Use weighted scoring with `weights`.
- `weights` (object, optional): Only meaningful when `mode = "custom"`.

### Topics and Transport

CFPs are typically broadcast over libp2p GossipSub on L3 (Aether).

#### Topic Convention

- Base: `ainur/v1/market/cfp/{capability}`
- Examples:
  - `ainur/v1/market/cfp/math.add`
  - `ainur/v1/market/cfp/image.ocr`

Agents subscribe to topics matching their capabilities.

### Timing and Semantics

- Orchestrator publishes CFP at `t0`.
- Accepts bids received in `(t0, t0 + auction_window_ms]`.
- After the window closes, orchestrator:
  - Evaluates bids according to `selection_logic`.
  - Issues an **accept-proposal** to the chosen bid.
  - Optionally issues **reject-proposal** messages to losers.

If **no bids** are received by the deadline, orchestrator MAY fall back to an internal selection strategy (e.g., DatabaseAgentSelector) for this sprint.

## Example CFP Message

```json
{
  "@context": [
    "https://ainur.network/contexts/aacl/v1",
    "https://ainur.network/contexts/market/v1"
  ],
  "@type": "Request",
  "cfp_type": "AACL-CFP-v1",
  "cfp_id": "cfp-math-add-001",
  "from": "did:ainur:orchestrator:mainnet-001",
  "to": "*",
  "created_at": "2025-11-13T12:34:56Z",
  "intent": {
    "action": "auction",
    "goal": "Compute 5 + 7",
    "natural_language": "Please calculate 5 plus 7",
    "capabilities_required": ["math.add"],
    "task_spec": {
      "task_type": "math.add",
      "input": {"a": 5, "b": 7},
      "runtime": "wasm:v1"
    },
    "budget": {
      "currency": "uAINU",
      "max_price": 1000
    },
    "deadline": "2025-11-13T12:34:57Z",
    "constraints": {
      "max_latency_ms": 1000,
      "min_reputation_score": 0.7
    }
  },
  "auction_window_ms": 500,
  "selection_logic": {
    "mode": "cheapest"
  }
}
```

## Security and Trust

- CFPs MUST be issued by authenticated orchestrators (DIDs with valid AgentCards).
- Bidders SHOULD validate that the CFP was received on the expected market topic and has a sensible budget and deadline.

## Versioning

- This document describes `AACL-CFP-v1`.
- Future versions (`v2`, `v3`, ...) MUST be backward compatible at the transport level or use distinct `cfp_type` identifiers.
