# AINU Smart Contracts - Deployment Guide

## Prerequisites

1. **Install Foundry**:
   ```bash
   curl -L https://foundry.paradigm.xyz | bash
   foundryup
   ```

2. **Set up environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your keys
   ```

3. **Run tests**:
   ```bash
   forge test --gas-report
   ```

## Deployment Options

### Option 1: Local Deployment (Testing)

Start local Anvil node:
```bash
anvil
```

Deploy contracts:
```bash
forge script script/DeployLocal.s.sol:DeployLocalScript \
  --fork-url http://localhost:8545 \
  --broadcast
```

### Option 2: Testnet Deployment (Sepolia)

```bash
forge script script/Deploy.s.sol:DeployScript \
  --rpc-url $SEPOLIA_RPC_URL \
  --broadcast \
  --verify \
  --etherscan-api-key $ETHERSCAN_API_KEY
```

### Option 3: Mainnet Deployment

⚠️ **WARNING**: This deploys to Ethereum mainnet with real funds!

1. **Audit contracts** first (use Trail of Bits, OpenZeppelin, etc.)

2. **Set multisig** in .env:
   ```bash
   MULTISIG_ADDRESS=0x...  # Your Gnosis Safe or multisig
   ```

3. **Deploy**:
   ```bash
   forge script script/Deploy.s.sol:DeployScript \
     --rpc-url $MAINNET_RPC_URL \
     --broadcast \
     --verify \
     --etherscan-api-key $ETHERSCAN_API_KEY \
     --slow  # Add delay between transactions
   ```

4. **Verify deployment**:
   ```bash
   # Update .env with deployed addresses
   forge script script/Verify.s.sol:VerifyScript \
     --rpc-url $MAINNET_RPC_URL
   ```

## Post-Deployment Checklist

### 1. Contract Verification
- [ ] All contracts verified on Etherscan
- [ ] Contract source code matches deployment
- [ ] Constructor arguments correct

### 2. Configuration
- [ ] Staking contract exempt from burns
- [ ] Treasury contract exempt from burns
- [ ] Task execution contract authorized as collector
- [ ] Burn rate set correctly (5%)
- [ ] Proposal threshold correct (100K AINU)

### 3. Security
- [ ] Ownership transferred to multisig
- [ ] No functions callable by EOA
- [ ] Emergency pause tested
- [ ] Time locks functioning
- [ ] Reentrancy guards in place

### 4. Liquidity Setup
- [ ] Create Uniswap V3 pool (AINU/ETH)
- [ ] Add initial liquidity
- [ ] Set LP contract as burn-exempt
- [ ] Configure pool fees (0.3% or 1%)

### 5. Integrations
- [ ] Frontend updated with contract addresses
- [ ] Subgraph deployed for indexing
- [ ] API integrated with treasury
- [ ] Monitoring alerts configured

### 6. Governance
- [ ] Multisig signers verified
- [ ] Threshold set correctly (e.g., 3/5)
- [ ] First proposal created (test)
- [ ] Voting tested end-to-end

## Contract Addresses

After deployment, save addresses to `.env`:

```bash
# Mainnet
TOKEN_ADDRESS=0x...
STAKING_ADDRESS=0x...
GOVERNANCE_ADDRESS=0x...
TREASURY_ADDRESS=0x...

# Testnet (Sepolia)
SEPOLIA_TOKEN_ADDRESS=0x...
SEPOLIA_STAKING_ADDRESS=0x...
SEPOLIA_GOVERNANCE_ADDRESS=0x...
SEPOLIA_TREASURY_ADDRESS=0x...
```

## Integration Testing

Run full integration tests:
```bash
forge test --match-contract IntegrationTest -vvv
```

Specific flows:
```bash
# Test staking to voting
forge test --match-test testFullStakingToVotingFlow -vvv

# Test revenue distribution
forge test --match-test testFullRevenueDistributionFlow -vvv

# Test complete ecosystem
forge test --match-test testCompleteEcosystemFlow -vvv
```

## Interacting with Deployed Contracts

### Using Cast (CLI)

```bash
# Check token balance
cast call $TOKEN_ADDRESS "balanceOf(address)(uint256)" $YOUR_ADDRESS --rpc-url $RPC_URL

# Stake tokens
cast send $TOKEN_ADDRESS "approve(address,uint256)" $STAKING_ADDRESS 100000000000000000000000 --private-key $PRIVATE_KEY --rpc-url $RPC_URL
cast send $STAKING_ADDRESS "stake(uint256,uint256)" 100000000000000000000000 31536000 --private-key $PRIVATE_KEY --rpc-url $RPC_URL

# Create proposal
cast send $GOVERNANCE_ADDRESS "propose(string,uint8)" "My proposal" 2 --private-key $PRIVATE_KEY --rpc-url $RPC_URL

# Vote on proposal
cast send $GOVERNANCE_ADDRESS "castVote(uint256,bool)" 0 true --private-key $PRIVATE_KEY --rpc-url $RPC_URL
```

### Using Frontend

1. Connect wallet (MetaMask/WalletConnect)
2. Navigate to Staking page
3. Approve AINU tokens
4. Select stake amount and duration
5. Confirm transaction

## Gas Optimization

Estimated gas costs (Ethereum mainnet):

| Operation | Gas Cost | At 30 gwei | At 100 gwei |
|-----------|----------|------------|-------------|
| Token Deploy | 1,583,776 | $10.50 | $35.00 |
| Staking Deploy | 1,137,261 | $7.50 | $25.00 |
| Governance Deploy | 1,156,960 | $7.70 | $25.60 |
| Treasury Deploy | 1,159,491 | $7.70 | $25.60 |
| **Total Deployment** | **5,037,488** | **$33.40** | **$111.30** |
| Stake | ~160,000 | $1.06 | $3.54 |
| Vote | ~100,000 | $0.66 | $2.21 |
| Transfer | ~55,000 | $0.36 | $1.22 |

## Monitoring

### Events to Monitor

1. **Token Events**:
   - `Transfer`: Track all token movements
   - `BurnOnTransferToggled`: Monitor burn status changes
   - `TokensBurned`: Track deflationary pressure

2. **Staking Events**:
   - `Staked`: New stakes
   - `Unstaked`: Stake withdrawals
   - `Slashed`: Penalties applied

3. **Governance Events**:
   - `ProposalCreated`: New proposals
   - `VoteCast`: Voting activity
   - `ProposalExecuted`: Governance actions

4. **Treasury Events**:
   - `RevenueCollected`: Task fees
   - `AgentRewardClaimed`: Agent earnings
   - `GrantExecuted`: Grant distributions

### Set up monitoring:

```bash
# Using ethers.js
const token = new ethers.Contract(TOKEN_ADDRESS, ABI, provider);
token.on("Transfer", (from, to, amount) => {
  console.log(`Transfer: ${from} -> ${to}: ${amount}`);
});
```

## Troubleshooting

### Common Issues

1. **"Out of gas" errors**:
   - Increase gas limit: `--gas-limit 3000000`
   - Use `--legacy` for older networks

2. **"Nonce too low"**:
   - Reset nonce: `cast nonce $ADDRESS --rpc-url $RPC_URL`
   - Wait for pending transactions

3. **Verification fails**:
   - Check constructor args match
   - Wait 30s after deployment
   - Try `--num-of-optimizations 200`

4. **"Insufficient funds"**:
   - Check ETH balance for gas
   - Estimate costs first: `forge script --estimate-gas-price`

## Security Best Practices

1. **Never commit private keys**
2. **Use hardware wallets for mainnet**
3. **Test on testnet first**
4. **Audit before mainnet deployment**
5. **Set up monitoring and alerts**
6. **Use multisig for admin functions**
7. **Have emergency pause mechanism**
8. **Document all parameter changes**

## Emergency Procedures

### Pause Protocol
```bash
cast send $TOKEN_ADDRESS "pause()" --private-key $PRIVATE_KEY --rpc-url $RPC_URL
cast send $STAKING_ADDRESS "pause()" --private-key $PRIVATE_KEY --rpc-url $RPC_URL
```

### Emergency Withdraw (Treasury)
```bash
cast send $TREASURY_ADDRESS "emergencyWithdraw(address,uint256)" $TOKEN_ADDRESS $AMOUNT --private-key $PRIVATE_KEY --rpc-url $RPC_URL
```

### Cancel Malicious Proposal
```bash
cast send $GOVERNANCE_ADDRESS "cancel(uint256)" $PROPOSAL_ID --private-key $PRIVATE_KEY --rpc-url $RPC_URL
```

## Support

- Documentation: https://docs.ainur.network
- Discord: https://discord.gg/ainur
- GitHub: https://github.com/aidenlippert/zerostate

## License

MIT License - See LICENSE file for details
