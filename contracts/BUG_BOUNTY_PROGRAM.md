# AINU Bug Bounty Program 🐛💰

**Program Status**: ACTIVE  
**Launch Date**: November 19, 2025  
**Total Bounty Pool**: $100,000 USDC  
**Platform**: Immunefi  
**Duration**: Ongoing (minimum 30 days pre-launch)

---

## 📋 Program Overview

The AINU Protocol is committed to security and welcomes the community to help identify vulnerabilities. This bug bounty program rewards ethical hackers for responsibly disclosing security issues.

### Scope

**In-Scope Contracts:**
1. AINUToken.sol (ERC20 token with burn mechanics)
2. AINUStaking.sol (Staking with reward system)
3. AINUGovernance.sol (DAO governance)
4. AINUTreasury.sol (Revenue distribution)

**Smart Contract Addresses** (Testnet - Sepolia):
- AINUToken: `TBD after deployment`
- AINUStaking: `TBD after deployment`
- AINUGovernance: `TBD after deployment`
- AINUTreasury: `TBD after deployment`

**Repository:**
- GitHub: https://github.com/aidenlippert/zerostate
- Commit Hash: `TBD at launch`

---

## 💰 Reward Tiers

### Critical - $50,000
**Examples:**
- Direct theft of funds from contracts
- Permanent freezing of funds
- Unauthorized minting of tokens
- Protocol insolvency
- Privilege escalation to admin rights
- Bypassing contract pause mechanisms

**Requirements:**
- Detailed proof-of-concept (PoC) exploit
- Step-by-step reproduction
- Suggested fix
- No public disclosure

### High - $25,000
**Examples:**
- Theft of unclaimed rewards
- Manipulation of reward calculations
- Bypassing time locks or vesting
- Flash loan attacks on governance
- DoS affecting core functionality
- Slashing evasion

**Requirements:**
- Working PoC with test case
- Clear impact description
- Reproduction steps
- Suggested mitigation

### Medium - $10,000
**Examples:**
- Griefing attacks (DoS on specific users)
- Incorrect reward calculations causing loss
- State inconsistencies
- Front-running vulnerabilities
- Gas griefing attacks
- Failed edge case handling

**Requirements:**
- Detailed description
- PoC or test case
- Impact assessment
- Suggested fix

### Low - $2,500
**Examples:**
- Minor calculation errors
- Non-critical state inconsistencies
- Informational security issues
- Best practice violations
- Code quality issues affecting security

**Requirements:**
- Clear description
- Impact explanation
- Suggested improvement

### Informational - $500
**Examples:**
- Style guide violations with security implications
- Missing event emissions
- Unclear error messages
- Documentation issues

**Requirements:**
- Description of issue
- Why it could become a security concern

---

## 🎯 High-Priority Vulnerabilities

We're especially interested in vulnerabilities related to:

### 1. Reward System (NEW - Critical!)
- **Reward calculation manipulation**
- **Double-claiming of rewards**
- **Timestamp attacks on reward accrual**
- **Multiplier exploitation**
- **Auto-compound exploits**
- **Tier upgrade manipulation**

### 2. Staking Mechanics
- **Lock time bypass**
- **Slashing evasion**
- **Staking ratio manipulation**
- **Reentrancy in stake/unstake**

### 3. Governance
- **Vote manipulation**
- **Proposal execution bypass**
- **Timelock exploitation**
- **Quorum gaming**
- **Flash loan attacks on voting power**

### 4. Treasury & Revenue
- **Fee distribution errors**
- **Grant vesting bypass**
- **Revenue siphoning**
- **Burn mechanism exploitation**

### 5. Token Economics
- **Burn exemption manipulation**
- **Transfer burn bypass**
- **Supply tracking errors**
- **Circulating supply manipulation**

---

## 📜 Program Rules

### ✅ Eligible Submissions

1. **Responsible Disclosure**
   - Report privately first
   - Give team 90 days to fix before public disclosure
   - No exploitation beyond PoC

2. **Original Findings**
   - First to report gets the bounty
   - No duplicate submissions
   - Must be previously unknown

3. **Severity Assessment**
   - Based on potential loss
   - Likelihood of exploitation
   - Ease of discovery

4. **Quality Requirements**
   - Clear explanation
   - Reproducible PoC
   - Suggested fix (for higher tiers)

### ❌ Out of Scope

1. **Already Known Issues**
   - Issues in public GitHub issues
   - Previously reported vulnerabilities
   - Documented limitations

2. **Social Engineering**
   - Phishing attacks
   - Physical attacks
   - Social manipulation

3. **Network Layer**
   - DDoS attacks
   - DNS attacks
   - Network-level exploits

4. **Third-Party Code**
   - OpenZeppelin libraries (unless our integration is flawed)
   - Foundry framework
   - Solidity compiler bugs

5. **Front-End Issues**
   - UI bugs (unless they lead to funds loss)
   - Display errors
   - Wallet integration issues

6. **Theoretical Issues**
   - No PoC
   - No clear impact
   - Requires impractical conditions

7. **Gas Optimization**
   - Pure gas optimization without security impact
   - Code style issues
   - Minor inefficiencies

---

## 📝 Submission Process

### 1. Prepare Your Report

Include the following:

**Basic Information:**
- Your name/handle
- Contact information (email, Telegram, Discord)
- Severity assessment (Critical/High/Medium/Low)

**Vulnerability Details:**
- Contract and function affected
- Root cause analysis
- Attack vector description
- Prerequisites for exploitation

**Proof of Concept:**
- Foundry test case demonstrating the issue
- Step-by-step reproduction guide
- Expected vs actual behavior
- Actual loss or impact calculation

**Suggested Fix:**
- Code changes required
- Alternative approaches
- Trade-offs to consider

**Example PoC Structure:**
```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUStaking.sol";
import "../src/AINUToken.sol";

contract ExploitTest is Test {
    AINUToken token;
    AINUStaking staking;
    address attacker = address(0x1337);

    function setUp() public {
        token = new AINUToken(address(this));
        staking = new AINUStaking(address(token), address(this));
        
        // Setup scenario
        token.transfer(attacker, 1000000e18);
    }

    function testExploit() public {
        vm.startPrank(attacker);
        
        // 1. Initial state
        // 2. Exploit steps
        // 3. Demonstrate impact
        
        vm.stopPrank();
        
        // Assert exploit succeeded
        // Show funds stolen, etc.
    }
}
```

### 2. Submit via Immunefi

**Primary Channel:**
- Platform: Immunefi.com
- Program: AINU Protocol
- Link: `https://immunefi.com/bounty/ainuprotocol/` (TBD)

**Alternative Channels:**
- Email: security@ainuprotocol.io (encrypted with PGP key)
- Telegram: @AINUSecurity (for urgent issues)
- Discord: AINU Security Channel

### 3. Review Process

**Timeline:**
- Initial response: Within 24 hours
- Severity confirmation: Within 3 days
- Fix development: 7-14 days (depending on severity)
- Bounty payout: Within 7 days of fix deployment

**Communication:**
- We'll keep you updated throughout
- You can ask questions anytime
- We may request additional information

### 4. Bounty Payout

**Payment Method:**
- USDC (default)
- AINU tokens (optional, at market rate)
- ETH (if preferred)

**Payment Process:**
- Provide wallet address (ERC20 compatible)
- We'll send payment within 7 days
- Transaction will be publicly verifiable

---

## 🏆 Hall of Fame

Researchers who responsibly disclose vulnerabilities will be listed here (with permission):

| Researcher | Date | Severity | Bounty | Description |
|------------|------|----------|--------|-------------|
| TBD | - | - | - | - |

---

## 🚫 Known Issues & Limitations

These are documented limitations and will NOT receive bounties:

### 1. Burn Exemption Requirement
**Description**: Users and staking contract must be manually exempt from burns to stake  
**Impact**: If not configured, staking would burn 5% of deposits  
**Mitigation**: Documented in deployment guide, checked in tests  
**Status**: DOCUMENTED

### 2. Integer Division Rounding
**Description**: Reward calculations use integer division which causes minor rounding  
**Impact**: Users may receive 1-2 wei less in rewards (negligible)  
**Mitigation**: Multiply before divide, use basis points for precision  
**Status**: ACCEPTABLE

### 3. No Upgradability
**Description**: Contracts are not upgradeable  
**Impact**: Bugs require new deployment and migration  
**Mitigation**: Thorough testing, bug bounty, conservative launch  
**Status**: BY DESIGN

### 4. Governance Delay
**Description**: Proposals have 24-hour timelock minimum  
**Impact**: Cannot respond immediately to emergencies  
**Mitigation**: Emergency pause function, short timelock for urgent proposals  
**Status**: BY DESIGN

### 5. Timestamp Dependence
**Description**: Reward calculations use `block.timestamp`  
**Impact**: Miners can manipulate timestamps by ~15 seconds  
**Mitigation**: Minimal impact on APR calculations, acceptable risk  
**Status**: ACCEPTABLE

---

## 📊 Program Statistics

### Target Metrics
- Submissions reviewed: Target 100+
- Unique vulnerabilities found: Target 5-10
- Critical issues: Target 0 (we hope!)
- Average response time: <24 hours
- Researcher satisfaction: >90%

### Current Stats (Updated Weekly)
- **Submissions Received**: 0
- **Bounties Paid**: $0
- **Average Bounty**: N/A
- **Researchers**: 0
- **Vulnerabilities Fixed**: 0

---

## 🔐 Security Best Practices

### For Researchers

1. **Test on Testnet First**
   - Use Sepolia testnet
   - Don't attack mainnet
   - Request test tokens if needed

2. **Responsible Disclosure**
   - Report privately
   - Don't publicize before fix
   - Allow reasonable time for remediation

3. **Professional Communication**
   - Be respectful
   - Provide technical details
   - Suggest fixes constructively

4. **Follow Guidelines**
   - Stay in scope
   - Provide PoC
   - Don't perform unauthorized testing

### For Protocol

1. **Fast Response**
   - Acknowledge within 24h
   - Triage severity quickly
   - Keep researcher informed

2. **Fair Assessment**
   - Use consistent criteria
   - Explain bounty decisions
   - Consider researcher effort

3. **Timely Fixes**
   - Critical: <7 days
   - High: <14 days
   - Medium: <30 days
   - Low: <60 days

4. **Transparent Process**
   - Publish post-mortems
   - Credit researchers (with permission)
   - Update known issues list

---

## 📞 Contact Information

### Security Team
- **Email**: security@ainuprotocol.io
- **PGP Key**: [Link to public key]
- **Telegram**: @AINUSecurity
- **Discord**: AINU Official Server - #security

### Immunefi Program
- **Platform**: Immunefi.com
- **Program Page**: https://immunefi.com/bounty/ainuprotocol/
- **Status**: ACTIVE

### Emergency Contact
For critical vulnerabilities requiring immediate attention:
- **Emergency Email**: emergency@ainuprotocol.io
- **Response Time**: <1 hour (24/7 monitoring)

---

## 📚 Resources for Researchers

### Documentation
- **Smart Contract Source**: https://github.com/aidenlippert/zerostate/contracts
- **Technical Documentation**: [Link to docs]
- **Architecture Overview**: PRODUCTION_ARCHITECTURE.md
- **Deployment Guide**: DEPLOYMENT_GUIDE.md
- **Economic Model**: REVALIDATION_COMPLETE.md

### Testing Environment
- **Testnet**: Sepolia
- **Faucets**: 
  - https://sepoliafaucet.com
  - https://www.alchemy.com/faucets/ethereum-sepolia
- **Test AINU Tokens**: Request via security@ainuprotocol.io
- **Foundry Setup**: See README.md

### Common Attack Vectors
- **Reentrancy**: https://github.com/pcaversaccio/reentrancy-attacks
- **Flash Loans**: https://github.com/Arachnid/uscc/tree/master/submissions-2021/ricmoo
- **Governance Attacks**: https://blog.openzeppelin.com/the-dangers-of-price-oracles
- **Math Errors**: https://github.com/crytic/not-so-smart-contracts

---

## 🎓 Researcher Tips

### Finding High-Value Bugs

1. **Focus on State Changes**
   - Look for functions that modify critical state
   - Check ordering of operations
   - Verify proper checks before state updates

2. **Test Edge Cases**
   - Zero values
   - Maximum values
   - Boundary conditions
   - Unexpected orderings

3. **Check Access Control**
   - Who can call this function?
   - Are there bypasses?
   - Can roles be escalated?

4. **Analyze Token Flows**
   - Where do tokens come from?
   - Where do they go?
   - Can they get stuck?
   - Can they be double-spent?

5. **Consider Interactions**
   - How do contracts interact?
   - What about external calls?
   - Can order be manipulated?

### Writing Good Reports

1. **Be Specific**
   - Exact line numbers
   - Specific function names
   - Concrete impact values

2. **Provide Context**
   - Why is this a vulnerability?
   - What's the worst-case scenario?
   - Who is affected?

3. **Include PoC**
   - Working test case
   - Clear reproduction steps
   - Actual exploit code

4. **Suggest Fixes**
   - Code changes
   - Alternative approaches
   - Trade-offs

---

## 🏁 Program Launch Checklist

### Pre-Launch (Week 5)
- [x] Create bug bounty documentation
- [ ] Set up Immunefi program
- [ ] Fund bounty wallet with $100K USDC
- [ ] Deploy contracts to Sepolia testnet
- [ ] Test submission process
- [ ] Set up security email and PGP
- [ ] Create monitoring dashboard

### Launch Day (November 19, 2025)
- [ ] Activate Immunefi program
- [ ] Announce on Twitter, Discord, Telegram
- [ ] Post on security forums (Immunefi, HackerOne)
- [ ] Share on Reddit (r/ethdev, r/ethereum)
- [ ] Email security researchers
- [ ] Update website with bug bounty info

### Ongoing
- [ ] Monitor submissions daily
- [ ] Respond within 24 hours
- [ ] Triage and assess severity
- [ ] Coordinate fixes with dev team
- [ ] Pay bounties promptly
- [ ] Update stats weekly
- [ ] Publish post-mortems

---

## 📈 Success Metrics

### Program Goals
1. **Coverage**: 100% of in-scope contracts reviewed by community
2. **Quality**: Average severity of submissions High or above
3. **Speed**: <24h response time maintained
4. **Reputation**: >4.5/5 stars on Immunefi
5. **Security**: Zero critical issues on mainnet

### KPIs
- **Submissions/Week**: Target 5-10
- **Valid Submissions**: Target 30%+
- **Response Time**: Target <12 hours
- **Fix Time (Critical)**: Target <7 days
- **Bounty Payout Time**: Target <7 days
- **Researcher Satisfaction**: Target >90%

---

## 🎉 Special Incentives

### Early Bird Bonus
**First 30 days**: +25% bonus on all bounties  
**First critical find**: +$10,000 bonus  
**First valid submission**: +$1,000 bonus

### Continuous Participation
**3+ valid submissions**: +10% on all future bounties  
**5+ valid submissions**: +20% on all future bounties  
**Hall of Fame**: Permanent recognition + AINU tokens

### Referral Program
**Refer a researcher**: 10% of their first bounty  
**Referred finds critical**: +$5,000 bonus

---

**Last Updated**: November 12, 2025  
**Program Version**: 1.0  
**Next Review**: December 1, 2025

**Questions?** Contact security@ainuprotocol.io

