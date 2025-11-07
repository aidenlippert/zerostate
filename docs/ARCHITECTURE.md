# ZeroState Architecture & Design Decisions

**Last Updated:** November 7, 2025  
**Project Status:** Sprint 6 (Monitoring & Observability)  
**Maturity:** Pre-mainnet, production hardening phase

---

## Table of Contents
1. [Vision & Use Cases](#vision--use-cases)
2. [Architecture Overview](#architecture-overview)
3. [Economic Model](#economic-model)
4. [Technical Deep Dive](#technical-deep-dive)
5. [Current Focus & Timeline](#current-focus--timeline)

---

## Vision & Use Cases

### Target Workloads

**Primary:** Lightweight, ephemeral compute tasks (seconds to minutes)
- **AI Inference:** Small model inference (BERT, small CNNs, sentiment analysis)
- **Data Processing:** ETL pipelines, data validation, format conversion
- **API Aggregation:** Multi-source data fetching and enrichment
- **Content Moderation:** Image/text analysis with pre-trained models
- **Edge Computing:** IoT data processing, sensor aggregation

**Sweet Spot:**
- **Duration:** 1-30 seconds (default 30s timeout)
- **Memory:** 1-128 MB (default 50MB limit)
- **Deterministic:** WASM sandboxing ensures reproducible results
- **Stateless:** No persistent storage, ephemeral compute

**NOT Suitable For:**
- Long-running jobs (hours/days)
- GPU-intensive workloads
- Large dataset processing (>128MB)
- Stateful applications requiring persistence

### Why ZeroState Over Existing Solutions?

**vs. Traditional Serverless (AWS Lambda, Cloud Functions):**
- ✅ No vendor lock-in, fully decentralized
- ✅ Micropayments per task (no monthly fees)
- ✅ Cryptographic proofs of execution (receipts)
- ❌ Higher latency (P2P overhead vs centralized)
- ❌ Lower scale ceiling (for now)

**vs. Blockchain Compute (Ethereum, Akash, iExec):**
- ✅ Orders of magnitude cheaper (off-chain settlement)
- ✅ Faster execution (no consensus delay)
- ✅ Privacy-preserving (ephemeral guilds, no public ledger)
- ❌ Weaker finality guarantees (reputation vs cryptographic consensus)

**vs. IPFS/Filecoin:**
- Different layer: ZeroState is compute, IPFS is storage
- Can integrate: Fetch data from IPFS, process in ZeroState, store results back

---

## Architecture Overview

### Three-Tier Network Design

```
┌─────────────────────────────────────────────────────────────┐
│                    APPLICATION LAYER                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │ Task Creators│  │   Executors  │  │   Observers  │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           ▲
                           │
┌─────────────────────────────────────────────────────────────┐
│                    EXECUTION LAYER                           │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              Ephemeral Guilds (Private Groups)       │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐          │   │
│  │  │ WASM Run │  │ Receipts │  │  Costs   │          │   │
│  │  └──────────┘  └──────────┘  └──────────┘          │   │
│  └──────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                           ▲
                           │
┌─────────────────────────────────────────────────────────────┐
│                    P2P NETWORK LAYER                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Routing    │  │  Discovery   │  │   Gossip     │      │
│  │  (Q-Learning)│  │   (HNSW)     │  │ (libp2p)     │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
                           ▲
                           │
┌─────────────────────────────────────────────────────────────┐
│                    ECONOMIC LAYER                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │   Payment    │  │  Reputation  │  │ Settlements  │      │
│  │   Channels   │  │   Scoring    │  │              │      │
│  └──────────────┘  └──────────────┘  └──────────────┘      │
└─────────────────────────────────────────────────────────────┘
```

### Backbone Layer (Future Sprint 7-8)

**Status:** Planned, not yet implemented  
**Purpose:** Persistent indexing and discovery layer

**Components:**
1. **Distributed Registry:**
   - Long-lived nodes (always-on servers)
   - Store reputation scores, blacklists, capability indices
   - Provide bootstrapping for new nodes

2. **Capability Index:**
   - HNSW vector index of executor capabilities
   - Enables fast semantic search ("find executors good at NLP tasks")
   - Currently in-memory per node → will be distributed

3. **Settlement Anchoring (Optional):**
   - Anchor dispute resolutions on-chain (Ethereum L2, Cosmos, etc.)
   - NOT for every task, only disputed settlements
   - Provides ultimate finality for high-value tasks

**Why Deferred?**
- Core P2P layer works without it (decentralized discovery via libp2p DHT)
- Adds complexity and centralization risk
- Focus on proving core mechanics first

---

## Architecture Deep Dive

### Guild Task Distribution

**Model:** Single executor per task (not redundant)

**Why Not Redundant Execution?**
- **Cost:** Redundancy 3x would 3x costs
- **Use Case:** Most tasks are deterministic (WASM is deterministic)
- **Trust:** Receipts + reputation provide accountability without redundancy

**Trust Model:**
1. **Executor Selection:** Creator picks high-reputation executor
2. **Execution:** Single executor runs task, generates receipt
3. **Attestation:** Observers (witnesses) verify receipt hash
4. **Reputation:** Successful execution increases score, failures blacklist

**When Redundancy Matters (Future):**
- Non-deterministic tasks (external API calls, random seeds)
- High-value tasks requiring Byzantine fault tolerance
- Possible extension: Optional redundancy flag in manifest

### Consensus Mechanism

**Current:** **Reputation-based soft consensus** (no blockchain)

**How it Works:**
1. Executor generates cryptographic receipt (Ed25519 signed)
2. Witnesses attest to receipt hash (multi-party signatures)
3. Receipt published to DHT (verifiable by anyone)
4. Disputes resolved via reputation system (blacklisting)

**Optional Blockchain Integration (Planned):**
- **NOT for every task** (too slow, too expensive)
- **Only for disputes:**
  ```
  Task Creator ──dispute──> Blockchain Smart Contract
                           (submits executor receipt + witness signatures)
                           ↓
                           Arbitration (slashing if fraud detected)
  ```

**Target Chains (TBD):**
- Ethereum L2 (Arbitrum, Optimism) for finality
- Cosmos (IBC compatible)
- Custom rollup (if needed)

**Design Philosophy:**
- Off-chain first, on-chain only when necessary
- Reputation is the primary incentive, blockchain is the ultimate backup

---

## Economic Model

### Currency & Payments

**Current:** Abstract units (no specific token)

**Denomination:** Floating-point currency units
- Example: Task costs 0.5 units, executor deposits 10 units in channel
- Unit value determined by market (could be stablecoins, native tokens, or fiat)

**Payment Flow:**
1. **Channel Opening:** Both parties deposit (e.g., Creator: 100, Executor: 100)
2. **Off-Chain Payments:** Signed payment proofs (sequence numbers prevent replay)
3. **Settlement:** Channel closed, final balances settled
4. **Disputes:** Challenge period for fraudulent claims

**Future Token Options:**
1. **Stablecoin-backed:** Pay in USDC/DAI for price stability
2. **Native Token:** ZeroState token for staking, governance, fee burns
3. **Multi-Currency:** Support BTC Lightning, ETH channels, etc.

**Design Decision:**
- Deferred tokenomics until product-market fit
- Infrastructure-first, token-economics second
- Avoid "token for token's sake" trap

### Reputation Recovery

**Blacklist is NOT permanent!**

**Blacklist Triggers:**
- Reputation score < 0.3 (30%)
- Automatic blacklisting for 24 hours (configurable)

**Recovery Path:**
1. **Automatic Expiry:** Blacklist expires after 24 hours
2. **Score Rebuilding:** Start executing tasks again, rebuild score
3. **Manual Removal:** System operator can whitelist (for false positives)

**Score Decay:**
- Time-based decay (half-life: 7 days by default)
- Incentivizes continuous good behavior
- Old bad behavior eventually forgotten if reformed

**Permanent Bans (Future):**
- Severe violations (DoS attacks, fraud) could trigger longer/permanent bans
- Requires governance mechanism (not yet implemented)

### Pricing Discovery

**Current:** Manual, negotiated pricing

**How It Works:**
1. Task creator specifies `MaxTotalPrice` in manifest
2. Executor decides whether to accept based on:
   - Estimated duration (PricePerSecond × Duration)
   - Estimated memory (PricePerMB × Memory)
   - Total = min(estimated cost, MaxTotalPrice)
3. If profitable, executor accepts; otherwise, rejects

**Future: Dynamic Pricing (Sprint 7+)**
1. **Market-Driven:**
   - Task creators bid, executors quote prices
   - Supply/demand determines rates
   - High-reputation executors command premium

2. **Auction-Based:**
   - Task posted with budget, executors bid
   - Lowest bidder (with min reputation) wins

3. **Algorithmic:**
   - Network tracks average prices per task type
   - Automatic price suggestions based on historical data

**Why Manual for Now?**
- Simpler to implement and test
- Market price discovery needs critical mass
- Avoid premature optimization

---

## Technical Deep Dive

### WASM Limitations & Sweet Spot

**Constraints:**
- **Memory:** 128 MB max (Wazero runtime limit)
- **Timeout:** 30 seconds default
- **No Networking:** Sandboxed, no external API calls
- **Deterministic:** Same input → same output (critical for receipts)

**What Fits:**
- ✅ **Model Inference:** BERT (110M params ~440MB model, can quantize to <100MB)
- ✅ **Image Processing:** Resize, crop, filters (single images <128MB)
- ✅ **Data Validation:** Schema validation, checksums, parsing
- ✅ **Simple Aggregations:** Sum, mean, filter operations
- ✅ **Compression:** GZIP, Brotli encoding/decoding

**What Doesn't Fit:**
- ❌ **Training:** Too long, too memory-intensive
- ❌ **Large Models:** GPT-3 (175B params) - impossible
- ❌ **Video Processing:** 4K video exceeds 128MB easily
- ❌ **External APIs:** No network access from WASM

**Future Expansions:**
- **Longer Timeouts:** Allow opt-in for 5-minute tasks (higher cost)
- **Streaming:** Support chunked input/output for large data
- **Controlled Networking:** Whitelist-based external API calls (with determinism caveats)

### Vector Embeddings (HNSW Search)

**Purpose:** Semantic search for executor capabilities

**Current Implementation:**
- **Capability Vectors:** 128-dimensional embeddings
- **Source:** Manual feature engineering (not ML-based yet)
- **Dimensions:** Task type, performance metrics, resource availability

**Example Capability Vector:**
```json
{
  "task_types": {
    "image_processing": 0.9,
    "nlp": 0.6,
    "data_etl": 0.8
  },
  "performance": {
    "cpu_cores": 0.7,      // Normalized: 8 cores → 0.7
    "memory_gb": 0.5,      // Normalized: 16GB → 0.5
    "avg_latency_ms": 0.8  // Normalized: 50ms → 0.8 (lower is better)
  },
  "reputation": 0.85,
  "pricing_tier": 0.4      // Normalized: mid-tier
}
```

**Embedding Generation:**
1. **Manual:** Executor registers capabilities in manifest
2. **Feature Vector:** System converts to 128-dim float array
3. **HNSW Index:** Insert into hierarchical navigable small world graph
4. **Search:** Creator query "find executors good at NLP" → vector search

**Future: ML-Based Embeddings (Sprint 8+)**
- **Pre-trained Models:** Use sentence transformers (SBERT) for task descriptions
- **Task History:** Learn embeddings from executor performance data
- **Recommendation System:** Collaborative filtering for executor selection

**Why Manual for Now?**
- Simpler, no ML training pipeline needed
- Deterministic, explainable
- Good enough for initial use cases

### Q-Routing Convergence

**Problem:** Peers join/leave, network topology changes

**Q-Learning Adaptation:**
1. **Exploration:** New peers randomly route messages to learn paths
2. **Exploitation:** Use learned Q-values to pick best routes
3. **Decay:** Old Q-values decay to adapt to topology changes

**Convergence Speed:**
- **Stable Network:** ~100-500 messages per peer to converge
- **Churn (10% join/leave per minute):** Continuous adaptation, no "convergence"
- **Trade-off:** Epsilon (exploration rate) controls adaptability vs stability

**Warm-Start Strategies:**
1. **Bootstrap from DHT:** Query DHT for peer capabilities, initialize Q-values
2. **Gossip Q-Tables:** Peers share Q-values during handshake (faster convergence)
3. **Persistent Q-Tables:** Save Q-values to disk, reload on restart

**Why Q-Learning vs Static Routing?**
- **Dynamic:** Adapts to network conditions (latency, congestion)
- **Decentralized:** No global coordinator
- **Proven:** Used in networking (packet routing, load balancing)

**Limitations:**
- **Scalability:** O(N²) Q-table size (peer-to-peer pairwise)
- **Future:** Consider hierarchical routing for 10k+ node networks

---

## Current Focus & Timeline

### Sprint 6: Monitoring & Observability (Current)

**Status:** Phase 1 complete (Prometheus metrics)  
**Progress:** 4/16 tasks (25%)

**Completed (Nov 7, 2025):**
- ✅ Task 1: Core metrics infrastructure
- ✅ Task 2: P2P network metrics
- ✅ Task 3: Execution layer metrics
- ✅ Task 4: Economic layer metrics

**Next (Nov 8-15):**
- Task 5-7: Grafana dashboards, alert rules, provisioning
- Task 8-10: OpenTelemetry tracing, Jaeger integration
- Task 11-12: Structured logging, log aggregation
- Task 13-14: Health check endpoints, Kubernetes probes
- Task 15-16: Monitoring stack deployment, integration tests

### Production Timeline

**Testnet Launch:** Q1 2026 (3-4 months)
- Sprint 7: Distributed tracing & logging (Dec 2025)
- Sprint 8: Security hardening (Jan 2026)
- Sprint 9: Load testing & optimization (Feb 2026)
- Sprint 10: Testnet deployment (Mar 2026)

**Mainnet Launch:** Q3 2026 (6-9 months)
- Sprint 11-12: Backbone layer (persistent index, registry)
- Sprint 13: Tokenomics & payment integration
- Sprint 14: Audits (security, economic model)
- Sprint 15: Mainnet gradual rollout

**Key Milestones:**
- **Now:** 254 tests passing, core features complete
- **Q1 2026:** Public testnet, developer documentation
- **Q2 2026:** Beta partners, real-world use cases
- **Q3 2026:** Mainnet launch, token generation event (if applicable)

**Conservative Estimate:** 9-12 months to mainnet (accounting for unknowns)

---

## Design Philosophy

### Pragmatic Decentralization
- **Not maximally decentralized:** Reputation system has soft trust
- **Not blockchain-first:** Off-chain unless absolutely necessary
- **Trade-offs:** Speed and cost over censorship resistance

### Incremental Complexity
- **Start simple:** Reputation before blockchain
- **Add features as needed:** Backbone layer when scale requires it
- **Avoid premature optimization:** Manual pricing before market mechanisms

### Production-Ready Culture
- **Testing:** 100% test coverage on critical paths
- **Monitoring:** Metrics-first development (Sprint 6 focus)
- **Documentation:** Architecture docs alongside code

---

## Open Questions & Future Research

1. **Backbone Centralization Risk:**
   - How to prevent registry nodes from becoming chokepoints?
   - Byzantine fault tolerance for backbone layer?

2. **Dispute Resolution:**
   - Who pays for on-chain arbitration gas?
   - What's the economic model for arbitrators?

3. **Privacy:**
   - How to hide task inputs from executors (TEE integration)?
   - Zero-knowledge proofs for receipt verification?

4. **Scale:**
   - Can Q-routing handle 10k+ nodes?
   - When to shard the network into sub-networks?

---

**Questions? Feedback?**
This is a living document. As architecture evolves, this doc will be updated.
Last major revision: November 7, 2025
