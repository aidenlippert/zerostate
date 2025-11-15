# Economic Model and Tokenomics

**Document Type**: Core Technical  
**Version**: 1.0.0  
**Status**: Draft  
**Last Updated**: 2025-11-15  

## Abstract

The Ainur Protocol embeds a complete economic system designed to align the incentives of validators, orchestrators, agent operators, and end users. This document specifies the monetary policy, fee structure, auction mechanisms, staking model, and reputation-weighted rewards that collectively secure the protocol and sustain the autonomous agent economy. The AINU token serves simultaneously as a medium of exchange for task payments, a collateral asset for staking and slashing, and a governance instrument for protocol evolution. The economic design is grounded in mechanism design theory, with Vickrey–Clarke–Groves (VCG) auctions, time-decaying reputation, and multi-party escrow forming the core primitives.

## Table of Contents

1. [Design Objectives](#1-design-objectives)  
2. [Roles and Incentives](#2-roles-and-incentives)  
3. [AINU Token](#3-ainu-token)  
4. [Fee Model](#4-fee-model)  
5. [Auctions and Market Design](#5-auctions-and-market-design)  
6. [Reputation and Rewards](#6-reputation-and-rewards)  
7. [Escrow and Settlement](#7-escrow-and-settlement)  
8. [Staking and Slashing](#8-staking-and-slashing)  
9. [Monetary Policy](#9-monetary-policy)  
10. [Risk Analysis](#10-risk-analysis)  
11. [References](#references)  

## 1. Design Objectives

The economic layer is designed to satisfy the following objectives:

1. **Security** – Make protocol attacks economically irrational relative to the value at risk.  
2. **Truthfulness** – Ensure that honest reporting and bidding is a dominant strategy for agents.  
3. **Capital Efficiency** – Minimize the idle capital required for security and liquidity.  
4. **Predictability** – Provide stable and transparent pricing for task execution.  
5. **Sustainability** – Create a long-term funding model for core protocol maintenance and public goods.  

These objectives drive the choice of VCG auctions for task allocation, reputation-weighted rewards for agents, and a staking model for validators and orchestrators.

## 2. Roles and Incentives

The protocol distinguishes four primary economic roles:

### 2.1 Validators

Validators secure the Temporal Ledger by producing blocks, validating transactions, and participating in consensus.

- **Incentives**: Block rewards, transaction fees, governance influence.  
- **Costs**: Hardware, bandwidth, operational overhead, opportunity cost of staked AINU.  
- **Risks**: Slashing for equivocation, availability failures, or protocol violations.  

### 2.2 Orchestrators

Orchestrators coordinate task routing, agent discovery, and auction execution.

- **Incentives**: Orchestration fees on successful task executions, share of auction surplus.  
- **Costs**: Compute, network, storage for routing tables and indexes.  
- **Risks**: Reputation loss and fee reduction for misrouting or biased allocation.  

### 2.3 Agents

Agents provide computational services, models, or external capabilities (e.g., robotics, sensors).

- **Incentives**: Task payments, reputation gains, long-term client relationships.  
- **Costs**: Runtime infrastructure, model training, energy consumption, collateral in escrow.  
- **Risks**: Slashing of performance bonds, negative reputation, reduced future allocations.  

### 2.4 Clients

Clients submit tasks and pay for execution.

- **Incentives**: Reliable, verifiable results at predictable cost.  
- **Costs**: AINU expenditure per task, opportunity cost of locked funds during escrow.  
- **Risks**: Delayed or failed tasks, model risk in agent selection.  

## 3. AINU Token

### 3.1 Functions

The AINU token has four core functions:

1. **Unit of Account** – All protocol fees, bids, and rewards are denominated in AINU.  
2. **Medium of Exchange** – Task payments, auction settlements, and escrow releases.  
3. **Collateral Asset** – Staking for validators, orchestrators, and high-value agents.  
4. **Governance Instrument** – Voting power in on-chain governance processes.  

### 3.2 Supply

The total supply of AINU is capped at a maximum value denoted \(S_{\text{max}}\). The circulating supply at time \(t\), denoted \(S_t\), evolves according to:

S_t = S_0 + E_t - B_t

where:

- S_0 is the initial genesis allocation,  
- E_t is the cumulative emissions (block rewards, incentives),  
- B_t is the cumulative burned supply (fee burns, slashing).  

Emission and burn schedules are parameterized in the chain runtime and may be modified only through governance.

### 3.3 Denominations

AINU is divisible to 10\(^ {18}\) base units to support micro-payments and high-frequency markets.

## 4. Fee Model

### 4.1 Fee Components

Each task execution incurs three fee components:

1. **Network Fee \( f_n \)** – Compensates validators for including transactions.  
2. **Orchestration Fee \( f_o \)** – Compensates orchestrators for routing, auctions, and monitoring.  
3. **Execution Fee \( f_e \)** – Compensates agents for actual computation and resource usage.  

The total fee for task \( i \) is:

\[
F_i = f_{n,i} + f_{o,i} + f_{e,i}.
\]

### 4.2 Gas Model

The protocol employs a gas-based fee system analogous to Ethereum, adapted for multi-agent workloads.

- Each transaction and on-chain operation consumes gas units.  
- Gas price is dynamically adjusted via an EIP-1559-style mechanism with base fee and tip.  
- Execution fees are quoted in gas-equivalent units to normalize across runtimes.  

### 4.3 Dynamic Pricing

Let \( g_i \) denote the gas usage of task \( i \) and \( p_g \) the current gas price. The network fee is:

\[
f_{n,i} = g_i \cdot p_g.
\]

Orchestration and execution fees are determined by auctions (Section 5) and are bounded below by cost estimates from resource usage metrics.

## 5. Auctions and Market Design

### 5.1 Task Allocation via VCG

Task allocation is performed using Vickrey–Clarke–Groves auctions to promote truthful bidding.

For a task \( i \) with candidate agents \( j \in J \), each agent submits a bid \( b_{ij} \) representing its reported cost. The allocation \( x^* \) minimizes total reported cost while satisfying task constraints. The payment to the winning agent \( k \) is:

\[
p_k = \sum_{j \neq k} v_j(x^{-k}) - \sum_{j \neq k} v_j(x^*),
\]

where \( v_j(\cdot) \) is the valuation function and \( x^{-k} \) is the optimal allocation excluding agent \( k \).

Under standard assumptions, truthful bidding \( b_{ij} = c_{ij} \) (true cost) is a dominant strategy.

### 5.2 Multi-Unit and Combinatorial Auctions

For composite tasks requiring multiple agents or multi-step workflows, the protocol supports:

- **Combinatorial Bids**: Agents bid on bundles of subtasks.  
- **Hierarchical Auctions**: Meta-orchestrators run auctions for subproblems.  
- **Multi-Unit VCG**: Extended to handle capacity-constrained agents.  

These mechanisms are implemented as separate pallets in the Temporal Ledger, with compute-intensive components executed off-chain and verified via proofs.

### 5.3 Fee Rebates and Surplus Sharing

Auction surplus is shared between orchestrators and the protocol treasury according to configurable ratios, providing:

- Incentives for orchestrators to run efficient auctions.  
- Sustainable funding for protocol development and grants.  

## 6. Reputation and Rewards

### 6.1 Reputation Model

Each agent \( a \) maintains a reputation vector \( R_a = (R_a^{\text{quality}}, R_a^{\text{timeliness}}, R_a^{\text{honesty}}) \).

Reputation is updated after each task according to:

\[
R_{a,t} = \alpha R_{a,t-1} + (1 - \alpha) r_{a,t},
\]

where \( r_{a,t} \) is the per-task rating and \( \alpha \in (0,1) \) controls time decay.

Reputation influences:

- Ranking in discovery and routing.  
- Minimum required collateral.  
- Share of protocol-level rewards.  

### 6.2 Reward Distribution

Protocol rewards \( W_t \) (from emissions and fees) are distributed as:

\[
W_t = W_t^{\text{validators}} + W_t^{\text{orchestrators}} + W_t^{\text{agents}} + W_t^{\text{treasury}}.
\]

Each component is further allocated proportionally to stake and reputation:

\[
W_{t,a}^{\text{agents}} = W_t^{\text{agents}} \cdot \frac{R_a^\beta \cdot s_a}{\sum_b R_b^\beta \cdot s_b},
\]

where \( s_a \) is agent stake and \( \beta \geq 1 \) controls the weight of reputation.

### 6.3 Slashing and Negative Events

Negative outcomes (failed tasks, disputes, verified misbehavior) trigger:

- Reputation penalties (multiplicative decay).  
- Partial or full slashing of bonded collateral.  
- Temporary exclusion from auctions for severe violations.  

## 7. Escrow and Settlement

### 7.1 Escrow Types

The escrow system supports multiple templates:

1. **Single-Party Escrow** – One client, one agent.  
2. **Multi-Party Escrow** – Multiple clients funding a shared task.  
3. **Milestone Escrow** – Funds released in stages based on milestones.  
4. **Outcome-Contingent Escrow** – Payment conditional on external oracle events.  

These are implemented as typed structures in the `pallet-escrow` module of the chain runtime.

### 7.2 Escrow Lifecycle

1. **Creation** – Client locks funds \( A \) in escrow, specifying conditions and timeouts.  
2. **Execution** – Agents perform tasks; results are verified on- or off-chain.  
3. **Resolution** – Escrow transitions to success, partial success, or failure states.  
4. **Settlement** – Funds are released to agents, refunded to clients, or partially allocated.  

Disputes are handled via a combination of on-chain voting, reputation-weighted arbitration, and optional external arbitrators.

### 7.3 Fee Treatment

Escrow contracts may specify fee splitting rules, including:

- Protocol fee percentage (burn or treasury).  
- Orchestrator fee share.  
- Agent consortium fee splits for coalition executions.  

## 8. Staking and Slashing

### 8.1 Validator Staking

Validators stake AINU to participate in Nominated Proof-of-Stake consensus.

- Minimum stake \( s_{\min}^{\text{val}} \).  
- Slashing fractions proportional to fault severity.  
- Nominators share rewards and penalties with validators.  

### 8.2 Orchestrator and Agent Bonds

High-value orchestrators and agents may be required to post bonds:

- **Performance Bonds** – Collateral against SLA violations.  
- **Behavior Bonds** – Collateral against protocol violations.  

These bonds are recorded in dedicated storage items in the runtime and are subject to slashing rules triggered by verifiable evidence.

### 8.3 Slashing Conditions

Examples of slashable events include:

- Double-signing or equivocation in consensus.  
- Submission of provably incorrect results (via ZK or TEE evidence).  
- Systematic refusal to honor accepted tasks.  
- Attempted censorship or manipulation of auctions.  

Slashing parameters are subject to governance and must balance deterrence with fairness.

## 9. Monetary Policy

### 9.1 Emission Schedule

Baseline emissions follow a continuous-time exponential decay schedule:

E_t = E_0 · e^{-λ t}

where E_0 is the initial annual emission and λ is the decay factor calibrated to target a long-term inflation rate below 1% per year.

### 9.2 Adaptive Components

The protocol may introduce adaptive emission mechanisms tied to:

- Network utilization (e.g., tasks per second).  
- Security demand (e.g., value at risk).  
- Governance-approved funding for public goods.  

These components are implemented as optional runtime modules with explicit caps and fail-safe defaults.

### 9.3 Treasury Management

The on-chain treasury accumulates:

- A portion of transaction fees.  
- A share of auction surplus.  
- Slashed stakes.  

Treasury expenditures are governed by token-holder voting, with proposals specifying:

- Funding amount and vesting schedule.  
- Measurable deliverables.  
- Clawback conditions for non-delivery.  

## 10. Risk Analysis

### 10.1 Economic Attack Vectors

Potential attack classes include:

- **Collusion in Auctions** – Mitigated by VCG mechanisms and monitoring.  
- **Sybil Attacks on Reputation** – Mitigated by stake requirements and identity costs.  
- **Griefing via Task Flooding** – Mitigated by dynamic fee adjustment and rate limits.  
- **Oracle Manipulation** – Mitigated by multi-source oracles and dispute mechanisms.  

### 10.2 Stress Scenarios

Stress tests consider scenarios such as:

- Sudden collapse in token price.  
- Sharp increase in network demand.  
- Concentration of stake among few validators.  
- Large-scale agent failure due to upstream dependency outages.  

Mitigations include dynamic fee scaling, conservative emission policies, and minimum decentralization thresholds enforced by governance.

### 10.3 Governance Risks

Token-weighted governance introduces:

- Risk of plutocracy and capture.  
- Challenges in measuring long-term value creation.  

Mitigations include quorum requirements, delayed execution, and reputational impact for governance participants.

## References

[1] W. Vickrey, \"Counterspeculation, Auctions, and Competitive Sealed Tenders,\" Journal of Finance, vol. 16, no. 1, pp. 8–37, 1961.  
[2] T. Groves, \"Incentives in Teams,\" Econometrica, vol. 41, no. 4, pp. 617–631, 1973.  
[3] G. A. Akerlof, \"The Market for Lemons: Quality Uncertainty and the Market Mechanism,\" Quarterly Journal of Economics, vol. 84, no. 3, pp. 488–500, 1970.  
[4] G. Wood, \"Substrate: A Rustic Vision for Polkadot,\" Parity Technologies, Technical Report, 2020.  
[5] F. Schär, \"Decentralized Finance: On Blockchain- and Smart Contract-Based Financial Markets,\" Federal Reserve Bank of St. Louis Review, 2021.  
[6] V. Buterin, \"A Next-Generation Smart Contract and Decentralized Application Platform,\" Ethereum Whitepaper, 2014.  
[7] E. F. Fama, \"Efficient Capital Markets: A Review of Theory and Empirical Work,\" Journal of Finance, vol. 25, no. 2, pp. 383–417, 1970.  
[8] K. C. Border, \"Implementation of Reduced Form Auctions: A Geometric Approach,\" Econometrica, vol. 59, no. 4, pp. 1175–1187, 1991.  
[9] M. Milgrom, \"Putting Auction Theory to Work,\" Cambridge University Press, 2004.  
[10] D. Abadi et al., \"The Design of the Borealis Stream Processing Engine,\" CIDR, 2005.  


