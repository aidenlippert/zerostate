# L3 Aether Topics v1.0

**Status**: Draft  
**Version**: 1.0.0  
**Date**: 2025-11-13  

## Abstract

This specification defines the standardized topic structure for all pub/sub communication in the Ainur protocol. This serves as the "DNS" layer for agent-to-agent and protocol-to-agent communication over the peer-to-peer network.

## Motivation

A decentralized agent mesh requires a consistent, hierarchical topic structure that enables:
- **Discovery**: Agents can find relevant conversations
- **Routing**: Messages reach intended recipients efficiently
- **Sharding**: Network scales horizontally by geographic/logical boundaries
- **Filtering**: Subscribers only receive relevant messages

## Specification

### Topic Structure

All Aether topics follow this canonical format:

```
ainur/v{version}/{shard_id}/{layer}/{message_type}/{topic}
```

**Components**:

1. **Protocol Prefix**: `ainur` - Identifies Ainur protocol messages
2. **Version**: `v1`, `v2`, etc. - Protocol version for backward compatibility
3. **Shard ID**: Geographic or logical shard identifier
   - `global` - Network-wide announcements
   - `shard_{region}` - Regional shards (e.g., `shard_us-west`, `shard_eu-central`)
   - `shard_{custom}` - Custom logical shards
4. **Layer**: Protocol layer identifier
   - `l1_consensus` - Blockchain/consensus layer messages
   - `l2_verity` - Identity and verification messages
   - `l3_aether` - P2P network messages
   - `l4_concordat` - Agent communication messages
   - `l5_cognition` - Runtime and execution messages
   - `l6_economy` - Economic/payment messages
5. **Message Type**: Category of message within layer
6. **Topic**: Specific subject or agent identifier

### Examples

#### L4 Concordat (Agent Communication)

```
# Call for Proposals (CFP) - Global image OCR auction
ainur/v1/global/l4_concordat/cfp/image-ocr

# Proposal (Bid) response
ainur/v1/global/l4_concordat/proposal/image-ocr/response-123

# Task result
ainur/v1/global/l4_concordat/inform/task-456
```

#### L3 Aether (Network State)

```
# Agent presence announcement (heartbeat)
ainur/v1/shard_us-west/l3_aether/presence/did:ainur:agent:abc123

# Agent capability update
ainur/v1/global/l3_aether/capability/did:ainur:agent:abc123

# Network topology change
ainur/v1/shard_eu-central/l3_aether/topology/update
```

#### L5 Cognition (Runtime)

```
# Runtime health status
ainur/v1/shard_us-west/l5_cognition/health/runtime-node-001

# WASM module availability
ainur/v1/global/l5_cognition/module/math-agent-v1.0
```

#### L2 Verity (Identity)

```
# DID resolution request
ainur/v1/global/l2_verity/did/resolve/did:ainur:agent:abc123

# Verifiable Credential update
ainur/v1/global/l2_verity/vc/update/did:ainur:agent:abc123
```

#### L6 Economy (Payments)

```
# Payment escrow creation
ainur/v1/global/l6_economy/escrow/create/task-789

# Payment release
ainur/v1/global/l6_economy/escrow/release/task-789
```

### Topic Subscription Patterns

Implementations MUST support wildcard subscriptions:

```
# Subscribe to all CFPs globally
ainur/v1/global/l4_concordat/cfp/*

# Subscribe to all messages in US West shard
ainur/v1/shard_us-west/*/*/*

# Subscribe to all agent presence in a shard
ainur/v1/shard_*/l3_aether/presence/*
```

### Message Encoding

All messages published to Aether topics MUST be:
- **Format**: JSON or Protocol Buffers
- **Encoding**: UTF-8
- **Compression**: Optional gzip for messages >1KB
- **Signing**: Required (see L2 Verity spec)

Example message envelope:

```json
{
  "topic": "ainur/v1/global/l4_concordat/cfp/image-ocr",
  "timestamp": "2025-11-13T10:00:00Z",
  "ttl": 3600,
  "sender_did": "did:ainur:orchestrator:xyz",
  "signature": "base64_signature",
  "payload": {
    // Layer-specific payload (see L4 AACL spec)
  }
}
```

## Implementation Requirements

### MUST Support

1. **Topic Publishing**: Send messages to any valid topic
2. **Topic Subscription**: Subscribe to topics with wildcards
3. **Message Filtering**: Filter by sender_did, timestamp, or custom criteria
4. **TTL Enforcement**: Discard messages older than TTL

### SHOULD Support

1. **Topic Discovery**: Query available topics in a shard
2. **Backpressure**: Rate limiting on high-volume topics
3. **Persistence**: Optional message replay for new subscribers

### MAY Support

1. **Topic Aliases**: Short names for common topics
2. **Priority Routing**: Express lanes for time-sensitive messages

## Security Considerations

1. **Topic Spoofing**: Implementations MUST verify sender_did signature
2. **Topic Flooding**: Implementations SHOULD rate-limit publishers
3. **Topic Enumeration**: Sensitive topics MAY use encryption or private channels
4. **Shard Isolation**: Cross-shard messages SHOULD be validated at boundaries

## Interoperability

This specification is designed to work with:
- **libp2p PubSub**: GossipSub or FloodSub
- **MQTT**: For lightweight clients
- **Redis PubSub**: For local development
- **Kafka**: For high-throughput production systems

## Versioning

Future versions MAY introduce:
- `ainur/v2/...` - Breaking changes to topic structure
- New layers (e.g., `l7_governance`)
- New shard types (e.g., `shard_capability_ocr`)

## References

- [libp2p PubSub Specification](https://github.com/libp2p/specs/tree/master/pubsub)
- [MQTT Topic Best Practices](https://www.hivemq.com/blog/mqtt-essentials-part-5-mqtt-topics-best-practices/)
- [FIPA Agent Communication Language](http://www.fipa.org/specs/fipa00061/)

## Changelog

- **v1.0.0** (2025-11-13): Initial specification

---

**License**: Apache 2.0  
**Maintainer**: Ainur Protocol Working Group
