# AINU Smart Contracts

Solidity smart contracts for the AINU token and Ainur decentralized agent marketplace.

## Overview

This directory contains the core smart contracts for Phase 3 (Economy) of the Ainur project:

1. **AINUToken.sol** - ERC-20 token with burn mechanics
2. **AINUStaking.sol** - Tiered staking for agents
3. **AINUGovernance.sol** - On-chain governance system
4. **AINUTreasury.sol** - Protocol revenue management

## Architecture

```
┌─────────────────┐
│   AINUToken     │  ERC-20 with 5% burn on transfer
└────────┬────────┘
         │
         ├──────────────────┐
         │                  │
┌────────▼────────┐  ┌─────▼──────────┐
│  AINUStaking    │  │  AINUTreasury  │
│  - Tier system  │  │  - Fee split   │
│  - Time weight  │  │  - Rewards     │
│  - Slashing     │  │  - Burns       │
└────────┬────────┘  └────────────────┘
         │
┌────────▼────────┐
│ AINUGovernance  │
│  - Proposals    │
│  - Voting       │
│  - Timelock     │
└─────────────────┘
```

## Contracts

### AINUToken

**Features**:
- Fixed supply: 10,000,000,000 AINU
- 18 decimals
- 5% burn on transfers (togglable)
- Burn exemptions for DEX pools
- EIP-2612 Permit support
- Pausable

**Key Functions**:
```solidity
function toggleBurnOnTransfer(bool enabled)
function setExemptFromBurn(address account, bool exempt)
function circulatingSupply() returns (uint256)
function totalBurned() returns (uint256)
```

### AINUStaking

**Features**:
- 3 tiers: Basic (1K), Standard (10K), Premium (100K)
- Time-weighted voting power (up to 4x for 4-year stakes)
- Slashing for misbehavior
- Lockup periods

**Key Functions**:
```solidity
function stake(uint256 amount, uint256 lockDuration)
function addToStake(uint256 amount)
function unstake()
function slash(address staker, uint256 percentage, string reason)
function getVotingPower(address staker) returns (uint256)
```

**Slashing Rules**:
- Downtime >10%: 5% slash
- Failed task: 1% slash
- Malicious behavior: 100% slash

### AINUGovernance

**Features**:
- 3 proposal types (Protocol, Treasury, Parameter)
- Quorum requirements (60%/40%/20%)
- Approval thresholds (75%/66%/50%)
- 7-day voting period
- 48-hour timelock

**Key Functions**:
```solidity
function propose(string description, ProposalType type) returns (uint256)
function castVote(uint256 proposalId, bool support)
function queue(uint256 proposalId)
function execute(uint256 proposalId)
function getProposalState(uint256 proposalId) returns (ProposalState)
```

**Proposal Flow**:
1. Create proposal (requires 100K AINU staked)
2. 7-day voting period
3. Check quorum & approval
4. Queue for execution
5. Wait 48-hour timelock
6. Execute

### AINUTreasury

**Features**:
- Revenue collection from task fees
- Automatic split: 70% agent, 20% node, 5% protocol, 5% burn
- Grant management
- Buyback and burn

**Key Functions**:
```solidity
function collectTaskFee(uint256 amount, address agent, address node)
function claimAgentRewards()
function claimNodeRewards()
function createGrant(address recipient, uint256 amount, string description)
function buybackAndBurn(uint256 amount)
```

## Development

### Prerequisites

```bash
# Install Foundry
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Install dependencies
forge install OpenZeppelin/openzeppelin-contracts
```

### Build

```bash
forge build
```

### Test

```bash
# Run all tests
forge test

# Run with gas reporting
forge test --gas-report

# Run specific test
forge test --match-test testStaking

# Run with verbosity
forge test -vvv
```

### Coverage

```bash
forge coverage
```

### Deploy

```bash
# Deploy to Sepolia testnet
forge script script/Deploy.s.sol --rpc-url $SEPOLIA_RPC_URL --broadcast --verify

# Deploy to mainnet
forge script script/Deploy.s.sol --rpc-url $MAINNET_RPC_URL --broadcast --verify
```

## Token Distribution

Total Supply: **10,000,000,000 AINU**

```
40% - Ecosystem & Rewards     (4,000,000,000)
20% - Team & Advisors          (2,000,000,000)
15% - Foundation & Treasury    (1,500,000,000)
10% - Public Sale              (1,000,000,000)
10% - Strategic Investors      (1,000,000,000)
5%  - Initial Liquidity          (500,000,000)
```

## Economics

### Task Fee Example

User submits task costing **1 AINU**:
- 0.70 AINU → Agent (executor)
- 0.20 AINU → Node Operator
- 0.05 AINU → Protocol Treasury
- 0.05 AINU → Burned

### Staking Tiers

| Tier | Stake | Revenue Share | Benefits |
|------|-------|---------------|----------|
| Basic | 1K AINU | 70% | Simple tasks |
| Standard | 10K AINU | 75% | Priority queue, complex tasks |
| Premium | 100K AINU | 80% | Enterprise tasks, guaranteed allocation |

### Voting Power

Formula: `votingPower = stakedAmount × timeWeight`

Time weights:
- No lock: 1x
- 1 year: 2x
- 2 years: 3x
- 4 years: 4x

Example:
- 10K AINU staked for 4 years = 40K voting power
- 10K AINU unstaked = 10K voting power

## Security

### Audits

- [ ] Internal review (Week 5)
- [ ] Trail of Bits audit (Week 6)
- [ ] Quantstamp audit (Week 6)

### Bug Bounty

Maximum payout: **1,000,000 AINU** (~$100K at $0.10/AINU)

Critical vulnerabilities:
- Token minting bugs
- Governance takeover
- Treasury drain

### Best Practices

✅ ReentrancyGuard on all external calls
✅ SafeERC20 for token transfers
✅ Checks-Effects-Interactions pattern
✅ Pausable for emergency stops
✅ Timelocks on critical operations
✅ Access control (Ownable)

## Testing Strategy

1. **Unit Tests**: Test each function in isolation
2. **Integration Tests**: Test contract interactions
3. **Fuzz Tests**: Random input testing
4. **Invariant Tests**: Property-based testing
5. **Gas Optimization**: Profile and optimize

Target coverage: **95%+**

## Deployment Checklist

### Testnet (Sepolia)

- [ ] Deploy AINUToken
- [ ] Deploy AINUStaking
- [ ] Deploy AINUGovernance
- [ ] Deploy AINUTreasury
- [ ] Configure contracts
- [ ] Verify on Etherscan
- [ ] Test all functions
- [ ] Run E2E scenarios

### Mainnet

- [ ] Final security audit
- [ ] Multi-sig setup
- [ ] Deploy contracts
- [ ] Verify on Etherscan
- [ ] Transfer ownership to multi-sig
- [ ] Announce deployment
- [ ] Add liquidity to DEX
- [ ] List on CEX

## License

MIT

## Contact

- Email: dev@ainur.network
- Discord: discord.gg/ainur
- Twitter: @AinurNetwork

## References

- [EIP-20: Token Standard](https://eips.ethereum.org/EIPS/eip-20)
- [EIP-2612: Permit](https://eips.ethereum.org/EIPS/eip-2612)
- [OpenZeppelin Contracts](https://docs.openzeppelin.com/contracts/)
- [Foundry Book](https://book.getfoundry.sh/)
