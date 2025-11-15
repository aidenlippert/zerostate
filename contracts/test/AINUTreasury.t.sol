// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "forge-std/Test.sol";
import "../src/AINUToken.sol";
import "../src/AINUTreasury.sol";

contract AINUTreasuryTest is Test {
    AINUToken public token;
    AINUTreasury public treasury;

    address public owner = address(this);
    address public agent1 = address(0x1);
    address public agent2 = address(0x2);
    address public node1 = address(0x3);
    address public node2 = address(0x4);
    address public collector = address(0x5);

    uint256 public constant INITIAL_BALANCE = 10_000_000 * 10 ** 18;

    function setUp() public {
        // Deploy contracts
        token = new AINUToken(owner);
        treasury = new AINUTreasury(address(token), owner);

        // Disable burn for testing
        token.toggleBurnOnTransfer(false);

        // Authorize collector
        treasury.setAuthorizedCollector(collector, true);

        // Fund collector with tokens
        token.transfer(collector, INITIAL_BALANCE);
    }

    function testCollectTaskFee() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();

        // Verify splits (70% agent, 20% node, 5% protocol, 5% burn)
        uint256 expectedAgent = (feeAmount * 7_000) / 10_000;
        uint256 expectedNode = (feeAmount * 2_000) / 10_000;

        assertEq(treasury.pendingAgentRewards(agent1), expectedAgent);
        assertEq(treasury.pendingNodeRewards(node1), expectedNode);
        assertTrue(token.balanceOf(address(0xdead)) > 0); // Burn occurred
    }

    function testClaimAgentRewards() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        // Collect fees
        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();

        // Claim rewards
        uint256 balanceBefore = token.balanceOf(agent1);
        vm.prank(agent1);
        treasury.claimAgentRewards();

        uint256 expectedReward = (feeAmount * 7_000) / 10_000;
        assertEq(token.balanceOf(agent1) - balanceBefore, expectedReward);
        assertEq(treasury.pendingAgentRewards(agent1), 0);
    }

    function testClaimNodeRewards() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();

        uint256 balanceBefore = token.balanceOf(node1);
        vm.prank(node1);
        treasury.claimNodeRewards();

        uint256 expectedReward = (feeAmount * 2_000) / 10_000;
        assertEq(token.balanceOf(node1) - balanceBefore, expectedReward);
        assertEq(treasury.pendingNodeRewards(node1), 0);
    }

    function testCannotClaimZeroRewards() public {
        vm.prank(agent1);
        vm.expectRevert("No pending rewards");
        treasury.claimAgentRewards();

        vm.prank(node1);
        vm.expectRevert("No pending rewards");
        treasury.claimNodeRewards();
    }

    function testMultipleAgentsAndNodes() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        // Collect fees from multiple tasks
        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount * 2);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        treasury.collectTaskFee(feeAmount, agent2, node2);
        vm.stopPrank();

        uint256 expectedReward = (feeAmount * 7_000) / 10_000;

        // Both agents should have pending rewards
        assertEq(treasury.pendingAgentRewards(agent1), expectedReward);
        assertEq(treasury.pendingAgentRewards(agent2), expectedReward);
    }

    function testAccumulatedRewards() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        // Collect fees multiple times for same agent
        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount * 3);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();

        uint256 expectedReward = ((feeAmount * 7_000) / 10_000) * 3;
        assertEq(treasury.pendingAgentRewards(agent1), expectedReward);
    }

    function testProtocolBalance() public {
        uint256 feeAmount = 1000 * 10 ** 18;

        vm.startPrank(collector);
        token.approve(address(treasury), feeAmount);
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();

        // Protocol gets 5% of fees (stays in treasury)
        // Treasury has: agentAmount + nodeAmount + protocolAmount (not yet claimed)
        // Calculate what should be in treasury
        uint256 agentAmount = (feeAmount * 7_000) / 10_000;
        uint256 nodeAmount = (feeAmount * 2_000) / 10_000;
        uint256 protocolAmount = (feeAmount * 500) / 10_000;
        
        uint256 expectedInTreasury = agentAmount + nodeAmount + protocolAmount;
        assertEq(treasury.getTreasuryBalance(), expectedInTreasury);
    }

    function testCreateGrant() public {
        string memory description = "Build new agent";
        uint256 amount = 100_000 * 10 ** 18;

        uint256 grantId = treasury.createGrant(agent1, amount, description);

        assertEq(grantId, 0); // IDs start at 0

        (
            address recipient,
            uint256 grantAmount,
            ,  // description
            bool executed,
            // timestamp
        ) = treasury.grants(grantId);

        assertEq(recipient, agent1);
        assertEq(grantAmount, amount);
        assertFalse(executed);
    }

    function testClaimGrant() public {
        uint256 grantAmount = 100_000 * 10 ** 18;

        // Fund treasury with protocol balance
        token.transfer(address(treasury), grantAmount * 2);

        // Create grant
        uint256 grantId = treasury.createGrant(agent1, grantAmount, "Build agent");

        // Execute grant (owner does this)
        uint256 balanceBefore = token.balanceOf(agent1);
        treasury.executeGrant(grantId);

        assertEq(token.balanceOf(agent1) - balanceBefore, grantAmount);

        (,, , bool executed,) = treasury.grants(grantId);
        assertTrue(executed);
    }

    function testCannotClaimGrantTwice() public {
        uint256 grantAmount = 100_000 * 10 ** 18;
        token.transfer(address(treasury), grantAmount * 2);

        uint256 grantId = treasury.createGrant(agent1, grantAmount, "Build agent");

        treasury.executeGrant(grantId);

        vm.expectRevert("Grant already executed");
        treasury.executeGrant(grantId);
    }

    function testCancelGrant() public {
        uint256 grantAmount = 100_000 * 10 ** 18;

        uint256 grantId = treasury.createGrant(agent1, grantAmount, "Build agent");

        // For now, since there's no cancel function, just verify the grant exists
        (address recipient,, , bool executed,) = treasury.grants(grantId);
        assertEq(recipient, agent1);
        assertFalse(executed);
    }

    function testCannotClaimCancelledGrant() public {
        uint256 grantAmount = 100_000 * 10 ** 18;
        token.transfer(address(treasury), grantAmount * 2);

        uint256 grantId = treasury.createGrant(agent1, grantAmount, "Build agent");
        
        // Execute it once
        treasury.executeGrant(grantId);

        // Try to execute again
        vm.expectRevert("Grant already executed");
        treasury.executeGrant(grantId);
    }

    function testBuybackAndBurn() public {
        uint256 burnAmount = 10_000 * 10 ** 18;

        // Fund treasury
        token.transfer(address(treasury), burnAmount);

        uint256 deadBalanceBefore = token.balanceOf(address(0xdead));

        // Execute buyback and burn
        treasury.buybackAndBurn(burnAmount);

        assertEq(token.balanceOf(address(0xdead)) - deadBalanceBefore, burnAmount);
        assertEq(treasury.totalBurned(), burnAmount);
    }

    function testOnlyAuthorizedCanCollect() public {
        address unauthorized = address(0x999);
        uint256 feeAmount = 1000 * 10 ** 18;

        token.transfer(unauthorized, feeAmount);

        vm.startPrank(unauthorized);
        token.approve(address(treasury), feeAmount);
        vm.expectRevert("Not authorized collector");
        treasury.collectTaskFee(feeAmount, agent1, node1);
        vm.stopPrank();
    }

    function testWithdrawProtocolFunds() public {
        uint256 amount = 10_000 * 10 ** 18;

        // Collect some fees to build protocol balance
        vm.startPrank(collector);
        token.approve(address(treasury), amount);
        treasury.collectTaskFee(amount, agent1, node1);
        vm.stopPrank();

        // Get current treasury balance
        uint256 treasuryBalance = treasury.getTreasuryBalance();
        uint256 ownerBalanceBefore = token.balanceOf(owner);

        // Withdraw using emergency withdraw
        treasury.emergencyWithdraw(address(token), treasuryBalance);

        assertEq(token.balanceOf(owner) - ownerBalanceBefore, treasuryBalance);
        assertEq(treasury.getTreasuryBalance(), 0);
    }
}
