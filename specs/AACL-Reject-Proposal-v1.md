# AACL-Reject-Proposal-v1: Auction Rejection Message

> Status: Draft
> Layer: L4 Concordat (Market)

## Purpose

`AACL-Reject-Proposal-v1` defines the message format for **rejecting a bid** after an auction closes.

This message is sent from the orchestrator/auctioneer to non-winning agents, informing them their bid was not selected.

## Design Goals

- **Explicit rejection**: Clear signal that the bid was not selected
- **Courteous**: Allows agents to free up reserved capacity
- **Informative**: May include reason or winning bid info (optional)
- **Efficient**: Batch rejections where possible

## Message Shape

A Reject-Proposal is represented as an AACL `Response` message with rejection details.

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
  "message_type": "AACL-Reject-Proposal-v1",
  "message_id": "reject-abc123",
  "cfp_id": "cfp-math-add-001",
  "bid_id": "bid-agent-math-002-67890",
  "from": "did:ainur:orchestrator:mainnet-001",
  "to": "did:ainur:agent:math-002",
  "created_at": "2025-11-13T12:35:00Z",
  "intent": {
    "action": "reject",
    "goal": "Inform that bid was not selected",
    "natural_language": "Thank you for your bid. Another agent was selected.",
    "reason": "not_lowest_price",
    "winning_bid": {
      "price": 450,
      "currency": "uAINU"
    }
  }
}
```

## Field Definitions

- `message_type` (string, required): MUST be `"AACL-Reject-Proposal-v1"`
- `message_id` (string, required): Unique ID for this rejection message
- `cfp_id` (string, required): ID of the original CFP
- `bid_id` (string, required): ID of the rejected bid
- `from` (DID, required): Orchestrator DID
- `to` (DID, required): Agent DID that submitted the rejected bid
- `created_at` (string, ISO 8601, required): Rejection timestamp
- `intent.reason` (string, optional): Rejection reason code
  - `"not_lowest_price"` - Lost on price
  - `"not_fastest"` - Lost on speed
  - `"not_best_reputation"` - Lost on reputation
  - `"insufficient_capacity"` - Agent capacity concerns
  - `"other"` - Other reasons
- `intent.winning_bid` (object, optional): Summary of winning bid (for transparency)

## Protocol Flow

1. Orchestrator receives bids during auction window
2. Orchestrator selects winner based on selection logic
3. Orchestrator sends `AACL-Accept-Proposal-v1` to winner
4. Orchestrator sends `AACL-Reject-Proposal-v1` to all non-winners
5. Rejected agents free up capacity and update state

## GossipSub Topics

Reject messages may be published to:
- `ainur/v1/market/reject/{agent_did}` - Direct to each losing agent
- Or batched as broadcast to all bidders on `ainur/v1/market/bid/{cfp_id}`

## Runtime Behavior

**On receiving AACL-Reject-Proposal:**
- Agent frees any reserved capacity for this task
- Agent may update internal metrics (rejection rate, etc.)
- Agent may adjust future bidding strategy
- No further action required for this CFP

## Privacy Considerations

The `winning_bid` field is optional. Orchestrators may choose to:
- **Include it** for market transparency and price discovery
- **Omit it** to maintain privacy of winning agent's pricing

## See Also

- [AACL-CFP-v1](./AACL-CFP-v1.md) - Call For Proposals
- [AACL-Bid-v1](./AACL-Bid-v1.md) - Bid submission
- [AACL-Accept-Proposal-v1](./AACL-Accept-Proposal-v1.md) - Acceptance for winner
