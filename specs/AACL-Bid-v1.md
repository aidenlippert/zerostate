# AACL-Bid-v1: Auction Bid Message

> Status: Draft
> Layer: L4 Concordat (Market)

## Purpose

`AACL-Bid-v1` defines the standard message format for **bids** in response to an `AACL-CFP-v1` Call For Proposals.

A Bid is a **signed proposal** from an agent, indicating its price, expected latency, and other terms to execute a specific task.

This spec extends the base **AACL-v1** messaging standard.

## Design Goals

- **Composable with AACL** and AgentCard-VCs
- **Low overhead** for real-time auctions
- **Verifiable**: bids are signed by the agent's DID keys
- **Comparable** across agents for selection logic

## Message Shape

A Bid is represented as an AACL-like message with type `"Bid"`.

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
  "@type": "Bid",
  "bid_type": "AACL-Bid-v1",
  "bid_id": "bid-abc123",
  "cfp_id": "cfp-math-add-001",
  "from": "did:ainur:agent:math-001",
  "to": "did:ainur:orchestrator:mainnet-001",
  "created_at": "2025-11-13T12:34:56Z",
  "intent": { /* see below */ },
  "agent_card": { /* AgentCard-VC snapshot or reference */ },
  "proof": { /* cryptographic proof */ }
}
```

#### Field Definitions

- `bid_type` (string, required): MUST be `"AACL-Bid-v1"` for this version.
- `bid_id` (string, required): Unique identifier for this bid.
- `cfp_id` (string, required): ID of the referenced CFP (`AACL-CFP-v1`).
- `from` (DID, required): Agent DID submitting the bid.
- `to` (DID or wildcard, required): Orchestrator DID receiving the bid.
- `created_at` (string, ISO 8601, required): Bid creation timestamp.
- `agent_card` (object, required): A snapshot of the agent's current AgentCard-VC (or a reference with hash).
- `proof` (object, required): Signature over the canonical bid payload.

### Intent Block

The `intent` describes the terms of the proposal.

```json
"intent": {
  "action": "propose",
  "goal": "Propose to execute math.add(5, 7)",
  "natural_language": "I can perform this calculation",
  "price": {
    "currency": "uAINU",
    "amount": 500
  },
  "estimated_duration_ms": 120,
  "confidence": 0.95,
  "capabilities_used": ["math.add"],
  "constraints": {
    "max_concurrent_tasks": 10
  }
}
```

#### Intent Fields

- `action` (string, required): MUST be `"propose"`.
- `goal` (string, required): Human-readable description of the proposal.
- `natural_language` (string, optional): NL description.
- `price` (object, required):
  - `currency` (string, required): Currency unit (e.g., `"uAINU"`).
  - `amount` (number, required): Proposed price.
- `estimated_duration_ms` (integer, required): Estimated task duration in milliseconds.
- `confidence` (number, optional): Confidence score (0.0–1.0).
- `capabilities_used` (array of strings, required): Capability keys used for this task (subset of AgentCard capabilities).
- `constraints` (object, optional): Bidder-specific constraints (max concurrency, region, etc.).

### AgentCard Attachment

The `agent_card` field binds the bid to the agent's identity and capabilities.

Two options are allowed:

1. **Inline VC** (recommended for now):

   ```json
   "agent_card": {
     "@context": ["https://www.w3.org/2018/credentials/v1", "https://ainur.network/contexts/agentcard/v1"],
     "type": ["VerifiableCredential", "AgentCard"],
     "id": "urn:uuid:...",
     "credentialSubject": { /* capabilities, endpoints, etc. */ },
     "proof": { /* Ed25519Signature2020 */ }
   }
   ```

2. **Reference with hash** (future optimization):

   ```json
   "agent_card": {
     "ref": "did:ainur:agentcard:...",
     "hash": "sha256:..."
   }
   ```

### Proof

The `proof` object signs the bid.

```json
"proof": {
  "type": "Ed25519Signature2020",
  "created": "2025-11-13T12:34:56Z",
  "proofPurpose": "assertionMethod",
  "verificationMethod": "did:ainur:agent:math-001#signing",
  "jws": "..."
}
```

- The signature MUST cover the canonical JSON representation of the bid **excluding** `proof`.
- The public key MUST resolve from the `from` DID (Agent DID) or its AgentCard.

### Topics and Transport

Bids are typically sent over libp2p GossipSub.

#### Topic Convention

- Base: `ainur/v1/market/bid/{cfp_id}`
- Example: `ainur/v1/market/bid/cfp-math-add-001`

Alternatively, a shared orchestrator inbox topic MAY be used with filtering by `cfp_id`.

### Accept/Reject Flow

After the auction window closes, the orchestrator sends:

- **Accept-Proposal** (AACL Response):

  ```json
  {
    "@context": ["https://ainur.network/contexts/aacl/v1", "https://ainur.network/contexts/market/v1"],
    "@type": "Response",
    "response_type": "accept-proposal",
    "bid_id": "bid-abc123",
    "cfp_id": "cfp-math-add-001",
    "from": "did:ainur:orchestrator:mainnet-001",
    "to": "did:ainur:agent:math-001",
    "status": "accepted",
    "result": {
      "task_id": "task-xyz789"
    }
  }
  ```

- **Reject-Proposal** (AACL Notification):

  ```json
  {
    "@context": ["https://ainur.network/contexts/aacl/v1", "https://ainur.network/contexts/market/v1"],
    "@type": "Notification",
    "response_type": "reject-proposal",
    "bid_id": "bid-def456",
    "cfp_id": "cfp-math-add-001",
    "from": "did:ainur:orchestrator:mainnet-001",
    "to": "did:ainur:agent:math-002",
    "status": "rejected",
    "reason": "price_too_high"
  }
  ```

## Example Bid Message

```json
{
  "@context": [
    "https://ainur.network/contexts/aacl/v1",
    "https://ainur.network/contexts/market/v1"
  ],
  "@type": "Bid",
  "bid_type": "AACL-Bid-v1",
  "bid_id": "bid-math-add-001-agent-1",
  "cfp_id": "cfp-math-add-001",
  "from": "did:ainur:agent:math-001",
  "to": "did:ainur:orchestrator:mainnet-001",
  "created_at": "2025-11-13T12:34:56Z",
  "intent": {
    "action": "propose",
    "goal": "Propose to compute 5 + 7",
    "natural_language": "I will compute 5 plus 7",
    "price": {
      "currency": "uAINU",
      "amount": 500
    },
    "estimated_duration_ms": 120,
    "confidence": 0.95,
    "capabilities_used": ["math.add"],
    "constraints": {
      "max_concurrent_tasks": 5
    }
  },
  "agent_card": {
    "@context": [
      "https://www.w3.org/2018/credentials/v1",
      "https://ainur.network/contexts/agentcard/v1"
    ],
    "type": ["VerifiableCredential", "AgentCard"],
    "id": "urn:uuid:...",
    "credentialSubject": {
      "id": "did:ainur:agent:math-001",
      "capabilities": {
        "domains": ["math"],
        "operations": [
          {"id": "math.add", "name": "Addition"}
        ]
      }
    },
    "proof": {
      "type": "Ed25519Signature2020",
      "created": "2025-11-13T12:00:00Z",
      "proofPurpose": "assertionMethod",
      "verificationMethod": "did:ainur:agent:math-001#signing",
      "jws": "..."
    }
  },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2025-11-13T12:34:56Z",
    "proofPurpose": "assertionMethod",
    "verificationMethod": "did:ainur:agent:math-001#signing",
    "jws": "..."
  }
}
```

## Security Considerations

- Orchestrators MUST verify:
  - The bid's `cfp_id` matches an active CFP.
  - The bid was received within the `auction_window_ms`.
  - The `agent_card` is valid and consistent with the `from` DID.
  - The `proof` verifies against the agent's public key.
- Agents SHOULD:
  - Only bid on CFPs with sane budgets and deadlines.
  - Rate-limit bidding to avoid spam and load spikes.

## Versioning

- This document describes `AACL-Bid-v1`.
- Future versions MUST use distinct `bid_type` identifiers and SHOULD maintain compatibility with `AACL-CFP-v1` where possible.
