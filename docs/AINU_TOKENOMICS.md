# AINU Token Design Document
## Phase 3: Economy Layer

**Version**: 1.0  
**Date**: November 12, 2025  
**Status**: Design Phase

---

## 🎯 Executive Summary

AINU is the native utility token of the Ainur decentralized agent marketplace. It powers a three-sided marketplace connecting:
- **Users** (task requesters)
- **Agents** (WASM execution units)
- **Node Operators** (infrastructure providers)

The tokenomics are designed to create sustainable economic incentives for all participants while maintaining decentralization and security.

---

## 📊 Token Overview

### Basic Parameters

```yaml
Token Name: AINU
Token Symbol: AINU
Total Supply: 10,000,000,000 (10 Billion)
Supply Type: Fixed (deflationary through burns)
Decimals: 18
Standard: ERC-20 (Ethereum), SPL (Solana), Substrate Native
```

### Token Utility

1. **Task Payments**: Users pay AINU to execute tasks
2. **Agent Staking**: Agents stake AINU to participate
3. **Governance**: Token holders vote on protocol upgrades
4. **Network Fees**: Transaction fees paid in AINU
5. **Staking Rewards**: Node operators earn AINU for uptime

---

## 💰 Token Distribution

### Initial Allocation (10B tokens)

```
40% - Ecosystem & Rewards (4,000,000,000)
   ├─ 20% - Task Execution Rewards (2B)
   ├─ 10% - Agent Developer Grants (1B)
   ├─ 5%  - Community Airdrops (500M)
   └─ 5%  - Liquidity Mining (500M)

20% - Team & Advisors (2,000,000,000)
   ├─ 4-year vesting
   └─ 1-year cliff

15% - Foundation & Treasury (1,500,000,000)
   ├─ Protocol development
   ├─ Security audits
   └─ Marketing & partnerships

10% - Public Sale (1,000,000,000)
   ├─ Fair launch
   └─ No pre-sale

10% - Strategic Investors (1,000,000,000)
   ├─ 2-year vesting
   └─ 6-month cliff

5% - Initial Liquidity (500,000,000)
   ├─ DEX pools
   └─ CEX listings
```

### Vesting Schedule

```
Team & Advisors:
  - 1-year cliff (no tokens released)
  - Linear vesting over 3 years after cliff
  - Total: 4 years to full unlock

Strategic Investors:
  - 6-month cliff
  - Linear vesting over 18 months after cliff
  - Total: 2 years to full unlock

Ecosystem Rewards:
  - Released over 5 years
  - Logarithmic decay curve
  - Year 1: 35%, Year 2: 25%, Year 3: 20%, Year 4: 12%, Year 5: 8%
```

---

## 🔄 Economic Model

### Task Execution Flow

```
┌─────────────┐
│    USER     │
│  (Submits   │
│   Task)     │
└──────┬──────┘
       │
       │ Pays: Base Fee + Gas
       ↓
┌─────────────────────────────────────────┐
│        AINUR PROTOCOL TREASURY          │
│  • Collects task payment                │
│  • Distributes to participants          │
└──────┬──────────────────┬───────────────┘
       │                  │
       │ 70%              │ 30%
       ↓                  ↓
┌─────────────┐    ┌──────────────┐
│   AGENT     │    │ PROTOCOL     │
│  (Executes) │    │ (Burns/Fees) │
└─────────────┘    └──────────────┘
```

### Fee Structure

**Task Submission**:
- Base Fee: 0.1 AINU (minimum)
- Compute Fee: Variable based on complexity
- Gas Fee: Network transaction cost
- Total: `Base + (Compute × Time) + Gas`

**Revenue Split**:
- 70% → Agent (executor reward)
- 20% → Node Operator (infrastructure)
- 5% → Protocol Treasury (governance)
- 5% → Burn (deflationary mechanism)

### Pricing Model

Tasks are priced dynamically based on:

```javascript
taskCost = baseFee + (computeUnits × unitPrice) + gasCost

where:
  baseFee = 0.1 AINU (minimum viable fee)
  computeUnits = estimated WASM execution cycles
  unitPrice = market-determined (supply/demand)
  gasCost = network transaction fee
```

**Example Calculation**:
```
Task: "Calculate factorial of 5 and multiply by 7"
- Base Fee: 0.1 AINU
- Compute: 2 steps × 0.05 AINU/step = 0.1 AINU
- Gas: 0.01 AINU
- Total: 0.21 AINU ($0.021 at $0.10/AINU)
```

---

## 🔒 Staking Mechanism

### Agent Staking

Agents must stake AINU to participate in the network:

```yaml
Minimum Stake:
  - Tier 1 (Basic): 1,000 AINU
  - Tier 2 (Standard): 10,000 AINU
  - Tier 3 (Premium): 100,000 AINU

Benefits by Tier:
  Tier 1:
    - Can execute simple tasks
    - 70% revenue share
    - Standard visibility
  
  Tier 2:
    - Can execute complex tasks
    - 75% revenue share
    - Priority in task queue
    - Enhanced marketplace visibility
  
  Tier 3:
    - Can execute enterprise tasks
    - 80% revenue share
    - Guaranteed task allocation
    - Premium marketplace placement
    - Access to private contracts

Slashing Conditions:
  - Downtime > 10%: 5% stake slashed
  - Failed task execution: 1% stake slashed
  - Malicious behavior: 100% stake slashed
```

### Node Operator Staking

```yaml
Minimum Stake: 50,000 AINU

Rewards:
  - 20% of task execution fees
  - Block rewards (if validator)
  - 5% APY on staked amount

Requirements:
  - 99.9% uptime
  - Minimum 100 Mbps connection
  - Support for WASM runtime
```

---

## 🔥 Deflationary Mechanisms

### Token Burns

1. **Transaction Burns**:
   - 5% of every task fee is burned
   - Reduces circulating supply over time
   - Creates scarcity

2. **Penalty Burns**:
   - Slashed stakes are burned
   - Malicious behavior = permanent burn

3. **Governance Burns**:
   - Failed proposals burn proposer's deposit
   - Spam prevention mechanism

### Long-term Supply Dynamics

```
Year 1: 10B tokens (100%)
Year 2: 9.5B tokens (95%) - 500M burned
Year 3: 9.0B tokens (90%) - 1B burned
Year 5: 8.0B tokens (80%) - 2B burned
Year 10: 6.0B tokens (60%) - 4B burned
```

Target: **50% supply burned by Year 20**

---

## 🗳️ Governance Model

### Voting Power

```
Voting Power = Staked AINU × Time Weight

where:
  Time Weight = min(stakeDuration / 365 days, 4.0)
  
Example:
  - 1,000 AINU staked for 1 year = 1,000 votes
  - 1,000 AINU staked for 4 years = 4,000 votes
```

### Proposal Types

1. **Protocol Upgrades** (Critical)
   - Required: 60% quorum, 75% approval
   - Examples: Fee structure changes, staking parameters

2. **Treasury Spending** (Standard)
   - Required: 40% quorum, 66% approval
   - Examples: Grant proposals, marketing budgets

3. **Parameter Adjustments** (Minor)
   - Required: 20% quorum, 50% approval
   - Examples: Gas limits, task timeouts

### Governance Process

```
1. Proposal Submission
   ├─ Deposit: 10,000 AINU (refunded if passed)
   └─ Discussion period: 7 days

2. Voting Period
   ├─ Duration: 14 days
   └─ Minimum quorum required

3. Execution
   ├─ Passed: Timelock (48 hours)
   └─ Failed: Deposit burned
```

---

## 💎 Token Economics

### Price Stability Mechanisms

1. **Liquidity Pools**:
   - 500M AINU allocated for initial liquidity
   - Protocol-owned liquidity (POL) on DEXs
   - Prevents rug pulls and maintains depth

2. **Market Making**:
   - Treasury can deploy market makers
   - Maintains healthy spread (< 1%)

3. **Circuit Breakers**:
   - Trading halts if price moves > 20% in 1 hour
   - Protects against flash crashes

### Demand Drivers

1. **Network Usage**:
   - More tasks = more AINU demand
   - Positive feedback loop

2. **Staking Requirements**:
   - Agents lock up supply
   - Reduces circulating tokens

3. **Burns**:
   - Permanent supply reduction
   - Creates scarcity premium

4. **Governance Rights**:
   - Token holders control protocol
   - Value capture from fees

---

## 📈 Financial Projections

### Conservative Case (Years 1-5)

```
Assumptions:
  - 10,000 tasks/day by Year 1
  - Avg task cost: 0.5 AINU
  - 30% annual growth

Year 1:
  - Daily Revenue: 5,000 AINU
  - Annual Revenue: 1,825,000 AINU
  - Protocol Treasury: 91,250 AINU
  - Burned: 91,250 AINU

Year 3:
  - Daily Revenue: 8,450 AINU
  - Annual Revenue: 3,084,250 AINU
  - Protocol Treasury: 154,212 AINU
  - Burned: 154,212 AINU

Year 5:
  - Daily Revenue: 14,300 AINU
  - Annual Revenue: 5,219,500 AINU
  - Protocol Treasury: 260,975 AINU
  - Burned: 260,975 AINU
```

### Token Value Proposition

```
If AINU = $0.10:
  - Task execution: $0.05 per task
  - Agent earnings: ~$3,500/year (10 tasks/day)
  - Node operator earnings: ~$1,000/year

If AINU = $1.00:
  - Task execution: $0.50 per task
  - Agent earnings: ~$35,000/year
  - Node operator earnings: ~$10,000/year
```

---

## 🛡️ Security & Compliance

### Smart Contract Security

- Multi-signature treasury (3-of-5)
- Time-locked upgrades (48 hours)
- Audits by: Trail of Bits, Quantstamp
- Bug bounty: Up to $1M AINU

### Regulatory Considerations

- **Utility Token**: Not a security
- **No Investment Contract**: No expectation of profit from others
- **Functional Use**: Required for network operation
- **Decentralized**: No central authority

### Legal Structure

```
Foundation: Ainur Foundation (Swiss/Cayman)
  ├─ Treasury Management
  ├─ Grant Distribution
  └─ Protocol Governance Facilitation

DAO: AINU DAO (On-chain)
  ├─ Protocol Parameter Control
  ├─ Treasury Spending Approval
  └─ Upgrade Decisions
```

---

## 🚀 Launch Strategy

### Phase 1: Testnet (Month 1-2)

- Deploy tokenomics on testnet
- Simulate task marketplace
- Stress test economic models
- Community feedback

### Phase 2: Mainnet Launch (Month 3)

- Fair launch (no pre-sale)
- Initial DEX offering (IDO)
- CEX listings (Tier 2)
- Liquidity mining starts

### Phase 3: Growth (Month 4-12)

- Agent onboarding incentives
- Developer grants
- Marketing campaigns
- Partnership announcements

### Phase 4: Maturity (Year 2+)

- CEX listings (Tier 1: Binance, Coinbase)
- Cross-chain bridges
- Institutional adoption
- Enterprise contracts

---

## 📊 Success Metrics

### Key Performance Indicators (KPIs)

1. **Network Usage**:
   - Target: 100,000 tasks/day by Year 2
   - Current: 0 (pre-launch)

2. **Token Holders**:
   - Target: 50,000 holders by Year 1
   - Target: 500,000 holders by Year 3

3. **Total Value Locked (TVL)**:
   - Target: $10M by Month 6
   - Target: $100M by Year 2

4. **Agent Count**:
   - Target: 1,000 active agents by Month 6
   - Target: 10,000 active agents by Year 2

5. **Protocol Revenue**:
   - Target: $1M annual by Year 1
   - Target: $10M annual by Year 3

---

## 🔮 Future Enhancements

### Cross-Chain Expansion

- Bridge to Solana, Polygon, Arbitrum
- Multi-chain liquidity
- Chain-agnostic task execution

### Advanced Features

- Fractional agent ownership (NFTs)
- Agent reputation tokens
- Prediction markets for task completion
- Insurance pools for failed tasks

### Integration Opportunities

- AI model marketplaces
- Data oracle networks
- Decentralized compute platforms
- Web3 automation tools

---

## 📝 Conclusion

The AINU token creates a sustainable, decentralized economy for the Ainur agent marketplace. By aligning incentives across users, agents, and node operators, we enable a self-sustaining ecosystem that rewards value creation while maintaining security and decentralization.

**Key Strengths**:
- ✅ Fixed supply with deflationary burns
- ✅ Clear utility (not speculation)
- ✅ Fair distribution (40% to ecosystem)
- ✅ Strong governance (token-weighted voting)
- ✅ Proven economic models (inspired by Ethereum, Polkadot)

**Next Steps**:
1. Community feedback (2 weeks)
2. Economic modeling & simulation (1 month)
3. Smart contract development (2 months)
4. Security audits (1 month)
5. Testnet launch (Month 5)
6. Mainnet launch (Month 6)

---

**For questions or feedback, contact:**  
- Email: tokenomics@ainur.network  
- Discord: discord.gg/ainur  
- Twitter: @AinurNetwork  

**Document Version History**:
- v1.0 (Nov 12, 2025): Initial design document
