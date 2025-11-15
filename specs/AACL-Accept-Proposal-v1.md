# AACL-Accept-Proposal-v1: Auction Acceptance Message

> Status: Draft
> Layer: L4 Concordat (Market)

## Purpose

`AACL-Accept-Proposal-v1` defines the message format for **accepting a bid** after an auction closes.

This message is sent from the orchestrator/auctioneer to the winning agent, formally awarding the task contract.

## Design Goals

- **Explicit contract award**: Clear signal that the agent won and should execute
- **Traceable**: Links back to CFP and winning bid
- **Actionable**: Contains all info needed for agent to start execution

## Message Shape

An Accept-Proposal is represented as an AACL `Response` message with acceptance details.

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
  "@type": "Response",
  "message_type": "AACL-Accept-Proposal-v1",
  "message_id": "accept-abc123",
  "cfp_id": "cfp-math-add-001",
  "bid_id": "bid-agent-math-001-12345",
  "from": "did:ainur:orchestrator:mainnet-001",
  "to": "did:ainur:agent:math-001",
  "created_at": "2025-11-13T12:35:00Z",
  "intent": {
    "action": "accept",
    "goal": "Award task execution contract",
    "natural_language": "Your bid has been accepted. Please execute the task.",
    "contract": {
      "task_id": "task-12345",
      "agreed_price": 500,
      "currency": "uAINU",
      "deadline": "2025-11-13T12:36:00Z"
    },
    "task_spec": {
      "type": "math.add",
      "input": {"a": 5, "b": 7}
    }
  },
  "proof": {
    "type": "Ed25519Signature2020",
    "created": "2025-11-13T12:35:00Z",
    "verificationMethod": "did:ainur:orchestrator:mainnet-001#key-1",
    "signatureValue": "..."
  }
}
```

## Field Definitions

- `message_type` (string, required): MUST be `"AACL-Accept-Proposal-v1"`
- `message_id` (string, required): Unique ID for this acceptance message
- `cfp_id` (string, required): ID of the original CFP
- `bid_id` (string, required): ID of the winning bid being accepted
- `from` (DID, required): Orchestrator DID
- `to` (DID, required): Winning agent DID
- `created_at` (string, ISO 8601, required): Acceptance timestamp
- `intent.contract` (object, required): The agreed contract terms
- `proof` (object, required): Signature from orchestrator

## Protocol Flow

1. Orchestrator receives bids during auction window
2. Orchestrator selects winner based on selection logic
3. Orchestrator sends `AACL-Accept-Proposal-v1` to winner
4. Agent receives acceptance and begins task execution
5. Agent updates internal state to mark task as "contracted"

## GossipSub Topics

Accept messages are published to:
- `ainur/v1/market/accept/{agent_did}` - Direct to winning agent
- Or as a direct reply via agent-to-agent messaging

## Runtime Behavior

**On receiving AACL-Accept-Proposal:**
- Agent verifies signature from orchestrator
- Agent reserves capacity for the task
- Agent prepares execution environment
- Agent may send back an acknowledgment (future extension)

## See Also

- [AACL-CFP-v1](./AACL-CFP-v1.md) - Call For Proposals
- [AACL-Bid-v1](./AACL-Bid-v1.md) - Bid submission
- [AACL-Reject-Proposal-v1](./AACL-Reject-Proposal-v1.md) - Rejection for non-winners
