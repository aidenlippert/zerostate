// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";
import "../src/AINUStaking.sol";
import "../src/AINUGovernance.sol";
import "../src/AINUTreasury.sol";

/**
 * @title Integration Tests
 * @notice Tests cross-contract interactions and full workflows
 */
contract IntegrationTest is Test {
    AINUToken public token;
    AINUStaking public staking;
    AINUGovernance public governance;
    AINUTreasury public treasury;

    address public owner = address(this);
    address public agent = address(0x1);
    address public node = address(0x2);
    address public user1 = address(0x3);
    address public user2 = address(0x4);

    uint256 public constant INITIAL_BALANCE = 10_000_000 * 10 ** 18;

    function setUp() public {
        // Deploy all contracts
        token = new AINUToken(owner);
        staking = new AINUStaking(address(token), owner);
        governance = new AINUGovernance(address(staking), owner);
        treasury = new AINUTreasury(address(token), owner);

        // Configure
        token.setExemptFromBurn(address(staking), true);
        token.setExemptFromBurn(address(treasury), true);
        token.toggleBurnOnTransfer(false); // Disable for testing

        treasury.setAuthorizedCollector(owner, true);

        // Distribute tokens
        token.transfer(agent, INITIAL_BALANCE);
        token.transfer(node, INITIAL_BALANCE);
        token.transfer(user1, INITIAL_BALANCE);
        token.transfer(user2, INITIAL_BALANCE);
    }

    function testFullStakingToVotingFlow() public {
        uint256 stakeAmount = 200_000 * 10 ** 18;

        // User1 stakes tokens
        vm.startPrank(user1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        vm.stopPrank();

        // User2 stakes tokens
        vm.startPrank(user2);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 730 days);
        vm.stopPrank();

        // Verify stakes
        assertEq(staking.totalStaked(), stakeAmount * 2);

        // User1 creates a proposal
        vm.prank(user1);
        uint256 proposalId = governance.propose(
            "Increase agent rewards to 75%",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // Both users vote FOR (so proposal succeeds)
        vm.prank(user1);
        governance.castVote(proposalId, true);

        vm.prank(user2);
        governance.castVote(proposalId, true); // Changed to true

        // Check proposal state
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.ACTIVE));

        // Fast forward past voting period
        vm.warp(block.timestamp + 7 days + 1);

        // Proposal should succeed (both voted FOR, quorum reached)
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.SUCCEEDED));
    }

    function testFullRevenueDistributionFlow() public {
        uint256 feeAmount = 10_000 * 10 ** 18;

        // Collect task fees
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent, node);

        // Verify splits
        uint256 expectedAgent = (feeAmount * 7_000) / 10_000; // 70%
        uint256 expectedNode = (feeAmount * 2_000) / 10_000; // 20%

        assertEq(treasury.pendingAgentRewards(agent), expectedAgent);
        assertEq(treasury.pendingNodeRewards(node), expectedNode);

        // Agent claims rewards
        uint256 agentBalanceBefore = token.balanceOf(agent);
        vm.prank(agent);
        treasury.claimAgentRewards();
        assertEq(token.balanceOf(agent) - agentBalanceBefore, expectedAgent);

        // Node claims rewards
        uint256 nodeBalanceBefore = token.balanceOf(node);
        vm.prank(node);
        treasury.claimNodeRewards();
        assertEq(token.balanceOf(node) - nodeBalanceBefore, expectedNode);

        // Verify pending rewards cleared
        assertEq(treasury.pendingAgentRewards(agent), 0);
        assertEq(treasury.pendingNodeRewards(node), 0);
    }

    function testStakeVoteAndEarnFlow() public {
        uint256 stakeAmount = 100_000 * 10 ** 18;
        uint256 feeAmount = 5_000 * 10 ** 18;

        // Agent stakes tokens
        vm.startPrank(agent);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 0);
        vm.stopPrank();

        // Agent earns from task execution
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent, node);

        // Agent claims rewards
        vm.prank(agent);
        treasury.claimAgentRewards();

        // Agent adds rewards to stake
        uint256 earned = (feeAmount * 7_000) / 10_000;
        vm.startPrank(agent);
        token.approve(address(staking), earned);
        staking.addToStake(earned);
        vm.stopPrank();

        // Verify total stake
        AINUStaking.Stake memory stake = staking.getStake(agent);
        assertEq(stake.amount, stakeAmount + earned);
    }

    function testGrantProposalAndExecution() public {
        uint256 stakeAmount = 200_000 * 10 ** 18;
        uint256 grantAmount = 50_000 * 10 ** 18;

        // User1 stakes to gain voting power
        vm.startPrank(user1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        vm.stopPrank();

        // Create grant proposal
        uint256 grantId = treasury.createGrant(agent, grantAmount, "Build new math agent");

        // Fund treasury
        token.transfer(address(treasury), grantAmount * 2);

        // Execute grant (in real scenario, would go through governance vote)
        treasury.executeGrant(grantId);

        // Verify grant executed
        (,, , bool executed,) = treasury.grants(grantId);
        assertTrue(executed);
    }

    function testSlashingAndGovernance() public {
        uint256 stakeAmount = 100_000 * 10 ** 18;

        // Agent stakes
        vm.startPrank(agent);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        vm.stopPrank();

        // Authorize owner to slash
        staking.setAuthorizedSlasher(owner, true);

        // Slash for downtime (5%)
        staking.slash(agent, 500, "90% downtime");

        // Verify slash
        AINUStaking.Stake memory stake = staking.getStake(agent);
        assertTrue(stake.isSlashed);
        assertEq(stake.slashedAmount, (stakeAmount * 500) / 10_000);
    }

    function testMultipleAgentsCompetingForRewards() public {
        address agent1 = address(0x11);
        address agent2 = address(0x22);
        address agent3 = address(0x33);

        uint256 taskFee = 1_000 * 10 ** 18;

        // Distribute tokens
        token.transfer(agent1, taskFee);
        token.transfer(agent2, taskFee);
        token.transfer(agent3, taskFee);

        // Three agents complete tasks
        token.approve(address(treasury), taskFee * 3);
        treasury.collectTaskFee(taskFee, agent1, node);
        treasury.collectTaskFee(taskFee, agent2, node);
        treasury.collectTaskFee(taskFee, agent3, node);

        // Verify all have pending rewards
        uint256 expectedReward = (taskFee * 7_000) / 10_000;
        assertEq(treasury.pendingAgentRewards(agent1), expectedReward);
        assertEq(treasury.pendingAgentRewards(agent2), expectedReward);
        assertEq(treasury.pendingAgentRewards(agent3), expectedReward);

        // Node gets 3x rewards
        uint256 expectedNodeReward = ((taskFee * 2_000) / 10_000) * 3;
        assertEq(treasury.pendingNodeRewards(node), expectedNodeReward);
    }

    function testBurnOnTransferWithTreasury() public {
        // Enable burns
        token.toggleBurnOnTransfer(true);

        // User1 transfers to user2 (should burn 5%)
        uint256 amount = 1_000 * 10 ** 18;
        uint256 burnAmount = (amount * 500) / 10_000;

        // Remove owner exemption for this test
        token.setExemptFromBurn(owner, false);

        vm.prank(user1);
        token.transfer(user2, amount);

        // Verify burn occurred
        assertEq(token.balanceOf(address(0xdead)), burnAmount);

        // Staking transfers should NOT burn (exempt)
        vm.startPrank(user1);
        token.approve(address(staking), amount);
        staking.stake(amount, 0);
        vm.stopPrank();

        // No additional burn should occur
        assertEq(token.balanceOf(address(0xdead)), burnAmount);
    }

    function testCompleteEcosystemFlow() public {
        // 1. Deploy and configure (already done in setUp)

        // 2. Multiple users stake
        uint256 stakeAmount = 100_000 * 10 ** 18;
        
        vm.startPrank(user1);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 365 days);
        vm.stopPrank();

        vm.startPrank(user2);
        token.approve(address(staking), stakeAmount);
        staking.stake(stakeAmount, 730 days);
        vm.stopPrank();

        // 3. Agents execute tasks and earn
        uint256 taskFee = 5_000 * 10 ** 18;
        token.approve(address(treasury), taskFee * 3);
        treasury.collectTaskFee(taskFee, agent, node);
        treasury.collectTaskFee(taskFee, agent, node);
        treasury.collectTaskFee(taskFee, agent, node);

        // 4. Create governance proposal
        vm.prank(user1);
        uint256 proposalId = governance.propose(
            "Update protocol parameters",
            AINUGovernance.ProposalType.PARAMETER_CHANGE
        );

        // 5. Vote on proposal
        vm.prank(user1);
        governance.castVote(proposalId, true);

        vm.prank(user2);
        governance.castVote(proposalId, true);

        // 6. Execute after voting period
        vm.warp(block.timestamp + 7 days + 1);
        governance.queue(proposalId);
        vm.warp(block.timestamp + 48 hours + 1);
        governance.execute(proposalId);

        // 7. Verify final state
        assertEq(staking.totalStaked(), stakeAmount * 2);
        assertTrue(treasury.totalRevenue() > 0);
        assertEq(uint8(governance.getProposalState(proposalId)), uint8(AINUGovernance.ProposalState.EXECUTED));
    }
}
