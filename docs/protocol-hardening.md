# ZeroState Protocol Hardening Roadmap

## Current State Analysis

### ✅ Solid Foundation (Completed)
- libp2p transport with QUIC
- Kademlia DHT (k=20, α=3)
- Ed25519 signatures & DID
- Circuit Relay v2
- Q-routing (reinforcement learning)
- HNSW semantic search
- mDNS local discovery

### 🔴 Critical Protocol Gaps

## Priority 1: Protocol Versioning & Compatibility

**Problem**: No protocol version negotiation
- Breaking changes will fragment network
- No backward compatibility mechanism
- Nodes can't detect version mismatches

**Solution**: Protocol Version Negotiation
```go
const (
    ProtocolVersion = "1.0.0"
    MinCompatibleVersion = "1.0.0"
)

type ProtocolHandshake struct {
    Version    string
    Features   []string  // ["dht", "relay", "auth"]
    Extensions map[string]interface{}
}
```

**Deliverables**:
- [ ] Version handshake on connection
- [ ] Feature detection/advertisement
- [ ] Graceful degradation for older peers
- [ ] Metrics: protocol_version_mismatches_total

---

## Priority 2: Message Ordering & Reliability

**Problem**: No ordering guarantees for DHT writes
- Card updates can arrive out-of-order
- No replay protection beyond auth timestamp
- Concurrent updates cause conflicts

**Solution**: Vector Clocks + Causal Ordering
```go
type CardUpdate struct {
    Card       SignedAgentCard
    VectorClock map[string]uint64  // peerID -> sequence
    PrevHash    string             // Hash of previous version
}
```

**Deliverables**:
- [ ] Vector clock implementation
- [ ] Conflict resolution strategy (LWW vs merge)
- [ ] Update chain validation
- [ ] Metrics: card_conflicts_total, card_ordering_violations

---

## Priority 3: Backpressure & Flow Control

**Problem**: No flow control on content exchange
- Receiver can be overwhelmed
- No rate limiting on DHT lookups
- Memory exhaustion possible

**Solution**: Token Bucket + Window-based Flow Control
```go
type FlowController struct {
    TokenBucket   *rate.Limiter
    WindowSize    int
    InFlight      int
    SendWindow    chan struct{}
}
```

**Deliverables**:
- [ ] Per-peer rate limiting
- [ ] Sliding window for content transfer
- [ ] Backpressure signals in protocol
- [ ] Metrics: flow_control_throttles_total, window_size_gauge

---

## Priority 4: Gossip Pub-Sub for Card Updates

**Problem**: DHT polling is inefficient
- Have to query DHT to detect updates
- No push notifications
- Stale card data

**Solution**: libp2p GossipSub for Card Announcements
```go
const CardUpdateTopic = "/zerostate/cards/1.0.0"

type CardAnnouncement struct {
    DID       string
    CID       string
    Timestamp int64
    Signature string
}
```

**Deliverables**:
- [ ] GossipSub mesh for card announcements
- [ ] Topic-based filtering (region, capability)
- [ ] Announcement validation
- [ ] Metrics: gossip_messages_total, mesh_peers_gauge

---

## Priority 5: DHT Provider Record TTL & Refresh

**Problem**: Provider records never expire
- Stale/offline nodes stay in DHT
- No automatic refresh mechanism

**Solution**: TTL-based Provider Records
```go
const (
    ProviderRecordTTL = 1 * time.Hour
    RefreshInterval   = 30 * time.Minute
)

// Auto-refresh loop
func (n *Node) autoRefreshProviders(ctx context.Context) {
    ticker := time.NewTicker(RefreshInterval)
    for {
        select {
        case <-ticker.C:
            n.republishCards(ctx)
        case <-ctx.Done():
            return
        }
    }
}
```

**Deliverables**:
- [ ] Configurable TTL for provider records
- [ ] Auto-refresh background task
- [ ] Stale record cleanup
- [ ] Metrics: provider_refreshes_total, stale_records_cleaned

---

## Priority 6: Content Verification Chain

**Problem**: No integrity verification for resolved content
- DHT returns CID but no proof it matches
- Middleman attacks possible
- No chain of trust

**Solution**: Content Hash Verification + Merkle Proofs
```go
func (n *Node) ResolveAndVerify(ctx context.Context, cid string) ([]byte, error) {
    content, err := n.ResolveAgentCard(ctx, cid)
    if err != nil {
        return nil, err
    }
    
    // Verify content hash matches CID
    computedCID := ComputeCID(content)
    if computedCID != cid {
        return nil, errors.New("content hash mismatch")
    }
    
    // Verify signature
    if err := n.validator.VerifySignedCard(ctx, content); err != nil {
        return nil, err
    }
    
    return content, nil
}
```

**Deliverables**:
- [ ] Automatic hash verification
- [ ] Signature chain validation
- [ ] Merkle proof support (future)
- [ ] Metrics: verification_failures_total{reason}

---

## Priority 7: Connection Pooling & Reuse

**Problem**: New connection per DHT operation
- High latency overhead
- Connection explosion under load
- Port exhaustion

**Solution**: Connection Pool Manager
```go
type ConnectionPool struct {
    conns     map[peer.ID]*PooledConn
    maxIdle   time.Duration
    maxConns  int
}

type PooledConn struct {
    streams    chan network.Stream
    lastUsed   time.Time
    inUse      int32
}
```

**Deliverables**:
- [ ] Per-peer connection pooling
- [ ] Stream multiplexing
- [ ] Idle connection cleanup
- [ ] Metrics: pooled_connections_gauge, stream_reuse_total

---

## Priority 8: Failure Detection & Health Checks

**Problem**: No peer health monitoring
- Dead peers stay in routing table
- No failure detection
- Cascading failures

**Solution**: Heartbeat + Failure Detector
```go
type FailureDetector struct {
    heartbeatInterval time.Duration
    failureThreshold  int
    suspicionTimeout  time.Duration
}

func (fd *FailureDetector) MonitorPeer(peerID peer.ID) {
    missed := 0
    ticker := time.NewTicker(fd.heartbeatInterval)
    
    for range ticker.C {
        if !fd.ping(peerID) {
            missed++
            if missed >= fd.failureThreshold {
                fd.MarkFailed(peerID)
            }
        } else {
            missed = 0
        }
    }
}
```

**Deliverables**:
- [ ] Periodic peer health checks
- [ ] Adaptive timeout (RTT-based)
- [ ] Dead peer removal
- [ ] Metrics: peer_failures_total, heartbeat_misses_total

---

## Priority 9: Request Deduplication

**Problem**: Duplicate DHT lookups waste resources
- Same CID requested multiple times
- No request coalescing
- Cache misses

**Solution**: In-Flight Request Deduplication
```go
type RequestDeduplicator struct {
    inflight map[string]*inflightRequest
    mu       sync.Mutex
}

type inflightRequest struct {
    result chan RequestResult
    wait   sync.WaitGroup
}

func (rd *RequestDeduplicator) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
    rd.mu.Lock()
    if req, ok := rd.inflight[key]; ok {
        rd.mu.Unlock()
        // Wait for in-flight request
        result := <-req.result
        return result.Data, result.Err
    }
    
    // First request - execute
    req := &inflightRequest{result: make(chan RequestResult, 1)}
    rd.inflight[key] = req
    rd.mu.Unlock()
    
    data, err := fn()
    req.result <- RequestResult{Data: data, Err: err}
    close(req.result)
    
    rd.mu.Lock()
    delete(rd.inflight, key)
    rd.mu.Unlock()
    
    return data, err
}
```

**Deliverables**:
- [ ] Request deduplication layer
- [ ] TTL-based cache
- [ ] Cache invalidation on updates
- [ ] Metrics: deduplicated_requests_total, cache_hit_rate

---

## Priority 10: Bandwidth Accounting & QoS

**Problem**: No bandwidth management
- Relay abuse possible
- No QoS for important messages
- Fair-share violations

**Solution**: Token Bucket QoS + Per-Peer Accounting
```go
type BandwidthManager struct {
    buckets map[peer.ID]*TokenBucket
    classes map[MessageType]Priority
}

const (
    PriorityHigh   Priority = 3  // Heartbeats, health
    PriorityNormal Priority = 2  // Card updates
    PriorityLow    Priority = 1  // Bulk content
)

func (bm *BandwidthManager) Allow(peerID peer.ID, msgType MessageType, bytes int64) bool {
    priority := bm.classes[msgType]
    bucket := bm.buckets[peerID]
    
    return bucket.TakeWithPriority(bytes, priority)
}
```

**Deliverables**:
- [ ] Per-peer bandwidth tracking
- [ ] Priority queues
- [ ] Rate limiting
- [ ] Metrics: bandwidth_bytes_total{peer,direction}, qos_drops_total{priority}

---

## Implementation Phases

### Phase 1: Stability (Weeks 1-2)
- Protocol versioning
- Message ordering (vector clocks)
- Content verification chain

### Phase 2: Performance (Weeks 3-4)
- Connection pooling
- Request deduplication
- Flow control

### Phase 3: Reliability (Weeks 5-6)
- Failure detection
- DHT provider refresh
- Backpressure

### Phase 4: Scalability (Weeks 7-8)
- GossipSub integration
- Bandwidth accounting
- QoS implementation

---

## Testing Strategy

### Protocol Conformance Tests
```go
func TestProtocolVersionNegotiation(t *testing.T) {
    // Test version compatibility matrix
    // Test feature detection
    // Test graceful degradation
}

func TestMessageOrdering(t *testing.T) {
    // Concurrent updates with vector clocks
    // Out-of-order delivery scenarios
    // Conflict resolution
}

func TestFlowControl(t *testing.T) {
    // Slow receiver handling
    // Backpressure propagation
    // Window size adjustment
}
```

### Chaos Engineering
```bash
# Network partitions
tc qdisc add dev eth0 root netem loss 30% delay 100ms

# Bandwidth limits
tc qdisc add dev eth0 root tbf rate 1mbit burst 32kbit latency 400ms

# Peer churn
for i in {1..10}; do
    docker kill zerostate-edge-$i
    sleep 5
    docker start zerostate-edge-$i
done
```

### Load Testing
```go
func BenchmarkDHTLookup(b *testing.B) {
    // Concurrent DHT lookups
    // Measure P50/P95/P99
}

func BenchmarkContentExchange(b *testing.B) {
    // Various payload sizes
    // Different network conditions
}
```

---

## Success Metrics

**Reliability**
- Message loss rate < 0.01%
- Out-of-order delivery < 1%
- Protocol version mismatches handled gracefully

**Performance**
- DHT lookup P95 < 100ms (unchanged)
- Connection reuse > 80%
- Request dedup hit rate > 50%

**Scalability**
- Support 10k concurrent peers
- Bandwidth fair-share violation < 5%
- GossipSub mesh stable at 1k nodes

---

## Non-Goals (For Now)

- Byzantine fault tolerance (future)
- Formal verification (future)
- State channels implementation (separate epic)
- WASM execution (separate epic)
- Multi-region sharding (after single-region stable)

---

## References

- libp2p specs: https://github.com/libp2p/specs
- GossipSub spec: https://github.com/libp2p/specs/tree/master/pubsub/gossipsub
- Kademlia DHT: https://pdos.csail.mit.edu/~petar/papers/maymounkov-kademlia-lncs.pdf
- Vector clocks: Lamport timestamps vs Version vectors
