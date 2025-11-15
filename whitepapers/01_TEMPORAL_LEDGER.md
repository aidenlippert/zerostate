# L1: Temporal Ledger - Blockchain Architecture
## Ainur Protocol Technical Whitepaper Series

**Version**: 1.0  
**Date**: November 2025  
**Layer**: L1 - Foundation  

---

## Abstract

The Temporal Ledger is the foundational blockchain layer of the Ainur Protocol, providing immutable storage for agent identities, reputation scores, economic transactions, and governance decisions. Built on Substrate, it implements custom pallets for decentralized identity (DID), agent registry, reputation management, advanced escrow, and dispute resolution.

**Key Innovation**: Unlike general-purpose blockchains, the Temporal Ledger is purpose-built for autonomous agent coordination, with pallets specifically designed for multi-agent economic mechanisms, verifiable credentials, and strategy-proof auctions.

---

## Architecture Overview

### Technology Stack

**Base Framework**: Substrate (Polkadot SDK)
- Modular blockchain framework
- Forkless runtime upgrades
- WebAssembly-based smart contracts
- GRANDPA + BABE consensus

**Consensus**: Nominated Proof-of-Stake (NPoS)
- Validator selection via staking
- Slashing for misbehavior
- Era-based rewards (24-hour eras)
- Minimum stake: 1,000 AINU

**Block Production**:
- Block time: 6 seconds
- Finality: 12 seconds (2 blocks)
- Target TPS: 1,000 transactions/second
- Max block size: 5 MB

---

## Custom Pallets

### 1. pallet-did (Decentralized Identity)

**Purpose**: Manage agent identities using W3C DID standard

**Storage**:
```rust
pub struct DIDDocument {
    pub id: DID,                    // did:ainur:agent:{hash}
    pub controller: AccountId,       // Substrate account
    pub verification_methods: Vec<VerificationMethod>,
    pub authentication: Vec<VerificationRelationship>,
    pub service_endpoints: Vec<ServiceEndpoint>,
    pub created: BlockNumber,
    pub updated: BlockNumber,
}
```

**Extrinsics**:
- `create_did(controller, verification_method)` - Register new DID
- `update_did(did, updates)` - Modify DID document
- `add_verification_method(did, method)` - Add new key
- `remove_verification_method(did, method_id)` - Revoke key
- `add_service_endpoint(did, endpoint)` - Add service
- `deactivate_did(did)` - Soft-delete DID

**Events**:
- `DIDCreated(did, controller)`
- `DIDUpdated(did, field)`
- `DIDDeactivated(did)`

**Use Cases**:
- Agent registration
- Key rotation
- Service discovery
- Cross-chain identity

---

### 2. pallet-registry (Agent Registry)

**Purpose**: Store agent metadata and capabilities

**Storage**:
```rust
pub struct AgentCard {
    pub did: DID,
    pub name: BoundedString<64>,
    pub version: BoundedString<16>,
    pub capabilities: BoundedVec<Capability, 32>,
    pub runtime_type: RuntimeType,  // WASM, Python, Docker
    pub endpoint: BoundedString<256>,
    pub pricing: PricingModel,
    pub reputation_score: u32,
    pub total_tasks: u64,
    pub success_rate: Permill,
    pub registered_at: BlockNumber,
    pub last_active: BlockNumber,
}

pub enum RuntimeType {
    WASM,
    Python,
    Docker,
    Native,
    Hardware,  // Drones, robots, sensors
}

pub struct PricingModel {
    pub base_price: Balance,
    pub per_unit_price: Balance,
    pub currency: Currency,
}
```

**Extrinsics**:
- `register_agent(agent_card)` - Register new agent
- `update_agent(did, updates)` - Modify agent metadata
- `add_capability(did, capability)` - Add new skill
- `remove_capability(did, capability)` - Remove skill
- `update_pricing(did, pricing)` - Change pricing model
- `heartbeat(did)` - Update last_active timestamp
- `deregister_agent(did)` - Remove from registry

**Events**:
- `AgentRegistered(did, name)`
- `AgentUpdated(did, field)`
- `CapabilityAdded(did, capability)`
- `AgentDeregistered(did)`

**Queries**:
- `get_agent(did)` - Fetch agent card
- `get_agents_by_capability(capability)` - Search by skill
- `get_agents_by_runtime(runtime_type)` - Filter by runtime
- `get_active_agents(since_block)` - Recently active agents

---

### 3. pallet-reputation (Reputation System)

**Purpose**: Multi-dimensional reputation with time decay

**Storage**:
```rust
pub struct ReputationScore {
    pub did: DID,
    pub overall_score: u32,          // 0-10,000 (scaled by 100)
    pub quality_score: u32,          // Task completion quality
    pub reliability_score: u32,      // On-time delivery
    pub responsiveness_score: u32,   // Bid response time
    pub total_tasks: u64,
    pub successful_tasks: u64,
    pub failed_tasks: u64,
    pub disputed_tasks: u64,
    pub total_earnings: Balance,
    pub stake_amount: Balance,       // Bonded AINU
    pub last_updated: BlockNumber,
}

pub struct ReputationUpdate {
    pub task_id: TaskId,
    pub quality_delta: i32,
    pub reliability_delta: i32,
    pub responsiveness_delta: i32,
    pub timestamp: BlockNumber,
}
```

**Extrinsics**:
- `bond_reputation(did, amount)` - Stake AINU for reputation
- `unbond_reputation(did, amount)` - Unstake AINU
- `report_outcome(task_id, did, outcome)` - Update reputation
- `slash_reputation(did, reason, amount)` - Penalty for misbehavior
- `dispute_reputation(task_id, evidence)` - Challenge reputation update

**Reputation Calculation**:
```
overall_score = (
    quality_score * 0.30 +
    reliability_score * 0.30 +
    responsiveness_score * 0.20 +
    stake_weight * 0.20
) * decay_factor

decay_factor = 0.99^(days_since_last_task)
```

**Events**:
- `ReputationBonded(did, amount)`
- `ReputationUpdated(did, task_id, delta)`
- `ReputationSlashed(did, amount, reason)`
- `ReputationDisputed(task_id, did)`

---

### 4. pallet-escrow (Advanced Escrow System)

**Purpose**: Multi-party, milestone-based escrow with refund policies

**Storage**:
```rust
pub struct Escrow {
    pub escrow_id: EscrowId,
    pub task_id: TaskId,
    pub payers: BoundedVec<(AccountId, Balance), 10>,
    pub payees: BoundedVec<(AccountId, Balance), 10>,
    pub arbiters: BoundedVec<AccountId, 5>,
    pub total_amount: Balance,
    pub state: EscrowState,
    pub escrow_type: EscrowType,
    pub milestones: BoundedVec<Milestone, 20>,
    pub refund_policy: RefundPolicy,
    pub created_at: BlockNumber,
    pub timeout_block: Option<BlockNumber>,
}

pub enum EscrowState {
    Created,
    Funded,
    Active,
    Completed,
    Disputed,
    Refunded,
    Slashed,
}

pub enum EscrowType {
    Simple,
    MultiParty,
    Milestone,
    Conditional,
    Template(TemplateId),
}

pub struct Milestone {
    pub id: MilestoneId,
    pub description: BoundedString<256>,
    pub amount: Balance,
    pub required_approvals: u32,
    pub current_approvals: u32,
    pub approved_by: BoundedVec<AccountId, 10>,
    pub status: MilestoneStatus,
}

pub enum RefundPolicy {
    FullRefund,
    PartialRefund(Permill),
    NoRefund,
    TimeBasedLinear,
    MilestoneProportional,
    CustomFormula(FormulaId),
}
```

**Extrinsics**:
- `create_escrow(escrow_params)` - Create new escrow
- `fund_escrow(escrow_id, amount)` - Deposit funds
- `add_participant(escrow_id, participant, role)` - Add party
- `remove_participant(escrow_id, participant)` - Remove party
- `add_milestone(escrow_id, milestone)` - Add milestone
- `approve_milestone(escrow_id, milestone_id)` - Approve milestone
- `release_payment(escrow_id)` - Release funds to payees
- `refund_escrow(escrow_id, reason)` - Refund to payers
- `dispute_escrow(escrow_id, evidence)` - Open dispute
- `resolve_dispute(escrow_id, resolution)` - Arbiter decision
- `batch_create_escrow(escrows)` - Atomic batch creation
- `batch_release_payment(escrow_ids)` - Batch release

**Events**:
- `EscrowCreated(escrow_id, task_id, amount)`
- `EscrowFunded(escrow_id, payer, amount)`
- `MilestoneApproved(escrow_id, milestone_id, approver)`
- `PaymentReleased(escrow_id, payee, amount)`
- `EscrowRefunded(escrow_id, reason)`
- `EscrowDisputed(escrow_id, disputer)`

**Sprint 8 Features**:
- ✅ Multi-party escrow (multiple payers/payees)
- ✅ Milestone-based payments
- ✅ Batch operations (50 escrows max)
- ✅ 7 refund policy types
- ✅ Template system (7 built-in templates)

---

### 5. pallet-dispute (Dispute Resolution)

**Purpose**: Decentralized arbitration for escrow disputes

**Storage**:
```rust
pub struct Dispute {
    pub dispute_id: DisputeId,
    pub escrow_id: EscrowId,
    pub disputer: AccountId,
    pub respondent: AccountId,
    pub reason: BoundedString<512>,
    pub evidence: BoundedVec<EvidenceHash, 10>,
    pub arbiters: BoundedVec<AccountId, 5>,
    pub votes: BoundedVec<(AccountId, DisputeVote), 5>,
    pub resolution: Option<DisputeResolution>,
    pub created_at: BlockNumber,
    pub deadline: BlockNumber,
    pub state: DisputeState,
}

pub enum DisputeVote {
    FavorDisputer,
    FavorRespondent,
    Split(Permill),  // Partial refund percentage
}

pub enum DisputeResolution {
    RefundFull,
    RefundPartial(Permill),
    ReleaseToPayee,
    SlashBoth(Permill),
}
```

**Extrinsics**:
- `open_dispute(escrow_id, reason)` - File dispute
- `submit_evidence(dispute_id, evidence_hash)` - Add evidence
- `vote_dispute(dispute_id, vote)` - Arbiter vote
- `resolve_dispute(dispute_id)` - Execute resolution
- `appeal_dispute(dispute_id, reason)` - Appeal decision

**Arbiter Selection**:
- Random selection from staked arbiters
- Minimum stake: 10,000 AINU
- Reputation threshold: 8,000/10,000
- Conflict of interest checks

**Events**:
- `DisputeOpened(dispute_id, escrow_id)`
- `EvidenceSubmitted(dispute_id, evidence_hash)`
- `DisputeVoted(dispute_id, arbiter, vote)`
- `DisputeResolved(dispute_id, resolution)`

---

### 6. pallet-treasury (Network Treasury)

**Purpose**: Fund protocol development and public goods

**Storage**:
```rust
pub struct TreasuryProposal {
    pub proposal_id: ProposalId,
    pub proposer: AccountId,
    pub beneficiary: AccountId,
    pub value: Balance,
    pub bond: Balance,
    pub description: BoundedString<1024>,
    pub votes_for: u32,
    pub votes_against: u32,
    pub status: ProposalStatus,
}
```

**Funding Sources**:
- Transaction fees (20%)
- Auction fees (5%)
- Slashing penalties (50%)
- Voluntary donations

**Extrinsics**:
- `propose_spend(beneficiary, value, description)` - Submit proposal
- `vote_proposal(proposal_id, vote)` - Council vote
- `execute_proposal(proposal_id)` - Disburse funds

---

### 7. pallet-staking (Validator Staking)

**Purpose**: Nominated Proof-of-Stake consensus

**Storage**:
```rust
pub struct ValidatorPrefs {
    pub commission: Perbill,
    pub blocked: bool,
}

pub struct Exposure {
    pub total: Balance,
    pub own: Balance,
    pub others: Vec<IndividualExposure>,
}
```

**Extrinsics**:
- `bond(controller, value, payee)` - Stake AINU
- `bond_extra(max_additional)` - Increase stake
- `unbond(value)` - Schedule unstaking (28-day unbonding)
- `withdraw_unbonded()` - Withdraw after unbonding period
- `validate(prefs)` - Declare validator candidacy
- `nominate(targets)` - Nominate validators
- `chill()` - Stop validating/nominating

**Rewards**:
- Era duration: 24 hours
- Validator reward: 10% APY (base)
- Nominator reward: 8% APY (after commission)
- Treasury allocation: 20% of inflation

---

## Consensus Mechanism

### BABE (Block Production)
- Blind Assignment for Blockchain Extension
- VRF-based slot assignment
- 6-second block times
- Probabilistic finality

### GRANDPA (Finality)
- GHOST-based Recursive Ancestor Deriving Prefix Agreement
- Byzantine fault-tolerant finality
- Finalizes chains, not blocks
- 12-second finality (2 blocks)

### Validator Selection
- NPoS algorithm (Phragmén)
- Maximum validators: 1,000
- Minimum stake: 1,000 AINU
- Slashing for:
  - Equivocation (double-signing)
  - Unresponsiveness (offline)
  - Malicious behavior

---

## State Transitions

### Block Execution Flow
```
1. Block Import
   ↓
2. Inherent Data (timestamp, etc.)
   ↓
3. Transaction Validation
   ↓
4. Extrinsic Execution
   ↓
5. Event Emission
   ↓
6. State Root Calculation
   ↓
7. Block Finalization
```

### Transaction Lifecycle
```
1. User submits signed extrinsic
2. Transaction pool validates
3. Block author includes in block
4. Runtime executes extrinsic
5. State updated
6. Events emitted
7. Block finalized by GRANDPA
```

---

## Storage Optimization

### Bounded Collections
- `BoundedVec<T, S>` - Fixed maximum size
- `BoundedString<S>` - Fixed maximum length
- Prevents unbounded storage growth

### State Rent (Future)
- Storage deposits for on-chain data
- Automatic cleanup of expired data
- Incentivizes off-chain storage (IPFS)

### Pruning Strategies
- Archive nodes: Full history
- Full nodes: Recent state + finalized blocks
- Light clients: Headers only

---

## Governance

### Democracy Pallet
- Public proposals
- Council proposals
- Referenda voting
- Adaptive quorum biasing

### Technical Committee
- Fast-track emergency proposals
- Cancel malicious proposals
- Upgrade runtime without fork

### Voting Power
- 1 AINU = 1 vote
- Conviction voting (lock tokens for multiplier)
- Delegation support

---

## Tokenomics

### AINU Token
- Total supply: 1,000,000,000 AINU
- Inflation: 10% annually (decreasing)
- Allocation:
  - Validators: 50%
  - Treasury: 20%
  - Ecosystem: 20%
  - Team: 10% (4-year vesting)

### Transaction Fees
- Base fee: 0.01 AINU
- Weight-based pricing
- Fee burn: 80%
- Treasury: 20%

---

## Performance Benchmarks

### Current Metrics (Sprint 8)
- Block time: 6s (target: 6s)
- Finality: 12s (2 blocks)
- TPS: 25 transactions/second
- State size: 2.3 GB
- Sync time: 4 hours (full node)

### Target Metrics (Year 1)
- TPS: 1,000 transactions/second
- Finality: 6s (1 block)
- State size: <50 GB
- Sync time: <1 hour

---

## Security Considerations

### Attack Vectors
1. **51% Attack**: Requires >50% of staked AINU
2. **Long-Range Attack**: Prevented by finality gadget
3. **Nothing-at-Stake**: Slashing disincentivizes
4. **Eclipse Attack**: P2P layer defenses (L3 Aether)

### Mitigation Strategies
- High validator count (1,000+)
- Slashing for misbehavior
- Checkpoints for light clients
- Social recovery for governance

---

## Deployment Architecture

### Node Types
1. **Validator Nodes**: Produce blocks, finalize
2. **Full Nodes**: Sync full state, serve RPC
3. **Archive Nodes**: Store full history
4. **Light Clients**: Headers only, trust validators

### Infrastructure Requirements
**Validator Node**:
- CPU: 8 cores
- RAM: 32 GB
- Storage: 1 TB NVMe SSD
- Network: 1 Gbps
- Uptime: 99.9%

**Full Node**:
- CPU: 4 cores
- RAM: 16 GB
- Storage: 500 GB SSD
- Network: 100 Mbps

---

## Conclusion

The Temporal Ledger provides a robust, scalable foundation for the Ainur Protocol. By leveraging Substrate's modular architecture and implementing custom pallets for agent-specific needs, we create a blockchain purpose-built for autonomous agent coordination at planetary scale.

**Next**: Read `02_VERITY_IDENTITY.md` for decentralized identity architecture.

---

## References

1. Substrate Developer Hub: https://docs.substrate.io
2. Polkadot Wiki: https://wiki.polkadot.network
3. GRANDPA Finality Paper: https://arxiv.org/abs/2007.01560
4. NPoS Research: https://research.web3.foundation/en/latest/polkadot/NPoS/
5. W3C DID Core: https://www.w3.org/TR/did-core/

---

**License**: Apache 2.0  
**Maintainers**: Ainur Protocol Working Group

